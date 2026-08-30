package postgres

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// fieldKind is the value type a filterable field accepts.
type fieldKind uint8

const (
	kindString fieldKind = iota
	kindInt
	kindDate
	kindTime
)

// column describes how a domain field reaches SQL.
type column struct {
	name string
	kind fieldKind
}

// dividendColumns is the complete set of dividend fields a filter may name.
//
// A field that is not in this map cannot be filtered or sorted on. Because the
// filter parser only ever produces domain.Field values and every one of them
// is looked up here, a caller has no way to reach a column this map does not
// list, let alone to inject SQL: field names are never taken from the request.
var dividendColumns = map[domain.Field]column{
	domain.FieldCompany:          {"company_symbol", kindString},
	domain.FieldYear:             {"year", kindInt},
	domain.FieldDividendType:     {"dividend_type", kindString},
	domain.FieldAnnouncementDate: {"announcement_date", kindDate},
	domain.FieldGMSReferenceDate: {"gms_reference_date", kindDate},
	domain.FieldGMSDate:          {"gms_date", kindDate},
	domain.FieldRecordDate:       {"record_date", kindDate},
	domain.FieldExDividendDate:   {"ex_dividend_date", kindDate},
	domain.FieldPaymentStartDate: {"payment_start_date", kindDate},
	domain.FieldPaymentEndDate:   {"payment_end_date", kindDate},
	domain.FieldCreateTime:       {"created_at", kindTime},
	domain.FieldUpdateTime:       {"updated_at", kindTime},
}

// companyColumns is the complete set of company fields a filter may name.
var companyColumns = map[domain.Field]column{
	domain.FieldSymbol:      {"symbol", kindString},
	domain.FieldDisplayName: {"name", kindString},
	domain.FieldCreateTime:  {"created_at", kindTime},
	domain.FieldUpdateTime:  {"updated_at", kindTime},
}

// conditions accumulates a WHERE clause and its bound arguments.
//
// Every literal becomes a placeholder; nothing from the request is ever
// interpolated into the SQL text.
type conditions struct {
	columns map[domain.Field]column
	args    []any
}

// build renders p as a SQL boolean expression, appending its literals to args.
func (c *conditions) build(p domain.Predicate) (string, error) {
	switch node := p.(type) {
	case nil:
		return "TRUE", nil

	case domain.And:
		return c.join(node.Operands, " AND ", "TRUE")

	case domain.Or:
		return c.join(node.Operands, " OR ", "FALSE")

	case domain.Not:
		inner, err := c.build(node.Operand)
		if err != nil {
			return "", err
		}
		return "NOT (" + inner + ")", nil

	case domain.Compare:
		return c.compare(node)

	default:
		return "", fmt.Errorf("%w: unsupported filter expression %T", domain.ErrInvalidArgument, p)
	}
}

// join renders operands separated by sep, using identity when there are none.
func (c *conditions) join(operands []domain.Predicate, sep, identity string) (string, error) {
	if len(operands) == 0 {
		return identity, nil
	}
	parts := make([]string, 0, len(operands))
	for _, operand := range operands {
		part, err := c.build(operand)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	if len(parts) == 1 {
		return parts[0], nil
	}
	return "(" + strings.Join(parts, sep) + ")", nil
}

// compare renders a single field-to-literal comparison.
func (c *conditions) compare(cmp domain.Compare) (string, error) {
	col, ok := c.columns[cmp.Field]
	if !ok {
		return "", fmt.Errorf("%w: field %q cannot be filtered on", domain.ErrInvalidArgument, cmp.Field)
	}

	// Comparing against null asks whether BVB reported the field at all. This
	// is the only way to reach dividends with no ex-dividend date, which the
	// Java service's date-range queries excluded outright.
	if _, isNull := cmp.Value.(domain.NullValue); isNull {
		switch cmp.Op {
		case domain.OpEqual:
			return col.name + " IS NULL", nil
		case domain.OpNotEqual:
			return col.name + " IS NOT NULL", nil
		default:
			return "", fmt.Errorf("%w: %s does not support comparing %s against null",
				domain.ErrInvalidArgument, cmp.Op, cmp.Field)
		}
	}

	arg, err := c.literal(col, cmp)
	if err != nil {
		return "", err
	}

	if cmp.Op == domain.OpHas {
		if col.kind != kindString {
			return "", fmt.Errorf("%w: the : operator only applies to text fields, not %s",
				domain.ErrInvalidArgument, cmp.Field)
		}
		// AIP-160's `:` means "contains" on a string. The pattern is built
		// from a placeholder, so the value stays data; only the wildcards
		// themselves are literal SQL.
		return col.name + " ILIKE '%' || " + c.bind(escapeLike(arg.(string))) + " || '%' ESCAPE '\\'", nil
	}

	op, err := sqlOperator(cmp.Op)
	if err != nil {
		return "", err
	}
	return col.name + " " + op + " " + c.bind(arg), nil
}

// literal converts the filter's value to the Go type the column expects,
// rejecting a mismatch rather than letting the database coerce it.
func (c *conditions) literal(col column, cmp domain.Compare) (any, error) {
	mismatch := func() error {
		return fmt.Errorf("%w: %s does not accept the value given for it",
			domain.ErrInvalidArgument, cmp.Field)
	}

	switch value := cmp.Value.(type) {
	case domain.StringValue:
		if col.kind != kindString {
			return nil, mismatch()
		}
		return string(value), nil
	case domain.IntValue:
		if col.kind != kindInt {
			return nil, mismatch()
		}
		return int64(value), nil
	case domain.DateValue:
		if col.kind != kindDate {
			return nil, mismatch()
		}
		return domain.Date(value).Time(), nil
	case domain.TimeValue:
		if col.kind != kindTime {
			return nil, mismatch()
		}
		return time.Time(value), nil
	default:
		return nil, mismatch()
	}
}

// bind appends an argument and returns its placeholder.
func (c *conditions) bind(arg any) string {
	c.args = append(c.args, arg)
	return fmt.Sprintf("$%d", len(c.args))
}

// sqlOperator maps a domain operator onto SQL.
func sqlOperator(op domain.Op) (string, error) {
	switch op {
	case domain.OpEqual:
		return "=", nil
	case domain.OpNotEqual:
		// IS DISTINCT FROM rather than <>, so that "not equal to this date"
		// also matches rows where BVB reported no date. Plain <> evaluates to
		// NULL there and silently drops them.
		return "IS DISTINCT FROM", nil
	case domain.OpLess:
		return "<", nil
	case domain.OpLessOrEqual:
		return "<=", nil
	case domain.OpGreater:
		return ">", nil
	case domain.OpGreaterOrEqual:
		return ">=", nil
	default:
		return "", fmt.Errorf("%w: unsupported operator %s", domain.ErrInvalidArgument, op)
	}
}

// escapeLike neutralises the wildcards in a value used with ILIKE, so that a
// filter for "100%" matches the literal text rather than everything.
func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

// orderClause renders an AIP-132 ordering, always ending in a unique tiebreak
// column so that paging through a result set cannot repeat or skip rows.
func orderClause(terms []domain.OrderTerm, cols map[domain.Field]column, tiebreak string) (string, error) {
	parts := make([]string, 0, len(terms)+1)
	for _, term := range terms {
		col, ok := cols[term.Field]
		if !ok {
			return "", fmt.Errorf("%w: field %q cannot be sorted on", domain.ErrInvalidArgument, term.Field)
		}
		direction := "ASC"
		if term.Descending {
			direction = "DESC"
		}
		// NULLS LAST in both directions: a dividend BVB has not scheduled yet
		// is least interesting either way, and Postgres would otherwise put it
		// first on a descending sort.
		parts = append(parts, col.name+" "+direction+" NULLS LAST")
	}
	parts = append(parts, tiebreak+" ASC")
	return strings.Join(parts, ", "), nil
}
