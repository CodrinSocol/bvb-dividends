package apiserver

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/encoding/protojson"

	dividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/genproto/bvb/dividends/v1"
)

// openAPISpec is the specification generated from the protos by buf.
//
// It is embedded rather than read from disk so a deployed binary always serves
// the contract it was built from, and it is generated rather than written so
// the documentation cannot drift from the service.
//
//go:embed openapi/openapi.yaml
var openAPISpec []byte

// Options configure the HTTP handler.
type Options struct {
	// Logger receives access logs and unexpected failures.
	Logger *slog.Logger

	// AllowedOrigins are the browser origins permitted to call the API.
	AllowedOrigins []string

	// RequestTimeout bounds any single request.
	RequestTimeout time.Duration

	// Ready reports whether the service can serve traffic, for /readyz.
	Ready func(context.Context) error
}

// NewHandler builds the API's HTTP handler.
func NewHandler(ctx context.Context, server *Server, options Options) (http.Handler, error) {
	log := options.Logger
	if log == nil {
		log = slog.Default()
	}
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = 30 * time.Second
	}

	gateway := runtime.NewServeMux(
		runtime.WithErrorHandler(errorHandler(log)),
		runtime.WithRoutingErrorHandler(routingErrorHandler(log)),
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions: protojson.MarshalOptions{
				// lowerCamelCase field names and omitted default values are
				// what proto3 JSON specifies and what every Google API emits.
				UseProtoNames:   false,
				EmitUnpopulated: false,
				Indent:          "",
			},
			UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
		}),
	)

	// Registered against the implementation directly: the gateway calls the
	// service in-process, so the binary serves REST without opening a gRPC
	// port or dialling itself.
	if err := dividendsv1.RegisterDividendsServiceHandlerServer(ctx, gateway, server); err != nil {
		return nil, fmt.Errorf("register the dividends service: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/", gateway)
	mux.HandleFunc("GET /healthz", healthHandler)
	mux.HandleFunc("GET /readyz", readyHandler(options.Ready, log))
	mux.HandleFunc("GET /openapi.yaml", specHandler)
	mux.HandleFunc("GET /docs", docsHandler)
	// Unqualified: a method-qualified catch-all conflicts with /v1/ under
	// the Go 1.22 pattern rules, and any method on an unknown path is a 404.
	mux.HandleFunc("/", notFoundHandler(log))

	return chain(mux,
		withRecovery(log),
		withRequestID,
		withLogging(log),
		withTimeout(options.RequestTimeout),
		withCORS(options.AllowedOrigins),
	), nil
}

// routingErrorHandler renders the gateway's own routing failures — an unknown
// method on a known path, for instance — in the same AIP-193 shape as
// everything else.
func routingErrorHandler(log *slog.Logger) runtime.RoutingErrorHandlerFunc {
	return func(
		ctx context.Context,
		mux *runtime.ServeMux,
		marshaler runtime.Marshaler,
		w http.ResponseWriter,
		r *http.Request,
		httpStatus int,
	) {
		err := runtime.HTTPStatusError{HTTPStatus: httpStatus, Err: fmt.Errorf("%s %s", r.Method, r.URL.Path)}
		errorHandler(log)(ctx, mux, marshaler, w, r, &err)
	}
}

// healthHandler reports that the process is running.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"SERVING"}`))
}

// readyHandler reports whether the service can serve traffic, which for a
// read-only API means it can reach the database.
func readyHandler(ready func(context.Context) error, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if ready == nil {
			_, _ = w.Write([]byte(`{"status":"SERVING"}`))
			return
		}
		if err := ready(r.Context()); err != nil {
			log.WarnContext(r.Context(), "readiness check failed", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"NOT_SERVING"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"SERVING"}`))
	}
}

// specHandler serves the generated OpenAPI specification.
func specHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(openAPISpec)
}

// docsPage renders the specification with Scalar's standalone viewer.
const docsPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>BVB Dividends API</title>
</head>
<body>
<div id="app"></div>
<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
<script>
  Scalar.createApiReference('#app', { url: '/openapi.yaml', theme: 'default' })
</script>
</body>
</html>
`

// docsHandler serves a browsable rendering of the specification.
func docsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsPage))
}
