package companies

import (
	"context"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// Repository stores companies.
type Repository interface {
	common.Repository[common.Symbol, *Company]

	// Exists reports whether a company with the given symbol is stored.
	Exists(ctx context.Context, symbol common.Symbol) (bool, error)

	// Count returns the number of stored companies. The import uses it to
	// decide between a full backfill and an incremental run.
	Count(ctx context.Context) (int64, error)
}

// Source is where companies come from, upstream of this service.
//
// It is declared here and implemented over the BVB client in infra/bvb, so the
// import depends on the shape of the data rather than on the fact that BVB
// speaks SOAP.
type Source interface {
	// RecentlyAnnouncing returns the companies that announced a dividend
	// within the last days days.
	RecentlyAnnouncing(ctx context.Context, days int) ([]*Company, error)
}
