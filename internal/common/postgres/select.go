package postgres

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
)

// Select queries and scans a slice of T in one call.
func Select[T any](
	ctx context.Context,
	db *DB,
	scanner pgx.RowToFunc[T],
	query string,
	args ...any,
) ([]T, error) {
	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "executing query")
	}

	result, err := pgx.CollectRows(rows, scanner)
	if err != nil {
		return nil, errors.Wrap(err, "collecting rows")
	}

	return result, nil
}

// SelectOne queries and scans a single T in one call.
//
//nolint:ireturn // The scanned row type is the caller's, not an abstraction.
func SelectOne[T any](
	ctx context.Context,
	db *DB,
	scanner pgx.RowToFunc[T],
	query string,
	args ...any,
) (T, error) {
	var zero T

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return zero, errors.Wrap(err, "executing query")
	}

	result, err := pgx.CollectOneRow(rows, scanner)
	if err != nil {
		return zero, errors.Wrap(err, "collecting row")
	}

	return result, nil
}
