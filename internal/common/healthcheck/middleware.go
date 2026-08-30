// Package healthcheck contains utilities for working with HTTP-based health
// checks.
package healthcheck

import (
	"net/http"
)

// The paths the checks answer on.
const (
	healthzPath   = "/healthz"
	livenessPath  = healthzPath + "/live"
	readinessPath = healthzPath + "/ready"
)

// Middleware is a health check middleware.
type Middleware func(http.Handler) http.Handler

// Liveness reports whether the process is running.
type Liveness interface {
	Live() bool
}

// LivenessFunc is a liveness checker function.
type LivenessFunc func() bool

// Live implements [Liveness].
func (fn LivenessFunc) Live() bool { return fn() }

// Readiness reports whether the process can serve traffic.
type Readiness interface {
	Ready() bool
}

// ReadinessFunc is a readiness checker function.
type ReadinessFunc func() bool

// Ready implements [Readiness].
func (fn ReadinessFunc) Ready() bool { return fn() }

// NewMiddleware creates a liveness and readiness [Middleware].
//
// It runs before everything else, so that a probe is answered even when the
// handlers below it are not able to serve, and so that an orchestrator can tell
// "not ready yet" from "not running".
func NewMiddleware(liveness Liveness, readiness Readiness) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				next.ServeHTTP(w, r)

				return
			}

			var healthy bool
			switch r.URL.Path {
			case livenessPath:
				healthy = liveness.Live()
			case readinessPath:
				healthy = readiness.Ready()
			default:
				next.ServeHTTP(w, r)

				return
			}

			status, body := http.StatusOK, "ok"
			if !healthy {
				status, body = http.StatusServiceUnavailable, "nok"
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		})
	}
}
