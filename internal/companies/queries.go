package companies

import "github.com/CodrinSocol/bvb-dividends-ro/internal/common"

// GetCompanyQuery contains the data needed to read one company.
type GetCompanyQuery struct {
	Symbol common.Symbol
}
