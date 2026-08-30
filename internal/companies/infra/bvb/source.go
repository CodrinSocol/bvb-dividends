// Package bvb reads companies from the BVB financials web service.
package bvb

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// balanceYears is how many years of filings are asked for when enumerating the
// market.
//
// The service has no operation that lists companies; the nearest thing is the
// set of issuers that filed a balance for a year. The current year returns
// nothing until the filing season reaches it, and a company that has since
// delisted only appears in the years it filed, so several years are asked for
// and the answers unioned.
const balanceYears = 3

// Client is the part of the BVB client this slice uses.
//
// It is declared here, by the consumer, so the slice can be tested against a
// fake without a SOAP server, and so that what it depends on is the two methods
// it calls rather than a whole client.
type Client interface {
	GetLastDividends(ctx context.Context, days int) ([]bvbclient.Identification, error)
	GetAvailableBalances(ctx context.Context, year int, reportType bvbclient.ReportType) ([]bvbclient.SymbolBalance, error)
}

// Source implements [companies.Source] over the BVB client.
type Source struct {
	log    *slog.Logger
	client Client
	now    func() time.Time
}

// NewSource returns a new instance of [Source].
func NewSource(log *slog.Logger, client Client) *Source {
	return &Source{log: log.WithGroup("companies").WithGroup("bvb"), client: client, now: time.Now}
}

var _ companies.Source = (*Source)(nil)

// RecentlyAnnouncing returns the companies that announced a dividend within the
// last days days.
func (s *Source) RecentlyAnnouncing(ctx context.Context, days int) ([]*companies.Company, error) {
	identifications, err := s.client.GetLastDividends(ctx, days)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return toCompanies(identifications, s.log), nil
}

// All returns every company that has filed a balance in recent years.
//
// The companies come back with a symbol and no name, because that is all the
// operation reports; the name is filled in by the first import that reads their
// dividends, which is why storing one must not overwrite a name already known.
func (s *Source) All(ctx context.Context) ([]*companies.Company, error) {
	currentYear := s.now().Year()

	symbols := make([]string, 0, 512)
	for offset := range balanceYears {
		year := currentYear - offset

		balances, err := s.client.GetAvailableBalances(ctx, year, bvbclient.ReportTypeAnnual)
		if err != nil {
			return nil, errors.Wrapf(err, "list the issuers filing for %d", year)
		}

		s.log.DebugContext(ctx, "listed the issuers filing for a year",
			slog.Int("year", year),
			slog.Int("issuers", len(balances)))

		for _, balance := range balances {
			symbols = append(symbols, balance.Symbol)
		}
	}

	return toCompaniesFromSymbols(symbols, s.log), nil
}
