package app

import (
	"context"
	"log/slog"

	"go.uber.org/fx"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/scheduler"
)

// newScheduler provides the scheduler, running every job the slices contributed
// through the jobs group.
//
// The jobs run under the application's root context, and stopping cancels them
// and waits for them to unwind, so a shutdown interrupts an import rather than
// killing it mid-write.
func newScheduler(
	ctx context.Context,
	log *slog.Logger,
	jobs []scheduler.Job,
	lc fx.Lifecycle,
) *scheduler.Scheduler {
	s := scheduler.New(log, jobs)

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			s.Start(ctx)

			return nil
		},
		OnStop: func(_ context.Context) error {
			s.Stop()

			return nil
		},
	})

	return s
}
