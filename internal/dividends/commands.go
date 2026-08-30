package dividends

import (
	"errors"
	"log/slog"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// DefaultConcurrency is how many companies are fetched at once.
//
// BVB is a public service run by a stock exchange, not a CDN, and the import is
// not urgent; a handful of connections keeps a backfill of several hundred
// companies to minutes without leaning on it. The previous importer was fully
// sequential.
const DefaultConcurrency = 4

// ImportDividendsCommand commands one import of the dividends declared by the
// given companies.
type ImportDividendsCommand struct {
	// Symbols are the companies to fetch. They come from the companies import,
	// which is the only thing that passes between the two slices.
	Symbols []common.Symbol

	// DryRun fetches and maps everything but writes nothing.
	DryRun bool

	// Concurrency overrides how many companies are fetched at once. Zero uses
	// [DefaultConcurrency].
	Concurrency int
}

// ImportDividendsResult is the outcome of one import.
//
// It is returned rather than only logged, so the caller can exit non-zero, and
// so a scheduled run that quietly imported nothing is visible as a fact rather
// than as an absence of error messages. The previous importer logged failures
// and returned nothing.
type ImportDividendsResult struct {
	CompaniesSeen     int
	CompaniesImported int
	DividendsSeen     int
	DividendsWritten  int
	Failures          []error
	Duration          time.Duration
}

// FailedCompanies returns how many companies could not be imported.
func (r ImportDividendsResult) FailedCompanies() int { return len(r.Failures) }

// Err combines the per-company failures into one error, or returns nil when the
// run had none.
func (r ImportDividendsResult) Err() error {
	if len(r.Failures) == 0 {
		return nil
	}

	return errors.Join(r.Failures...)
}

// LogValue renders the result for structured logging.
func (r ImportDividendsResult) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("companies_seen", r.CompaniesSeen),
		slog.Int("companies_imported", r.CompaniesImported),
		slog.Int("dividends_seen", r.DividendsSeen),
		slog.Int("dividends_written", r.DividendsWritten),
		slog.Int("companies_failed", r.FailedCompanies()),
		slog.Duration("duration", r.Duration),
	)
}
