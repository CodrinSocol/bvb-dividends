package bvb

import (
	"log/slog"
	"strings"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// dateTimeLayouts are the forms BVB's xsd:dateTime values have been seen in.
//
// The schedule fields are calendar dates that BVB transmits as timestamps, so
// whatever time component arrives is discarded. The offset-bearing layouts come
// first so an explicit offset is honoured rather than reinterpreted.
var dateTimeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02",
}

// toDividends converts the dividends BVB reported for one company.
func toDividends(
	symbol common.Symbol,
	infos []bvbclient.DividendInfo,
	log *slog.Logger,
) []*dividends.Dividend {
	result := make([]*dividends.Dividend, 0, len(infos))
	for _, info := range infos {
		result = append(result, toDividend(symbol, info, log))
	}

	return result
}

// toDividend converts one reported dividend, deriving its stable identifier.
//
// One unreadable value spoils only its own field: a dividend BVB reported a
// malformed date for still arrives, with that date absent, rather than being
// dropped along with everything else it said.
func toDividend(symbol common.Symbol, info bvbclient.DividendInfo, log *slog.Logger) *dividends.Dividend {
	dividend := &dividends.Dividend{
		Company: symbol,
		Year:    info.Year,
		Type:    strings.TrimSpace(info.DividendType),
		Amounts: dividends.Amounts{
			GrossPerShareNaturalPerson: parseAmount(info.DividendForNaturalPersons, "DividendForNaturalPersons", symbol, log),
			GrossPerShareLegalPerson:   parseAmount(info.DividendForLegalPersons, "DividendForLegalPersons", symbol, log),
			Total:                      parseAmount(info.DividendsTotal, "DividendsTotal", symbol, log),
		},
		Schedule: dividends.Schedule{
			AnnouncementDate: parseDate(info.AnnouncementDate, "AnnouncementDate", symbol, log),
			GMSReferenceDate: parseDate(info.ReferenceDateForGMS, "ReferenceDateForGMS", symbol, log),
			GMSDate:          parseDate(info.GMSDate, "GMSDate", symbol, log),
			RecordDate:       parseDate(info.RecordDate, "RecordDate", symbol, log),
			ExDividendDate:   parseDate(info.ExDividendDate, "ExDividendDate", symbol, log),
			PaymentStartDate: parseDate(info.StartPaymentDate, "StartPaymentDate", symbol, log),
			PaymentEndDate:   parseDate(info.EndPaymentDate, "EndPaymentDate", symbol, log),
		},
		DistributionMethod: strings.TrimSpace(info.MethodOfDividendDistribution),
	}
	dividend.ID = dividends.NewID(dividend.NaturalKey())

	return dividend
}

// parseAmount reads a reported decimal, treating an absent or unparseable value
// as "not reported" rather than as zero.
func parseAmount(raw string, field string, symbol common.Symbol, log *slog.Logger) common.Amount {
	text := strings.TrimSpace(raw)
	if text == "" {
		return common.NoAmount
	}

	amount, err := common.ParseAmount(text)
	if err != nil {
		log.Warn("ignoring an amount BVB reported in an unexpected form",
			slog.String("company", symbol.String()),
			slog.String("field", field),
			slog.String("value", text))

		return common.NoAmount
	}

	return amount
}

// parseDate reads a reported xsd:dateTime as a calendar date.
func parseDate(raw string, field string, symbol common.Symbol, log *slog.Logger) common.Date {
	text := strings.TrimSpace(raw)
	if text == "" {
		return common.NoDate
	}

	for _, layout := range dateTimeLayouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			return common.DateOf(parsed)
		}
	}

	log.Warn("ignoring a date BVB reported in an unexpected form",
		slog.String("company", symbol.String()),
		slog.String("field", field),
		slog.String("value", text))

	return common.NoDate
}
