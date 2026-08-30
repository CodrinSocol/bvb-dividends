package apiserver_test

import (
	"context"
	"fmt"
	"sort"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// The stubs below hold fixtures in memory and implement just enough of the
// repository ports to exercise the HTTP surface. The SQL those ports are
// really backed by is covered by the integration tests in libs/postgres.

type stubCompanies struct{ items []domain.Company }

func (s *stubCompanies) Get(_ context.Context, symbol domain.Symbol) (domain.Company, error) {
	for _, company := range s.items {
		if company.Symbol == symbol {
			return company, nil
		}
	}
	return domain.Company{}, fmt.Errorf("company %s: %w", symbol, domain.ErrNotFound)
}

func (s *stubCompanies) Exists(_ context.Context, symbol domain.Symbol) (bool, error) {
	for _, company := range s.items {
		if company.Symbol == symbol {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubCompanies) List(_ context.Context, query domain.CompanyQuery) (domain.Page[domain.Company], error) {
	return paginate(s.items, query.Page), nil
}

func (s *stubCompanies) Count(context.Context) (int64, error) { return int64(len(s.items)), nil }

func (s *stubCompanies) Upsert(_ context.Context, companies ...domain.Company) error {
	s.items = append(s.items, companies...)
	return nil
}

type stubDividends struct{ items []domain.Dividend }

func (s *stubDividends) Get(_ context.Context, id domain.ID) (domain.Dividend, error) {
	for _, dividend := range s.items {
		if dividend.ID == id {
			return dividend, nil
		}
	}
	return domain.Dividend{}, fmt.Errorf("dividend %s: %w", id, domain.ErrNotFound)
}

// List applies only what the HTTP tests need: the company restriction and the
// one filter shape they assert on. Full filter translation is tested against
// real SQL in libs/postgres, and filter compilation in libs/apiserver/aipquery.
func (s *stubDividends) List(_ context.Context, query domain.DividendQuery) (domain.Page[domain.Dividend], error) {
	matched := make([]domain.Dividend, 0, len(s.items))
	for _, dividend := range s.items {
		if query.Company != nil && dividend.Company != *query.Company {
			continue
		}
		if !matches(dividend, query.Where) {
			continue
		}
		matched = append(matched, dividend)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID.String() < matched[j].ID.String() })
	return paginate(matched, query.Page), nil
}

func (s *stubDividends) Upsert(_ context.Context, dividends ...domain.Dividend) (int, error) {
	s.items = append(s.items, dividends...)
	return len(dividends), nil
}

// matches evaluates the small subset of predicates the HTTP tests use.
func matches(dividend domain.Dividend, where domain.Predicate) bool {
	switch node := where.(type) {
	case nil:
		return true
	case domain.And:
		for _, operand := range node.Operands {
			if !matches(dividend, operand) {
				return false
			}
		}
		return true
	case domain.Compare:
		if node.Field == domain.FieldExDividendDate {
			if _, isNull := node.Value.(domain.NullValue); isNull {
				present := dividend.Schedule.ExDividendDate.Valid()
				return (node.Op == domain.OpEqual) != present
			}
		}
		return true
	default:
		return true
	}
}

// paginate slices items the way the repositories do, reporting the full total.
func paginate[T any](items []T, page domain.PageRequest) domain.Page[T] {
	total := int64(len(items))
	start := page.Offset
	if start > total {
		start = total
	}
	end := start + int64(page.Size)
	if end > total {
		end = total
	}
	return domain.Page[T]{Items: items[start:end], Total: total}
}
