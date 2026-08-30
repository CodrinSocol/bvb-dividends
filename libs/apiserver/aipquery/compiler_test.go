package aipquery_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/apiserver/aipquery"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

func dividendCompiler(t *testing.T) *aipquery.Compiler {
	t.Helper()
	c, err := aipquery.NewDividendCompiler()
	if err != nil {
		t.Fatalf("NewDividendCompiler: %v", err)
	}
	return c
}

func mustDate(t *testing.T, s string) domain.Date {
	t.Helper()
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

func TestCompileFilter(t *testing.T) {
	c := dividendCompiler(t)

	tests := []struct {
		name   string
		filter string
		want   domain.Predicate
	}{
		{
			name:   "empty filter matches everything",
			filter: "   ",
			want:   nil,
		},
		{
			name:   "integer comparison",
			filter: `year >= 2024`,
			want:   domain.Compare{Field: domain.FieldYear, Op: domain.OpGreaterOrEqual, Value: domain.IntValue(2024)},
		},
		{
			name:   "string equality",
			filter: `company = "SNP"`,
			want:   domain.Compare{Field: domain.FieldCompany, Op: domain.OpEqual, Value: domain.StringValue("SNP")},
		},
		{
			name:   "nested date path is a single field",
			filter: `schedule.ex_dividend_date > "2026-01-01"`,
			want: domain.Compare{
				Field: domain.FieldExDividendDate,
				Op:    domain.OpGreater,
				Value: domain.DateValue(mustDate(t, "2026-01-01")),
			},
		},
		{
			name:   "null asks whether BVB reported the field",
			filter: `schedule.ex_dividend_date = null`,
			want:   domain.Compare{Field: domain.FieldExDividendDate, Op: domain.OpEqual, Value: domain.NullValue{}},
		},
		{
			name:   "has operator",
			filter: `dividend_type : "cash"`,
			want:   domain.Compare{Field: domain.FieldDividendType, Op: domain.OpHas, Value: domain.StringValue("cash")},
		},
		{
			name:   "negation",
			filter: `NOT company = "SNP"`,
			want: domain.Not{Operand: domain.Compare{
				Field: domain.FieldCompany, Op: domain.OpEqual, Value: domain.StringValue("SNP"),
			}},
		},
		{
			name:   "conjunction",
			filter: `year >= 2024 AND company = "SNP"`,
			want: domain.And{Operands: []domain.Predicate{
				domain.Compare{Field: domain.FieldYear, Op: domain.OpGreaterOrEqual, Value: domain.IntValue(2024)},
				domain.Compare{Field: domain.FieldCompany, Op: domain.OpEqual, Value: domain.StringValue("SNP")},
			}},
		},
		{
			name:   "disjunction",
			filter: `company = "SNP" OR company = "TLV"`,
			want: domain.Or{Operands: []domain.Predicate{
				domain.Compare{Field: domain.FieldCompany, Op: domain.OpEqual, Value: domain.StringValue("SNP")},
				domain.Compare{Field: domain.FieldCompany, Op: domain.OpEqual, Value: domain.StringValue("TLV")},
			}},
		},
		{
			name:   "timestamp literal",
			filter: `create_time > timestamp("2026-01-01T00:00:00Z")`,
			want: domain.Compare{
				Field: domain.FieldCreateTime,
				Op:    domain.OpGreater,
				Value: domain.TimeValue(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.CompileFilter(tt.filter)
			if err != nil {
				t.Fatalf("CompileFilter(%q): %v", tt.filter, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CompileFilter(%q)\n got: %#v\nwant: %#v", tt.filter, got, tt.want)
			}
		})
	}
}

// A filter is caller-supplied text that ends up shaping a SQL query, so
// anything that is not a well-formed expression over a declared field must be
// refused outright rather than partially interpreted.
func TestCompileFilterRejectsBadInput(t *testing.T) {
	c := dividendCompiler(t)

	tests := map[string]string{
		"sql injection through a value":      `year = 1) OR 1=1 --`,
		"sql injection through quoting":      `company = "SNP'; DROP TABLE dividend; --"` + ` AND year = 1)`,
		"trailing sql comment":               `company = "SNP" --`,
		"undeclared field":                   `secret_column = "x"`,
		"field not exposed for filtering":    `id = "abc"`,
		"nested field that does not exist":   `schedule.settlement_date > "2026-01-01"`,
		"type mismatch, text against year":   `year = "many"`,
		"type mismatch, number against text": `company = 42`,
		"malformed date":                     `schedule.ex_dividend_date > "17/04/2026"`,
		"malformed timestamp":                `create_time > "not-a-time"`,
		"bare unquoted value":                `company = SNP`,
		"unbalanced parenthesis":             `(year = 2024`,
		"empty comparison":                   `year =`,
	}

	for name, filter := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := c.CompileFilter(filter)
			if err == nil {
				t.Fatalf("CompileFilter(%q) succeeded, returning %#v; want an error", filter, got)
			}
			if !errors.Is(err, domain.ErrInvalidArgument) {
				t.Errorf("CompileFilter(%q) error = %v, want ErrInvalidArgument", filter, err)
			}
		})
	}
}

// A quoted string is data. It must survive compilation intact so it can be
// bound as a parameter, not be rejected or mangled for looking like SQL.
func TestCompileFilterKeepsQuotedTextAsData(t *testing.T) {
	c := dividendCompiler(t)

	const hostile = `'; DROP TABLE dividend; --`
	got, err := c.CompileFilter(`dividend_type = "` + hostile + `"`)
	if err != nil {
		t.Fatalf("CompileFilter: %v", err)
	}

	want := domain.Compare{
		Field: domain.FieldDividendType,
		Op:    domain.OpEqual,
		Value: domain.StringValue(hostile),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestCompileOrderBy(t *testing.T) {
	c := dividendCompiler(t)

	got, err := c.CompileOrderBy("schedule.ex_dividend_date desc, year")
	if err != nil {
		t.Fatalf("CompileOrderBy: %v", err)
	}
	want := []domain.OrderTerm{
		{Field: domain.FieldExDividendDate, Descending: true},
		{Field: domain.FieldYear},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	empty, err := c.CompileOrderBy("")
	if err != nil {
		t.Fatalf("CompileOrderBy(\"\"): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("empty order_by produced %d terms, want none", len(empty))
	}
}

func TestCompileOrderByRejectsUnknownField(t *testing.T) {
	c := dividendCompiler(t)

	for _, orderBy := range []string{"secret_column", "id desc", "year sideways"} {
		if _, err := c.CompileOrderBy(orderBy); !errors.Is(err, domain.ErrInvalidArgument) {
			t.Errorf("CompileOrderBy(%q) error = %v, want ErrInvalidArgument", orderBy, err)
		}
	}
}

func TestCompanyCompilerHasItsOwnFields(t *testing.T) {
	c, err := aipquery.NewCompanyCompiler()
	if err != nil {
		t.Fatalf("NewCompanyCompiler: %v", err)
	}

	if _, err := c.CompileFilter(`symbol = "SNP"`); err != nil {
		t.Errorf("company filter on symbol: %v", err)
	}
	// A dividend field must not be reachable through the company compiler.
	if _, err := c.CompileFilter(`schedule.ex_dividend_date = null`); !errors.Is(err, domain.ErrInvalidArgument) {
		t.Errorf("company filter accepted a dividend field: %v", err)
	}
}
