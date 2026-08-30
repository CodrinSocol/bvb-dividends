package apiserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	rpccode "google.golang.org/genproto/googleapis/rpc/code"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// errorBody is the AIP-193 error envelope.
//
// grpc-gateway's default handler emits a bare {"code": 5, "message": "..."},
// where the code is a gRPC enum value. AIP-193 and every Google JSON API
// instead nest the error and report the HTTP status code alongside the
// canonical status name, so this replaces the default handler rather than
// wrapping it.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Status  string            `json:"status"`
	Details []json.RawMessage `json:"details,omitempty"`
}

// errorHandler renders a failed call as an AIP-193 error body.
func errorHandler(log *slog.Logger) runtime.ErrorHandlerFunc {
	return func(
		ctx context.Context,
		_ *runtime.ServeMux,
		_ runtime.Marshaler,
		w http.ResponseWriter,
		r *http.Request,
		err error,
	) {
		st := status.Convert(err)
		httpStatus := runtime.HTTPStatusFromCode(st.Code())

		if httpStatus >= http.StatusInternalServerError {
			// The response deliberately says nothing useful about an internal
			// failure, so the log is the only place the detail survives.
			log.ErrorContext(ctx, "request failed",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("code", st.Code().String()),
				slog.String("error", err.Error()))
		}

		body := errorBody{Error: errorDetail{
			Code:    httpStatus,
			Message: st.Message(),
			Status:  canonicalStatus(st.Code()),
			Details: marshalDetails(st),
		}}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			log.ErrorContext(ctx, "could not write the error response", slog.String("error", err.Error()))
		}
	}
}

// canonicalStatus renders a status code as its google.rpc.Code name.
//
// codes.Code's own String method returns Go-style names such as "NotFound",
// whereas AIP-193 and every Google JSON API report the canonical enum name,
// "NOT_FOUND". Callers match on this string, so it has to be the canonical one.
func canonicalStatus(code codes.Code) string {
	if name, ok := rpccode.Code_name[int32(code)]; ok {
		return name
	}
	return rpccode.Code_UNKNOWN.String()
}

// marshalDetails renders any structured details attached to the status.
func marshalDetails(st *status.Status) []json.RawMessage {
	proto := st.Proto()
	if proto == nil || len(proto.GetDetails()) == 0 {
		return nil
	}

	details := make([]json.RawMessage, 0, len(proto.GetDetails()))
	for _, detail := range proto.GetDetails() {
		encoded, err := protojson.Marshal(detail)
		if err != nil {
			continue
		}
		details = append(details, encoded)
	}
	return details
}

// notFoundHandler answers a path the gateway does not route, in the same shape
// as every other error the API returns.
func notFoundHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := status.Errorf(codes.NotFound, "%s is not a path this API serves", r.URL.Path)
		errorHandler(log)(r.Context(), nil, nil, w, r, err)
	}
}
