package domain

import "time"

// Field names an attribute that may be filtered or sorted on.
//
// It is a closed set: the AIP-160 filter parser and the AIP-132 ordering
// parser only ever produce these values, so a caller cannot name a column that
// is not meant to be queryable, and the SQL layer can map every field it sees.
type Field string

// Fields of a dividend.
const (
	FieldCompany          Field = "company"
	FieldYear             Field = "year"
	FieldDividendType     Field = "dividend_type"
	FieldAnnouncementDate Field = "schedule.announcement_date"
	FieldGMSReferenceDate Field = "schedule.gms_reference_date"
	FieldGMSDate          Field = "schedule.gms_date"
	FieldRecordDate       Field = "schedule.record_date"
	FieldExDividendDate   Field = "schedule.ex_dividend_date"
	FieldPaymentStartDate Field = "schedule.payment_start_date"
	FieldPaymentEndDate   Field = "schedule.payment_end_date"
)

// Fields of a company.
const (
	FieldSymbol      Field = "symbol"
	FieldDisplayName Field = "display_name"
)

// Fields shared by both resources.
const (
	FieldCreateTime Field = "create_time"
	FieldUpdateTime Field = "update_time"
)

// Op is a comparison operator in a filter expression.
type Op uint8

// The comparison operators AIP-160 defines, as this API supports them.
const (
	OpEqual Op = iota + 1
	OpNotEqual
	OpLess
	OpLessOrEqual
	OpGreater
	OpGreaterOrEqual

	// OpHas is AIP-160's `:` operator. On the string fields this API exposes
	// it means "contains, case-insensitively".
	OpHas
)

// String returns the operator as it appears in a filter expression.
func (o Op) String() string {
	switch o {
	case OpEqual:
		return "="
	case OpNotEqual:
		return "!="
	case OpLess:
		return "<"
	case OpLessOrEqual:
		return "<="
	case OpGreater:
		return ">"
	case OpGreaterOrEqual:
		return ">="
	case OpHas:
		return ":"
	default:
		return "?"
	}
}

// Value is a literal on the right-hand side of a comparison.
type Value interface{ isValue() }

// StringValue is a quoted or bare string literal.
type StringValue string

// IntValue is an integer literal.
type IntValue int64

// DateValue is a YYYY-MM-DD literal compared against a calendar date field.
type DateValue Date

// TimeValue is an RFC 3339 literal compared against a timestamp field.
type TimeValue time.Time

// NullValue is the `null` literal, used to match fields BVB has not reported.
// It is the only way to reach dividends with no ex-dividend date, which the
// Java service's date-range queries excluded entirely.
type NullValue struct{}

func (StringValue) isValue() {}
func (IntValue) isValue()    {}
func (DateValue) isValue()   {}
func (TimeValue) isValue()   {}
func (NullValue) isValue()   {}

// Predicate is a node in a filter expression tree.
type Predicate interface{ isPredicate() }

// And matches when every operand matches. An empty And matches everything.
type And struct{ Operands []Predicate }

// Or matches when any operand matches. An empty Or matches nothing.
type Or struct{ Operands []Predicate }

// Not inverts its operand.
type Not struct{ Operand Predicate }

// Compare matches a single field against a literal.
type Compare struct {
	Field Field
	Op    Op
	Value Value
}

func (And) isPredicate()     {}
func (Or) isPredicate()      {}
func (Not) isPredicate()     {}
func (Compare) isPredicate() {}

// OrderTerm is one clause of an AIP-132 order_by expression.
type OrderTerm struct {
	Field      Field
	Descending bool
}

// PageRequest is a resolved slice of a result set.
//
// It is an offset and a size rather than a page number: the API hands out
// opaque AIP-158 page tokens, and the offset is what a decoded token carries.
type PageRequest struct {
	Size   int
	Offset int64
}

// Page is one page of results together with the total the query matched.
//
// Total is carried because AIP-158 allows a total_size on the response, and
// because the Java service computed it and then discarded it before returning.
type Page[T any] struct {
	Items []T
	Total int64
}

// DividendQuery selects and orders dividends.
type DividendQuery struct {
	// Company restricts results to one company. A nil Company means every
	// company, which is what the AIP-159 `companies/-` wildcard resolves to.
	Company *Symbol

	// Where is the filter expression, or nil to match everything.
	Where Predicate

	// OrderBy is applied in order. An empty OrderBy uses the repository's
	// default ordering.
	OrderBy []OrderTerm

	Page PageRequest
}

// CompanyQuery selects and orders companies.
type CompanyQuery struct {
	Where   Predicate
	OrderBy []OrderTerm
	Page    PageRequest
}
