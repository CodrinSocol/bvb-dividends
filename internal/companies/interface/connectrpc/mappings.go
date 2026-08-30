package connectrpc

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	bvbdividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/go/gen/v1"
)

// ResourceName renders a company's resource name, for example "companies/SNP".
//
// The name is built from the pattern the proto declares, through the type
// generated from it, so the pattern is written down once.
func ResourceName(symbol common.Symbol) string {
	return bvbdividendsv1.CompanyResourceName{Company: symbol.String()}.String()
}

// ParseResourceName reads a company resource name.
func ParseResourceName(name string) (common.Symbol, error) {
	var parsed bvbdividendsv1.CompanyResourceName
	if err := parsed.UnmarshalString(name); err != nil {
		return "", common.ErrEntityInvalid.
			WithProblem("name", name+" is not a company resource name").
			WithUnderlying(err)
	}

	return common.ParseSymbol(parsed.Company)
}

// toProto renders a company as its API resource.
func toProto(company *companies.Company) *bvbdividendsv1.Company {
	return &bvbdividendsv1.Company{
		Name:        ResourceName(company.Symbol),
		Symbol:      company.Symbol.String(),
		DisplayName: company.DisplayName,
		CreateTime:  timestamppb.New(company.CreateTime),
		UpdateTime:  timestamppb.New(company.UpdateTime),
	}
}

func toProtos(list []*companies.Company) []*bvbdividendsv1.Company {
	result := make([]*bvbdividendsv1.Company, len(list))
	for i, company := range list {
		result[i] = toProto(company)
	}

	return result
}
