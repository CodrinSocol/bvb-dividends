package companies_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeSource stands in for BVB.
type fakeSource struct {
	companies []*companies.Company
	err       error
	days      int
}

func (f *fakeSource) RecentlyAnnouncing(_ context.Context, days int) ([]*companies.Company, error) {
	f.days = days

	return f.companies, f.err
}

// fakeRepository is an in-memory [companies.Repository].
type fakeRepository struct {
	stored   map[common.Symbol]*companies.Company
	upserted [][]*companies.Company
}

func newFakeRepository(stored ...*companies.Company) *fakeRepository {
	byLabel := make(map[common.Symbol]*companies.Company, len(stored))
	for _, company := range stored {
		byLabel[company.Symbol] = company
	}

	return &fakeRepository{stored: byLabel}
}

func (f *fakeRepository) Get(_ context.Context, symbol common.Symbol) (*companies.Company, error) {
	company, ok := f.stored[symbol]
	if !ok {
		return nil, common.ErrEntityNotFound
	}

	return company, nil
}

func (f *fakeRepository) List(context.Context, common.ListQuery) (common.ListResult[*companies.Company], error) {
	return common.ListResult[*companies.Company]{}, nil
}

func (f *fakeRepository) Exists(_ context.Context, symbol common.Symbol) (bool, error) {
	_, ok := f.stored[symbol]

	return ok, nil
}

func (f *fakeRepository) Count(context.Context) (int64, error) { return int64(len(f.stored)), nil }

func (f *fakeRepository) Upsert(_ context.Context, list ...*companies.Company) (int, error) {
	f.upserted = append(f.upserted, list)
	for _, company := range list {
		f.stored[company.Symbol] = company
	}

	return len(list), nil
}

// An empty database means nothing has ever been imported, so the first run asks
// for the whole history rather than for yesterday.
func TestImportBackfillsAnEmptyDatabase(t *testing.T) {
	t.Parallel()

	source := &fakeSource{companies: []*companies.Company{{Symbol: "SNP", DisplayName: "OMV PETROM S.A."}}}
	repo := newFakeRepository()
	svc := companies.NewService(quietLogger(), repo, source)

	result, err := svc.ImportCompanies(t.Context(), companies.ImportCompaniesCommand{})
	if err != nil {
		t.Fatalf("ImportCompanies: %v", err)
	}

	if source.days != companies.BackfillDays {
		t.Errorf("asked for %d days, want the backfill window of %d", source.days, companies.BackfillDays)
	}
	if result.Window != companies.BackfillDays {
		t.Errorf("reported window = %d, want %d", result.Window, companies.BackfillDays)
	}
	if len(result.Symbols) != 1 || result.Symbols[0] != "SNP" {
		t.Errorf("symbols = %v, want [SNP]", result.Symbols)
	}
}

func TestImportIsIncrementalOnceSomethingIsStored(t *testing.T) {
	t.Parallel()

	source := &fakeSource{}
	repo := newFakeRepository(&companies.Company{Symbol: "TLV"})
	svc := companies.NewService(quietLogger(), repo, source)

	if _, err := svc.ImportCompanies(t.Context(), companies.ImportCompaniesCommand{}); err != nil {
		t.Fatalf("ImportCompanies: %v", err)
	}

	if source.days != companies.IncrementalDays {
		t.Errorf("asked for %d days, want the incremental window of %d", source.days, companies.IncrementalDays)
	}
}

func TestImportHonoursAnExplicitWindow(t *testing.T) {
	t.Parallel()

	source := &fakeSource{}
	svc := companies.NewService(quietLogger(), newFakeRepository(), source)

	if _, err := svc.ImportCompanies(t.Context(), companies.ImportCompaniesCommand{Days: 30}); err != nil {
		t.Fatalf("ImportCompanies: %v", err)
	}

	if source.days != 30 {
		t.Errorf("asked for %d days, want 30", source.days)
	}
}

// A dry run is how the SOAP field names are checked against the live service,
// so it must reach BVB and report what it found without writing anything.
func TestDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	source := &fakeSource{companies: []*companies.Company{{Symbol: "SNP"}, {Symbol: "TLV"}}}
	repo := newFakeRepository()
	svc := companies.NewService(quietLogger(), repo, source)

	result, err := svc.ImportCompanies(t.Context(), companies.ImportCompaniesCommand{Days: 1, DryRun: true})
	if err != nil {
		t.Fatalf("ImportCompanies: %v", err)
	}

	if len(repo.upserted) != 0 {
		t.Errorf("a dry run wrote %d batches", len(repo.upserted))
	}
	if len(result.Symbols) != 2 {
		t.Errorf("a dry run reported %d symbols, want 2", len(result.Symbols))
	}
	if result.Written != 0 {
		t.Errorf("a dry run reported %d rows written", result.Written)
	}
}

// A window BVB has no announcements for is a fact, not a failure, and must not
// look like one.
func TestImportOfAnEmptyWindowSucceeds(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository(&companies.Company{Symbol: "TLV"})
	svc := companies.NewService(quietLogger(), repo, &fakeSource{})

	result, err := svc.ImportCompanies(t.Context(), companies.ImportCompaniesCommand{Days: 1})
	if err != nil {
		t.Fatalf("ImportCompanies: %v", err)
	}
	if len(result.Symbols) != 0 || result.Written != 0 {
		t.Errorf("result = %+v, want an empty import", result)
	}
	if len(repo.upserted) != 0 {
		t.Error("an empty import still wrote to the repository")
	}
}

func TestImportReportsAFailedFetch(t *testing.T) {
	t.Parallel()

	source := &fakeSource{err: common.ErrEntityInvalid}
	svc := companies.NewService(quietLogger(), newFakeRepository(), source)

	if _, err := svc.ImportCompanies(t.Context(), companies.ImportCompaniesCommand{Days: 1}); err == nil {
		t.Fatal("a failed fetch was reported as a successful import")
	}
}

func TestGetCompany(t *testing.T) {
	t.Parallel()

	repo := newFakeRepository(&companies.Company{Symbol: "SNP", DisplayName: "OMV PETROM S.A."})
	svc := companies.NewService(quietLogger(), repo, &fakeSource{})

	company, err := svc.GetCompany(t.Context(), companies.GetCompanyQuery{Symbol: "SNP"})
	if err != nil {
		t.Fatalf("GetCompany: %v", err)
	}
	if company.DisplayName != "OMV PETROM S.A." {
		t.Errorf("display name = %q", company.DisplayName)
	}

	if _, err := svc.GetCompany(t.Context(), companies.GetCompanyQuery{Symbol: "NOPE"}); err == nil {
		t.Error("an unknown company was returned rather than reported missing")
	}
}
