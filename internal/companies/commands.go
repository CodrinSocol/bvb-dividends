package companies

import (
	"log/slog"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// Import windows, in days.
const (
	// BackfillDays is how far back the first import reaches. BVB's service
	// takes a window in days, so twenty years of history is asked for as a
	// number of days, the same way the Java service did it.
	BackfillDays = 20 * 365

	// IncrementalDays is the window of a routine run. The import runs daily, so
	// one day covers everything announced since the last run.
	IncrementalDays = 1
)

// ImportCompaniesCommand commands one import of the companies that have
// announced a dividend.
type ImportCompaniesCommand struct {
	// Days is the window to ask BVB for. Zero selects it automatically: a full
	// backfill when nothing has been imported yet, one day otherwise.
	Days int

	// DryRun fetches and maps everything but writes nothing, so a run can be
	// checked against the live service without touching the database.
	DryRun bool
}

// ImportCompaniesResult is the outcome of one import.
//
// It is returned rather than only logged, so that a scheduled run which quietly
// imported nothing is visible as a fact rather than as an absence of error
// messages, and so that the symbols it saw can be handed to the dividends
// import without reading them back out of the database.
type ImportCompaniesResult struct {
	// Window is the number of days the import asked BVB for.
	Window int

	// Symbols are the companies BVB reported, deduplicated.
	Symbols []common.Symbol

	// Written is how many rows the import actually changed.
	Written int

	// Duration is how long the import took.
	Duration time.Duration
}

// LogValue renders the result for structured logging.
func (r ImportCompaniesResult) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("window_days", r.Window),
		slog.Int("companies_seen", len(r.Symbols)),
		slog.Int("companies_written", r.Written),
		slog.Duration("duration", r.Duration),
	)
}
