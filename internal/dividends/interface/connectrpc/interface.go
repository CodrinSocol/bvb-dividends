// Package connectrpc exposes the dividends subdomain over ConnectRPC.
package connectrpc

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/aip"
	commoncrpc "github.com/CodrinSocol/bvb-dividends-ro/internal/common/connectrpc"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	bvbdividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/go/gen/v1"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/go/gen/v1/bvbdividendsv1connect"
)

// Page size bounds, as documented on the request messages. The upper bound is
// also declared as a protovalidate constraint, so a larger request is refused
// before it reaches here.
const (
	pageSizeDefault = 50
	pageSizeMax     = 1000
)

// CompanyChecker reports whether a company is known.
//
// The dividends slice cannot read the companies slice's table, so it declares
// the one question it needs answered and the composition root in cmd/ hands it
// the companies service. That is the whole of the coupling between the two on
// the read side.
type CompanyChecker interface {
	Exists(ctx context.Context, symbol common.Symbol) (bool, error)
}

// DividendsConnectRPCInterface implements
// [bvbdividendsv1connect.DividendsServiceHandler].
type DividendsConnectRPCInterface struct {
	log       *slog.Logger
	svc       *dividends.Service
	companies CompanyChecker
}

// NewDividendsConnectRPCInterface creates a new
// [DividendsConnectRPCInterface] and registers it on the server.
func NewDividendsConnectRPCInterface(
	log *slog.Logger,
	svc *dividends.Service,
	companies CompanyChecker,
	srv *commoncrpc.Server,
) *DividendsConnectRPCInterface {
	intf := &DividendsConnectRPCInterface{
		log:       log.WithGroup("dividends").WithGroup("interface").WithGroup("connectrpc"),
		svc:       svc,
		companies: companies,
	}

	// The filter and ordering are checked against the resource message itself,
	// so what a caller may filter on is whatever the proto declares.
	resourceMapping := map[string]proto.Message{
		bvbdividendsv1connect.DividendsServiceListDividendsProcedure: (*bvbdividendsv1.Dividend)(nil),
	}

	commoncrpc.RegisterService[bvbdividendsv1connect.DividendsServiceHandler](
		srv,
		bvbdividendsv1connect.NewDividendsServiceHandler,
		intf,
		connect.WithInterceptors(
			aip.NewFilteringInterceptor(resourceMapping),
			aip.NewOrderingInterceptor(resourceMapping),
		),
	)

	return intf
}

var _ bvbdividendsv1connect.DividendsServiceHandler = (*DividendsConnectRPCInterface)(nil)

// GetDividend returns one dividend by resource name.
func (i *DividendsConnectRPCInterface) GetDividend(
	ctx context.Context,
	req *connect.Request[bvbdividendsv1.GetDividendRequest],
) (*connect.Response[bvbdividendsv1.Dividend], error) {
	company, id, err := ParseResourceName(req.Msg.GetName())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	dividend, err := i.svc.GetDividend(ctx, dividends.GetDividendQuery{Company: company, ID: id})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return connect.NewResponse(toProto(dividend)), nil
}

// ListDividends returns a page of dividends for one company, or for every
// company when the parent uses the `companies/-` wildcard.
func (i *DividendsConnectRPCInterface) ListDividends(
	ctx context.Context,
	req *connect.Request[bvbdividendsv1.ListDividendsRequest],
) (*connect.Response[bvbdividendsv1.ListDividendsResponse], error) {
	company, err := ParseParent(req.Msg.GetParent())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// Listing a named company that does not exist is a 404, not an empty page:
	// the caller asked about a resource, and "no dividends" would read as an
	// answer about a company that exists.
	if company != nil {
		exists, err := i.companies.Exists(ctx, *company)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if !exists {
			return nil, common.ErrEntityNotFound.WithMessageExtension("company " + company.String())
		}
	}

	result, err := i.svc.ListDividends(ctx, dividends.ListDividendsQuery{
		Company: company,
		List: common.ListQuery{
			PageSize:  PageSize(req.Msg.GetPageSize()),
			PageToken: req.Msg.GetPageToken(),
			Filter:    aip.FilterFromContext(ctx),
			OrderBy:   aip.OrderByFromContext(ctx),
		},
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	response := &bvbdividendsv1.ListDividendsResponse{
		Dividends:     toProtos(result.Items),
		NextPageToken: result.NextPageToken,
	}
	if result.TotalSize != nil {
		//nolint:gosec // Bounded by the dividends table.
		response.TotalSize = int32(*result.TotalSize)
	}

	return connect.NewResponse(response), nil
}

// PageSize applies the documented default and bound to a requested page size.
//
// AIP-158 requires a size above the maximum to be coerced down rather than
// refused.
func PageSize(requested int32) int32 {
	switch {
	case requested <= 0:
		return pageSizeDefault
	case requested > pageSizeMax:
		return pageSizeMax
	default:
		return requested
	}
}
