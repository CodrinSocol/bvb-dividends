package aip

import (
	"context"

	"connectrpc.com/connect"
	"github.com/cockroachdb/errors"
	"go.einride.tech/aip/pagination"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

var (
	errRequestMessageNotProto  = errors.New("request message not a proto message")
	errResponseMessageNotProto = errors.New("response message not a proto message")
)

// pageTokenEnvelope wraps the repository's own cursor with a checksum of the
// request that produced it.
type pageTokenEnvelope struct {
	RequestChecksum uint32
	PageToken       string
}

type paginationResponse interface {
	GetNextPageToken() string
}

func processPaginationRequest(req connect.AnyRequest, preq pagination.Request) (uint32, error) {
	givenPageToken := preq.GetPageToken()

	checksum, err := pagination.CalculateRequestChecksum(preq)
	if err != nil {
		return 0, common.ErrPageTokenInvalid.WithUnderlying(err)
	}

	var pageToken string
	if givenPageToken != "" {
		var pte pageTokenEnvelope
		if err := pagination.DecodePageTokenStruct(givenPageToken, &pte); err != nil {
			return 0, common.ErrPageTokenInvalid.WithUnderlying(err)
		}

		if checksum != pte.RequestChecksum {
			return 0, common.ErrPaginationQueryModified
		}

		pageToken = pte.PageToken
	}

	reqProtoMsg, ok := req.Any().(protoreflect.ProtoMessage)
	if !ok {
		return 0, errors.WithStack(errRequestMessageNotProto)
	}

	// The unwrapped cursor replaces the envelope, so the service and the
	// repository below it only ever see their own token.
	reqMsg := reqProtoMsg.ProtoReflect()
	if fd := reqMsg.Descriptor().Fields().ByName("page_token"); fd != nil {
		reqMsg.Set(fd, protoreflect.ValueOfString(pageToken))
	}

	return checksum, nil
}

func processPaginationResponse(res connect.AnyResponse, pres paginationResponse, requestChecksum uint32) error {
	resProtoMsg, ok := res.Any().(protoreflect.ProtoMessage)
	if !ok {
		return errors.WithStack(errResponseMessageNotProto)
	}

	nextPageToken := pres.GetNextPageToken()
	if nextPageToken == "" {
		return nil
	}

	encodedPageToken := pagination.EncodePageTokenStruct(pageTokenEnvelope{
		RequestChecksum: requestChecksum,
		PageToken:       nextPageToken,
	})

	resMsg := resProtoMsg.ProtoReflect()
	if fd := resMsg.Descriptor().Fields().ByName("next_page_token"); fd != nil {
		resMsg.Set(fd, protoreflect.ValueOfString(encodedPageToken))
	}

	return nil
}

// NewPaginationInterceptor creates a [connect.UnaryInterceptorFunc] that
// validates pagination requests.
//
// Continuing a listing with a different filter or ordering invalidates the page
// token in flight, and would otherwise return a page of something else without
// saying so. The interceptor prevents that by wrapping every next_page_token in
// an envelope carrying a checksum of the request that produced it; when the
// token comes back, the checksum is compared against the new request before the
// real cursor is unwrapped and passed downstream.
func NewPaginationInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			paginationReq, ok := req.Any().(pagination.Request)
			if !ok {
				return next(ctx, req)
			}

			checksum, err := processPaginationRequest(req, paginationReq)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			res, err := next(ctx, req)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			paginationRes, ok := res.Any().(paginationResponse)
			if !ok {
				return res, nil
			}

			if err := processPaginationResponse(res, paginationRes, checksum); err != nil {
				return nil, errors.WithStack(err)
			}

			return res, nil
		}
	}
}
