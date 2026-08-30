package postgres

import (
	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/repo/postgres/gen"
)

// toCompany converts a stored row into the entity.
//
// The symbol was normalised before it was written, so it is taken as-is rather
// than parsed again: re-parsing here would turn a row that is already in the
// database into an error nobody could act on.
func toCompany(row gen.CompanyCompany) *companies.Company {
	return &companies.Company{
		Symbol:      common.Symbol(row.Symbol),
		DisplayName: row.DisplayName,
		CreateTime:  row.CreateTime,
		UpdateTime:  row.UpdateTime,
	}
}

func toCompanies(rows []gen.CompanyCompany) []*companies.Company {
	result := make([]*companies.Company, len(rows))
	for i, row := range rows {
		result[i] = toCompany(row)
	}

	return result
}
