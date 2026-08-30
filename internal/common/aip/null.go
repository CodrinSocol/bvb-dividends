package aip

import (
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/types/known/structpb"
)

// NullIdent is the name a filter uses to ask whether a value was reported at
// all, as in `schedule.ex_dividend_date = null`.
//
// AIP-160's grammar has no null literal, so it arrives as a bare identifier and
// has to be declared for the expression to type-check. It is the only way to
// reach a dividend BVB has announced but not yet dated; the previous
// service's date-range queries excluded those entirely.
const NullIdent = "null"

// rewriteNullComparisons turns a comparison against the `null` identifier into
// a comparison against a real CEL null, which is what cel2sql renders as
// `IS NULL` and `IS NOT NULL`.
//
// Left as an identifier it would become the SQL expression `= null`, which is
// never true and would silently return nothing.
func rewriteNullComparisons(exp *expr.Expr) {
	if exp == nil {
		return
	}

	switch k := exp.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		if k.IdentExpr.GetName() == NullIdent {
			exp.ExprKind = &expr.Expr_ConstExpr{
				ConstExpr: &expr.Constant{
					ConstantKind: &expr.Constant_NullValue{NullValue: structpb.NullValue_NULL_VALUE},
				},
			}
		}
	case *expr.Expr_SelectExpr:
		rewriteNullComparisons(k.SelectExpr.GetOperand())
	case *expr.Expr_CallExpr:
		rewriteNullComparisons(k.CallExpr.GetTarget())
		for _, arg := range k.CallExpr.GetArgs() {
			rewriteNullComparisons(arg)
		}
	case *expr.Expr_ListExpr:
		for _, e := range k.ListExpr.GetElements() {
			rewriteNullComparisons(e)
		}
	}
}
