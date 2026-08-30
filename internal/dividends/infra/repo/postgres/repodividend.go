package postgres

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Masterminds/squirrel"
	cockroach "github.com/cockroachdb/errors"
	"github.com/google/uuid"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	commonpg "github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/infra/repo/postgres/gen"
)

// filterableRelation is what a read, a filter and an ordering all go through.
const filterableRelation = "dividend.dividends_filterable"

// DividendPostgresRepository is a PostgreSQL implementation of
// [dividends.Repository].
type DividendPostgresRepository struct {
	log *slog.Logger
	db  *commonpg.DB
	q   *gen.Queries
}

// NewDividendPostgresRepository returns a new instance of
// [DividendPostgresRepository].
func NewDividendPostgresRepository(log *slog.Logger, db *commonpg.DB) *DividendPostgresRepository {
	return &DividendPostgresRepository{
		log: log.WithGroup("dividends").WithGroup("pg"),
		db:  db,
		q:   gen.New(db.Pool()),
	}
}

var _ dividends.Repository = (*DividendPostgresRepository)(nil)

// Get returns one dividend.
//
// The identifier alone locates it, so the company is checked rather than used:
// a resource name pairing a real dividend with the wrong company would
// otherwise resolve, and a name has to mean exactly one thing.
func (r *DividendPostgresRepository) Get(
	ctx context.Context,
	company *common.Symbol,
	id dividends.ID,
) (*dividends.Dividend, error) {
	row, err := r.q.GetDividend(ctx, id.UUID())
	if commonpg.IsNoRows(err) {
		return nil, notFound(id)
	}
	if err != nil {
		return nil, cockroach.Wrapf(err, "get dividend %s", id)
	}

	dividend := toDividend(row)
	if company != nil && dividend.Company != *company {
		return nil, notFound(id)
	}

	return dividend, nil
}

// List returns one page of dividends, restricted to one company unless company
// is nil, which is the AIP-159 wildcard for every company.
func (r *DividendPostgresRepository) List(
	ctx context.Context,
	company *common.Symbol,
	qry common.ListQuery,
) (common.ListResult[*dividends.Dividend], error) {
	options := []commonpg.ListOption{commonpg.WithTotalSize()}
	if company != nil {
		// The parent is not a filter the caller wrote, so it is applied as a
		// clause rather than being folded into the filter expression.
		options = append(options, commonpg.WithClause(squirrel.Eq{"company_symbol": company.String()}))
	}

	result, err := commonpg.List(ctx, r.db, qry, filterableRelation,
		func(ctx context.Context, ids []uuid.UUID) ([]*dividends.Dividend, error) {
			rows, err := r.q.ListDividendsByIDs(ctx, ids)
			if err != nil {
				return nil, cockroach.Wrap(err, "list dividends")
			}

			return toDividends(rows), nil
		},
		options...,
	)
	if err != nil {
		return common.ListResult[*dividends.Dividend]{}, cockroach.WithStack(err)
	}

	return result, nil
}

// Upsert writes dividends, returning how many rows it actually changed.
//
// The whole batch is one round trip, and a dividend BVB has not changed is left
// alone. Each company's dividends are written independently of every other
// company's, so an interruption costs at most the company in flight; the
// previous importer wrapped the whole run in one transaction and deleted a
// company's dividends before re-inserting them, so an interruption could
// leave a company with none at all.
func (r *DividendPostgresRepository) Upsert(ctx context.Context, list ...*dividends.Dividend) (int, error) {
	if len(list) == 0 {
		return 0, nil
	}

	params := make([]gen.UpsertDividendParams, len(list))
	for i, dividend := range list {
		if err := dividend.Validate(); err != nil {
			return 0, cockroach.WithStack(err)
		}

		params[i] = toUpsertParams(dividend)
	}

	results := r.q.UpsertDividend(ctx, params)
	defer func() {
		if err := results.Close(); err != nil {
			r.log.ErrorContext(ctx, "closing the upsert batch", slog.Any("err", err))
		}
	}()

	written := 0

	var firstErr error

	results.QueryRow(func(index int, _ uuid.UUID, err error) {
		switch {
		case err == nil:
			written++
		case errors.Is(err, commonpg.ErrNoRows):
			// Nothing about this dividend changed, so the statement updated no
			// row and returned none.
		case firstErr == nil:
			if wrapped, isConstraint := commonpg.AsConstraintViolation(err); isConstraint {
				firstErr = wrapped

				return
			}

			firstErr = cockroach.Wrapf(err, "upsert dividend %s", params[index].ID)
		}
	})

	if firstErr != nil {
		return 0, firstErr
	}

	return written, nil
}

// notFound is what both a missing dividend and one asked for under the wrong
// company look like to a caller: whether a dividend exists somewhere else is
// not something a wrong resource name should reveal.
func notFound(id dividends.ID) error {
	return common.ErrEntityNotFound.WithMessageExtension("dividend " + id.String())
}
