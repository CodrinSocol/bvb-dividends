// Command importer refreshes companies and dividends from BVB.
//
// It runs once and exits, which suits a cron entry, a Kubernetes CronJob or a
// systemd timer. Passing -schedule instead keeps it resident and runs the
// import daily at noon Bucharest time, which is what the Spring service this
// replaces did from inside the API process.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/app"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/bvbsoap"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/config"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/logging"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/postgres"
)

// dailyImportHour is when a scheduled run starts, in Bucharest time: after the
// market's morning announcements and well before the close.
const dailyImportHour = 12

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "importer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		days        = flag.Int("days", 0, "how many days back to import; 0 backfills on an empty database and imports one day otherwise")
		dryRun      = flag.Bool("dry-run", false, "fetch and map everything but write nothing")
		schedule    = flag.Bool("schedule", false, "stay resident and import daily at noon Europe/Bucharest")
		concurrency = flag.Int("concurrency", app.DefaultConcurrency, "how many companies to fetch at once")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		if err := postgres.Migrate(ctx, pool); err != nil {
			return err
		}
	}

	source := bvbsoap.New(
		bvbsoap.WithEndpoint(cfg.BVB.Endpoint),
		bvbsoap.WithHTTPClient(&http.Client{Timeout: cfg.BVB.Timeout}),
		bvbsoap.WithLogger(log),
		bvbsoap.WithRetry(cfg.BVB.Attempts, 500*time.Millisecond),
	)
	importer := app.NewImporter(
		source,
		postgres.NewCompanyRepository(pool),
		postgres.NewDividendRepository(pool),
		log,
	)
	importer.SetConcurrency(*concurrency)

	options := app.Options{Days: *days, DryRun: *dryRun}
	if *schedule {
		return runScheduled(ctx, importer, options, log)
	}
	return runOnce(ctx, importer, options, log)
}

// runOnce performs a single import and reports whether it was complete.
//
// A run where some companies failed still exits non-zero, so a cron entry that
// mails on failure says something rather than silently importing a fraction of
// the market night after night.
func runOnce(ctx context.Context, importer *app.Importer, options app.Options, log *slog.Logger) error {
	report, err := importer.Run(ctx, options)
	if err != nil {
		return err
	}
	if failures := app.JoinFailures(report); failures != nil {
		return fmt.Errorf("%d of %d companies could not be imported: %w",
			report.FailedCompanies(), report.CompaniesSeen, failures)
	}
	log.Info("import complete", slog.Any("report", report))
	return nil
}

// runScheduled imports once at startup and then daily at noon Bucharest time.
func runScheduled(ctx context.Context, importer *app.Importer, options app.Options, log *slog.Logger) error {
	location, err := time.LoadLocation("Europe/Bucharest")
	if err != nil {
		return fmt.Errorf("load the Bucharest time zone: %w", err)
	}

	for {
		// A failing scheduled run is logged rather than fatal: the next day's
		// run should still happen.
		if err := runOnce(ctx, importer, options, log); err != nil {
			log.Error("scheduled import failed", slog.String("error", err.Error()))
		}

		next := nextRun(time.Now().In(location))
		log.Info("waiting for the next scheduled import", slog.Time("next_run", next))

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Info("stopped")
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// nextRun returns the next daily import time at or after now.
func nextRun(now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), dailyImportHour, 0, 0, 0, now.Location())
	if today.After(now) {
		return today
	}
	return today.AddDate(0, 0, 1)
}
