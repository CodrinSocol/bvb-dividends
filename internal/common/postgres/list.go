package postgres

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/observeinc/cel2sql/v3"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/aip"
)

type listOptions struct {
	idColumn        string
	returnTotalSize bool
	clauses         squirrel.And
}

func newDefaultListOptions() *listOptions {
	return &listOptions{
		idColumn:        "id",
		returnTotalSize: false,
		clauses:         make(squirrel.And, 0),
	}
}

// ListOption configures a listing.
type ListOption func(*listOptions)

// WithIDColumn names the identifying column, which is the tie-breaker the
// keyset cursor always ends with. It defaults to "id".
func WithIDColumn(idColumn string) ListOption {
	return func(options *listOptions) { options.idColumn = idColumn }
}

// WithTotalSize asks for the total size of the collection across all pages.
// It costs a second query whose price grows with the table.
func WithTotalSize() ListOption {
	return func(options *listOptions) { options.returnTotalSize = true }
}

// WithClause adds a SQL clause that is not expressible as a caller filter, such
// as restricting a child collection to its parent.
func WithClause(clause squirrel.Sqlizer) ListOption {
	return func(options *listOptions) { options.clauses = append(options.clauses, clause) }
}

// buildCursorWhere turns a decoded cursor into the "everything strictly after
// this row" predicate, over however many columns the ordering has.
//
//nolint:ireturn // squirrel builds predicates out of interface values.
func buildCursorWhere(orderBy common.OrderBy, cursorValues []any) squirrel.Sqlizer {
	or := squirrel.Or{}

	if len(cursorValues) != len(orderBy) {
		return or
	}

	for i, ob := range orderBy {
		and := squirrel.And{}

		// All preceding columns must be strictly equal.
		for j := range i {
			and = append(and, squirrel.Eq{aip.ColumnOf(orderBy[j].Path): cursorValues[j]})
		}

		if ob.Direction == common.OrderDirectionAsc {
			and = append(and, squirrel.Gt{aip.ColumnOf(ob.Path): cursorValues[i]})
		} else {
			and = append(and, squirrel.Lt{aip.ColumnOf(ob.Path): cursorValues[i]})
		}

		or = append(or, and)
	}

	return or
}

type idAndCursorRow[ID any] struct {
	ID     ID
	Cursor string
}

// List applies a [common.ListQuery] to one relation and returns the page it
// selects.
//
// It runs in two steps. The first selects only the identifiers and the cursor
// from the filterable relation, applying the caller's filter, the ordering and
// the keyset predicate; the second hands those identifiers to queryFn, which is
// the slice's own generated query for reading whole rows.
//
// Pagination is keyset rather than offset: the token carries the ordering
// values of the last row on the page, so a page is stable while rows are being
// imported underneath it, and a deep page costs no more than a shallow one.
// The identifying column is always appended to the ordering, so the ordering is
// total and no row can straddle a page boundary.
//
// queryFn must return its rows in the order of the identifiers it was given —
// `ORDER BY array_position(...)` — because `WHERE id = ANY(...)` does not
// preserve one, and the ordering the caller asked for is decided by the first
// query.
//
//nolint:cyclop,funlen // The stages read as one sequence; splitting them would hide the flow.
func List[ID any, ROW any](
	ctx context.Context,
	db *DB,
	qry common.ListQuery,
	relation string,
	queryFn func(context.Context, []ID) ([]ROW, error),
	opts ...ListOption,
) (common.ListResult[ROW], error) {
	var empty common.ListResult[ROW]

	options := newDefaultListOptions()
	for _, optionFn := range opts {
		optionFn(options)
	}

	if !qry.OrderBy.ContainsPath(options.idColumn) {
		qry.OrderBy = append(qry.OrderBy, common.OrderByClause{
			Path:      options.idColumn,
			Direction: common.OrderDirectionAsc,
		})
	}

	base := squirrel.Select().From(relation).Where(options.clauses).PlaceholderFormat(squirrel.Dollar)

	if qry.Filter.NotEmpty() {
		sqlWhere, err := cel2sql.Convert(qry.Filter.Ast, cel2sql.WithContext(ctx), cel2sql.WithLogger(db.log))
		if err != nil {
			return empty, common.ErrFilterInvalid.WithUnderlying(err)
		}

		base = base.Where(sqlWhere)
	}

	var totalSize *uint32
	if options.returnTotalSize {
		countSQL, countArgs, err := base.Column("COUNT(*)").ToSql()
		if err != nil {
			return empty, errors.Wrap(err, "build count SQL")
		}

		count, err := SelectOne(ctx, db, pgx.RowTo[uint32], countSQL, countArgs...)
		if err != nil {
			return empty, errors.Wrap(err, "count query execution failed")
		}

		totalSize = &count
	}

	if qry.PageToken != "" {
		cursorValues, err := decodeCursor(qry.PageToken)
		if err != nil {
			return empty, err
		}

		base = base.Where(buildCursorWhere(qry.OrderBy, cursorValues))
	}

	cursorCols := make([]string, 0, len(qry.OrderBy))
	for _, ob := range qry.OrderBy {
		cursorCols = append(cursorCols, aip.ColumnOf(ob.Path))
	}

	//nolint:gosec // PageSize is bounded by protovalidate and defaulted by the caller.
	qb := base.
		Column(options.idColumn + ", JSONB_BUILD_ARRAY(" + strings.Join(cursorCols, ", ") + ")").
		Limit(uint64(qry.PageSize))
	for _, ob := range qry.OrderBy {
		qb = qb.OrderBy(aip.ColumnOf(ob.Path) + " " + ob.Direction.String())
	}

	sql, args, err := qb.ToSql()
	if err != nil {
		return empty, errors.Wrap(err, "build SQL")
	}

	keys, err := Select(ctx, db, pgx.RowToStructByPos[idAndCursorRow[ID]], sql, args...)
	if err != nil {
		return empty, errors.Wrap(err, "query execution failed")
	}

	ids := make([]ID, len(keys))
	for i, k := range keys {
		ids[i] = k.ID
	}

	var nextPageToken string
	if qry.PageSize > 0 && len(keys) == int(qry.PageSize) {
		nextPageToken = base64.StdEncoding.EncodeToString([]byte(keys[len(keys)-1].Cursor))
	}

	rows, err := queryFn(ctx, ids)
	if err != nil {
		return empty, errors.WithStack(err)
	}

	return common.ListResult[ROW]{Items: rows, NextPageToken: nextPageToken, TotalSize: totalSize}, nil
}

// decodeCursor reads the ordering values the previous page ended on.
func decodeCursor(pageToken string) ([]any, error) {
	decoded, err := base64.StdEncoding.DecodeString(pageToken)
	if err != nil {
		return nil, common.ErrPageTokenInvalid.WithUnderlying(err)
	}

	var cursorValues []any
	if err := json.Unmarshal(decoded, &cursorValues); err != nil {
		return nil, common.ErrPageTokenInvalid.WithUnderlying(err)
	}

	return cursorValues, nil
}
