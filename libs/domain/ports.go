package domain

import "context"

// CompanyRepository stores companies.
type CompanyRepository interface {
	// Upsert inserts the companies, refreshing the name and update time of any
	// that already exist. The Java service inserted only when absent, so a
	// company that changed its legal name kept the old one forever.
	Upsert(ctx context.Context, companies ...Company) error

	// Get returns the company with the given symbol, or an error matching
	// ErrNotFound.
	Get(ctx context.Context, symbol Symbol) (Company, error)

	// Exists reports whether a company with the given symbol is stored.
	Exists(ctx context.Context, symbol Symbol) (bool, error)

	// List returns one page of companies matching q.
	List(ctx context.Context, q CompanyQuery) (Page[Company], error)

	// Count returns the number of stored companies. The importer uses it to
	// decide between a full backfill and an incremental run.
	Count(ctx context.Context) (int64, error)
}

// DividendRepository stores dividends.
type DividendRepository interface {
	// Get returns the dividend with the given ID, or an error matching
	// ErrNotFound.
	Get(ctx context.Context, id ID) (Dividend, error)

	// List returns one page of dividends matching q.
	List(ctx context.Context, q DividendQuery) (Page[Dividend], error)

	// Upsert inserts or updates dividends by ID and returns how many rows it
	// wrote. Because IDs are derived from the natural key, re-importing a
	// dividend updates the existing row instead of creating a second one, and
	// its creation time and resource name survive.
	Upsert(ctx context.Context, dividends ...Dividend) (int, error)
}

// Source is where dividend data comes from, upstream of this service.
//
// It is declared here, and implemented by the BVB SOAP adapter, so the import
// use case depends on the shape of the data rather than on SOAP.
type Source interface {
	// RecentlyAnnouncing returns the companies that announced a dividend
	// within the last days days.
	RecentlyAnnouncing(ctx context.Context, days int) ([]Company, error)

	// DividendsFor returns every dividend the given company has declared.
	DividendsFor(ctx context.Context, company Company) ([]Dividend, error)
}
