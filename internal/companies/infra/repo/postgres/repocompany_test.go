package postgres_test

import (
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/aip"
	commonpg "github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres/pgtest"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	companiespg "github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/repo/postgres"
	bvbdividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/go/gen/v1"
)

// newRepository returns a repository over an empty companies table, or skips
// when no test database is configured.
func newRepository(t *testing.T) (*companiespg.CompanyPostgresRepository, *commonpg.DB) {
	t.Helper()

	db := pgtest.NewDB(t, companiespg.NewNamespace())
	pgtest.Truncate(t, db, "company.companies")

	return companiespg.NewCompanyPostgresRepository(slog.New(slog.NewTextHandler(io.Discard, nil)), db), db
}

func TestUpsertAndGet(t *testing.T) {
	repo, _ := newRepository(t)

	written, err := repo.Upsert(t.Context(),
		&companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."},
		&companies.Company{Symbol: "TLV", DisplayName: "BANCA TRANSILVANIA S.A."},
	)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if written != 2 {
		t.Errorf("wrote %d rows, want 2", written)
	}

	company, err := repo.Get(t.Context(), "SNP")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if company.DisplayName != "OMV PETROM S.A." {
		t.Errorf("display name = %q", company.DisplayName)
	}
	if company.CreateTime.IsZero() || company.UpdateTime.IsZero() {
		t.Error("the timestamps the database sets were not read back")
	}
}

func TestGetOfAnUnknownCompany(t *testing.T) {
	repo, _ := newRepository(t)

	_, err := repo.Get(t.Context(), "NOPE")
	if err == nil {
		t.Fatal("an unknown company was returned")
	}
	if !isNotFound(err) {
		t.Errorf("error = %v, want a not-found error", err)
	}
}

// A company that changed its legal name must end up with the new one. The
// previous service inserted only when absent, so a renamed company kept the
// old name forever.
func TestUpsertRefreshesAChangedName(t *testing.T) {
	repo, _ := newRepository(t)

	if _, err := repo.Upsert(t.Context(), &companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM"}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	before, err := repo.Get(t.Context(), "SNP")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	written, err := repo.Upsert(t.Context(), &companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if written != 1 {
		t.Errorf("a changed name wrote %d rows, want 1", written)
	}

	after, err := repo.Get(t.Context(), "SNP")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if after.DisplayName != "OMV PETROM S.A." {
		t.Errorf("display name = %q, want the new one", after.DisplayName)
	}
	if !after.UpdateTime.After(before.UpdateTime) {
		t.Error("the update time did not move when the name changed")
	}
	if !after.CreateTime.Equal(before.CreateTime) {
		t.Error("the creation time moved on a re-import")
	}
}

// Re-importing a company BVB has not changed must leave the row alone, so its
// update time keeps meaning "last changed" rather than "last seen", which is
// what AIP-142 defines it as.
func TestUpsertOfAnUnchangedCompanyWritesNothing(t *testing.T) {
	repo, _ := newRepository(t)

	company := &companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."}
	if _, err := repo.Upsert(t.Context(), company); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	before, err := repo.Get(t.Context(), "SNP")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	written, err := repo.Upsert(t.Context(), company)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if written != 0 {
		t.Errorf("an unchanged company wrote %d rows, want 0", written)
	}

	after, err := repo.Get(t.Context(), "SNP")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !after.UpdateTime.Equal(before.UpdateTime) {
		t.Error("the update time moved although nothing changed")
	}
}

func TestExistsAndCount(t *testing.T) {
	repo, _ := newRepository(t)

	if count, err := repo.Count(t.Context()); err != nil || count != 0 {
		t.Fatalf("Count on an empty table = %d, %v", count, err)
	}
	if exists, err := repo.Exists(t.Context(), "SNP"); err != nil || exists {
		t.Fatalf("Exists before the import = %v, %v", exists, err)
	}

	if _, err := repo.Upsert(t.Context(), &companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if count, err := repo.Count(t.Context()); err != nil || count != 1 {
		t.Errorf("Count = %d, %v, want 1", count, err)
	}
	if exists, err := repo.Exists(t.Context(), "SNP"); err != nil || !exists {
		t.Errorf("Exists = %v, %v, want true", exists, err)
	}
}

// The listing is keyset-paginated, so following the tokens must visit every
// company exactly once, in the order asked for.
func TestListPaginatesInOrder(t *testing.T) {
	repo, _ := newRepository(t)

	stored := []*companies.Company{
		{Symbol: "AAA", DisplayName: "A"},
		{Symbol: "BBB", DisplayName: "B"},
		{Symbol: "CCC", DisplayName: "C"},
		{Symbol: "DDD", DisplayName: "D"},
		{Symbol: "EEE", DisplayName: "E"},
	}
	if _, err := repo.Upsert(t.Context(), stored...); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	var seen []common.Symbol
	query := common.ListQuery{PageSize: 2}

	for range len(stored) {
		result, err := repo.List(t.Context(), query)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, company := range result.Items {
			seen = append(seen, company.Symbol)
		}
		if result.TotalSize == nil || *result.TotalSize != uint32(len(stored)) {
			t.Errorf("total size = %v, want %d", result.TotalSize, len(stored))
		}
		if result.NextPageToken == "" {
			break
		}
		query.PageToken = result.NextPageToken
	}

	if len(seen) != len(stored) {
		t.Fatalf("paging visited %d companies, want %d: %v", len(seen), len(stored), seen)
	}
	for i, company := range stored {
		if seen[i] != company.Symbol {
			t.Errorf("position %d = %q, want %q", i, seen[i], company.Symbol)
		}
	}
}

func isNotFound(err error) bool {
	var domainErr *common.Error

	return errors.As(err, &domainErr) && domainErr.Code() == common.ErrorCodeNotFound
}

// The whole path a list request takes: an AIP-160 expression, checked against
// the proto message, compiled to SQL, run against the view. Testing the halves
// separately would only prove they agree with each other.
func TestListAppliesAFilter(t *testing.T) {
	repo, _ := newRepository(t)

	if _, err := repo.Upsert(t.Context(),
		&companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."},
		&companies.Company{Symbol: "TLV", DisplayName: "BANCA TRANSILVANIA S.A."},
		&companies.Company{Symbol: "BRD", DisplayName: "BRD - GROUPE SOCIETE GENERALE"},
	); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cases := map[string]struct {
		filter string
		want   []common.Symbol
	}{
		"by symbol":                 {`symbol = "SNP"`, []common.Symbol{"SNP"}},
		"by name, case insensitive": {`display_name : "BANCA"`, []common.Symbol{"TLV"}},
		"a disjunction":             {`symbol = "SNP" OR symbol = "BRD"`, []common.Symbol{"BRD", "SNP"}},
		"a negation":                {`NOT symbol = "SNP"`, []common.Symbol{"BRD", "TLV"}},
		"the resource name":         {`name = "companies/TLV"`, []common.Symbol{"TLV"}},
		"nothing matches":           {`symbol = "NOPE"`, nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			filter, err := aip.CompileFilter(&bvbdividendsv1.Company{}, tc.filter)
			if err != nil {
				t.Fatalf("CompileFilter(%q): %v", tc.filter, err)
			}

			result, err := repo.List(t.Context(), common.ListQuery{PageSize: 10, Filter: filter})
			if err != nil {
				t.Fatalf("List(%q): %v", tc.filter, err)
			}

			got := make([]common.Symbol, len(result.Items))
			for i, company := range result.Items {
				got[i] = company.Symbol
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("%s: got %v, want %v", tc.filter, got, tc.want)
			}
		})
	}
}

// Ordering is the caller's, and it decides the page boundaries too.
func TestListOrders(t *testing.T) {
	repo, _ := newRepository(t)

	if _, err := repo.Upsert(t.Context(),
		&companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."},
		&companies.Company{Symbol: "TLV", DisplayName: "BANCA TRANSILVANIA S.A."},
		&companies.Company{Symbol: "BRD", DisplayName: "BRD - GROUPE SOCIETE GENERALE"},
	); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	result, err := repo.List(t.Context(), common.ListQuery{
		PageSize: 10,
		OrderBy:  common.OrderBy{{Path: "display_name", Direction: common.OrderDirectionDesc}},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	got := make([]common.Symbol, len(result.Items))
	for i, company := range result.Items {
		got[i] = company.Symbol
	}
	if want := []common.Symbol{"SNP", "BRD", "TLV"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
