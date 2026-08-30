package app

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"github.com/cockroachdb/errors"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/healthcheck"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/logging"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/openapi"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/scheduler"
)

// Interface marks an application interface — a slice's ConnectRPC handler, or
// any other protocol adapter.
//
// Nothing depends on an interface, so fx would never construct one. Binding it
// to this type and adding it to the interfaces group gives it a consumer: the
// invoke below asks for the whole group, which forces every constructor in it
// to run.
//
// Example:
//
//	fx.Provide(fx.Annotate(
//	    NewMyInterface,
//	    fx.As(new(app.Interface)),
//	    fx.ResultTags(`group:"interfaces"`),
//	)),
type Interface any

// fxLogger routes the dependency graph's own events into the application log,
// at debug level.
//
// Left alone, fx narrates every constructor it runs on stdout in a format of
// its own, which is noise beside the structured log the service writes and
// useless in front of a one-shot import. At debug level it is still there when
// a wiring problem needs it.
func fxLogger(log *slog.Logger) fxevent.Logger {
	logger := &fxevent.SlogLogger{Logger: log.WithGroup("fx")}
	logger.UseLogLevel(slog.LevelDebug)

	return logger
}

// core is what every run of the binary needs, whether it serves or not.
func core() []fx.Option {
	return []fx.Option{
		fx.Provide(newContext),
		fx.Provide(logging.NewLogger),
		fx.WithLogger(fxLogger),
		fx.Provide(fx.Annotate(
			newDB,
			fx.ParamTags(``, ``, `group:"pgnamespaces"`),
		)),
	}
}

// Run runs the service: it serves the API and the web application, and runs the
// scheduled jobs, until it is signalled to stop.
func Run(opts ...fx.Option) {
	fx.New(slices.Concat(
		core(),
		[]fx.Option{
			fx.Provide(newHTTPConfig),
			fx.Provide(newLiveness),
			fx.Provide(newReadiness),
			fx.Provide(healthcheck.NewMiddleware),
			fx.Provide(openapi.NewMiddleware),
			fx.Provide(newWebMiddleware),
			fx.Provide(newHTTPServerAndServeMux),
			fx.Provide(newHTTPServer),
			fx.Provide(newHTTPServeMux),
			fx.Provide(newConnectRPCServer),
			fx.Provide(fx.Annotate(
				newScheduler,
				fx.ParamTags(``, ``, `group:"jobs"`),
			)),
			// Nothing depends on the server or the scheduler, so they are asked
			// for explicitly; without this fx would construct neither.
			fx.Invoke(func(_ *http.Server, _ *scheduler.Scheduler) {}),
			fx.Invoke(fx.Annotate(
				func(_ []Interface) {},
				fx.ParamTags(`group:"interfaces"`),
			)),
		},
		opts,
	)...).Run()
}

// Task is a unit of work a one-shot run performs.
type Task func(ctx context.Context) error

// RunTask starts only what a background task needs - configuration, logging and
// the database, with its migrations - runs the task, and shuts down again.
//
// It is how `bvb-dividends import` and `bvb-dividends migrate` run: the same
// wiring as the service, without opening a port, so a one-shot run cannot
// collide with a running instance.
//
// newTask is a constructor for the [Task], taking whatever it depends on. The
// task is built during startup but run after it, because fx runs every invoke
// before any start hook: run from an invoke, an import would reach the database
// before the database had applied its migrations.
func RunTask(ctx context.Context, newTask any, opts ...fx.Option) error {
	var task Task

	application := fx.New(slices.Concat(
		core(),
		[]fx.Option{fx.Provide(newTask), fx.Populate(&task)},
		opts,
	)...)

	if err := application.Start(ctx); err != nil {
		return errors.WithStack(err)
	}

	taskErr := task(ctx)

	if err := application.Stop(ctx); err != nil && taskErr == nil {
		return errors.WithStack(err)
	}

	return taskErr
}
