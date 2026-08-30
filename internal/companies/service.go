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

// importScope is what one import run decided to ask BVB for.
type importScope struct {
	// all takes every company the service knows of.
	all bool

	// window is how many days of announcements to ask for, when not taking the
	// whole market.
	window int
}

// ImportCompanies refreshes the companies this service knows about.
//
// It returns the symbols it saw, which is what the dividends import needs: the
// two slices fetch from BVB separately, and this is the only thing that passes
// between them.
func (svc *Service) ImportCompanies(
	ctx context.Context,
	cmd ImportCompaniesCommand,
) (ImportCompaniesResult, error) {
	started := time.Now()

	scope, err := svc.scope(ctx, cmd)
	if err != nil {
		return ImportCompaniesResult{}, err
	}

	svc.log.InfoContext(ctx, "importing companies from BVB",
		slog.Bool("whole_market", scope.all),
		slog.Int("window_days", scope.window),
		slog.Bool("dry_run", cmd.DryRun))

	list, err := svc.fetch(ctx, scope)
	if err != nil {
		return ImportCompaniesResult{}, err
	}

	result := ImportCompaniesResult{
		Window:  scope.window,
		All:     scope.all,
		Symbols: symbolsOf(list),
	}

	if !cmd.DryRun && len(list) > 0 {
		written, err := svc.repo.Upsert(ctx, list...)
		if err != nil {
			return ImportCompaniesResult{}, errors.Wrap(err, "store companies")
		}

		result.Written = written
	}

	result.Duration = time.Since(started)
	svc.log.InfoContext(ctx, "imported companies from BVB", slog.Any("result", result))

	return result, nil
}

// scope decides what to ask BVB for.
//
// An empty database means nothing has ever been imported, so the first run
// takes the whole market: the dividends import fetches against the symbols this
// one returns, and starting from only the companies that announced inside a
// window would leave the rest of the exchange unasked about indefinitely.
func (svc *Service) scope(ctx context.Context, cmd ImportCompaniesCommand) (importScope, error) {
	if cmd.All {
		return importScope{all: true}, nil
	}
	if cmd.Days > 0 {
		return importScope{window: cmd.Days}, nil
	}

	count, err := svc.repo.Count(ctx)
	if err != nil {
		return importScope{}, errors.Wrap(err, "decide the import scope")
	}
	if count == 0 {
		return importScope{all: true}, nil
	}

	return importScope{window: IncrementalDays}, nil
}

// fetch reads the companies the scope asks for.
func (svc *Service) fetch(ctx context.Context, scope importScope) ([]*Company, error) {
	if scope.all {
		list, err := svc.source.All(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "list every company")
		}

		return list, nil
	}

	list, err := svc.source.RecentlyAnnouncing(ctx, scope.window)
	if err != nil {
		return nil, errors.Wrap(err, "list companies announcing dividends")
	}

	return list, nil
}

func symbolsOf(list []*Company) []common.Symbol {
	symbols := make([]common.Symbol, len(list))
	for i, company := range list {
		symbols[i] = company.Symbol
	}

	return symbols
}
