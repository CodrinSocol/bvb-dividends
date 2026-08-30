package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"time"

	"go.uber.org/fx"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/config"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/connectrpc"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/healthcheck"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/openapi"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/web"
)

const httpReadHeaderTimeout = 30 * time.Second

// HTTPConfig contains HTTP-related application config.
type HTTPConfig struct {
	Host string `env:"HTTP_SERVER_HOST" envDefault:""`
	Port string `env:"HTTP_SERVER_PORT" envDefault:"8080" validate:"required"`
}

func newHTTPConfig() (HTTPConfig, error) {
	cfg, err := config.Load[HTTPConfig]()
	if err != nil {
		return cfg, fmt.Errorf("create http server: %w", err)
	}

	return cfg, nil
}

// WebAssets is the built single-page application, as embedded in the binary.
//
// It is supplied by the composition root rather than read here, because
// the embed directive cannot reach outside the package it is written in.
type WebAssets fs.FS

func newWebMiddleware(assets WebAssets, log *slog.Logger) web.Middleware {
	return web.NewMiddleware(assets, log, connectrpc.PathPrefix+"/")
}

// newHTTPHandler wraps the given handler in the application middlewares.
//
// They are listed in the order in which they see a request. The health checks
// come first so a probe is answered whatever else is wrong; the API
// documentation next; and the web application last, because it is the fallback
// that claims every path the API did not.
func newHTTPHandler(
	next http.Handler,
	healthCheckMiddleware healthcheck.Middleware,
	openAPIMiddleware openapi.Middleware,
	webMiddleware web.Middleware,
) http.Handler {
	middlewares := []func(http.Handler) http.Handler{
		healthCheckMiddleware,
		openAPIMiddleware,
		webMiddleware,
	}

	handler := next
	for _, middleware := range slices.Backward(middlewares) {
		handler = middleware(handler)
	}

	return handler
}

// httpServerAndServeMux keeps the server and its mux together, so that fx can
// hand the mux to the ConnectRPC server and the server to the lifecycle without
// constructing either of them twice.
type httpServerAndServeMux struct {
	srv *http.Server
	mux *http.ServeMux
}

func newHTTPServerAndServeMux(
	cfg HTTPConfig,
	log *slog.Logger,
	healthCheckMiddleware healthcheck.Middleware,
	openAPIMiddleware openapi.Middleware,
	webMiddleware web.Middleware,
	lc fx.Lifecycle,
) *httpServerAndServeMux {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           newHTTPHandler(mux, healthCheckMiddleware, openAPIMiddleware, webMiddleware),
		ReadHeaderTimeout: httpReadHeaderTimeout,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Listen before returning, so that a port already in use is a
			// startup failure rather than a log line nobody reads.
			var listenConfig net.ListenConfig

			listener, err := listenConfig.Listen(ctx, "tcp", srv.Addr)
			if err != nil {
				return fmt.Errorf("listen on %s: %w", srv.Addr, err)
			}

			log.InfoContext(ctx, "serving", slog.String("addr", listener.Addr().String()))

			go func() {
				if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.ErrorContext(ctx, "HTTP server failed", slog.Any("err", err))
				}
			}()

			return nil
		},
		OnStop: srv.Shutdown,
	})

	return &httpServerAndServeMux{srv: srv, mux: mux}
}

func newHTTPServer(srvAndMux *httpServerAndServeMux) *http.Server { return srvAndMux.srv }

func newHTTPServeMux(srvAndMux *httpServerAndServeMux) *http.ServeMux { return srvAndMux.mux }

// readinessTimeout bounds the probe, so a database that has stopped answering
// reports as not ready rather than leaving the probe hanging.
const readinessTimeout = 2 * time.Second

// newLiveness reports that the process is running. Anything more would make a
// liveness probe restart the process for a fault a restart cannot fix.
func newLiveness() healthcheck.Liveness {
	return healthcheck.LivenessFunc(func() bool { return true })
}

// newReadiness reports whether the service can serve traffic, which for a
// read-only API means it can reach the database.
func newReadiness(ctx context.Context, db *postgres.DB) healthcheck.Readiness {
	return healthcheck.ReadinessFunc(func() bool {
		probeCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
		defer cancel()

		return db.Ping(probeCtx) == nil
	})
}
