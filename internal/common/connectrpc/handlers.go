package connectrpc

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"

	"connectrpc.com/connect"
)

const (
	programCountersBuffSize = 32
	skipCallersSizeDefault  = 2
	skipCallersSizeHandler  = 3
)

// cleanStack renders the stack of the goroutine that panicked, without the
// frames belonging to the recovery itself.
func cleanStack(skip int) string {
	pc := make([]uintptr, programCountersBuffSize)

	n := runtime.Callers(skip+skipCallersSizeDefault, pc)
	if n == 0 {
		return "No stack available"
	}

	frames := runtime.CallersFrames(pc[:n])

	var sb strings.Builder
	for {
		frame, more := frames.Next()
		fmt.Fprintf(&sb, "%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line)

		if !more {
			break
		}
	}

	return sb.String()
}

// newRecoverHandler turns a panic in one RPC into a failure of that RPC.
//
// The service is a single process that also runs the scheduled import, so a
// panic in a handler must not take the process down with it.
func newRecoverHandler(log *slog.Logger) func(context.Context, connect.Spec, http.Header, any) error {
	return func(ctx context.Context, spec connect.Spec, _ http.Header, err any) error {
		log.ErrorContext(ctx, "panic recovered in RPC handler",
			slog.String("rpc.procedure", spec.Procedure),
			slog.Any("rpc.stream_type", spec.StreamType),
			slog.Any("error", err),
			slog.String("stack_trace", cleanStack(skipCallersSizeHandler)))

		return connect.NewError(connect.CodeInternal, errInternal)
	}
}
