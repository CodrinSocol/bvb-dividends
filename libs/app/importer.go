// Package app holds the service's use cases: importing from BVB and querying
// what was imported. It depends on the domain's ports, never on an adapter.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// Import windows, in days.
const (
	// BackfillDays is how far back the first import reaches. BVB's service
	// takes a window in days, so twenty years of history is asked for as a
	// number of days, the same way the Java service did it.
	BackfillDays = 20 * 365

	// IncrementalDays is the window of a routine run. The importer runs daily,
	// so one day covers everything announced since the last run.
	IncrementalDays = 1
)

// DefaultConcurrency is how many companies are fetched at once.
//
// BVB is a public service run by a stock exchange, not a CDN, and the import
// is not urgent; a handful of connections keeps a backfill of several hundred
// companies to minutes without leaning on it. The Java importer was fully
// sequential.
const DefaultConcurrency = 4

// Importer refreshes stored companies and dividends from BVB.
type Importer struct {
	source      domain.Source
	companies   domain.CompanyRepository
	dividends   domain.DividendRepository
	log         *slog.Logger
	concurrency int
}

// NewImporter returns an Importer reading from source into the repositories.
func NewImporter(
	source domain.Source,
	companies domain.CompanyRepository,
	dividends domain.DividendRepository,
	log *slog.Logger,
) *Importer {
	return &Importer{
		source:      source,
		companies:   companies,
		dividends:   dividends,
		log:         log,
		concurrency: DefaultConcurrency,
	}
}

// SetConcurrency overrides how many companies are fetched at once.
func (i *Importer) SetConcurrency(n int) {
	if n > 0 {
		i.concurrency = n
	}
}

// Options control one import run.
type Options struct {
	// Days is the window to ask BVB for. Zero selects it automatically: a full
	// backfill when nothing has been imported yet, one day otherwise.
	Days int

	// DryRun fetches and maps everything but writes nothing, so a run can be
	// checked against the live service without touching the database.
	DryRun bool
}

// Report is the outcome of one import run.
//
// It is returned rather than only logged, so the caller can exit non-zero, and
// so a scheduled run that quietly imported nothing is visible as a fact rather
// than as an absence of error messages. The Java importer logged failures and
// returned nothing.
type Report struct {
	Window            int
	CompaniesSeen     int
	CompaniesImported int
	DividendsSeen     int
	DividendsWritten  int
	Failures          []error
	Duration          time.Duration
}

// FailedCompanies returns how many companies could not be imported.
func (r Report) FailedCompanies() int { return len(r.Failures) }

// LogValue renders the report for structured logging.
func (r Report) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("window_days", r.Window),
		slog.Int("companies_seen", r.CompaniesSeen),
		slog.Int("companies_imported", r.CompaniesImported),
		slog.Int("dividends_seen", r.DividendsSeen),
		slog.Int("dividends_written", r.DividendsWritten),
		slog.Int("companies_failed", r.FailedCompanies()),
		slog.Duration("duration", r.Duration),
	)
}

// Run imports the companies that announced a dividend in the chosen window,
// together with every dividend each of them has declared.
//
// A company that fails is recorded and the run continues, because one
// unavailable company must not cost the rest of the market its update. The
// error returned is non-nil only when the run could not proceed at all.
func (i *Importer) Run(ctx context.Context, options Options) (Report, error) {
	started := time.Now()

	window, err := i.window(ctx, options)
	if err != nil {
		return Report{}, err
	}

	i.log.Info("starting BVB import",
		slog.Int("window_days", window),
		slog.Bool("dry_run", options.DryRun))

	companies, err := i.source.RecentlyAnnouncing(ctx, window)
	if err != nil {
		return Report{}, fmt.Errorf("list companies announcing dividends: %w", err)
	}

	report := Report{Window: window, CompaniesSeen: len(companies)}
	if len(companies) == 0 {
		report.Duration = time.Since(started)
		i.log.Info("BVB reported no announcements in the window", slog.Any("report", report))
		return report, nil
	}

	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(i.concurrency)

	for _, company := range companies {
		group.Go(func() error {
			seen, written, err := i.importCompany(groupCtx, company, options.DryRun)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				// A failure here is this company's, not the run's. Cancelling
				// the group would discard the companies still in flight.
				report.Failures = append(report.Failures, err)
				i.log.Error("could not import a company",
					slog.String("company", company.Symbol.String()),
					slog.String("error", err.Error()))
				return nil
			}
			report.CompaniesImported++
			report.DividendsSeen += seen
			report.DividendsWritten += written
			return nil
		})
	}

	// The only error the group can carry is a cancelled context, since
	// per-company failures are recorded rather than returned.
	if err := group.Wait(); err != nil {
		return report, fmt.Errorf("import interrupted: %w", err)
	}

	report.Duration = time.Since(started)
	i.log.Info("finished BVB import", slog.Any("report", report))
	return report, nil
}

// window decides how far back to ask BVB for announcements.
func (i *Importer) window(ctx context.Context, options Options) (int, error) {
	if options.Days > 0 {
		return options.Days, nil
	}

	count, err := i.companies.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("decide import window: %w", err)
	}
	if count == 0 {
		return BackfillDays, nil
	}
	return IncrementalDays, nil
}

// importCompany stores one company and every dividend it has declared,
// returning how many dividends were seen and how many rows changed.
//
// The company is written before its dividends because a dividend references
// it, and each company is written independently so that one failure is
// isolated to that company. The Java importer wrapped the whole run in a
// single transaction and deleted a company's dividends before re-inserting
// them, so an interruption mid-run could leave a company with none at all.
func (i *Importer) importCompany(ctx context.Context, company domain.Company, dryRun bool) (seen, written int, err error) {
	dividends, err := i.source.DividendsFor(ctx, company)
	if err != nil {
		return 0, 0, err
	}
	i.warnOnCollisions(company, dividends)

	if dryRun {
		return len(dividends), 0, nil
	}

	if err := i.companies.Upsert(ctx, company); err != nil {
		return 0, 0, err
	}
	written, err = i.dividends.Upsert(ctx, dividends...)
	if err != nil {
		return 0, 0, err
	}

	i.log.Debug("imported a company",
		slog.String("company", company.Symbol.String()),
		slog.Int("dividends_seen", len(dividends)),
		slog.Int("dividends_written", written))
	return len(dividends), written, nil
}

// warnOnCollisions reports dividends that share a natural key.
//
// Two such dividends derive the same identifier and the second silently
// replaces the first. That should not happen — a company declares at most one
// dividend of a type, for a year, going ex on a date — so if it ever does, it
// means the assumption behind the identifier is wrong and needs to be seen
// rather than discovered later as missing data.
func (i *Importer) warnOnCollisions(company domain.Company, dividends []domain.Dividend) {
	seen := make(map[domain.ID]domain.Dividend, len(dividends))
	for _, dividend := range dividends {
		if previous, collides := seen[dividend.ID]; collides {
			i.log.Warn("BVB reported two dividends with the same natural key; the later one wins",
				slog.String("company", company.Symbol.String()),
				slog.Int("year", dividend.Year),
				slog.String("type", dividend.Type),
				slog.String("ex_dividend_date", dividend.Schedule.ExDividendDate.String()),
				slog.String("id", dividend.ID.String()),
				slog.String("previous_total", previous.Amounts.Total.String()),
				slog.String("current_total", dividend.Amounts.Total.String()))
		}
		seen[dividend.ID] = dividend
	}
}

// JoinFailures combines a report's per-company failures into one error, or
// returns nil when the run had none.
func JoinFailures(report Report) error {
	if len(report.Failures) == 0 {
		return nil
	}
	return errors.Join(report.Failures...)
}
