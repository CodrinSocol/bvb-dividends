package postgres

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
)

// RunInTx runs fn inside a transaction, committing it when fn returns without
// an error and rolling it back otherwise.
//
//nolint:ireturn // R is the caller's own result type.
func RunInTx[R any](ctx context.Context, db *DB, fn func(pgx.Tx) (R, error)) (R, error) {
	var zero R

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return zero, errors.WithStack(err)
	}
	defer func() {
		// WithoutCancel so that a cancelled request still releases the
		// connection rather than leaving the transaction open until it times
		// out.
		err := tx.Rollback(context.WithoutCancel(ctx))
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			db.log.ErrorContext(ctx, "rollback failed", slog.Any("err", err))
		}
	}()

	r, err := fn(tx)
	if err != nil {
		return zero, errors.WithStack(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return zero, errors.WithStack(err)
	}

	return r, nil
}
