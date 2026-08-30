package companies

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// Service is the entry point to the companies subdomain.
type Service struct {
	log    *slog.Logger
	repo   Repository
	source Source
}

// NewService returns a new instance of [Service].
func NewService(log *slog.Logger, repo Repository, source Source) *Service {
	return &Service{log: log.WithGroup("companies"), repo: repo, source: source}
}

// GetCompany returns one company by ticker symbol.
func (svc *Service) GetCompany(ctx context.Context, qry GetCompanyQuery) (*Company, error) {
	company, err := svc.repo.Get(ctx, qry.Symbol)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return company, nil
}

// ListCompanies returns one page of companies.
func (svc *Service) ListCompanies(
	ctx context.Context,
	qry common.ListQuery,
) (common.ListResult[*Company], error) {
	result, err := svc.repo.List(ctx, qry)
	if err != nil {
		return common.ListResult[*Company]{}, errors.WithStack(err)
	}

	return result, nil
}

// Exists reports whether a company is known.
//
// It is here so that another slice can ask the question through an interface of
// its own, rather than reading this slice's table.
func (svc *Service) Exists(ctx context.Context, symbol common.Symbol) (bool, error) {
	exists, err := svc.repo.Exists(ctx, symbol)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return exists, nil
}

// ImportCompanies refreshes the companies that have announced a dividend.
//
// It returns the symbols it saw, which is what the dividends import needs: the
// two slices fetch from BVB separately, and this is the only thing that passes
// between them.
func (svc *Service) ImportCompanies(
	ctx context.Context,
	cmd ImportCompaniesCommand,
) (ImportCompaniesResult, error) {
	started := time.Now()

	window, err := svc.window(ctx, cmd)
	if err != nil {
		return ImportCompaniesResult{}, err
	}

	svc.log.InfoContext(ctx, "importing companies from BVB",
		slog.Int("window_days", window),
		slog.Bool("dry_run", cmd.DryRun))

	companies, err := svc.source.RecentlyAnnouncing(ctx, window)
	if err != nil {
		return ImportCompaniesResult{}, errors.Wrap(err, "list companies announcing dividends")
	}

	result := ImportCompaniesResult{Window: window, Symbols: symbolsOf(companies)}

	if !cmd.DryRun && len(companies) > 0 {
		written, err := svc.repo.Upsert(ctx, companies...)
		if err != nil {
			return ImportCompaniesResult{}, errors.Wrap(err, "store companies")
		}

		result.Written = written
	}

	result.Duration = time.Since(started)
	svc.log.InfoContext(ctx, "imported companies from BVB", slog.Any("result", result))

	return result, nil
}

// window decides how far back to ask BVB for announcements.
func (svc *Service) window(ctx context.Context, cmd ImportCompaniesCommand) (int, error) {
	if cmd.Days > 0 {
		return cmd.Days, nil
	}

	count, err := svc.repo.Count(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "decide import window")
	}
	if count == 0 {
		return BackfillDays, nil
	}

	return IncrementalDays, nil
}

func symbolsOf(companies []*Company) []common.Symbol {
	symbols := make([]common.Symbol, len(companies))
	for i, company := range companies {
		symbols[i] = company.Symbol
	}

	return symbols
}
