package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/postgres/sqlcgen"
)

// CompanyRepository stores companies in PostgreSQL.
type CompanyRepository struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

// NewCompanyRepository returns a repository backed by pool.
func NewCompanyRepository(pool *pgxpool.Pool) *CompanyRepository {
	return &CompanyRepository{pool: pool, queries: sqlcgen.New(pool)}
}

var _ domain.CompanyRepository = (*CompanyRepository)(nil)

// Get returns the company with the given symbol.
func (r *CompanyRepository) Get(ctx context.Context, symbol domain.Symbol) (domain.Company, error) {
	row, err := r.queries.GetCompany(ctx, string(symbol))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Company{}, fmt.Errorf("company %s: %w", symbol, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Company{}, fmt.Errorf("get company %s: %w", symbol, err)
	}
	return companyFromRow(row), nil
}

// Exists reports whether a company with the given symbol is stored.
func (r *CompanyRepository) Exists(ctx context.Context, symbol domain.Symbol) (bool, error) {
	exists, err := r.queries.CompanyExists(ctx, string(symbol))
	if err != nil {
		return false, fmt.Errorf("check company %s: %w", symbol, err)
	}
	return exists, nil
}

// Count returns the number of stored companies.
func (r *CompanyRepository) Count(ctx context.Context) (int64, error) {
	count, err := r.queries.CountCompanies(ctx)
	if err != nil {
		return 0, fmt.Errorf("count companies: %w", err)
	}
	return count, nil
}

// Upsert writes companies, refreshing the name of any that already exist.
func (r *CompanyRepository) Upsert(ctx context.Context, companies ...domain.Company) error {
	if len(companies) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := r.queries.WithTx(tx)
	for _, company := range companies {
		if err := company.Validate(); err != nil {
			return fmt.Errorf("company %s: %w", company.Symbol, err)
		}
		err := queries.UpsertCompany(ctx, sqlcgen.UpsertCompanyParams{
			Symbol: string(company.Symbol),
			Name:   company.Name,
		})
		if err != nil {
			return fmt.Errorf("upsert company %s: %w", company.Symbol, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit companies: %w", err)
	}
	return nil
}

// List returns one page of companies matching q.
func (r *CompanyRepository) List(ctx context.Context, q domain.CompanyQuery) (domain.Page[domain.Company], error) {
	var empty domain.Page[domain.Company]

	where := conditions{columns: companyColumns}
	clause := "TRUE"
	if q.Where != nil {
		built, err := where.build(q.Where)
		if err != nil {
			return empty, err
		}
		clause = built
	}

	order := q.OrderBy
	if len(order) == 0 {
		order = []domain.OrderTerm{{Field: domain.FieldSymbol}}
	}
	orderBy, err := orderClause(order, companyColumns, "symbol")
	if err != nil {
		return empty, err
	}

	sql := fmt.Sprintf(
		`SELECT symbol, name, created_at, updated_at, count(*) OVER () AS total_size
		 FROM company
		 WHERE %s
		 ORDER BY %s
		 LIMIT %s OFFSET %s`,
		clause,
		orderBy,
		where.bind(int64(q.Page.Size)),
		where.bind(q.Page.Offset),
	)

	rows, err := r.pool.Query(ctx, sql, where.args...)
	if err != nil {
		return empty, fmt.Errorf("list companies: %w", err)
	}
	defer rows.Close()

	page := domain.Page[domain.Company]{Items: make([]domain.Company, 0, q.Page.Size)}
	for rows.Next() {
		var row sqlcgen.Company
		var total int64
		if err := rows.Scan(&row.Symbol, &row.Name, &row.CreatedAt, &row.UpdatedAt, &total); err != nil {
			return empty, fmt.Errorf("scan company: %w", err)
		}
		page.Items = append(page.Items, companyFromRow(row))
		page.Total = total
	}
	if err := rows.Err(); err != nil {
		return empty, fmt.Errorf("list companies: %w", err)
	}
	return page, nil
}

// companyFromRow converts a database row to the domain entity.
func companyFromRow(row sqlcgen.Company) domain.Company {
	return domain.Company{
		Symbol:    domain.Symbol(row.Symbol),
		Name:      row.Name,
		CreatedAt: toTime(row.CreatedAt),
		UpdatedAt: toTime(row.UpdatedAt),
	}
}
