// Package scheduler runs jobs on a daily schedule inside the service process.
//
// The Spring service this project replaces ran its import from inside the API
// process on a fixed daily schedule, and this keeps that: one deployable, one
// thing to operate. A job is still an ordinary function, so the same work can
// be run once from the command line without the scheduler being involved.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is one unit of scheduled work.
type Job struct {
	// Name identifies the job in the log.
	Name string

	// Hour is the hour of the day, in Location, at which the job runs.
	Hour int

	// Location is the time zone Hour is interpreted in. A nil Location means
	// UTC.
	Location *time.Location

	// RunAtStart runs the job once when the scheduler starts, rather than
	// waiting for the first scheduled time.
	RunAtStart bool

	// Run does the work. It is expected to respect the context, which is
	// cancelled when the service shuts down.
	Run func(ctx context.Context) error
}

// Scheduler runs its jobs until it is stopped.
type Scheduler struct {
	log    *slog.Logger
	jobs   []Job
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New returns a Scheduler that will run the given jobs.
func New(log *slog.Logger, jobs []Job) *Scheduler {
	return &Scheduler{log: log.WithGroup("scheduler"), jobs: jobs}
}

// Start launches every job in its own goroutine and returns immediately.
//
// The jobs run under a context derived from ctx, so they stop either when the
// application shuts down or when [Scheduler.Stop] is called, whichever comes
// first.
func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	for _, job := range s.jobs {
		s.wg.Add(1)

		go func() {
			defer s.wg.Done()
			s.run(ctx, job)
		}()
	}
}

// Stop asks every job to stop and waits for the goroutines to unwind.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}

	s.wg.Wait()
}

// run executes one job on its schedule.
//
// A failing run is logged rather than fatal: the next day's run should still
// happen, and a service that stopped importing must not also stop serving what
// it has already imported.
func (s *Scheduler) run(ctx context.Context, job Job) {
	location := job.Location
	if location == nil {
		location = time.UTC
	}

	if job.RunAtStart {
		s.runOnce(ctx, job)
	}

	for {
		next := NextRun(time.Now().In(location), job.Hour)
		s.log.InfoContext(ctx, "waiting for the next scheduled run",
			slog.String("job", job.Name),
			slog.Time("next_run", next))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			s.log.InfoContext(ctx, "scheduled job stopped", slog.String("job", job.Name))

			return
		case <-timer.C:
		}

		s.runOnce(ctx, job)
	}
}

func (s *Scheduler) runOnce(ctx context.Context, job Job) {
	started := time.Now()
	s.log.InfoContext(ctx, "running a scheduled job", slog.String("job", job.Name))

	if err := job.Run(ctx); err != nil {
		s.log.ErrorContext(ctx, "a scheduled job failed",
			slog.String("job", job.Name),
			slog.Duration("duration", time.Since(started)),
			slog.Any("err", err))

		return
	}

	s.log.InfoContext(ctx, "a scheduled job finished",
		slog.String("job", job.Name),
		slog.Duration("duration", time.Since(started)))
}

// NextRun returns the next time at or after now at which the given hour of the
// day falls, in now's own location.
func NextRun(now time.Time, hour int) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if today.After(now) {
		return today
	}

	return today.AddDate(0, 0, 1)
}
