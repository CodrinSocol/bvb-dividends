//nolint:testpackage // newHTTPHandler is unexported, so the middleware ordering is only testable from within.
package app

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/healthcheck"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/openapi"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/web"
)

const (
	specBody  = "openapi: 3.1.0\n"
	indexBody = "<!doctype html><title>BVB Dividends</title>"
	apiBody   = "api response"
)

// newTestHTTPHandler assembles the real application middlewares the same way
// the HTTP server does, over a mux that answers one API path.
func newTestHTTPHandler(t *testing.T) http.Handler {
	t.Helper()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	assets := fstest.MapFS{
		"index.html":     {Data: []byte(indexBody)},
		"assets/app.css": {Data: []byte("body{}")},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/companies", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(apiBody))
	})

	return newHTTPHandler(
		mux,
		healthcheck.NewMiddleware(
			healthcheck.LivenessFunc(func() bool { return true }),
			healthcheck.ReadinessFunc(func() bool { return false }),
		),
		openapi.NewMiddleware(openapi.Spec(specBody)),
		web.NewMiddleware(fs.FS(assets), log, "/api/"),
	)
}

// The web application is the last middleware, so it claims every path the API,
// the documentation and the health checks did not — and an unknown path is a
// client-side route, not a 404.
func TestHTTPHandlerRouting(t *testing.T) {
	t.Parallel()

	handler := newTestHTTPHandler(t)

	cases := map[string]struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		"liveness":              {"/healthz/live", http.StatusOK, "ok"},
		"readiness when not ok": {"/healthz/ready", http.StatusServiceUnavailable, "nok"},
		"the specification":     {openapi.SpecPath, http.StatusOK, specBody},
		"an API method":         {"/api/v1/companies", http.StatusOK, apiBody},
		"the application":       {"/", http.StatusOK, indexBody},
		"a client-side route":   {"/companies/SNP", http.StatusOK, indexBody},
		"a built asset":         {"/assets/app.css", http.StatusOK, "body{}"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.path, nil))

			if recorder.Code != tc.wantStatus {
				t.Errorf("GET %s: status = %d, want %d", tc.path, recorder.Code, tc.wantStatus)
			}
			if got := recorder.Body.String(); got != tc.wantBody {
				t.Errorf("GET %s: body = %q, want %q", tc.path, got, tc.wantBody)
			}
		})
	}
}

// Without a web build the binary must still serve its API, rather than failing
// to start or answering everything with an empty page.
func TestTheAPIIsServedWithoutAWebBuild(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/companies", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(apiBody))
	})

	handler := newHTTPHandler(
		mux,
		healthcheck.NewMiddleware(
			healthcheck.LivenessFunc(func() bool { return true }),
			healthcheck.ReadinessFunc(func() bool { return true }),
		),
		openapi.NewMiddleware(openapi.Spec(specBody)),
		web.NewMiddleware(fstest.MapFS{}, log, "/api/"),
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/companies", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != apiBody {
		t.Errorf("the API answered %d %q without a web build", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("a page request answered %d, want 503 explaining the missing build", recorder.Code)
	}
}
