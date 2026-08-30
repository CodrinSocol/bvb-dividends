package postgres_test

import (
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/aip"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres/pgtest"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	companiespg "github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/repo/postgres"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	dividendspg "github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/infra/repo/postgres"
	bvbdividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/go/gen/v1"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// newRepository returns a dividends repository over empty tables, with the two
// companies the cases reference already stored: a dividend has a foreign key
// into them.
func newRepository(t *testing.T) *dividendspg.DividendPostgresRepository {
	t.Helper()

	db := pgtest.NewDB(t, companiespg.NewNamespace(), dividendspg.NewNamespace())
	pgtest.Truncate(t, db, "company.companies", "dividend.dividends")

	companyRepo := companiespg.NewCompanyPostgresRepository(quietLogger(), db)
	if _, err := companyRepo.Upsert(t.Context(),
		&companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."},
		&companies.Company{Symbol: "TLV", DisplayName: "BANCA TRANSILVANIA S.A."},
	); err != nil {
		t.Fatalf("store the companies a dividend references: %v", err)
	}

	return dividendspg.NewDividendPostgresRepository(quietLogger(), db)
}

func date(t *testing.T, s string) common.Date {
	t.Helper()

	if s == "" {
		return common.NoDate
	}

	d, err := common.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}

	return d
}

func newDividend(t *testing.T, symbol common.Symbol, year int, kind string, schedule dividends.Schedule) *dividends.Dividend {
	t.Helper()

	dividend := &dividends.Dividend{
		Company:  symbol,
		Year:     year,
		Type:     kind,
		Schedule: schedule,
	}
	dividend.ID = dividends.NewID(dividend.NaturalKey())

	return dividend
}

func symbolPtr(s common.Symbol) *common.Symbol { return &s }

func TestUpsertAndGet(t *testing.T) {
	repo := newRepository(t)

	total, err := common.ParseAmount("2085123456.7890")
	if err != nil {
		t.Fatalf("ParseAmount: %v", err)
	}

	dividend := newDividend(t, "SNP", 2024, "cash", dividends.Schedule{
		AnnouncementDate: date(t, "2025-03-01"),
		ExDividendDate:   date(t, "2025-06-10"),
		PaymentStartDate: date(t, "2025-06-25"),
		PaymentEndDate:   date(t, "2025-12-31"),
	})
	dividend.Amounts.Total = total
	dividend.DistributionMethod = "Bank transfer"

	written, err := repo.Upsert(t.Context(), dividend)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if written != 1 {
		t.Errorf("wrote %d rows, want 1", written)
	}

	stored, err := repo.Get(t.Context(), symbolPtr("SNP"), dividend.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Money must survive the round trip exactly; the Java DTO used Double.
	if !stored.Amounts.Total.Equal(total) {
		t.Errorf("total = %q, want %q exactly", stored.Amounts.Total, total)
	}
	// An unreported amount must come back absent rather than as zero.
	if stored.Amounts.GrossPerShareLegalPerson.Valid() {
		t.Error("an unreported amount came back as a value")
	}
	if stored.Schedule.ExDividendDate.String() != "2025-06-10" {
		t.Errorf("ex-dividend date = %q", stored.Schedule.ExDividendDate)
	}
	if stored.Schedule.RecordDate.Valid() {
		t.Error("an unreported date came back as a value")
	}
	if stored.DistributionMethod != "Bank transfer" {
		t.Errorf("distribution method = %q", stored.DistributionMethod)
	}
}

// A resource name pairing a real dividend with the wrong company must not
// resolve, and must not reveal that the dividend exists elsewhere.
func TestGetUnderTheWrongCompany(t *testing.T) {
	repo := newRepository(t)

	dividend := newDividend(t, "SNP", 2024, "cash", dividends.Schedule{ExDividendDate: date(t, "2025-06-10")})
	if _, err := repo.Upsert(t.Context(), dividend); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	_, err := repo.Get(t.Context(), symbolPtr("TLV"), dividend.ID)
	if err == nil {
		t.Fatal("a dividend resolved under the wrong company")
	}
	if !isNotFound(err) {
		t.Errorf("error = %v, want a not-found error", err)
	}
}

// Re-importing the same dividend must update the row it wrote the first time,
// so its creation time and its resource name survive. The Java service deleted
// and re-inserted with fresh identifiers on every daily run, which broke every
// saved link within a day.
func TestReimportKeepsTheSameRow(t *testing.T) {
	repo := newRepository(t)

	schedule := dividends.Schedule{ExDividendDate: date(t, "2025-06-10")}
	first := newDividend(t, "SNP", 2024, "cash", schedule)

	if _, err := repo.Upsert(t.Context(), first); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	before, err := repo.Get(t.Context(), symbolPtr("SNP"), first.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// The same dividend as re-reported the next day, with a figure now filled
	// in and a record date BVB has since published.
	second := newDividend(t, "SNP", 2024, "cash", dividends.Schedule{
		ExDividendDate: date(t, "2025-06-10"),
		RecordDate:     date(t, "2025-06-11"),
	})
	second.Amounts.Total, _ = common.ParseAmount("1234.5678")

	if second.ID != first.ID {
		t.Fatalf("the identifier changed across imports: %s then %s", first.ID, second.ID)
	}

	written, err := repo.Upsert(t.Context(), second)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if written != 1 {
		t.Errorf("a changed dividend wrote %d rows, want 1", written)
	}

	after, err := repo.Get(t.Context(), symbolPtr("SNP"), first.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !after.CreateTime.Equal(before.CreateTime) {
		t.Error("the creation time moved on a re-import")
	}
	if !after.UpdateTime.After(before.UpdateTime) {
		t.Error("the update time did not move although the dividend changed")
	}
	if after.Schedule.RecordDate.String() != "2025-06-11" {
		t.Errorf("record date = %q, want the newly reported one", after.Schedule.RecordDate)
	}
}

func TestReimportOfAnUnchangedDividendWritesNothing(t *testing.T) {
	repo := newRepository(t)

	dividend := newDividend(t, "SNP", 2024, "cash", dividends.Schedule{ExDividendDate: date(t, "2025-06-10")})
	if _, err := repo.Upsert(t.Context(), dividend); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	written, err := repo.Upsert(t.Context(), dividend)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if written != 0 {
		t.Errorf("an unchanged dividend wrote %d rows, want 0", written)
	}
}

// The listing is per company, and the AIP-159 wildcard is every company.
func TestListByParent(t *testing.T) {
	repo := newRepository(t)

	if _, err := repo.Upsert(t.Context(),
		newDividend(t, "SNP", 2024, "cash", dividends.Schedule{ExDividendDate: date(t, "2025-06-10")}),
		newDividend(t, "SNP", 2025, "cash", dividends.Schedule{ExDividendDate: date(t, "2026-06-10")}),
		newDividend(t, "TLV", 2025, "cash", dividends.Schedule{ExDividendDate: date(t, "2026-05-10")}),
	); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	oneCompany, err := repo.List(t.Context(), symbolPtr("SNP"), common.ListQuery{PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(oneCompany.Items) != 2 {
		t.Errorf("listing one company returned %d dividends, want 2", len(oneCompany.Items))
	}

	everyCompany, err := repo.List(t.Context(), nil, common.ListQuery{PageSize: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(everyCompany.Items) != 3 {
		t.Errorf("the wildcard returned %d dividends, want 3", len(everyCompany.Items))
	}
	if everyCompany.TotalSize == nil || *everyCompany.TotalSize != 3 {
		t.Errorf("total size = %v, want 3", everyCompany.TotalSize)
	}
}

// The state is derived twice — as a SQL expression for filtering, and in Go for
// the response — so the two must agree for every stage. This is the test that
// makes writing the rule twice safe.
func TestTheDerivedStateAgreesWithTheEntity(t *testing.T) {
	repo := newRepository(t)

	now := time.Now()
	today := common.DateOf(now)
	day := func(offset int) common.Date {
		return common.DateOf(today.Time().AddDate(0, 0, offset))
	}

	stored := []*dividends.Dividend{
		// No ex-dividend date at all.
		newDividend(t, "SNP", 2020, "undated", dividends.Schedule{RecordDate: day(3)}),
		// Ex-dividend date still in the future.
		newDividend(t, "SNP", 2021, "announced", dividends.Schedule{ExDividendDate: day(7)}),
		// Ex-dividend date today: already passed, because it is the first day
		// the share trades without entitlement.
		newDividend(t, "SNP", 2022, "ex-today", dividends.Schedule{ExDividendDate: day(0)}),
		// Passed, payment not yet started.
		newDividend(t, "SNP", 2023, "awaiting", dividends.Schedule{
			ExDividendDate:   day(-10),
			PaymentStartDate: day(5),
		}),
		// Payment window open.
		newDividend(t, "SNP", 2024, "paying", dividends.Schedule{
			ExDividendDate:   day(-30),
			PaymentStartDate: day(-5),
			PaymentEndDate:   day(30),
		}),
		// Payment window closed.
		newDividend(t, "SNP", 2025, "paid", dividends.Schedule{
			ExDividendDate:   day(-90),
			PaymentStartDate: day(-60),
			PaymentEndDate:   day(-30),
		}),
	}
	if _, err := repo.Upsert(t.Context(), stored...); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	result, err := repo.List(t.Context(), symbolPtr("SNP"), common.ListQuery{PageSize: 100})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Items) != len(stored) {
		t.Fatalf("listed %d dividends, want %d", len(result.Items), len(stored))
	}

	for _, dividend := range result.Items {
		if want := dividend.StateAt(now); dividend.State != want {
			t.Errorf("%s: the database derived %q, the entity derives %q",
				dividend.Type, dividend.State, want)
		}
	}
}

// Filtering on the state is why it is derived in SQL at all: "dividends you can
// still earn by buying the share today" is a question the Java service could
// not express, because its date-range queries excluded everything undated.
func TestListAppliesAFilter(t *testing.T) {
	repo := newRepository(t)

	today := common.DateOf(time.Now())
	day := func(offset int) common.Date {
		return common.DateOf(today.Time().AddDate(0, 0, offset))
	}

	if _, err := repo.Upsert(t.Context(),
		newDividend(t, "SNP", 2024, "cash", dividends.Schedule{ExDividendDate: day(-30)}),
		newDividend(t, "SNP", 2025, "cash", dividends.Schedule{ExDividendDate: day(30)}),
		newDividend(t, "SNP", 2026, "stock", dividends.Schedule{}),
	); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cases := map[string]struct {
		filter string
		want   []int
	}{
		"by year":               {`year = 2024`, []int{2024}},
		"by type":               {`dividend_type = "stock"`, []int{2026}},
		"still claimable":       {`state = "STATE_ANNOUNCED"`, []int{2025}},
		"announced but undated": {`schedule.ex_dividend_date = null`, []int{2026}},
		"scheduled at all":      {`schedule.ex_dividend_date != null`, []int{2024, 2025}},
		"a date range":          {`schedule.ex_dividend_date > "` + day(-1).String() + `"`, []int{2025}},
		"a conjunction":         {`year >= 2025 AND dividend_type = "cash"`, []int{2025}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			filter, err := aip.CompileFilter(&bvbdividendsv1.Dividend{}, tc.filter)
			if err != nil {
				t.Fatalf("CompileFilter(%q): %v", tc.filter, err)
			}

			result, err := repo.List(t.Context(), symbolPtr("SNP"), common.ListQuery{
				PageSize: 100,
				Filter:   filter,
				OrderBy:  common.OrderBy{{Path: "year", Direction: common.OrderDirectionAsc}},
			})
			if err != nil {
				t.Fatalf("List(%q): %v", tc.filter, err)
			}

			got := make([]int, len(result.Items))
			for i, dividend := range result.Items {
				got[i] = dividend.Year
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("%s: got years %v, want %v", tc.filter, got, tc.want)
			}
		})
	}
}

// Following the page tokens must visit every dividend exactly once, in the
// order asked for.
func TestListPaginatesInOrder(t *testing.T) {
	repo := newRepository(t)

	stored := make([]*dividends.Dividend, 0, 5)
	for year := 2020; year < 2025; year++ {
		stored = append(stored, newDividend(t, "SNP", year, "cash", dividends.Schedule{
			ExDividendDate: date(t, "2025-06-10"),
		}))
	}
	if _, err := repo.Upsert(t.Context(), stored...); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	var seen []int

	query := common.ListQuery{
		PageSize: 2,
		OrderBy:  common.OrderBy{{Path: "year", Direction: common.OrderDirectionDesc}},
	}

	for range len(stored) {
		result, err := repo.List(t.Context(), symbolPtr("SNP"), query)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, dividend := range result.Items {
			seen = append(seen, dividend.Year)
		}
		if result.NextPageToken == "" {
			break
		}
		query.PageToken = result.NextPageToken
	}

	if want := []int{2024, 2023, 2022, 2021, 2020}; !slices.Equal(seen, want) {
		t.Errorf("paging visited %v, want %v", seen, want)
	}
}

func isNotFound(err error) bool {
	var domainErr *common.Error

	return errors.As(err, &domainErr) && domainErr.Code() == common.ErrorCodeNotFound
}
