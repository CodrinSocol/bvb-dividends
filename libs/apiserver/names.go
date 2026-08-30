package apiserver

import (
	"fmt"

	"go.einride.tech/aip/resourcename"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// Resource name patterns, matching the google.api.resource options on the
// messages in proto/bvb/dividends/v1.
const (
	companyPattern  = "companies/{company}"
	dividendPattern = "companies/{company}/dividends/{dividend}"

	// wildcard is AIP-159's "every collection" segment: a parent of
	// `companies/-` lists dividends across all companies.
	wildcard = "-"
)

// companyName renders a company's resource name, for example "companies/SNP".
func companyName(symbol domain.Symbol) string {
	return resourcename.Sprint(companyPattern, symbol.String())
}

// dividendName renders a dividend's resource name.
func dividendName(company domain.Symbol, id domain.ID) string {
	return resourcename.Sprint(dividendPattern, company.String(), id.String())
}

// parseCompanyName reads a company resource name.
func parseCompanyName(name string) (domain.Symbol, error) {
	var companyID string
	if err := resourcename.Sscan(name, companyPattern, &companyID); err != nil {
		return "", fmt.Errorf("%w: %q is not a company resource name, expected %s",
			domain.ErrInvalidArgument, name, companyPattern)
	}
	return domain.ParseSymbol(companyID)
}

// parseCompanyParent reads the parent of a dividend collection.
//
// A nil symbol means the AIP-159 wildcard `companies/-`, which selects every
// company rather than one.
func parseCompanyParent(parent string) (*domain.Symbol, error) {
	var companyID string
	if err := resourcename.Sscan(parent, companyPattern, &companyID); err != nil {
		return nil, fmt.Errorf("%w: %q is not a company resource name, expected %s or companies/-",
			domain.ErrInvalidArgument, parent, companyPattern)
	}
	if companyID == wildcard {
		return nil, nil //nolint:nilnil // A nil symbol is the wildcard; see the doc comment.
	}

	symbol, err := domain.ParseSymbol(companyID)
	if err != nil {
		return nil, err
	}
	return &symbol, nil
}

// parseDividendName reads a dividend resource name, returning the company it
// belongs to and the dividend's identifier.
func parseDividendName(name string) (domain.Symbol, domain.ID, error) {
	var companyID, dividendID string
	if err := resourcename.Sscan(name, dividendPattern, &companyID, &dividendID); err != nil {
		return "", domain.ID{}, fmt.Errorf("%w: %q is not a dividend resource name, expected %s",
			domain.ErrInvalidArgument, name, dividendPattern)
	}

	symbol, err := domain.ParseSymbol(companyID)
	if err != nil {
		return "", domain.ID{}, err
	}
	id, err := domain.ParseID(dividendID)
	if err != nil {
		return "", domain.ID{}, fmt.Errorf("%w: %q is not a dividend identifier",
			domain.ErrInvalidArgument, dividendID)
	}
	return symbol, id, nil
}
