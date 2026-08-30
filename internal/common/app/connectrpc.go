package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"go.uber.org/fx"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/aip"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/connectrpc"
)

// newConnectRPCServer provides the ConnectRPC server the slices register on.
//
// The transcoder can only be built once every service is known, so mounting it
// happens on start, after the interfaces group has been constructed.
func newConnectRPCServer(log *slog.Logger, mux *http.ServeMux, lc fx.Lifecycle) (*connectrpc.Server, error) {
	srv, err := connectrpc.NewServer(log, mux, connect.WithInterceptors(
		aip.NewPaginationInterceptor(),
	))
	if err != nil {
		return nil, fmt.Errorf("create connectrpc server: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if !srv.HasServices() {
				return nil
			}

			return srv.Start(ctx)
		},
	})

	return srv, nil
}
