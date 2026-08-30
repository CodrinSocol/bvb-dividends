package main

import (
	"go.uber.org/fx"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/app"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	dividendsbvb "github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/infra/bvb"
	dividendspg "github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/infra/repo/postgres"
	dividendscrpc "github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/interface/connectrpc"
)

// dividendsOptions wires the dividends slice.
func dividendsOptions() []fx.Option {
	return []fx.Option{
		fx.Provide(fx.Annotate(
			dividendspg.NewNamespace,
			fx.ResultTags(`group:"pgnamespaces"`),
		)),
		fx.Provide(fx.Annotate(
			dividendspg.NewDividendPostgresRepository,
			fx.As(new(dividends.Repository)),
		)),
		fx.Provide(fx.Annotate(
			dividendsbvb.NewSource,
			fx.As(new(dividends.Source)),
		)),
		fx.Provide(dividends.NewService),

		// The one thing the dividends slice needs from the companies slice:
		// whether a company exists, so that listing an unknown one is a 404
		// rather than an empty page. The dividends interface declares the
		// question and this binds the companies service to it, which is the
		// whole of the coupling between the two on the read side.
		fx.Provide(func(svc *companies.Service) dividendscrpc.CompanyChecker { return svc }),

		fx.Provide(fx.Annotate(
			dividendscrpc.NewDividendsConnectRPCInterface,
			fx.As(new(app.Interface)),
			fx.ResultTags(`group:"interfaces"`),
		)),
	}
}
