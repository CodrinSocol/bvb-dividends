package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"go.uber.org/fx"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/scheduler"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	companiesbvb "github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/bvb"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	dividendsbvb "github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/infra/bvb"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// jobName identifies the scheduled import in the log.
const jobName = "bvb-import"

// bvbOptions provides the BVB client both slices read through.
//
// One client, so one connection pool and one retry policy face BVB, and each
// slice depends on the single method of it that it uses rather than on the
// whole thing.
func bvbOptions(cfg BVBConfig) []fx.Option {
	newClient := func(log *slog.Logger) *bvbclient.Client {
		return bvbclient.New(
			bvbclient.WithEndpoint(cfg.Endpoint),
			bvbclient.WithHTTPClient(&http.Client{Timeout: cfg.Timeout}),
			bvbclient.WithLogger(log),
			bvbclient.WithRetry(cfg.Attempts, cfg.Backoff),
		)
	}

	return []fx.Option{
		fx.Provide(newClient),
		fx.Provide(func(client *bvbclient.Client) companiesbvb.Client { return client }),
		fx.Provide(func(client *bvbclient.Client) dividendsbvb.Client { return client }),
	}
}

// importOptions registers the daily import as a scheduled job.
func importOptions(cfg ImportConfig) []fx.Option {
	if !cfg.Enabled {
		return nil
	}

	newJob := func(
		log *slog.Logger,
		companiesSvc *companies.Service,
		dividendsSvc *dividends.Service,
	) (scheduler.Job, error) {
		location, err := time.LoadLocation(cfg.TimeZone)
		if err != nil {
			return scheduler.Job{}, errors.Wrapf(err, "load the %s time zone", cfg.TimeZone)
		}

		return scheduler.Job{
			Name:       jobName,
			Hour:       cfg.Hour,
			Location:   location,
			RunAtStart: cfg.RunAtStart,
			Run: func(ctx context.Context) error {
				return refresh(ctx, log, companiesSvc, dividendsSvc, refreshOptions{
					Concurrency: cfg.Concurrency,
				})
			},
		}, nil
	}

	return []fx.Option{
		fx.Provide(fx.Annotate(newJob, fx.ResultTags(`group:"jobs"`))),
	}
}

// refreshOptions are the choices one import run makes.
type refreshOptions struct {
	// Days is the window of announcements to ask BVB for. Zero lets the
	// companies slice decide the scope:
	// the whole market when nothing has been imported yet, one day otherwise.
	Days int

	// All imports every company BVB knows of rather than only those that
	// announced inside the window.
	All bool

	// DryRun fetches and maps everything but writes nothing.
	DryRun bool

	// Concurrency is how many companies are fetched at once.
	Concurrency int
}

// refresh imports the companies that have announced a dividend, and then every
// dividend each of them has declared.
//
// This is the whole of the orchestration between the two slices: the companies
// import returns the symbols it saw, and the dividends import fetches against
// them. Neither slice knows about the other.
//
// A run in which some companies failed is reported as a failure, so that a cron
// entry which mails on failure says something rather than silently importing a
// fraction of the market night after night.
func refresh(
	ctx context.Context,
	log *slog.Logger,
	companiesSvc *companies.Service,
	dividendsSvc *dividends.Service,
	options refreshOptions,
) error {
	companiesResult, err := companiesSvc.ImportCompanies(ctx, companies.ImportCompaniesCommand{
		Days:   options.Days,
		All:    options.All,
		DryRun: options.DryRun,
	})
	if err != nil {
		return errors.Wrap(err, "import companies")
	}

	dividendsResult, err := dividendsSvc.ImportDividends(ctx, dividends.ImportDividendsCommand{
		Symbols:     companiesResult.Symbols,
		DryRun:      options.DryRun,
		Concurrency: options.Concurrency,
	})
	if err != nil {
		return errors.Wrap(err, "import dividends")
	}

	log.InfoContext(ctx, "import complete",
		slog.Any("companies", companiesResult),
		slog.Any("dividends", dividendsResult))

	if failures := dividendsResult.Err(); failures != nil {
		return errors.Wrapf(failures, "%d of %d companies could not be imported",
			dividendsResult.FailedCompanies(), dividendsResult.CompaniesSeen)
	}

	return nil
}
