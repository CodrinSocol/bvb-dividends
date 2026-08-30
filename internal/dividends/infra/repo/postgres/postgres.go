// Package postgres stores dividends in PostgreSQL.
package postgres

import (
	"embed"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
)

// namespaceName is the PostgreSQL schema this slice owns.
const namespaceName = "dividend"

// companyNamespace is the schema this one references.
const companyNamespace = "company"

//go:embed schema/*.sql
var schema embed.FS

// NewNamespace returns the dividends [postgres.Namespace].
//
// It depends on the companies namespace, because a dividend has a foreign key
// into it: a dividend belongs to the company that declared it, and there is
// nowhere for the reference to point until that table exists.
func NewNamespace() *postgres.Namespace {
	return &postgres.Namespace{
		Name:      namespaceName,
		DependsOn: []string{companyNamespace},
		Schema:    schema,
		Seed:      nil,
	}
}
