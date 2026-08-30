package aip

import (
	"context"

	"connectrpc.com/connect"
	"github.com/cockroachdb/errors"
	"go.einride.tech/aip/ordering"
	"google.golang.org/protobuf/proto"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

type ctxKeyOrderBy struct{}

func contextWithOrderBy(ctx context.Context, orderBy common.OrderBy) context.Context {
	return context.WithValue(ctx, ctxKeyOrderBy{}, orderBy)
}

// OrderByFromContext returns the [common.OrderBy] the ordering interceptor put
// on the context, or nothing when the caller asked for no particular order.
func OrderByFromContext(ctx context.Context) common.OrderBy {
	orderBy, ok := ctx.Value(ctxKeyOrderBy{}).(common.OrderBy)
	if !ok {
		return nil
	}

	return orderBy
}

func orderByToDomain(ob ordering.OrderBy) common.OrderBy {
	orderBy := make(common.OrderBy, len(ob.Fields))
	for i, f := range ob.Fields {
		direction := common.OrderDirectionAsc
		if f.Desc {
			direction = common.OrderDirectionDesc
		}

		orderBy[i] = common.OrderByClause{Path: f.Path, Direction: direction}
	}

	return orderBy
}

// NewOrderingInterceptor creates a [connect.UnaryInterceptorFunc] that parses
// the AIP-132 order_by of a request, validates it against the resource, and
// puts it on the context.
func NewOrderingInterceptor(resources map[string]proto.Message) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			orderingReq, ok := req.Any().(ordering.Request)
			if !ok {
				return next(ctx, req)
			}

			procedure := req.Spec().Procedure
			resource, ok := resources[procedure]
			if !ok {
				return nil, errors.New("no ordering resource configured for procedure: " + procedure)
			}

			ob, err := ordering.ParseOrderBy(orderingReq)
			if err != nil {
				return nil, common.ErrOrderByInvalid.WithUnderlying(err)
			}

			if err := ob.ValidateForMessage(resource); err != nil {
				return nil, common.ErrOrderByInvalid.WithUnderlying(err)
			}

			return next(contextWithOrderBy(ctx, orderByToDomain(ob)), req)
		}
	}
}
