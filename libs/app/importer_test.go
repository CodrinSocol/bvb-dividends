package app_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/app"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeSource stands in for BVB.
type fakeSource struct {
	companies []domain.Company
	dividends map[domain.Symbol][]domain.Dividend
	failFor   map[domain.Symbol]error
	listErr   error

	mu    sync.Mutex
	calls []domain.Symbol
	days  int
}

func (f *fakeSource) RecentlyAnnouncing(_ context.Context, days int) ([]domain.Company, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.days = days
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.companies, nil
}

func (f *fakeSource) DividendsFor(_ context.Context, company domain.Company) ([]domain.Dividend, error) {
	f.mu.Lock()
	f.calls = append(f.calls, company.Symbol)
	f.mu.Unlock()

	if err, failing := f.failFor[company.Symbol]; failing {
		return nil, err
	}
	return f.dividends[company.Symbol], nil
}

// fakeCompanies is an in-memory CompanyRepository.
type fakeCompanies struct {
	mu     sync.Mutex
	stored map[domain.Symbol]domain.Company
}

func newFakeCompanies() *fakeCompanies {
	return &fakeCompanies{stored: map[domain.Symbol]domain.Company{}}
}

func (f *fakeCompanies) Upsert(_ context.Context, companies ...domain.Company) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, company := range companies {
		f.stored[company.Symbol] = company
	}
	return nil
}

func (f *fakeCompanies) Get(context.Context, domain.Symbol) (domain.Company, error) {
	return domain.Company{}, domain.ErrNotFound
}
func (f *fakeCompanies) Exists(context.Context, domain.Symbol) (bool, error) { return false, nil }
func (f *fakeCompanies) List(context.Context, domain.CompanyQuery) (domain.Page[domain.Company], error) {
	return domain.Page[domain.Company]{}, nil
}

func (f *fakeCompanies) Count(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int64(len(f.stored)), nil
}

// fakeDividends is an in-memory DividendRepository.
type fakeDividends struct {
	mu     sync.Mutex
	stored map[domain.ID]domain.Dividend
}

func newFakeDividends() *fakeDividends {
	return &fakeDividends{stored: map[domain.ID]domain.Dividend{}}
}

func (f *fakeDividends) Upsert(_ context.Context, dividends ...domain.Dividend) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, dividend := range dividends {
		f.stored[dividend.ID] = dividend
	}
	return len(dividends), nil
}

func (f *fakeDividends) Get(context.Context, domain.ID) (domain.Dividend, error) {
	return domain.Dividend{}, domain.ErrNotFound
}
func (f *fakeDividends) List(context.Context, domain.DividendQuery) (domain.Page[domain.Dividend], error) {
	return domain.Page[domain.Dividend]{}, nil
}

func (f *fakeDividends) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.stored)
}

func dividendFor(company domain.Symbol, year int) domain.Dividend {
	d := domain.Dividend{Company: company, Year: year, Type: "cash"}
	d.ID = domain.NewID(d.NaturalKey())
	return d
}

// One company BVB cannot serve must not cost the rest of the market its
// update. The Java importer caught the exception per company but wrapped the
// whole run in one transaction, so a later failure could still undo earlier work.
func TestRunIsolatesPerCompanyFailures(t *testing.T) {
	source := &fakeSource{
		companies: []domain.Company{{Symbol: "SNP"}, {Symbol: "TLV"}, {Symbol: "BRD"}},
		dividends: map[domain.Symbol][]domain.Dividend{
			"SNP": {dividendFor("SNP", 2024), dividendFor("SNP", 2025)},
			"BRD": {dividendFor("BRD", 2025)},
		},
		failFor: map[domain.Symbol]error{"TLV": errors.New("bvb is unavailable")},
	}
	companies, dividends := newFakeCompanies(), newFakeDividends()
	importer := app.NewImporter(source, companies, dividends, quietLogger())

	report, err := importer.Run(context.Background(), app.Options{Days: 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.CompaniesSeen != 3 {
		t.Errorf("companies seen = %d, want 3", report.CompaniesSeen)
	}
	if report.CompaniesImported != 2 {
		t.Errorf("companies imported = %d, want 2", report.CompaniesImported)
	}
	if report.FailedCompanies() != 1 {
		t.Errorf("failures = %d, want 1", report.FailedCompanies())
	}
	if report.DividendsWritten != 3 {
		t.Errorf("dividends written = %d, want 3", report.DividendsWritten)
	}
	if got := dividends.count(); got != 3 {
		t.Errorf("stored %d dividends, want 3", got)
	}
	if err := app.JoinFailures(report); err == nil {
		t.Error("JoinFailures returned nil for a run with a failure")
	}
}

// The window is a backfill only when nothing has been imported yet.
func TestRunChoosesItsWindow(t *testing.T) {
	tests := []struct {
		name      string
		preloaded bool
		want      int
	}{
		{name: "first run backfills", preloaded: false, want: app.BackfillDays},
		{name: "later runs are incremental", preloaded: true, want: app.IncrementalDays},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &fakeSource{}
			companies := newFakeCompanies()
			if tt.preloaded {
				if err := companies.Upsert(context.Background(), domain.Company{Symbol: "SNP"}); err != nil {
					t.Fatalf("preload: %v", err)
				}
			}
			importer := app.NewImporter(source, companies, newFakeDividends(), quietLogger())

			report, err := importer.Run(context.Background(), app.Options{})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if source.days != tt.want {
				t.Errorf("asked BVB for %d days, want %d", source.days, tt.want)
			}
			if report.Window != tt.want {
				t.Errorf("report window = %d, want %d", report.Window, tt.want)
			}
		})
	}
}

func TestRunExplicitWindowWins(t *testing.T) {
	source := &fakeSource{}
	importer := app.NewImporter(source, newFakeCompanies(), newFakeDividends(), quietLogger())

	if _, err := importer.Run(context.Background(), app.Options{Days: 30}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if source.days != 30 {
		t.Errorf("asked BVB for %d days, want 30", source.days)
	}
}

// A dry run must be safe to point at the live service.
func TestRunDryRunWritesNothing(t *testing.T) {
	source := &fakeSource{
		companies: []domain.Company{{Symbol: "SNP"}},
		dividends: map[domain.Symbol][]domain.Dividend{"SNP": {dividendFor("SNP", 2025)}},
	}
	companies, dividends := newFakeCompanies(), newFakeDividends()
	importer := app.NewImporter(source, companies, dividends, quietLogger())

	report, err := importer.Run(context.Background(), app.Options{Days: 1, DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.DividendsSeen != 1 {
		t.Errorf("dividends seen = %d, want 1", report.DividendsSeen)
	}
	if report.DividendsWritten != 0 {
		t.Errorf("dividends written = %d, want 0 in a dry run", report.DividendsWritten)
	}
	if dividends.count() != 0 {
		t.Error("a dry run wrote dividends")
	}
	if count, _ := companies.Count(context.Background()); count != 0 {
		t.Error("a dry run wrote companies")
	}
}

// If BVB itself cannot be listed there is nothing to isolate, so the run fails.
func TestRunFailsWhenTheListingFails(t *testing.T) {
	source := &fakeSource{listErr: errors.New("bvb is down")}
	importer := app.NewImporter(source, newFakeCompanies(), newFakeDividends(), quietLogger())

	if _, err := importer.Run(context.Background(), app.Options{Days: 1}); err == nil {
		t.Fatal("expected an error when the company listing fails")
	}
}

func TestRunFetchesCompaniesConcurrently(t *testing.T) {
	source := &fakeSource{}
	for i := range 20 {
		symbol := domain.Symbol(string(rune('A' + i)))
		source.companies = append(source.companies, domain.Company{Symbol: symbol})
	}
	importer := app.NewImporter(source, newFakeCompanies(), newFakeDividends(), quietLogger())
	importer.SetConcurrency(4)

	report, err := importer.Run(context.Background(), app.Options{Days: 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.CompaniesImported != 20 {
		t.Errorf("imported %d companies, want 20", report.CompaniesImported)
	}
	if len(source.calls) != 20 {
		t.Errorf("made %d calls, want 20", len(source.calls))
	}
}
