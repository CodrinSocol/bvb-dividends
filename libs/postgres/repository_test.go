package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/postgres"
)

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func mustAmount(t *testing.T, s string) domain.Amount {
	t.Helper()
	a, err := domain.ParseAmount(s)
	if err != nil {
		t.Fatalf("ParseAmount(%q): %v", s, err)
	}
	return a
}

// newDividend builds a dividend with its derived identifier already set, the
// way the importer does.
func newDividend(company domain.Symbol, year int, kind string, exDate domain.Date) domain.Dividend {
	d := domain.Dividend{
		Company:  company,
		Year:     year,
		Type:     kind,
		Schedule: domain.Schedule{ExDividendDate: exDate},
	}
	d.ID = domain.NewID(d.NaturalKey())
	return d
}

// Re-importing the same dividends must update the rows in place rather than
// replace them. The Java service deleted every dividend of a company and
// re-inserted them on each daily run, which changed every identifier daily and
// reset every creation timestamp.
func TestUpsertDividendIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	companies := postgres.NewCompanyRepository(pool)
	dividends := postgres.NewDividendRepository(pool)

	if err := companies.Upsert(ctx, domain.Company{Symbol: "SNP", Name: "OMV PETROM S.A."}); err != nil {
		t.Fatalf("upsert company: %v", err)
	}

	first := newDividend("SNP", 2025, "cash", mustDate(t, "2026-04-17"))
	first.Amounts.GrossPerShareNaturalPerson = mustAmount(t, "0.0345")

	if _, err := dividends.Upsert(ctx, first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	stored, err := dividends.Get(ctx, first.ID)
	if err != nil {
		t.Fatalf("get after first upsert: %v", err)
	}

	// The next day's import reports the same dividend with a date now filled in.
	second := first
	second.Schedule.RecordDate = mustDate(t, "2026-04-18")
	if _, err := dividends.Upsert(ctx, second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	after, err := dividends.Get(ctx, first.ID)
	if err != nil {
		t.Fatalf("get after second upsert: %v", err)
	}

	if after.ID != stored.ID {
		t.Errorf("identifier changed across imports: %s then %s", stored.ID, after.ID)
	}
	if !after.CreatedAt.Equal(stored.CreatedAt) {
		t.Errorf("creation time was reset: %s then %s", stored.CreatedAt, after.CreatedAt)
	}
	if !after.UpdatedAt.After(stored.UpdatedAt) {
		t.Errorf("update time did not advance after a real change: %s then %s", stored.UpdatedAt, after.UpdatedAt)
	}
	if !after.Schedule.RecordDate.Equal(second.Schedule.RecordDate) {
		t.Errorf("record date = %v, want %v", after.Schedule.RecordDate, second.Schedule.RecordDate)
	}

	page, err := dividends.List(ctx, domain.DividendQuery{Page: domain.PageRequest{Size: 10}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("re-importing created a second row: total = %d, want 1", page.Total)
	}
}

// Re-importing an unchanged dividend must not touch the update time, or every
// consumer would see every dividend change every day.
func TestUpsertDividendSkipsUnchangedRows(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	companies := postgres.NewCompanyRepository(pool)
	dividends := postgres.NewDividendRepository(pool)

	if err := companies.Upsert(ctx, domain.Company{Symbol: "TLV", Name: "BANCA TRANSILVANIA S.A."}); err != nil {
		t.Fatalf("upsert company: %v", err)
	}

	dividend := newDividend("TLV", 2025, "cash", mustDate(t, "2026-05-02"))
	if _, err := dividends.Upsert(ctx, dividend); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	before, err := dividends.Get(ctx, dividend.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	changed, err := dividends.Upsert(ctx, dividend)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if changed != 0 {
		t.Errorf("re-importing an unchanged dividend wrote %d rows, want 0", changed)
	}

	after, err := dividends.Get(ctx, dividend.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("update time moved without a change: %s then %s", before.UpdatedAt, after.UpdatedAt)
	}
}

// A dividend BVB has announced but not yet scheduled has no ex-dividend date.
// The Java service's date-range queries used ExDividendDateAfter/Before, which
// excluded those rows from every result. They must be reachable here.
func TestListReachesDividendsWithNoExDate(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	companies := postgres.NewCompanyRepository(pool)
	dividends := postgres.NewDividendRepository(pool)

	if err := companies.Upsert(ctx, domain.Company{Symbol: "SNG", Name: "S.N.G.N. ROMGAZ S.A."}); err != nil {
		t.Fatalf("upsert company: %v", err)
	}

	scheduled := newDividend("SNG", 2025, "cash", mustDate(t, "2026-06-10"))
	undated := newDividend("SNG", 2026, "cash", domain.NoDate)
	if _, err := dividends.Upsert(ctx, scheduled, undated); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	all, err := dividends.List(ctx, domain.DividendQuery{Page: domain.PageRequest{Size: 10}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if all.Total != 2 {
		t.Fatalf("total = %d, want 2", all.Total)
	}

	onlyUndated, err := dividends.List(ctx, domain.DividendQuery{
		Where: domain.Compare{Field: domain.FieldExDividendDate, Op: domain.OpEqual, Value: domain.NullValue{}},
		Page:  domain.PageRequest{Size: 10},
	})
	if err != nil {
		t.Fatalf("list undated: %v", err)
	}
	if onlyUndated.Total != 1 {
		t.Fatalf("undated total = %d, want 1", onlyUndated.Total)
	}
	if got := onlyUndated.Items[0].ID; got != undated.ID {
		t.Errorf("undated filter returned %s, want %s", got, undated.ID)
	}

	// A range filter still excludes them, which is the correct SQL semantic:
	// an unknown date is not in any range.
	inRange, err := dividends.List(ctx, domain.DividendQuery{
		Where: domain.Compare{
			Field: domain.FieldExDividendDate,
			Op:    domain.OpGreater,
			Value: domain.DateValue(mustDate(t, "2020-01-01")),
		},
		Page: domain.PageRequest{Size: 10},
	})
	if err != nil {
		t.Fatalf("list in range: %v", err)
	}
	if inRange.Total != 1 {
		t.Errorf("range total = %d, want 1", inRange.Total)
	}
}

// Pagination must partition the result set exactly: no row seen twice, none
// missed. That needs a total-ordering tiebreak, since many dividends share an
// ex-dividend date.
func TestListPaginationIsStable(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	companies := postgres.NewCompanyRepository(pool)
	dividends := postgres.NewDividendRepository(pool)

	if err := companies.Upsert(ctx, domain.Company{Symbol: "FP", Name: "FONDUL PROPRIETATEA"}); err != nil {
		t.Fatalf("upsert company: %v", err)
	}

	const total = 25
	batch := make([]domain.Dividend, 0, total)
	sameDate := mustDate(t, "2026-07-01")
	for i := range total {
		batch = append(batch, newDividend("FP", 2000+i, "cash", sameDate))
	}
	if _, err := dividends.Upsert(ctx, batch...); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	seen := make(map[domain.ID]int)
	const pageSize = 7
	for offset := int64(0); offset < total; offset += pageSize {
		page, err := dividends.List(ctx, domain.DividendQuery{
			Page: domain.PageRequest{Size: pageSize, Offset: offset},
		})
		if err != nil {
			t.Fatalf("list at offset %d: %v", offset, err)
		}
		if page.Total != total {
			t.Errorf("total at offset %d = %d, want %d", offset, page.Total, total)
		}
		for _, item := range page.Items {
			seen[item.ID]++
		}
	}

	if len(seen) != total {
		t.Errorf("paging covered %d distinct dividends, want %d", len(seen), total)
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("dividend %s appeared %d times across pages", id, count)
		}
	}
}

func TestGetMissingDividendIsNotFound(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	dividends := postgres.NewDividendRepository(pool)

	missing := domain.NewID(domain.NaturalKey{Company: "NONE", Year: 1999, Type: "cash"})
	_, err := dividends.Get(ctx, missing)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("Get(missing) error = %v, want ErrNotFound", err)
	}
}

// A company that changes its legal name must pick the new one up. The Java
// repository inserted only when absent, so a renamed company kept its old name
// forever.
func TestUpsertCompanyRefreshesName(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	companies := postgres.NewCompanyRepository(pool)

	if err := companies.Upsert(ctx, domain.Company{Symbol: "M", Name: "MEDLIFE S.A."}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	before, err := companies.Get(ctx, "M")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if err := companies.Upsert(ctx, domain.Company{Symbol: "M", Name: "MED LIFE S.A."}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	after, err := companies.Get(ctx, "M")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if after.Name != "MED LIFE S.A." {
		t.Errorf("name = %q, want the refreshed one", after.Name)
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Error("creation time was reset by an update")
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Error("update time did not advance after a rename")
	}
}

// Money must survive the round trip through numeric(20,4) exactly. The Java
// DTO used Double, which rounded these values.
func TestAmountsRoundTripExactly(t *testing.T) {
	ctx := context.Background()
	pool := newTestPool(t)
	companies := postgres.NewCompanyRepository(pool)
	dividends := postgres.NewDividendRepository(pool)

	if err := companies.Upsert(ctx, domain.Company{Symbol: "BRD", Name: "BRD - GROUPE SOCIETE GENERALE"}); err != nil {
		t.Fatalf("upsert company: %v", err)
	}

	dividend := newDividend("BRD", 2025, "cash", mustDate(t, "2026-04-30"))
	dividend.Amounts = domain.Amounts{
		GrossPerShareNaturalPerson: mustAmount(t, "0.1234"),
		GrossPerShareLegalPerson:   mustAmount(t, "0.1234"),
		Total:                      mustAmount(t, "1234567890123.4567"),
	}
	if _, err := dividends.Upsert(ctx, dividend); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	stored, err := dividends.Get(ctx, dividend.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got, want := stored.Amounts.GrossPerShareNaturalPerson.String(), "0.1234"; got != want {
		t.Errorf("per-share amount = %q, want %q", got, want)
	}
	if got, want := stored.Amounts.Total.String(), "1234567890123.4567"; got != want {
		t.Errorf("total = %q, want %q", got, want)
	}
}
