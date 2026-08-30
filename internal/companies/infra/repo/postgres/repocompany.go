package postgres

import (
	"context"
	"errors"
	"log/slog"

	cockroach "github.com/cockroachdb/errors"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	commonpg "github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/repo/postgres/gen"
)

// filterableRelation is what a list request filters and orders over. It is the
// view rather than the table, because it also exposes the resource name, which
// is derived rather than stored.
const filterableRelation = "company.companies_filterable"

// idColumn is the identifying column, and the tie-breaker the keyset cursor
// ends with. A company has no surrogate key: the ticker is the identity.
const idColumn = "symbol"

// CompanyPostgresRepository is a PostgreSQL implementation of
// [companies.Repository].
type CompanyPostgresRepository struct {
	log *slog.Logger
	db  *commonpg.DB
	q   *gen.Queries
}

// NewCompanyPostgresRepository returns a new instance of
// [CompanyPostgresRepository].
func NewCompanyPostgresRepository(log *slog.Logger, db *commonpg.DB) *CompanyPostgresRepository {
	return &CompanyPostgresRepository{
		log: log.WithGroup("companies").WithGroup("pg"),
		db:  db,
		q:   gen.New(db.Pool()),
	}
}

var _ companies.Repository = (*CompanyPostgresRepository)(nil)

// Get returns the company with the given symbol.
func (r *CompanyPostgresRepository) Get(ctx context.Context, symbol common.Symbol) (*companies.Company, error) {
	row, err := r.q.GetCompany(ctx, symbol.String())
	if commonpg.IsNoRows(err) {
		return nil, common.ErrEntityNotFound.
			WithMessageExtension("company " + symbol.String()).
			WithUnderlying(cockroach.WithStack(err))
	}
	if err != nil {
		return nil, cockroach.Wrapf(err, "get company %s", symbol)
	}

	return toCompany(row), nil
}

// Exists reports whether a company with the given symbol is stored.
func (r *CompanyPostgresRepository) Exists(ctx context.Context, symbol common.Symbol) (bool, error) {
	exists, err := r.q.CompanyExists(ctx, symbol.String())
	if err != nil {
		return false, cockroach.Wrapf(err, "check company %s", symbol)
	}

	return exists, nil
}

// Count returns the number of stored companies.
func (r *CompanyPostgresRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.q.CountCompanies(ctx)
	if err != nil {
		return 0, cockroach.Wrap(err, "count companies")
	}

	return count, nil
}

// List returns one page of companies.
func (r *CompanyPostgresRepository) List(
	ctx context.Context,
	qry common.ListQuery,
) (common.ListResult[*companies.Company], error) {
	result, err := commonpg.List(ctx, r.db, qry, filterableRelation,
		func(ctx context.Context, symbols []string) ([]*companies.Company, error) {
			rows, err := r.q.ListCompaniesBySymbols(ctx, symbols)
			if err != nil {
				return nil, cockroach.Wrap(err, "list companies")
			}

			return toCompanies(rows), nil
		},
		commonpg.WithIDColumn(idColumn),
		commonpg.WithTotalSize(),
	)
	if err != nil {
		return common.ListResult[*companies.Company]{}, cockroach.WithStack(err)
	}

	return result, nil
}

// Upsert writes companies, returning how many rows it actually changed.
//
// The whole batch is one round trip, and a company whose details BVB has not
// changed is left alone, so its update time keeps meaning "last changed" rather
// than "last seen".
func (r *CompanyPostgresRepository) Upsert(ctx context.Context, list ...*companies.Company) (int, error) {
	if len(list) == 0 {
		return 0, nil
	}

	params := make([]gen.UpsertCompanyParams, len(list))
	for i, company := range list {
		if err := company.Validate(); err != nil {
			return 0, cockroach.WithStack(err)
		}

		params[i] = gen.UpsertCompanyParams{
			Symbol:      company.Symbol.String(),
			DisplayName: company.DisplayName,
		}
	}

	results := r.q.UpsertCompany(ctx, params)
	defer func() {
		if err := results.Close(); err != nil {
			r.log.ErrorContext(ctx, "closing the upsert batch", slog.Any("err", err))
		}
	}()

	written := 0
	var firstErr error

	results.QueryRow(func(index int, _ gen.CompanyCompany, err error) {
		switch {
		case err == nil:
			written++
		case errors.Is(err, commonpg.ErrNoRows):
			// Nothing about this company changed, so the statement updated no
			// row and returned none.
		case firstErr == nil:
			firstErr = cockroach.Wrapf(err, "upsert company %s", params[index].Symbol)
		}
	})

	if firstErr != nil {
		return 0, firstErr
	}

	return written, nil
}
