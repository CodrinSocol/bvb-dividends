// Package connectrpc exposes the companies subdomain over ConnectRPC.
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
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
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

// CompaniesConnectRPCInterface implements
// [bvbdividendsv1connect.CompaniesServiceHandler].
type CompaniesConnectRPCInterface struct {
	log *slog.Logger
	svc *companies.Service
}

// NewCompaniesConnectRPCInterface creates a new
// [CompaniesConnectRPCInterface] and registers it on the server.
func NewCompaniesConnectRPCInterface(
	log *slog.Logger,
	svc *companies.Service,
	srv *commoncrpc.Server,
) *CompaniesConnectRPCInterface {
	intf := &CompaniesConnectRPCInterface{
		log: log.WithGroup("companies").WithGroup("interface").WithGroup("connectrpc"),
		svc: svc,
	}

	// The filter and ordering are checked against the resource message itself,
	// so what a caller may filter on is whatever the proto declares.
	resourceMapping := map[string]proto.Message{
		bvbdividendsv1connect.CompaniesServiceListCompaniesProcedure: (*bvbdividendsv1.Company)(nil),
	}

	commoncrpc.RegisterService[bvbdividendsv1connect.CompaniesServiceHandler](
		srv,
		bvbdividendsv1connect.NewCompaniesServiceHandler,
		intf,
		connect.WithInterceptors(
			aip.NewFilteringInterceptor(resourceMapping),
			aip.NewOrderingInterceptor(resourceMapping),
		),
	)

	return intf
}

var _ bvbdividendsv1connect.CompaniesServiceHandler = (*CompaniesConnectRPCInterface)(nil)

// GetCompany returns one company by resource name.
func (i *CompaniesConnectRPCInterface) GetCompany(
	ctx context.Context,
	req *connect.Request[bvbdividendsv1.GetCompanyRequest],
) (*connect.Response[bvbdividendsv1.Company], error) {
	symbol, err := ParseResourceName(req.Msg.GetName())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	company, err := i.svc.GetCompany(ctx, companies.GetCompanyQuery{Symbol: symbol})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return connect.NewResponse(toProto(company)), nil
}

// ListCompanies returns a page of companies.
func (i *CompaniesConnectRPCInterface) ListCompanies(
	ctx context.Context,
	req *connect.Request[bvbdividendsv1.ListCompaniesRequest],
) (*connect.Response[bvbdividendsv1.ListCompaniesResponse], error) {
	result, err := i.svc.ListCompanies(ctx, common.ListQuery{
		PageSize:  PageSize(req.Msg.GetPageSize()),
		PageToken: req.Msg.GetPageToken(),
		Filter:    aip.FilterFromContext(ctx),
		OrderBy:   aip.OrderByFromContext(ctx),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	response := &bvbdividendsv1.ListCompaniesResponse{
		Companies:     toProtos(result.Items),
		NextPageToken: result.NextPageToken,
	}
	if result.TotalSize != nil {
		//nolint:gosec // BVB lists a few hundred companies.
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
