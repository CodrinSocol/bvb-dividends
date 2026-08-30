package connectrpc

import (
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/genproto/googleapis/type/money"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	bvbdividendsv1 "github.com/CodrinSocol/bvb-dividends-ro/libs/go/gen/v1"
)

// wildcard is AIP-159's "every collection" segment: a parent of `companies/-`
// lists dividends across all companies.
const wildcard = "-"

// ResourceName renders a dividend's resource name.
func ResourceName(company common.Symbol, id dividends.ID) string {
	return bvbdividendsv1.DividendResourceName{
		Company:  company.String(),
		Dividend: id.String(),
	}.String()
}

// ParseResourceName reads a dividend resource name, returning the company it
// belongs to and the dividend's identifier.
func ParseResourceName(name string) (common.Symbol, dividends.ID, error) {
	var parsed bvbdividendsv1.DividendResourceName
	if err := parsed.UnmarshalString(name); err != nil {
		return "", dividends.ID{}, common.ErrEntityInvalid.
			WithProblem("name", name+" is not a dividend resource name").
			WithUnderlying(err)
	}

	symbol, err := common.ParseSymbol(parsed.Company)
	if err != nil {
		return "", dividends.ID{}, err
	}

	id, err := dividends.ParseID(parsed.Dividend)
	if err != nil {
		return "", dividends.ID{}, err
	}

	return symbol, id, nil
}

// ParseParent reads the parent of a dividend collection.
//
// A nil symbol means the AIP-159 wildcard `companies/-`, which selects every
// company rather than one.
func ParseParent(parent string) (*common.Symbol, error) {
	var parsed bvbdividendsv1.CompanyResourceName
	if err := parsed.UnmarshalString(parent); err != nil {
		return nil, common.ErrEntityInvalid.
			WithProblem("parent", parent+" is not a company resource name").
			WithUnderlying(err)
	}

	if parsed.Company == wildcard {
		return nil, nil //nolint:nilnil // A nil symbol is the wildcard; see the doc comment.
	}

	symbol, err := common.ParseSymbol(parsed.Company)
	if err != nil {
		return nil, err
	}

	return &symbol, nil
}

// toProto renders a dividend as its API resource.
func toProto(dividend *dividends.Dividend) *bvbdividendsv1.Dividend {
	return &bvbdividendsv1.Dividend{
		Name: ResourceName(dividend.Company, dividend.ID),
		//nolint:gosec // Fiscal years are four digits.
		Year:                       int32(dividend.Year),
		DividendType:               dividend.Type,
		GrossPerShareNaturalPerson: toMoney(dividend.Amounts.GrossPerShareNaturalPerson),
		GrossPerShareLegalPerson:   toMoney(dividend.Amounts.GrossPerShareLegalPerson),
		TotalAmount:                toMoney(dividend.Amounts.Total),
		Schedule: &bvbdividendsv1.Schedule{
			AnnouncementDate: toDate(dividend.Schedule.AnnouncementDate),
			GmsReferenceDate: toDate(dividend.Schedule.GMSReferenceDate),
			GmsDate:          toDate(dividend.Schedule.GMSDate),
			RecordDate:       toDate(dividend.Schedule.RecordDate),
			ExDividendDate:   toDate(dividend.Schedule.ExDividendDate),
			PaymentStartDate: toDate(dividend.Schedule.PaymentStartDate),
			PaymentEndDate:   toDate(dividend.Schedule.PaymentEndDate),
		},
		DistributionMethod: dividend.DistributionMethod,
		State:              toState(dividend.State),
		CreateTime:         timestamppb.New(dividend.CreateTime),
		UpdateTime:         timestamppb.New(dividend.UpdateTime),
	}
}

func toProtos(list []*dividends.Dividend) []*bvbdividendsv1.Dividend {
	result := make([]*bvbdividendsv1.Dividend, len(list))
	for i, dividend := range list {
		result[i] = toProto(dividend)
	}

	return result
}

// toState renders the lifecycle state as its enum value.
//
// The states are named after the enum, so an unknown value can only mean the
// database derived something this build does not know about, which is
// unspecified rather than a guess.
func toState(state dividends.State) bvbdividendsv1.Dividend_State {
	if value, ok := bvbdividendsv1.Dividend_State_value[state.String()]; ok {
		return bvbdividendsv1.Dividend_State(value)
	}

	return bvbdividendsv1.Dividend_STATE_UNSPECIFIED
}

// toMoney renders an amount as google.type.Money, or nil when BVB did not
// report one. Nil is what distinguishes an unreported amount from zero.
//
// The split into whole units and nanos is exact: BVB reports at most four
// decimal places and the column stores four, so nothing is rounded away.
func toMoney(amount common.Amount) *money.Money {
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
		CurrencyCode: common.Currency,
		Units:        units.IntPart(),
		//nolint:gosec // |nanos| < 1e9 by construction.
		Nanos: int32(nanos.IntPart()),
	}
}

// toDate renders a calendar date as google.type.Date, or nil when BVB did not
// report one.
func toDate(d common.Date) *date.Date {
	if !d.Valid() {
		return nil
	}

	return &date.Date{
		//nolint:gosec // A calendar date fits in an int32 by construction.
		Year: int32(d.Year()),
		//nolint:gosec // Months are 1 to 12.
		Month: int32(d.Month()),
		//nolint:gosec // Days are 1 to 31.
		Day: int32(d.Day()),
	}
}
