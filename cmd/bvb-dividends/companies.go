package main

import (
	"go.uber.org/fx"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/app"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	companiesbvb "github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/bvb"
	companiespg "github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/repo/postgres"
	companiescrpc "github.com/CodrinSocol/bvb-dividends-ro/internal/companies/interface/connectrpc"
)

// companiesOptions wires the companies slice.
//
// The ConnectRPC interface is provided here too, but constructing it is what
// asking for the interfaces group does, so a one-shot run that never asks for
// that group gets the slice without its protocol adapter.
func companiesOptions() []fx.Option {
	return []fx.Option{
		// The slice's schema joins the set the database migrates on startup.
		fx.Provide(fx.Annotate(
			companiespg.NewNamespace,
			fx.ResultTags(`group:"pgnamespaces"`),
		)),
		fx.Provide(fx.Annotate(
			companiespg.NewCompanyPostgresRepository,
			fx.As(new(companies.Repository)),
		)),
		fx.Provide(fx.Annotate(
			companiesbvb.NewSource,
			fx.As(new(companies.Source)),
		)),
		fx.Provide(companies.NewService),
		fx.Provide(fx.Annotate(
			companiescrpc.NewCompaniesConnectRPCInterface,
			fx.As(new(app.Interface)),
			fx.ResultTags(`group:"interfaces"`),
		)),
	}
}
