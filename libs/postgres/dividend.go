package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
	"github.com/CodrinSocol/bvb-dividends-ro/libs/postgres/sqlcgen"
)

// dividendColumnList is the projection every dividend read shares.
const dividendColumnList = `id, company_symbol, year, dividend_type,
	gross_per_share_natural_person, gross_per_share_legal_person, total_amount,
	announcement_date, gms_reference_date, gms_date, record_date,
	ex_dividend_date, payment_start_date, payment_end_date,
	distribution_method, created_at, updated_at`

// DividendRepository stores dividends in PostgreSQL.
type DividendRepository struct {
	pool    *pgxpool.Pool
	queries *sqlcgen.Queries
}

// NewDividendRepository returns a repository backed by pool.
func NewDividendRepository(pool *pgxpool.Pool) *DividendRepository {
	return &DividendRepository{pool: pool, queries: sqlcgen.New(pool)}
}

var _ domain.DividendRepository = (*DividendRepository)(nil)

// Get returns the dividend with the given ID.
func (r *DividendRepository) Get(ctx context.Context, id domain.ID) (domain.Dividend, error) {
	row, err := r.queries.GetDividend(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Dividend{}, fmt.Errorf("dividend %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Dividend{}, fmt.Errorf("get dividend %s: %w", id, err)
	}
	return dividendFromRow(row), nil
}

// Upsert writes dividends, returning how many rows it actually changed.
//
// The whole batch runs in one transaction so a company's dividends are never
// half-written. Failures are isolated per company by the caller, which starts
// one call per company rather than one call for the whole import.
func (r *DividendRepository) Upsert(ctx context.Context, dividends ...domain.Dividend) (int, error) {
	if len(dividends) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	queries := r.queries.WithTx(tx)
	changed := 0
	for _, dividend := range dividends {
		rows, err := queries.UpsertDividend(ctx, upsertParams(dividend))
		if err != nil {
			return 0, fmt.Errorf("upsert dividend %s: %w", dividend.ID, err)
		}
		changed += int(rows)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit dividends: %w", err)
	}
	return changed, nil
}

// List returns one page of dividends matching q, together with the total the
// query matched.
//
// The filter is compiled to a parameterised WHERE clause; see query_builder.go.
// The total comes from a window function in the same statement rather than a
// second COUNT query, so the count and the page are consistent with each other.
func (r *DividendRepository) List(ctx context.Context, q domain.DividendQuery) (domain.Page[domain.Dividend], error) {
	var empty domain.Page[domain.Dividend]

	where := conditions{columns: dividendColumns}
	clauses := make([]string, 0, 2)

	// A nil company is the AIP-159 `companies/-` wildcard: every company.
	if q.Company != nil {
		clauses = append(clauses, "company_symbol = "+where.bind(string(*q.Company)))
	}
	if q.Where != nil {
		clause, err := where.build(q.Where)
		if err != nil {
			return empty, err
		}
		clauses = append(clauses, clause)
	}
	if len(clauses) == 0 {
		clauses = append(clauses, "TRUE")
	}

	order := q.OrderBy
	if len(order) == 0 {
		order = []domain.OrderTerm{{Field: domain.FieldExDividendDate, Descending: true}}
	}
	orderBy, err := orderClause(order, dividendColumns, "id")
	if err != nil {
		return empty, err
	}

	sql := fmt.Sprintf(
		`SELECT %s, count(*) OVER () AS total_size
		 FROM dividend
		 WHERE %s
		 ORDER BY %s
		 LIMIT %s OFFSET %s`,
		dividendColumnList,
		strings.Join(clauses, " AND "),
		orderBy,
		where.bind(int64(q.Page.Size)),
		where.bind(q.Page.Offset),
	)

	rows, err := r.pool.Query(ctx, sql, where.args...)
	if err != nil {
		return empty, fmt.Errorf("list dividends: %w", err)
	}
	defer rows.Close()

	page := domain.Page[domain.Dividend]{Items: make([]domain.Dividend, 0, q.Page.Size)}
	for rows.Next() {
		var row sqlcgen.Dividend
		var total int64
		if err := rows.Scan(
			&row.ID, &row.CompanySymbol, &row.Year, &row.DividendType,
			&row.GrossPerShareNaturalPerson, &row.GrossPerShareLegalPerson, &row.TotalAmount,
			&row.AnnouncementDate, &row.GmsReferenceDate, &row.GmsDate, &row.RecordDate,
			&row.ExDividendDate, &row.PaymentStartDate, &row.PaymentEndDate,
			&row.DistributionMethod, &row.CreatedAt, &row.UpdatedAt,
			&total,
		); err != nil {
			return empty, fmt.Errorf("scan dividend: %w", err)
		}
		page.Items = append(page.Items, dividendFromRow(row))
		page.Total = total
	}
	if err := rows.Err(); err != nil {
		return empty, fmt.Errorf("list dividends: %w", err)
	}
	return page, nil
}

// dividendFromRow converts a database row to the domain aggregate.
func dividendFromRow(row sqlcgen.Dividend) domain.Dividend {
	return domain.Dividend{
		ID:      domain.ID(row.ID),
		Company: domain.Symbol(row.CompanySymbol),
		Year:    int(row.Year),
		Type:    row.DividendType,
		Amounts: domain.Amounts{
			GrossPerShareNaturalPerson: toAmount(row.GrossPerShareNaturalPerson),
			GrossPerShareLegalPerson:   toAmount(row.GrossPerShareLegalPerson),
			Total:                      toAmount(row.TotalAmount),
		},
		Schedule: domain.Schedule{
			AnnouncementDate: toDate(row.AnnouncementDate),
			GMSReferenceDate: toDate(row.GmsReferenceDate),
			GMSDate:          toDate(row.GmsDate),
			RecordDate:       toDate(row.RecordDate),
			ExDividendDate:   toDate(row.ExDividendDate),
			PaymentStartDate: toDate(row.PaymentStartDate),
			PaymentEndDate:   toDate(row.PaymentEndDate),
		},
		DistributionMethod: row.DistributionMethod,
		CreatedAt:          toTime(row.CreatedAt),
		UpdatedAt:          toTime(row.UpdatedAt),
	}
}

// upsertParams converts the domain aggregate to insert parameters.
func upsertParams(d domain.Dividend) sqlcgen.UpsertDividendParams {
	return sqlcgen.UpsertDividendParams{
		ID:                         uuid.UUID(d.ID),
		CompanySymbol:              string(d.Company),
		Year:                       int32(d.Year), //nolint:gosec // BVB fiscal years are four digits.
		DividendType:               d.Type,
		GrossPerShareNaturalPerson: fromAmount(d.Amounts.GrossPerShareNaturalPerson),
		GrossPerShareLegalPerson:   fromAmount(d.Amounts.GrossPerShareLegalPerson),
		TotalAmount:                fromAmount(d.Amounts.Total),
		AnnouncementDate:           fromDate(d.Schedule.AnnouncementDate),
		GmsReferenceDate:           fromDate(d.Schedule.GMSReferenceDate),
		GmsDate:                    fromDate(d.Schedule.GMSDate),
		RecordDate:                 fromDate(d.Schedule.RecordDate),
		ExDividendDate:             fromDate(d.Schedule.ExDividendDate),
		PaymentStartDate:           fromDate(d.Schedule.PaymentStartDate),
		PaymentEndDate:             fromDate(d.Schedule.PaymentEndDate),
		DistributionMethod:         d.DistributionMethod,
	}
}
