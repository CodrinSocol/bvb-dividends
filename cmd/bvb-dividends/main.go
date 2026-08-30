// Command bvb-dividends serves the Bucharest Stock Exchange dividends API and
// the web application over it, and keeps them fed from BVB.
//
// Run with no arguments it serves and imports on a daily schedule, which is one
// process to deploy and one thing to operate. The two subcommands exist for the
// cases where that is not what is wanted:
//
//	bvb-dividends                        serve, and import on a schedule
//	bvb-dividends import [flags]         import once and exit
//	bvb-dividends migrate                apply the migrations and exit
//
// `import --dry-run` fetches and maps everything and writes nothing, which is
// how the reconstructed SOAP field names are checked against the live service.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"go.uber.org/fx"

	apispec "github.com/CodrinSocol/bvb-dividends-ro/api/proto/gen"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/app"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/config"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/openapi"
	commonpg "github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	"github.com/CodrinSocol/bvb-dividends-ro/web"
)

func main() {
	cfg, err := config.Load[Config]()
	if err != nil {
		fail(err)
	}

	mode, args := parseMode(os.Args[1:])

	switch mode {
	case "serve":
		serve(cfg)
	case "import":
		if err := runImport(cfg, args); err != nil {
			fail(err)
		}
	case "migrate":
		if err := runMigrate(cfg); err != nil {
			fail(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "bvb-dividends: unknown command %q\n\n", mode)
		usage()
		os.Exit(2)
	}
}

// parseMode reads the subcommand, defaulting to serving.
func parseMode(args []string) (string, []string) {
	if len(args) == 0 || args[0][0] == '-' {
		return "serve", args
	}

	return args[0], args[1:]
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  bvb-dividends                serve the API and the web application, and
                               import from BVB on a daily schedule
  bvb-dividends import [flags] import once and exit
  bvb-dividends migrate        apply the database migrations and exit

import flags:
  -days N        how many days back to ask BVB for; 0 backfills on an empty
                 database and imports one day otherwise
  -dry-run       fetch and map everything, write nothing
  -concurrency N how many companies to fetch at once
`)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "bvb-dividends: %+v\n", err)
	os.Exit(1)
}

// serve runs the service until it is signalled to stop.
func serve(cfg Config) {
	app.Run(slices.Concat(
		[]fx.Option{
			// Provided rather than supplied: fx.Supply would register the value
			// under its concrete type, and what the server asks for is the
			// interface.
			fx.Provide(func() app.WebAssets { return web.FS() }),
			fx.Provide(func() openapi.Spec { return apispec.OpenAPI }),
		},
		bvbOptions(cfg.BVB),
		companiesOptions(),
		dividendsOptions(),
		importOptions(cfg.Import),
	)...)
}

// runImport performs one import and exits.
//
// A run in which some companies failed exits non-zero, so a cron entry that
// mails on failure says something rather than silently importing a fraction of
// the market night after night.
func runImport(cfg Config, args []string) error {
	flags := flag.NewFlagSet("import", flag.ExitOnError)
	days := flags.Int("days", 0,
		"how many days back to import; 0 backfills on an empty database and imports one day otherwise")
	dryRun := flags.Bool("dry-run", false, "fetch and map everything but write nothing")
	concurrency := flags.Int("concurrency", cfg.Import.Concurrency, "how many companies to fetch at once")

	if err := flags.Parse(args); err != nil {
		return err
	}

	options := refreshOptions{Days: *days, DryRun: *dryRun, Concurrency: *concurrency}

	return app.RunTask(context.Background(),
		func(log *slog.Logger, companiesSvc *companies.Service, dividendsSvc *dividends.Service) error {
			return refresh(context.Background(), log, companiesSvc, dividendsSvc, options)
		},
		slices.Concat(bvbOptions(cfg.BVB), companiesOptions(), dividendsOptions())...,
	)
}

// runMigrate applies the migrations and exits, for a release step in a
// deployment that would rather not have them applied on startup.
func runMigrate(cfg Config) error {
	// The command exists for a deployment that turns automatic migration off,
	// so it turns it back on for this process. The database applies the
	// schemas as it starts; doing it a second time here would only apply them
	// twice.
	if err := os.Setenv("POSTGRES_MIGRATIONS_APPLY", "true"); err != nil {
		return err
	}

	return app.RunTask(context.Background(),
		// The hook is appended after the database's own, so it runs after the
		// schemas have been applied rather than before: fx runs an invoke before
		// the lifecycle it registers.
		func(lc fx.Lifecycle, log *slog.Logger, _ *commonpg.DB) {
			lc.Append(fx.Hook{OnStart: func(ctx context.Context) error {
				log.InfoContext(ctx, "migrations applied")

				return nil
			}})
		},
		slices.Concat(bvbOptions(cfg.BVB), companiesOptions(), dividendsOptions())...,
	)
}
