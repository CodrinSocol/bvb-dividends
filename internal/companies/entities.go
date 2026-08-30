package companies

import (
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// Company is a BVB-listed company that has announced at least one dividend.
//
// Companies are discovered through the import, never created directly, so the
// symbol BVB reports is the identity.
type Company struct {
	// Symbol is the ticker and the primary key, for example "SNP".
	Symbol common.Symbol

	// DisplayName is the full legal name as reported by BVB, for example
	// "OMV PETROM S.A.".
	DisplayName string

	// CreateTime is when the company was first imported.
	CreateTime time.Time

	// UpdateTime is when the company's details last changed.
	UpdateTime time.Time
}

// Validate reports whether the company is well formed enough to persist.
func (c Company) Validate() error {
	if _, err := common.ParseSymbol(c.Symbol.String()); err != nil {
		return err
	}

	return nil
}
