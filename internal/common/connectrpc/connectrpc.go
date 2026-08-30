// Package connectrpc contains utilities for working with ConnectRPC.
package connectrpc

import (
	"context"
	"log/slog"
	"net/http"
	"slices"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"connectrpc.com/vanguard"
	"github.com/cockroachdb/errors"
)

// PathPrefix is where the transcoder is mounted.
//
// It matches the path-prefix the OpenAPI document is generated with, so the
// specification describes the URLs the service actually serves, and it keeps
// the API under one prefix so that everything outside it can be given to the
// web application.
const PathPrefix = "/api"

// Server collects the ConnectRPC services an application exposes and mounts
// them, once, on a single HTTP handler.
type Server struct {
	mux            *http.ServeMux
	log            *slog.Logger
	services       []*vanguard.Service
	handlerOptions []connect.HandlerOption
}

// NewServer creates a new instance of [Server].
func NewServer(log *slog.Logger, mux *http.ServeMux, handlerOptions ...connect.HandlerOption) (*Server, error) {
	handlerOptionsDefault := []connect.HandlerOption{
		connect.WithRecover(newRecoverHandler(log)),
		connect.WithInterceptors(
			// protovalidate rejects a request that breaks the constraints the
			// protos declare before any handler sees it.
			validate.NewInterceptor(),
			newErrorInterceptor(log),
		),
	}

	return &Server{
		log:            log,
		mux:            mux,
		services:       make([]*vanguard.Service, 0),
		handlerOptions: slices.Concat(handlerOptionsDefault, handlerOptions),
	}, nil
}

// RegisterService registers a ConnectRPC service on the [Server].
//
// Every slice registers its own interface here, which is what lets two services
// share one HTTP handler, one set of interceptors and one transcoder without
// any of them knowing about the others.
func RegisterService[T any](
	srv *Server,
	registerFn func(svc T, opts ...connect.HandlerOption) (string, http.Handler),
	svc T,
	opts ...connect.HandlerOption,
) {
	allHandlerOptions := slices.Concat(srv.handlerOptions, opts)
	path, handler := registerFn(svc, allHandlerOptions...)
	srv.services = append(srv.services, vanguard.NewService(path, handler))
}

// HasServices returns true if any services were registered on this [Server].
func (s *Server) HasServices() bool { return len(s.services) > 0 }

// Start mounts every registered service behind a transcoder.
//
// The transcoder is what makes one set of handlers answer both protocols: a
// browser client speaks Connect, while curl and the OpenAPI document use the
// REST mapping the google.api.http annotations declare. Neither is a second
// implementation of the other.
func (s *Server) Start(ctx context.Context) error {
	s.log.InfoContext(ctx, "mounting the ConnectRPC services", slog.String("prefix", PathPrefix))

	transcoder, err := vanguard.NewTranscoder(s.services)
	if err != nil {
		return errors.Wrap(err, "create vanguard transcoder")
	}

	s.mux.Handle(PathPrefix+"/", http.StripPrefix(PathPrefix, transcoder))

	return nil
}
