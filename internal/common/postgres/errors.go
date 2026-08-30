package postgres

import (
	stderrors "errors"

	"github.com/cockroachdb/errors"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// AsConstraintViolation wraps err as a [common.ErrEntityConstraintViolation] if
// it is one, so that a broken constraint reaches the caller as an invalid
// argument rather than as an unexplained internal failure.
//
//nolint:revive // Returning (error, bool) mirrors the errors.As shape it wraps.
func AsConstraintViolation(err error) (error, bool) {
	var pgErr *pgconn.PgError
	if stderrors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgerrcode.IntegrityConstraintViolation,
			pgerrcode.RestrictViolation,
			pgerrcode.NotNullViolation,
			pgerrcode.ForeignKeyViolation,
			pgerrcode.UniqueViolation,
			pgerrcode.CheckViolation:
			return common.ErrEntityConstraintViolation.
				WithUnderlying(errors.WithStack(err)).
				WithProblem(pgErr.ColumnName, pgErr.Message), true
		}
	}

	return err, false
}

// IsNoRows reports whether err (or any error it wraps) is pgx's "no rows"
// sentinel.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
