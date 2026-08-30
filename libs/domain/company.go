package domain

import "time"

// Company is a BVB-listed company that has announced at least one dividend.
//
// Companies are discovered through the import, never created directly, so the
// symbol BVB reports is the identity.
type Company struct {
	// Symbol is the ticker and the primary key, for example "SNP".
	Symbol Symbol

	// Name is the full legal name as reported by BVB, for example
	// "OMV PETROM S.A.".
	Name string

	// CreatedAt is when the company was first imported.
	CreatedAt time.Time

	// UpdatedAt is when the company was last refreshed from BVB.
	UpdatedAt time.Time
}

// Validate reports whether the company is well formed enough to persist.
func (c Company) Validate() error {
	if _, err := ParseSymbol(string(c.Symbol)); err != nil {
		return err
	}
	return nil
}
