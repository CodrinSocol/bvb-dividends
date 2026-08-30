package connectrpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// errInternal is what a caller is told about a failure that was not foreseen.
//
// The API is public and unauthenticated, so an unexpected failure must not leak
// its internals; the detail survives in the log, with the stack the error
// carries.
var errInternal = errors.New("internal server error")

func errorCodeToConnectCode(code common.ErrorCode) connect.Code {
	switch code {
	case common.ErrorCodeAlreadyExists:
		return connect.CodeAlreadyExists
	case common.ErrorCodeInvalidArgument:
		return connect.CodeInvalidArgument
	case common.ErrorCodeNotFound:
		return connect.CodeNotFound
	case common.ErrorCodeUnavailable:
		return connect.CodeUnavailable
	default:
		return connect.CodeInternal
	}
}

// newErrorInterceptor translates a [common.Error] into a Connect status.
//
// This is the single place that decides what a failure looks like to a caller,
// so a slice returns a domain error and never builds a transport status by
// hand.
func newErrorInterceptor(log *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				return resp, nil
			}

			var cerr *connect.Error
			if errors.As(err, &cerr) {
				// Already a transport error: protovalidate and the
				// interceptors above produce these.
				return nil, err
			}

			var domainErr *common.Error
			if !errors.As(err, &domainErr) {
				log.ErrorContext(ctx, "internal server error",
					slog.String("procedure", req.Spec().Procedure),
					slog.Any("err", err))

				return nil, connect.NewError(connect.CodeInternal, errInternal)
			}

			code := errorCodeToConnectCode(domainErr.Code())
			if code == connect.CodeInternal {
				log.ErrorContext(ctx, domainErr.Message(),
					slog.String("procedure", req.Spec().Procedure),
					slog.Any("err", domainErr))

				return nil, connect.NewError(connect.CodeInternal, errInternal)
			}

			log.DebugContext(ctx, domainErr.Message(),
				slog.String("procedure", req.Spec().Procedure),
				slog.Any("err", domainErr))

			return nil, connect.NewError(code, domainErr)
		}
	}
}
