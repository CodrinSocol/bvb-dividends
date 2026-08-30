// Package bvb reads companies from the BVB financials web service.
package bvb

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// Client is the part of the BVB client this slice uses.
//
// It is declared here, by the consumer, so the slice can be tested against a
// fake without a SOAP server, and so that what it depends on is one method
// rather than a whole client.
type Client interface {
	GetLastDividends(ctx context.Context, days int) ([]bvbclient.Identification, error)
}

// Source implements [companies.Source] over the BVB client.
type Source struct {
	log    *slog.Logger
	client Client
}

// NewSource returns a new instance of [Source].
func NewSource(log *slog.Logger, client Client) *Source {
	return &Source{log: log.WithGroup("companies").WithGroup("bvb"), client: client}
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
