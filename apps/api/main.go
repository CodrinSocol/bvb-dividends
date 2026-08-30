// Command api serves the BVB dividends API over HTTP.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/apiserver"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/config"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/logging"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "api: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format)
	slog.SetDefault(log)

	// Signals cancel the root context, which unwinds everything below it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// `api migrate` applies migrations and exits, for use as a release step
	// where applying them on startup is not wanted.
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		if err := postgres.Migrate(ctx, pool); err != nil {
			return err
		}
		log.Info("migrations applied")
		return nil
	}

	if cfg.AutoMigrate {
		if err := postgres.Migrate(ctx, pool); err != nil {
			return err
		}
	}

	server, err := apiserver.NewServer(
		postgres.NewCompanyRepository(pool),
		postgres.NewDividendRepository(pool),
	)
	if err != nil {
		return err
	}

	handler, err := apiserver.NewHandler(ctx, server, apiserver.Options{
		Logger:         log,
		AllowedOrigins: cfg.API.AllowedOrigins,
		RequestTimeout: cfg.API.RequestTimeout,
		Ready:          pool.Ping,
	})
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              cfg.API.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.API.ReadHeaderTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("serving the dividends API",
			slog.String("addr", cfg.API.Addr),
			slog.Any("allowed_origins", cfg.API.AllowedOrigins))
		serverErrors <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil

	case <-ctx.Done():
		log.Info("shutting down", slog.Duration("grace_period", cfg.API.ShutdownGracePeriod))

		// A fresh context: the root one is already cancelled, and in-flight
		// requests are given their grace period to finish.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.API.ShutdownGracePeriod)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down cleanly: %w", err)
		}
		log.Info("stopped")
		return nil
	}
}
