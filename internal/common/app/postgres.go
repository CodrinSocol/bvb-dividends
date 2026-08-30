package app

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"
	"go.uber.org/fx"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
)

// newDB builds the connection pool from every namespace the slices contributed.
//
// The namespaces arrive as an fx group, so adding a slice adds its schema to
// the migrations without anything here changing.
func newDB(
	ctx context.Context,
	log *slog.Logger,
	namespaces []*postgres.Namespace,
	lc fx.Lifecycle,
) (*postgres.DB, error) {
	db, err := postgres.NewDB(ctx, log, namespaces)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	lc.Append(fx.Hook{
		OnStart: db.Start,
		OnStop:  db.Shutdown,
	})

	return db, nil
}
