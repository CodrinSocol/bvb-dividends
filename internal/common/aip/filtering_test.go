//nolint:testpackage // The declarations and the rewrites under test are unexported.
package aip

import (
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/observeinc/cel2sql/v3"
	"go.einride.tech/aip/filtering"

	bvbdividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/go/gen/v1"
)

// filterRequest is the smallest thing einride's parser accepts.
type filterRequest struct{ filter string }

func (r filterRequest) GetFilter() string { return r.filter }

// toSQL runs the whole pipeline the interceptor runs: parse and type-check
// against the resource, rewrite into the CEL dialect cel2sql understands, and
// convert.
func toSQL(t *testing.T, filter string) (string, error) {
	t.Helper()

	declarations, err := deriveDeclarations(&bvbdividendsv1.Dividend{})
	if err != nil {
		t.Fatalf("deriveDeclarations: %v", err)
	}

	parsed, err := filtering.ParseFilter(filterRequest{filter}, declarations)
	if err != nil {
		return "", err
	}

	checked := parsed.CheckedExpr
	normalizeAIPtoCEL(checked.GetExpr(), checked.GetTypeMap())
	rewriteNullComparisons(checked.GetExpr())
	flattenIdents(checked.GetExpr())

	return cel2sql.Convert(cel.CheckedExprToAst(checked))
}

func TestFilterCompilesToSQL(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		filter string
		want   string
	}{
		"integer comparison": {
			filter: `year >= 2024`,
			want:   `year >= 2024`,
		},
		"string equality": {
			filter: `dividend_type = "cash"`,
			want:   `dividend_type = 'cash'`,
		},
		// A nested proto field is addressed with a dot, but a dot in SQL is a
		// table qualifier, so it has to arrive as the single column that holds
		// it.
		"nested date": {
			filter: `schedule.ex_dividend_date > "2026-01-01"`,
			want:   `schedule_ex_dividend_date > '2026-01-01'`,
		},
		// AIP-160's `:` means "contains" on a scalar string.
		"has operator": {
			filter: `dividend_type : "cas"`,
			want:   `POSITION('cas' IN dividend_type) > 0`,
		},
		"timestamp comparison": {
			filter: `create_time > "2026-01-01T00:00:00Z"`,
			want:   `create_time > '2026-01-01T00:00:00Z'`,
		},
		"conjunction": {
			filter: `year >= 2024 AND dividend_type = "cash"`,
			want:   `year >= 2024 AND dividend_type = 'cash'`,
		},
		"disjunction": {
			filter: `year = 2024 OR year = 2025`,
			want:   `year = 2024 OR year = 2025`,
		},
		"negation": {
			filter: `NOT dividend_type = "cash"`,
			want:   `NOT (dividend_type = 'cash')`,
		},
		// The only way to reach a dividend BVB announced but has not dated.
		// The previous service's date-range queries excluded those entirely.
		"unreported value": {
			filter: `schedule.ex_dividend_date = null`,
			want:   `schedule_ex_dividend_date IS NULL`,
		},
		"reported value": {
			filter: `schedule.ex_dividend_date != null`,
			want:   `schedule_ex_dividend_date IS NOT NULL`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := toSQL(t, tc.filter)
			if err != nil {
				t.Fatalf("%s: %v", tc.filter, err)
			}
			if got != tc.want {
				t.Errorf("%s\n got: %s\nwant: %s", tc.filter, got, tc.want)
			}
		})
	}
}

// A field the resource does not declare must be refused while it is still a
// filter expression. Nothing a caller writes reaches SQL as an identifier, so a
// filter cannot name a column, let alone inject one.
func TestFilterRejectsWhatTheResourceDoesNotDeclare(t *testing.T) {
	t.Parallel()

	for name, filter := range map[string]string{
		"unknown field":           `settlement_date > "2026-01-01"`,
		"unknown nested field":    `schedule.settlement_date > "2026-01-01"`,
		"wrong type":              `year > "not a number"`,
		"amounts are not columns": `total_amount > 1`,
		"malformed expression":    `year >=`,
		"injection attempt":       `year = 1; DROP TABLE dividend.dividends`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if sql, err := toSQL(t, filter); err == nil {
				t.Errorf("%s was accepted and compiled to %s", filter, sql)
			}
		})
	}
}

// The empty filter is not an error; it matches everything.
func TestEmptyFilter(t *testing.T) {
	t.Parallel()

	declarations, err := deriveDeclarations(&bvbdividendsv1.Dividend{})
	if err != nil {
		t.Fatalf("deriveDeclarations: %v", err)
	}

	parsed, err := filtering.ParseFilter(filterRequest{""}, declarations)
	if err != nil {
		t.Fatalf("ParseFilter(\"\"): %v", err)
	}
	if parsed.CheckedExpr != nil {
		t.Error("the empty filter produced an expression")
	}
}

func TestColumnOf(t *testing.T) {
	t.Parallel()

	for path, want := range map[string]string{
		"year":                      "year",
		"schedule.ex_dividend_date": "schedule_ex_dividend_date",
	} {
		if got := ColumnOf(path); got != want {
			t.Errorf("ColumnOf(%q) = %q, want %q", path, got, want)
		}
	}
}
