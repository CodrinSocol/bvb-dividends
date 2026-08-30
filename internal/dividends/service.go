package dividends

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// Service is the entry point to the dividends subdomain.
type Service struct {
	log    *slog.Logger
	repo   Repository
	source Source
}

// NewService returns a new instance of [Service].
func NewService(log *slog.Logger, repo Repository, source Source) *Service {
	return &Service{log: log.WithGroup("dividends"), repo: repo, source: source}
}

// GetDividend returns one dividend.
func (svc *Service) GetDividend(ctx context.Context, qry GetDividendQuery) (*Dividend, error) {
	dividend, err := svc.repo.Get(ctx, &qry.Company, qry.ID)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return dividend, nil
}

// ListDividends returns one page of dividends.
func (svc *Service) ListDividends(
	ctx context.Context,
	qry ListDividendsQuery,
) (common.ListResult[*Dividend], error) {
	result, err := svc.repo.List(ctx, qry.Company, qry.List)
	if err != nil {
		return common.ListResult[*Dividend]{}, errors.WithStack(err)
	}

	return result, nil
}

// ImportDividends refreshes every dividend declared by the given companies.
//
// A company that fails is recorded and the run continues, because one
// unavailable company must not cost the rest of the market its update. The
// error returned is non-nil only when the run could not proceed at all; the
// per-company failures are in the result.
func (svc *Service) ImportDividends(
	ctx context.Context,
	cmd ImportDividendsCommand,
) (ImportDividendsResult, error) {
	started := time.Now()

	svc.log.InfoContext(ctx, "importing dividends from BVB",
		slog.Int("companies", len(cmd.Symbols)),
		slog.Bool("dry_run", cmd.DryRun))

	result := ImportDividendsResult{CompaniesSeen: len(cmd.Symbols)}
	if len(cmd.Symbols) == 0 {
		result.Duration = time.Since(started)

		return result, nil
	}

	concurrency := cmd.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}

	var mu sync.Mutex
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(concurrency)

	for _, symbol := range cmd.Symbols {
		group.Go(func() error {
			seen, written, err := svc.importCompany(groupCtx, symbol, cmd.DryRun)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				// A failure here is this company's, not the run's. Returning it
				// would cancel the group and discard the companies still in
				// flight.
				result.Failures = append(result.Failures, errors.Wrapf(err, "import %s", symbol))
				svc.log.ErrorContext(ctx, "could not import a company",
					slog.String("company", symbol.String()),
					slog.Any("err", err))

				return nil
			}

			result.CompaniesImported++
			result.DividendsSeen += seen
			result.DividendsWritten += written

			return nil
		})
	}

	// The only error the group can carry is a cancelled context, since
	// per-company failures are recorded rather than returned.
	if err := group.Wait(); err != nil {
		return result, errors.Wrap(err, "import interrupted")
	}

	result.Duration = time.Since(started)
	svc.log.InfoContext(ctx, "imported dividends from BVB", slog.Any("result", result))

	return result, nil
}

// importCompany stores every dividend one company has declared, returning how
// many were seen and how many rows changed.
//
// Each company is written independently, so one failure is isolated to that
// company. The previous importer wrapped the whole run in a single
// transaction and deleted a company's dividends before re-inserting them, so
// an interruption mid-run could leave a company with none at all.
func (svc *Service) importCompany(
	ctx context.Context,
	symbol common.Symbol,
	dryRun bool,
) (seen, written int, err error) {
	list, err := svc.source.DividendsFor(ctx, symbol)
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}

	svc.warnOnCollisions(symbol, list)

	if dryRun {
		return len(list), 0, nil
	}

	written, err = svc.repo.Upsert(ctx, list...)
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}

	svc.log.DebugContext(ctx, "imported a company",
		slog.String("company", symbol.String()),
		slog.Int("dividends_seen", len(list)),
		slog.Int("dividends_written", written))

	return len(list), written, nil
}

// warnOnCollisions reports dividends that share a natural key.
//
// Two such dividends derive the same identifier and the second silently
// replaces the first. That should not happen — a company declares at most one
// dividend of a type, for a year, going ex on a date — so if it ever does, it
// means the assumption behind the identifier is wrong and needs to be seen
// rather than discovered later as missing data.
func (svc *Service) warnOnCollisions(symbol common.Symbol, list []*Dividend) {
	seen := make(map[ID]*Dividend, len(list))

	for _, dividend := range list {
		if previous, collides := seen[dividend.ID]; collides {
			svc.log.Warn("BVB reported two dividends with the same natural key; the later one wins",
				slog.String("company", symbol.String()),
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
