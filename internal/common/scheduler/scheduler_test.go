package scheduler_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/scheduler"
)

const importHour = 12

func TestNextRun(t *testing.T) {
	t.Parallel()

	bucharest := time.FixedZone("EEST", 3*60*60)

	cases := map[string]struct {
		now  time.Time
		want time.Time
	}{
		"before the hour runs today": {
			now:  time.Date(2026, time.April, 17, 9, 30, 0, 0, bucharest),
			want: time.Date(2026, time.April, 17, importHour, 0, 0, 0, bucharest),
		},
		"after the hour runs tomorrow": {
			now:  time.Date(2026, time.April, 17, 12, 30, 0, 0, bucharest),
			want: time.Date(2026, time.April, 18, importHour, 0, 0, 0, bucharest),
		},
		// Exactly on the hour has already been served by the run that fired,
		// so the next one is tomorrow rather than immediately again.
		"on the hour runs tomorrow": {
			now:  time.Date(2026, time.April, 17, importHour, 0, 0, 0, bucharest),
			want: time.Date(2026, time.April, 18, importHour, 0, 0, 0, bucharest),
		},
		"across a month boundary": {
			now:  time.Date(2026, time.April, 30, 23, 0, 0, 0, bucharest),
			want: time.Date(2026, time.May, 1, importHour, 0, 0, 0, bucharest),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := scheduler.NextRun(tc.now, importHour); !got.Equal(tc.want) {
				t.Errorf("NextRun(%v) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}

// A job that fails must not stop the scheduler, or one bad night would end the
// daily import until somebody noticed.
func TestAFailingRunDoesNotStopTheScheduler(t *testing.T) {
	t.Parallel()

	runs := make(chan struct{}, 1)
	s := scheduler.New(slog.New(slog.NewTextHandler(io.Discard, nil)), []scheduler.Job{{
		Name:       "failing",
		Hour:       importHour,
		RunAtStart: true,
		Run: func(context.Context) error {
			runs <- struct{}{}

			return context.DeadlineExceeded
		},
	}})

	s.Start(t.Context())
	defer s.Stop()

	select {
	case <-runs:
	case <-time.After(time.Second):
		t.Fatal("the job was never run")
	}
}

// Stopping must return, rather than waiting for the next scheduled run.
func TestStopUnwindsAWaitingJob(t *testing.T) {
	t.Parallel()

	s := scheduler.New(slog.New(slog.NewTextHandler(io.Discard, nil)), []scheduler.Job{{
		Name: "waiting",
		Hour: importHour,
		Run:  func(context.Context) error { return nil },
	}})

	s.Start(t.Context())

	stopped := make(chan struct{})
	go func() {
		s.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return")
	}
}
