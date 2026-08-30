// Package postgres stores companies in PostgreSQL.
package postgres

import (
	"embed"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common/postgres"
)

// namespaceName is the PostgreSQL schema this slice owns.
const namespaceName = "company"

//go:embed schema/*.sql
var schema embed.FS

// NewNamespace returns the companies [postgres.Namespace].
//
// It depends on nothing: a company is the root of the resource hierarchy, and
// the dividends namespace is the one that has to wait.
func NewNamespace() *postgres.Namespace {
	return &postgres.Namespace{
		Name:      namespaceName,
		DependsOn: nil,
		Schema:    schema,
		Seed:      nil,
	}
}
