package aipquery

import (
	"fmt"
	"strings"
	"time"

	"go.einride.tech/aip/filtering"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// Compiler parses AIP-160 filters and AIP-132 orderings for one resource.
//
// A Compiler is read-only after construction and safe for concurrent use.
type Compiler struct {
	declarations *filtering.Declarations
	byPath       map[string]field
}

// NewDividendCompiler returns a Compiler for the fields of a Dividend.
func NewDividendCompiler() (*Compiler, error) { return newCompiler(dividendFields) }

// NewCompanyCompiler returns a Compiler for the fields of a Company.
func NewCompanyCompiler() (*Compiler, error) { return newCompiler(companyFields) }

// newCompiler declares fields to the type-checker and indexes them by path.
func newCompiler(fields []field) (*Compiler, error) {
	options := make([]filtering.DeclarationOption, 0, len(fields)+2)
	options = append(options,
		filtering.DeclareStandardFunctions(),
		filtering.DeclareIdent(nullIdent, filtering.TypeString),
	)

	byPath := make(map[string]field, len(fields))
	for _, f := range fields {
		options = append(options, filtering.DeclareIdent(f.path, f.declarationType()))
		byPath[f.path] = f
	}

	declarations, err := filtering.NewDeclarations(options...)
	if err != nil {
		return nil, fmt.Errorf("declare filter fields: %w", err)
	}
	return &Compiler{declarations: declarations, byPath: byPath}, nil
}

// invalidf builds a caller-facing error for a malformed filter.
func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidArgument, fmt.Sprintf(format, args...))
}

// CompileFilter parses filter and returns it as a domain predicate. An empty
// filter compiles to a nil predicate, which matches everything.
func (c *Compiler) CompileFilter(filter string) (domain.Predicate, error) {
	if strings.TrimSpace(filter) == "" {
		return nil, nil
	}

	parsed, err := filtering.ParseFilterString(filter, c.declarations)
	if err != nil {
		// The parser's message names the offending token and position, which is
		// exactly what the caller needs, but it is multi-line; flatten it.
		return nil, invalidf("filter is not valid: %s", strings.Join(strings.Fields(err.Error()), " "))
	}
	if parsed.CheckedExpr == nil {
		return nil, nil
	}
	return c.predicate(parsed.CheckedExpr.GetExpr())
}

// predicate converts one boolean node of the expression tree.
func (c *Compiler) predicate(e *expr.Expr) (domain.Predicate, error) {
	call := e.GetCallExpr()
	if call == nil {
		return nil, invalidf("filter must be a boolean expression")
	}

	args := call.GetArgs()
	switch function := call.GetFunction(); function {
	case filtering.FunctionAnd, filtering.FunctionFuzzyAnd:
		operands, err := c.operands(args)
		if err != nil {
			return nil, err
		}
		return domain.And{Operands: operands}, nil

	case filtering.FunctionOr:
		operands, err := c.operands(args)
		if err != nil {
			return nil, err
		}
		return domain.Or{Operands: operands}, nil

	case filtering.FunctionNot:
		if len(args) != 1 {
			return nil, invalidf("NOT takes a single operand")
		}
		operand, err := c.predicate(args[0])
		if err != nil {
			return nil, err
		}
		return domain.Not{Operand: operand}, nil

	case filtering.FunctionEquals:
		return c.comparison(domain.OpEqual, args)
	case filtering.FunctionNotEquals:
		return c.comparison(domain.OpNotEqual, args)
	case filtering.FunctionLessThan:
		return c.comparison(domain.OpLess, args)
	case filtering.FunctionLessEquals:
		return c.comparison(domain.OpLessOrEqual, args)
	case filtering.FunctionGreaterThan:
		return c.comparison(domain.OpGreater, args)
	case filtering.FunctionGreaterEquals:
		return c.comparison(domain.OpGreaterOrEqual, args)
	case filtering.FunctionHas:
		return c.comparison(domain.OpHas, args)

	default:
		return nil, invalidf("unsupported function %q in filter", function)
	}
}

// operands converts every argument of a conjunction or disjunction.
func (c *Compiler) operands(args []*expr.Expr) ([]domain.Predicate, error) {
	operands := make([]domain.Predicate, 0, len(args))
	for _, arg := range args {
		operand, err := c.predicate(arg)
		if err != nil {
			return nil, err
		}
		operands = append(operands, operand)
	}
	return operands, nil
}

// comparison converts a field-to-literal comparison.
func (c *Compiler) comparison(op domain.Op, args []*expr.Expr) (domain.Predicate, error) {
	if len(args) != 2 {
		return nil, invalidf("%s takes two operands", op)
	}

	path, ok := qualifiedName(args[0])
	if !ok {
		return nil, invalidf("the left side of %s must be a field name", op)
	}
	f, ok := c.byPath[path]
	if !ok {
		return nil, invalidf("field %q cannot be filtered on", path)
	}

	value, err := c.literal(f, args[1])
	if err != nil {
		return nil, err
	}
	return domain.Compare{Field: f.target, Op: op, Value: value}, nil
}

// literal converts the right-hand side of a comparison to a typed value.
func (c *Compiler) literal(f field, e *expr.Expr) (domain.Value, error) {
	// `field = null` asks whether BVB reported the field at all. It arrives as
	// a bare identifier, since AIP-160 has no null literal.
	if name, ok := qualifiedName(e); ok {
		if name == nullIdent {
			return domain.NullValue{}, nil
		}
		return nil, invalidf("%q is not a value; quote it if it is text", name)
	}

	// `timestamp("...")` is AIP-160's standard way of writing an instant.
	if call := e.GetCallExpr(); call != nil {
		if call.GetFunction() != filtering.FunctionTimestamp || len(call.GetArgs()) != 1 {
			return nil, invalidf("%q takes a literal value", f.path)
		}
		text := call.GetArgs()[0].GetConstExpr().GetStringValue()
		return parseTimestamp(f, text)
	}

	constant := e.GetConstExpr()
	if constant == nil {
		return nil, invalidf("%q takes a literal value", f.path)
	}

	switch value := constant.GetConstantKind().(type) {
	case *expr.Constant_Int64Value:
		if f.kind != kindInt {
			return nil, invalidf("%q does not compare against a number", f.path)
		}
		return domain.IntValue(value.Int64Value), nil

	case *expr.Constant_StringValue:
		switch f.kind {
		case kindString:
			return domain.StringValue(value.StringValue), nil
		case kindDate:
			date, err := domain.ParseDate(value.StringValue)
			if err != nil {
				return nil, invalidf("%q takes a date as YYYY-MM-DD, got %q", f.path, value.StringValue)
			}
			return domain.DateValue(date), nil
		case kindTimestamp:
			return parseTimestamp(f, value.StringValue)
		default:
			return nil, invalidf("%q does not compare against text", f.path)
		}

	default:
		return nil, invalidf("%q takes a literal value", f.path)
	}
}

// parseTimestamp reads an RFC 3339 instant for a timestamp field.
func parseTimestamp(f field, text string) (domain.Value, error) {
	if f.kind != kindTimestamp {
		return nil, invalidf("%q does not compare against a timestamp", f.path)
	}
	instant, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return nil, invalidf("%q takes an RFC 3339 timestamp, got %q", f.path, text)
	}
	return domain.TimeValue(instant), nil
}

// qualifiedName rebuilds a dotted field path from an identifier or a chain of
// selections, so that `schedule.ex_dividend_date` reads as one name.
func qualifiedName(e *expr.Expr) (string, bool) {
	switch kind := e.GetExprKind().(type) {
	case *expr.Expr_IdentExpr:
		return kind.IdentExpr.GetName(), true
	case *expr.Expr_SelectExpr:
		if kind.SelectExpr.GetTestOnly() {
			return "", false
		}
		parent, ok := qualifiedName(kind.SelectExpr.GetOperand())
		if !ok {
			return "", false
		}
		return parent + "." + kind.SelectExpr.GetField(), true
	default:
		return "", false
	}
}
