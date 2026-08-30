package apiserver

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// contextKey is this package's private context key type.
type contextKey int

const requestIDKey contextKey = iota

// requestIDHeader is the header a request ID is read from and echoed in.
const requestIDHeader = "X-Request-Id"

// RequestID returns the request's identifier, or "" outside a request.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// withRequestID gives every request an identifier, honouring one the caller
// supplied so a trace survives across a proxy, and echoes it in the response.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// statusRecorder captures the status code for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// withLogging writes one structured line per request.
func withLogging(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			recorder := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(recorder, r)

			if recorder.status == 0 {
				recorder.status = http.StatusOK
			}
			log.InfoContext(r.Context(), "handled request",
				slog.String("request_id", RequestID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("query", r.URL.RawQuery),
				slog.Int("status", recorder.status),
				slog.Int("bytes", recorder.bytes),
				slog.Duration("duration", time.Since(started)))
		})
	}
}

// withRecovery turns a panic into a 500 rather than a dropped connection, and
// keeps the process alive: one bad request must not take the service down.
func withRecovery(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The recovery path deliberately ignores the request context: it
			// has to write a response even when that context is already
			// cancelled, which is often why the handler panicked.
			//nolint:contextcheck // See above.
			defer func() {
				if recovered := recover(); recovered != nil {
					log.ErrorContext(r.Context(), "recovered from a panic",
						slog.String("request_id", RequestID(r.Context())),
						slog.String("path", r.URL.Path),
						slog.Any("panic", recovered))
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(
						`{"error":{"code":500,"message":"the request could not be completed","status":"INTERNAL"}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// withTimeout bounds how long any single request may run.
func withTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// withCORS allows the browser client to call the API from its own origin.
//
// Origins are matched against an explicit list rather than reflected, so
// enabling the browser client does not open the API to every site. The API is
// public and unauthenticated and sends no credentials, so this is about
// keeping the policy honest rather than about protecting a session.
func withCORS(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+requestIDHeader)
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// chain applies middleware so that the first listed is the outermost.
func chain(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		handler = middleware[i](handler)
	}
	return handler
}
