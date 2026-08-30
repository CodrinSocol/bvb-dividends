package dividends

import "github.com/CodrinSocol/bvb-dividends-ro/internal/common"

// GetDividendQuery contains the data needed to read one dividend.
type GetDividendQuery struct {
	// Company is the company the dividend was read within. A dividend is
	// located by its identifier alone, so this is checked rather than used: a
	// resource name pairing a real dividend with the wrong company would
	// otherwise resolve, and a name has to mean exactly one thing.
	Company common.Symbol

	ID ID
}

// ListDividendsQuery contains the data needed to read one page of dividends.
type ListDividendsQuery struct {
	// Company restricts the listing to one company. A nil Company is the
	// AIP-159 `companies/-` wildcard: every company rather than one.
	Company *common.Symbol

	List common.ListQuery
}
