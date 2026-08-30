package apiserver

import (
	"time"

	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
	dividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/genproto/bvb/dividends/v1"
)

// toCompanyProto renders a company as its API resource.
func toCompanyProto(company domain.Company) *dividendsv1.Company {
	return &dividendsv1.Company{
		Name:        companyName(company.Symbol),
		Symbol:      company.Symbol.String(),
		DisplayName: company.Name,
		CreateTime:  toTimestamp(company.CreatedAt),
		UpdateTime:  toTimestamp(company.UpdatedAt),
	}
}

// toDividendProto renders a dividend as its API resource, deriving the
// lifecycle state as of now.
func toDividendProto(dividend domain.Dividend, now time.Time) *dividendsv1.Dividend {
	return &dividendsv1.Dividend{
		Name:                       dividendName(dividend.Company, dividend.ID),
		Year:                       int32(dividend.Year), //nolint:gosec // Fiscal years are four digits.
		DividendType:               dividend.Type,
		GrossPerShareNaturalPerson: toMoney(dividend.Amounts.GrossPerShareNaturalPerson),
		GrossPerShareLegalPerson:   toMoney(dividend.Amounts.GrossPerShareLegalPerson),
		TotalAmount:                toMoney(dividend.Amounts.Total),
		Schedule: &dividendsv1.Schedule{
			AnnouncementDate: toDateProto(dividend.Schedule.AnnouncementDate),
			GmsReferenceDate: toDateProto(dividend.Schedule.GMSReferenceDate),
			GmsDate:          toDateProto(dividend.Schedule.GMSDate),
			RecordDate:       toDateProto(dividend.Schedule.RecordDate),
			ExDividendDate:   toDateProto(dividend.Schedule.ExDividendDate),
			PaymentStartDate: toDateProto(dividend.Schedule.PaymentStartDate),
			PaymentEndDate:   toDateProto(dividend.Schedule.PaymentEndDate),
		},
		DistributionMethod: dividend.DistributionMethod,
		State:              toStateProto(dividend.State(now)),
		CreateTime:         toTimestamp(dividend.CreatedAt),
		UpdateTime:         toTimestamp(dividend.UpdatedAt),
	}
}

// toMoney renders an amount as google.type.Money, or nil when BVB did not
// report one. Nil is what distinguishes an unreported amount from zero.
//
// The split into whole units and nanos is exact: BVB reports at most four
// decimal places and the column stores four, so nothing is rounded away.
func toMoney(amount domain.Amount) *money.Money {
	if !amount.Valid() {
		return nil
	}
	value := amount.Decimal()

	// Truncate rounds toward zero, so the fractional remainder keeps the sign
	// of the whole, which is what google.type.Money requires.
	units := value.Truncate(0)
	// Shift(9) scales the remainder to google.type.Money's nanos.
	nanos := value.Sub(units).Shift(9).Round(0)

	return &money.Money{
		CurrencyCode: domain.Currency,
		Units:        units.IntPart(),
		Nanos:        int32(nanos.IntPart()), //nolint:gosec // |nanos| < 1e9 by construction.
	}
}

// toDateProto renders a calendar date as google.type.Date, or nil when BVB did
// not report one.
func toDateProto(d domain.Date) *date.Date {
	if !d.Valid() {
		return nil
	}
	return &date.Date{
		Year:  int32(d.Year()), //nolint:gosec // Calendar years are four digits.
		Month: int32(d.Month()),
		Day:   int32(d.Day()), //nolint:gosec // Days of month are at most 31.
	}
}

// toTimestamp renders an instant, or nil for the zero time.
func toTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// toStateProto maps the domain's lifecycle state onto the wire enum.
func toStateProto(state domain.State) dividendsv1.Dividend_State {
	switch state {
	case domain.StateAnnounced:
		return dividendsv1.Dividend_STATE_ANNOUNCED
	case domain.StateExPassed:
		return dividendsv1.Dividend_STATE_EX_PASSED
	case domain.StatePaying:
		return dividendsv1.Dividend_STATE_PAYING
	case domain.StatePaid:
		return dividendsv1.Dividend_STATE_PAID
	case domain.StateUndated:
		return dividendsv1.Dividend_STATE_UNDATED
	default:
		return dividendsv1.Dividend_STATE_UNSPECIFIED
	}
}
