package dividends_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeSource stands in for BVB.
type fakeSource struct {
	byCompany map[common.Symbol][]*dividends.Dividend
	failFor   map[common.Symbol]error

	mu    sync.Mutex
	calls []common.Symbol
}

func (f *fakeSource) DividendsFor(_ context.Context, symbol common.Symbol) ([]*dividends.Dividend, error) {
	f.mu.Lock()
	f.calls = append(f.calls, symbol)
	f.mu.Unlock()

	if err, failing := f.failFor[symbol]; failing {
		return nil, err
	}

	return f.byCompany[symbol], nil
}

// fakeRepository is an in-memory [dividends.Repository].
type fakeRepository struct {
	mu     sync.Mutex
	stored map[dividends.ID]*dividends.Dividend
	err    error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{stored: map[dividends.ID]*dividends.Dividend{}}
}

func (f *fakeRepository) Get(context.Context, *common.Symbol, dividends.ID) (*dividends.Dividend, error) {
	return nil, common.ErrEntityNotFound
}

func (f *fakeRepository) List(
	context.Context,
	*common.Symbol,
	common.ListQuery,
) (common.ListResult[*dividends.Dividend], error) {
	return common.ListResult[*dividends.Dividend]{}, nil
}

func (f *fakeRepository) Upsert(_ context.Context, list ...*dividends.Dividend) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return 0, f.err
	}

	for _, dividend := range list {
		f.stored[dividend.ID] = dividend
	}

	return len(list), nil
}

func (f *fakeRepository) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.stored)
}

func dividendFor(symbol common.Symbol, year int) *dividends.Dividend {
	dividend := &dividends.Dividend{Company: symbol, Year: year, Type: "cash"}
	dividend.ID = dividends.NewID(dividend.NaturalKey())

	return dividend
}

func TestImportStoresEveryCompanysDividends(t *testing.T) {
	t.Parallel()

	source := &fakeSource{byCompany: map[common.Symbol][]*dividends.Dividend{
		"SNP": {dividendFor("SNP", 2024), dividendFor("SNP", 2025)},
		"TLV": {dividendFor("TLV", 2025)},
	}}
	repo := newFakeRepository()
	svc := dividends.NewService(quietLogger(), repo, source)

	result, err := svc.ImportDividends(t.Context(), dividends.ImportDividendsCommand{
		Symbols: []common.Symbol{"SNP", "TLV"},
	})
	if err != nil {
		t.Fatalf("ImportDividends: %v", err)
	}

	if result.CompaniesImported != 2 {
		t.Errorf("imported %d companies, want 2", result.CompaniesImported)
	}
	if result.DividendsSeen != 3 || result.DividendsWritten != 3 {
		t.Errorf("saw %d and wrote %d dividends, want 3 and 3", result.DividendsSeen, result.DividendsWritten)
	}
	if repo.count() != 3 {
		t.Errorf("the repository holds %d dividends, want 3", repo.count())
	}
	if result.Err() != nil {
		t.Errorf("a clean run reported failures: %v", result.Err())
	}
}

// One unavailable company must not cost the rest of the market its update. The
// failure is recorded rather than returned, so the run continues.
func TestOneFailingCompanyDoesNotStopTheRest(t *testing.T) {
	t.Parallel()

	source := &fakeSource{
		byCompany: map[common.Symbol][]*dividends.Dividend{
			"SNP": {dividendFor("SNP", 2025)},
			"TLV": {dividendFor("TLV", 2025)},
		},
		failFor: map[common.Symbol]error{"BRD": common.ErrEntityInvalid},
	}
	repo := newFakeRepository()
	svc := dividends.NewService(quietLogger(), repo, source)

	result, err := svc.ImportDividends(t.Context(), dividends.ImportDividendsCommand{
		Symbols: []common.Symbol{"SNP", "BRD", "TLV"},
	})
	if err != nil {
		t.Fatalf("ImportDividends returned an error for one failing company: %v", err)
	}

	if result.CompaniesImported != 2 {
		t.Errorf("imported %d companies, want 2", result.CompaniesImported)
	}
	if result.FailedCompanies() != 1 {
		t.Errorf("recorded %d failures, want 1", result.FailedCompanies())
	}
	if result.Err() == nil {
		t.Error("the result reported no failure although one company failed")
	}
	if repo.count() != 2 {
		t.Errorf("the repository holds %d dividends, want 2", repo.count())
	}
}

// A dry run is how the SOAP field names are checked against the live service.
func TestDryRunWritesNothing(t *testing.T) {
	t.Parallel()

	source := &fakeSource{byCompany: map[common.Symbol][]*dividends.Dividend{
		"SNP": {dividendFor("SNP", 2024), dividendFor("SNP", 2025)},
	}}
	repo := newFakeRepository()
	svc := dividends.NewService(quietLogger(), repo, source)

	result, err := svc.ImportDividends(t.Context(), dividends.ImportDividendsCommand{
		Symbols: []common.Symbol{"SNP"},
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("ImportDividends: %v", err)
	}

	if repo.count() != 0 {
		t.Errorf("a dry run wrote %d dividends", repo.count())
	}
	if result.DividendsSeen != 2 {
		t.Errorf("a dry run saw %d dividends, want 2", result.DividendsSeen)
	}
	if result.DividendsWritten != 0 {
		t.Errorf("a dry run reported %d dividends written", result.DividendsWritten)
	}
}

func TestImportOfNoCompaniesSucceeds(t *testing.T) {
	t.Parallel()

	svc := dividends.NewService(quietLogger(), newFakeRepository(), &fakeSource{})

	result, err := svc.ImportDividends(t.Context(), dividends.ImportDividendsCommand{})
	if err != nil {
		t.Fatalf("ImportDividends: %v", err)
	}
	if result.CompaniesSeen != 0 || result.DividendsWritten != 0 {
		t.Errorf("result = %+v, want an empty import", result)
	}
}

// Every company must be fetched exactly once, however many are fetched at a
// time.
func TestImportFetchesEachCompanyOnce(t *testing.T) {
	t.Parallel()

	symbols := []common.Symbol{"AAA", "BBB", "CCC", "DDD", "EEE", "FFF", "GGG"}
	source := &fakeSource{byCompany: map[common.Symbol][]*dividends.Dividend{}}
	for _, symbol := range symbols {
		source.byCompany[symbol] = []*dividends.Dividend{dividendFor(symbol, 2025)}
	}

	svc := dividends.NewService(quietLogger(), newFakeRepository(), source)

	if _, err := svc.ImportDividends(t.Context(), dividends.ImportDividendsCommand{
		Symbols:     symbols,
		Concurrency: 3,
	}); err != nil {
		t.Fatalf("ImportDividends: %v", err)
	}

	seen := map[common.Symbol]int{}
	for _, symbol := range source.calls {
		seen[symbol]++
	}
	if len(seen) != len(symbols) {
		t.Errorf("fetched %d companies, want %d", len(seen), len(symbols))
	}
	for symbol, count := range seen {
		if count != 1 {
			t.Errorf("fetched %s %d times, want once", symbol, count)
		}
	}
}

// A dividend that fails to store is this company's failure, not the run's.
func TestAFailedWriteIsRecordedPerCompany(t *testing.T) {
	t.Parallel()

	source := &fakeSource{byCompany: map[common.Symbol][]*dividends.Dividend{
		"SNP": {dividendFor("SNP", 2025)},
	}}
	repo := newFakeRepository()
	repo.err = common.ErrEntityConstraintViolation
	svc := dividends.NewService(quietLogger(), repo, source)

	result, err := svc.ImportDividends(t.Context(), dividends.ImportDividendsCommand{
		Symbols: []common.Symbol{"SNP"},
	})
	if err != nil {
		t.Fatalf("ImportDividends: %v", err)
	}
	if result.FailedCompanies() != 1 {
		t.Errorf("recorded %d failures, want 1", result.FailedCompanies())
	}
}
