package companies

import (
	"log/slog"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// IncrementalDays is the window of a routine run. The import runs daily, so
// one day covers everything announced since the last run.
const IncrementalDays = 1

// ImportCompaniesCommand commands one import of the companies this service
// knows about.
type ImportCompaniesCommand struct {
	// Days is the window of announcements to ask BVB for. Zero selects the
	// scope automatically: the whole market when nothing has been imported
	// yet, one day of announcements otherwise.
	Days int

	// All imports every company the service knows of rather than only those
	// that announced inside the window. A first import does this anyway; the
	// flag is for asking for it again later, when the market has changed.
	All bool

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
	// Window is the number of days the import asked BVB for, or zero when it
	// took the whole market instead.
	Window int

	// All reports whether the whole market was taken.
	All bool

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
		slog.Bool("whole_market", r.All),
		slog.Int("companies_seen", len(r.Symbols)),
		slog.Int("companies_written", r.Written),
		slog.Duration("duration", r.Duration),
	)
}
