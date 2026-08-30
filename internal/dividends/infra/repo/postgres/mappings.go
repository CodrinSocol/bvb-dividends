package postgres

import (
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/infra/repo/postgres/gen"
)

// The conversions between the database's representation and the domain's.
//
// They exist so the entities never import pgx: a value object decides what
// "absent" means, and this file is the only place that knows PostgreSQL spells
// it NULL.

// toAmount converts a numeric column to an [common.Amount], mapping NULL to an
// absent amount rather than to zero.
func toAmount(n pgtype.Numeric) common.Amount {
	if !n.Valid || n.NaN || n.Int == nil {
		return common.NoAmount
	}

	return common.NewAmount(decimal.NewFromBigInt(n.Int, n.Exp))
}

// fromAmount converts an [common.Amount] to a numeric column value.
func fromAmount(a common.Amount) pgtype.Numeric {
	if !a.Valid() {
		return pgtype.Numeric{Valid: false}
	}

	d := a.Decimal()

	return pgtype.Numeric{
		Int:   new(big.Int).Set(d.Coefficient()),
		Exp:   d.Exponent(),
		Valid: true,
	}
}

// toDate converts a date column to a [common.Date], mapping NULL to an absent
// date.
func toDate(d pgtype.Date) common.Date {
	if !d.Valid || d.InfinityModifier != pgtype.Finite {
		return common.NoDate
	}

	return common.DateOf(d.Time)
}

// fromDate converts a [common.Date] to a date column value.
func fromDate(d common.Date) pgtype.Date {
	if !d.Valid() {
		return pgtype.Date{Valid: false}
	}

	return pgtype.Date{Time: d.Time(), Valid: true}
}

// toDividend converts a stored row into the entity.
func toDividend(row gen.DividendDividendsFilterable) *dividends.Dividend {
	return &dividends.Dividend{
		ID:      dividends.ID(row.ID),
		Company: common.Symbol(row.CompanySymbol),
		Year:    int(row.Year),
		Type:    row.DividendType,
		Amounts: dividends.Amounts{
			GrossPerShareNaturalPerson: toAmount(row.GrossPerShareNaturalPerson),
			GrossPerShareLegalPerson:   toAmount(row.GrossPerShareLegalPerson),
			Total:                      toAmount(row.TotalAmount),
		},
		Schedule: dividends.Schedule{
			AnnouncementDate: toDate(row.ScheduleAnnouncementDate),
			GMSReferenceDate: toDate(row.ScheduleGmsReferenceDate),
			GMSDate:          toDate(row.ScheduleGmsDate),
			RecordDate:       toDate(row.ScheduleRecordDate),
			ExDividendDate:   toDate(row.ScheduleExDividendDate),
			PaymentStartDate: toDate(row.SchedulePaymentStartDate),
			PaymentEndDate:   toDate(row.SchedulePaymentEndDate),
		},
		DistributionMethod: row.DistributionMethod,
		State:              dividends.State(row.State),
		CreateTime:         row.CreateTime,
		UpdateTime:         row.UpdateTime,
	}
}

func toDividends(rows []gen.DividendDividendsFilterable) []*dividends.Dividend {
	result := make([]*dividends.Dividend, len(rows))
	for i, row := range rows {
		result[i] = toDividend(row)
	}

	return result
}

// toUpsertParams converts an entity into the row to write.
func toUpsertParams(dividend *dividends.Dividend) gen.UpsertDividendParams {
	return gen.UpsertDividendParams{
		ID:            dividend.ID.UUID(),
		CompanySymbol: dividend.Company.String(),
		//nolint:gosec // Fiscal years are four digits.
		Year:                       int32(dividend.Year),
		DividendType:               dividend.Type,
		GrossPerShareNaturalPerson: fromAmount(dividend.Amounts.GrossPerShareNaturalPerson),
		GrossPerShareLegalPerson:   fromAmount(dividend.Amounts.GrossPerShareLegalPerson),
		TotalAmount:                fromAmount(dividend.Amounts.Total),
		ScheduleAnnouncementDate:   fromDate(dividend.Schedule.AnnouncementDate),
		ScheduleGmsReferenceDate:   fromDate(dividend.Schedule.GMSReferenceDate),
		ScheduleGmsDate:            fromDate(dividend.Schedule.GMSDate),
		ScheduleRecordDate:         fromDate(dividend.Schedule.RecordDate),
		ScheduleExDividendDate:     fromDate(dividend.Schedule.ExDividendDate),
		SchedulePaymentStartDate:   fromDate(dividend.Schedule.PaymentStartDate),
		SchedulePaymentEndDate:     fromDate(dividend.Schedule.PaymentEndDate),
		DistributionMethod:         dividend.DistributionMethod,
	}
}
