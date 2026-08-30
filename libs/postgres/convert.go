package postgres

import (
	"math/big"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// Conversions between the database's representation and the domain's.
//
// They exist so the domain never imports pgx: a value object decides what
// "absent" means, and this file is the only place that knows Postgres spells
// it NULL.

// toAmount converts a numeric column to an Amount, mapping NULL to an absent
// amount rather than to zero.
func toAmount(n pgtype.Numeric) domain.Amount {
	if !n.Valid || n.NaN || n.Int == nil {
		return domain.NoAmount
	}
	return domain.NewAmount(decimal.NewFromBigInt(n.Int, n.Exp))
}

// fromAmount converts an Amount to a numeric column value.
func fromAmount(a domain.Amount) pgtype.Numeric {
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

// toDate converts a date column to a Date, mapping NULL to an absent date.
func toDate(d pgtype.Date) domain.Date {
	if !d.Valid || d.InfinityModifier != pgtype.Finite {
		return domain.NoDate
	}
	return domain.DateOf(d.Time)
}

// fromDate converts a Date to a date column value.
func fromDate(d domain.Date) pgtype.Date {
	if !d.Valid() {
		return pgtype.Date{Valid: false}
	}
	return pgtype.Date{Time: d.Time(), Valid: true}
}

// toTime converts a timestamptz column to a time.Time in UTC.
func toTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}
