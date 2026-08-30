package aipquery_test

import (
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/apiserver/aipquery"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// FuzzCompileFilter checks that no filter string, however malformed, can make
// the compiler panic or produce a predicate naming a field the SQL layer does
// not know how to map. Anything the compiler accepts must be safe to compile
// to SQL; anything else must be a clean error.
func FuzzCompileFilter(f *testing.F) {
	seeds := []string{
		"",
		`year = 2024`,
		`company = "SNP" AND schedule.ex_dividend_date > "2026-01-01"`,
		`schedule.ex_dividend_date = null`,
		`NOT (year < 2000 OR year > 2030)`,
		`dividend_type : "cash"`,
		`create_time > timestamp("2026-01-01T00:00:00Z")`,
		`year = 1) OR 1=1 --`,
		`company = "'; DROP TABLE dividend; --"`,
		`((((((((((`,
		"\x00\xff",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	compiler, err := aipquery.NewDividendCompiler()
	if err != nil {
		f.Fatalf("NewDividendCompiler: %v", err)
	}

	// Every field the compiler is allowed to emit.
	allowed := map[domain.Field]bool{
		domain.FieldCompany:          true,
		domain.FieldYear:             true,
		domain.FieldDividendType:     true,
		domain.FieldAnnouncementDate: true,
		domain.FieldGMSReferenceDate: true,
		domain.FieldGMSDate:          true,
		domain.FieldRecordDate:       true,
		domain.FieldExDividendDate:   true,
		domain.FieldPaymentStartDate: true,
		domain.FieldPaymentEndDate:   true,
		domain.FieldCreateTime:       true,
		domain.FieldUpdateTime:       true,
	}

	f.Fuzz(func(t *testing.T, filter string) {
		predicate, err := compiler.CompileFilter(filter)
		if err != nil {
			return // A rejected filter is a correct outcome.
		}
		walk(t, predicate, allowed)
	})
}

// walk asserts that every comparison in the tree names an allowed field.
func walk(t *testing.T, p domain.Predicate, allowed map[domain.Field]bool) {
	t.Helper()
	switch node := p.(type) {
	case nil:
	case domain.And:
		for _, operand := range node.Operands {
			walk(t, operand, allowed)
		}
	case domain.Or:
		for _, operand := range node.Operands {
			walk(t, operand, allowed)
		}
	case domain.Not:
		walk(t, node.Operand, allowed)
	case domain.Compare:
		if !allowed[node.Field] {
			t.Errorf("compiler emitted undeclared field %q", node.Field)
		}
	default:
		t.Errorf("compiler emitted unknown predicate type %T", p)
	}
}
