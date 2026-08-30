package aip

import (
	"strings"

	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
)

// ColumnOf is the column a filterable field is stored in.
//
// A nested proto field is addressed with a dot — `schedule.ex_dividend_date` —
// but SQL reads a dot as a table qualifier, so the column that holds it is the
// same path with underscores. Every relation a slice lists over exposes its
// filterable fields under exactly these names, which is what lets the filter
// surface be derived from the proto instead of restated in Go.
func ColumnOf(path string) string {
	return strings.ReplaceAll(path, ".", "_")
}

// flattenIdents rewrites every reference to a nested field into a single
// identifier naming its column.
//
// The type-checker resolves `schedule.ex_dividend_date` against the declaration
// of that name, but leaves the expression as a selection on an identifier
// called `schedule`; rendered as SQL that would read as a column of a table
// called `schedule`. Collapsing the chain to one identifier before conversion
// is what makes it read as the column it is.
func flattenIdents(exp *expr.Expr) {
	if exp == nil {
		return
	}

	switch k := exp.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		k.IdentExpr.Name = ColumnOf(k.IdentExpr.GetName())
	case *expr.Expr_SelectExpr:
		if path, ok := identPath(exp); ok {
			exp.ExprKind = &expr.Expr_IdentExpr{
				IdentExpr: &expr.Expr_Ident{Name: ColumnOf(path)},
			}

			return
		}

		flattenIdents(k.SelectExpr.GetOperand())
	case *expr.Expr_CallExpr:
		flattenIdents(k.CallExpr.GetTarget())
		for _, arg := range k.CallExpr.GetArgs() {
			flattenIdents(arg)
		}
	case *expr.Expr_ListExpr:
		for _, e := range k.ListExpr.GetElements() {
			flattenIdents(e)
		}
	}
}

// identPath returns the dotted path an expression names, if it is nothing but
// identifiers and selections — `a`, `a.b`, `a.b.c`.
func identPath(exp *expr.Expr) (string, bool) {
	switch k := exp.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		return k.IdentExpr.GetName(), true
	case *expr.Expr_SelectExpr:
		if k.SelectExpr.GetTestOnly() {
			return "", false
		}

		operand, ok := identPath(k.SelectExpr.GetOperand())
		if !ok {
			return "", false
		}

		return operand + "." + k.SelectExpr.GetField(), true
	default:
		return "", false
	}
}
