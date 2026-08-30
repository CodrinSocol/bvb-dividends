package dividends

import (
	"context"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// Repository stores dividends.
//
// It reads through [common.ChildEntityReader] because a dividend is always read
// within the company that declared it, which is what AIP-122 means by a
// hierarchical resource. A nil parent is the AIP-159 `companies/-` wildcard:
// every company rather than one.
type Repository interface {
	common.ChildEntityReader[*common.Symbol, ID, *Dividend]

	common.EntityUpserter[*Dividend]
}

// Source is where dividends come from, upstream of this service.
//
// It is declared here and implemented over the BVB client in infra/bvb, so the
// import depends on the shape of the data rather than on the fact that BVB
// speaks SOAP.
type Source interface {
	// DividendsFor returns every dividend the given company has declared.
	DividendsFor(ctx context.Context, symbol common.Symbol) ([]*Dividend, error)
}
