package app

import (
	"context"

	"go.uber.org/fx"
)

// newContext provides the application's root context.
//
// It is cancelled when the application stops, which is what tells the scheduler
// and any import in flight to unwind rather than being killed mid-write.
func newContext(lc fx.Lifecycle) context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			cancel()

			return nil
		},
	})

	return ctx
}
