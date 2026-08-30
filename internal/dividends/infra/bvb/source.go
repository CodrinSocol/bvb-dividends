// Package bvb reads dividends from the BVB financials web service.
package bvb

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/errors"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// Client is the part of the BVB client this slice uses.
//
// It is declared here, by the consumer, so the slice can be tested against a
// fake without a SOAP server, and so that what it depends on is one method
// rather than a whole client.
type Client interface {
	GetDividends(ctx context.Context, symbol string) ([]bvbclient.DividendInfo, error)
}

// Source implements [dividends.Source] over the BVB client.
type Source struct {
	log    *slog.Logger
	client Client
}

// NewSource returns a new instance of [Source].
func NewSource(log *slog.Logger, client Client) *Source {
	return &Source{log: log.WithGroup("dividends").WithGroup("bvb"), client: client}
}

var _ dividends.Source = (*Source)(nil)

// DividendsFor returns every dividend BVB has recorded for one company.
func (s *Source) DividendsFor(ctx context.Context, symbol common.Symbol) ([]*dividends.Dividend, error) {
	if _, err := common.ParseSymbol(symbol.String()); err != nil {
		return nil, errors.WithStack(err)
	}

	infos, err := s.client.GetDividends(ctx, symbol.String())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return toDividends(symbol, infos, s.log), nil
}
