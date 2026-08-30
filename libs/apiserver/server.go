// Package apiserver implements the DividendsService and serves it over REST.
//
// The service methods are the generated gRPC interface, and grpc-gateway maps
// them onto the REST paths declared by the google.api.http annotations in the
// protos. The gateway is registered against this implementation in-process, so
// there is no gRPC port and no loopback connection: one HTTP surface, with the
// proto definition still the single source of truth for it.
package apiserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/apiserver/aipquery"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
	dividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/genproto/bvb/dividends/v1"
)

// Server implements dividendsv1.DividendsServiceServer over the domain ports.
type Server struct {
	companies domain.CompanyRepository
	dividends domain.DividendRepository
	company   *aipquery.Compiler
	dividend  *aipquery.Compiler
	now       func() time.Time
}

// NewServer returns a Server reading from the given repositories.
func NewServer(companies domain.CompanyRepository, dividends domain.DividendRepository) (*Server, error) {
	companyCompiler, err := aipquery.NewCompanyCompiler()
	if err != nil {
		return nil, fmt.Errorf("build company filter compiler: %w", err)
	}
	dividendCompiler, err := aipquery.NewDividendCompiler()
	if err != nil {
		return nil, fmt.Errorf("build dividend filter compiler: %w", err)
	}
	return &Server{
		companies: companies,
		dividends: dividends,
		company:   companyCompiler,
		dividend:  dividendCompiler,
		now:       time.Now,
	}, nil
}

var _ dividendsv1.DividendsServiceServer = (*Server)(nil)

// GetCompany returns one company by resource name.
func (s *Server) GetCompany(ctx context.Context, request *dividendsv1.GetCompanyRequest) (*dividendsv1.Company, error) {
	symbol, err := parseCompanyName(request.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	company, err := s.companies.Get(ctx, symbol)
	if err != nil {
		return nil, toStatus(err)
	}
	return toCompanyProto(company), nil
}

// ListCompanies returns a page of companies.
func (s *Server) ListCompanies(
	ctx context.Context,
	request *dividendsv1.ListCompaniesRequest,
) (*dividendsv1.ListCompaniesResponse, error) {
	where, err := s.company.CompileFilter(request.GetFilter())
	if err != nil {
		return nil, toStatus(err)
	}
	orderBy, err := s.company.CompileOrderBy(request.GetOrderBy())
	if err != nil {
		return nil, toStatus(err)
	}
	page, token, err := aipquery.Page(request)
	if err != nil {
		return nil, toStatus(err)
	}

	result, err := s.companies.List(ctx, domain.CompanyQuery{Where: where, OrderBy: orderBy, Page: page})
	if err != nil {
		return nil, toStatus(err)
	}

	companies := make([]*dividendsv1.Company, 0, len(result.Items))
	for _, company := range result.Items {
		companies = append(companies, toCompanyProto(company))
	}
	return &dividendsv1.ListCompaniesResponse{
		Companies:     companies,
		NextPageToken: aipquery.NextPageToken(token, request, page, result.Total),
		TotalSize:     int32(result.Total), //nolint:gosec // BVB lists a few hundred companies.
	}, nil
}

// GetDividend returns one dividend by resource name.
func (s *Server) GetDividend(ctx context.Context, request *dividendsv1.GetDividendRequest) (*dividendsv1.Dividend, error) {
	company, id, err := parseDividendName(request.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	dividend, err := s.dividends.Get(ctx, id)
	if err != nil {
		return nil, toStatus(err)
	}
	// The identifier alone locates the dividend, so a name pairing a real
	// dividend with the wrong company would otherwise resolve. Refusing it
	// keeps a resource name meaning exactly one thing.
	if dividend.Company != company {
		return nil, status.Errorf(codes.NotFound, "dividend %q does not exist", request.GetName())
	}
	return toDividendProto(dividend, s.now()), nil
}

// ListDividends returns a page of dividends for one company, or for every
// company when the parent uses the `companies/-` wildcard.
func (s *Server) ListDividends(
	ctx context.Context,
	request *dividendsv1.ListDividendsRequest,
) (*dividendsv1.ListDividendsResponse, error) {
	company, err := parseCompanyParent(request.GetParent())
	if err != nil {
		return nil, toStatus(err)
	}
	where, err := s.dividend.CompileFilter(request.GetFilter())
	if err != nil {
		return nil, toStatus(err)
	}
	orderBy, err := s.dividend.CompileOrderBy(request.GetOrderBy())
	if err != nil {
		return nil, toStatus(err)
	}
	page, token, err := aipquery.Page(request)
	if err != nil {
		return nil, toStatus(err)
	}

	// Listing a named company that does not exist is a 404, not an empty page:
	// the caller asked about a resource, and "no dividends" would read as an
	// answer about a company that exists.
	if company != nil {
		exists, err := s.companies.Exists(ctx, *company)
		if err != nil {
			return nil, toStatus(err)
		}
		if !exists {
			return nil, status.Errorf(codes.NotFound, "company %q does not exist", request.GetParent())
		}
	}

	result, err := s.dividends.List(ctx, domain.DividendQuery{
		Company: company,
		Where:   where,
		OrderBy: orderBy,
		Page:    page,
	})
	if err != nil {
		return nil, toStatus(err)
	}

	now := s.now()
	dividends := make([]*dividendsv1.Dividend, 0, len(result.Items))
	for _, dividend := range result.Items {
		dividends = append(dividends, toDividendProto(dividend, now))
	}
	return &dividendsv1.ListDividendsResponse{
		Dividends:     dividends,
		NextPageToken: aipquery.NextPageToken(token, request, page, result.Total),
		TotalSize:     int32(result.Total), //nolint:gosec // Bounded by the dividend table.
	}, nil
}

// toStatus maps a domain error onto a gRPC status, which the gateway then
// renders as an AIP-193 error body. It is the only place that decides what a
// failure looks like to a caller.
func toStatus(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidArgument):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "the request was cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "the request took too long")
	default:
		// An unexpected failure must not leak its internals to a public,
		// unauthenticated API; the detail is logged by the middleware.
		return status.Error(codes.Internal, "the request could not be completed")
	}
}
