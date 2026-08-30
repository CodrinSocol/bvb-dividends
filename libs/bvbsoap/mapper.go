package bvbsoap

import (
	"log/slog"
	"strings"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
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

// toCompanies converts the identifications from GetLastDividends, discarding
// duplicates and anything without a usable symbol.
//
// BVB returns one identification per announced dividend, so a company that
// announced twice in the window appears twice. The Java client called
// .distinct() on objects with no equals method, which deduplicated nothing.
func toCompanies(identifications []dividendIdentification, log *slog.Logger) []domain.Company {
	companies := make([]domain.Company, 0, len(identifications))
	seen := make(map[domain.Symbol]struct{}, len(identifications))

	for _, identification := range identifications {
		symbol, err := domain.ParseSymbol(identification.Symbol)
		if err != nil {
			log.Warn("skipping company with an unusable symbol",
				slog.String("symbol", identification.Symbol),
				slog.String("error", err.Error()))
			continue
		}
		if _, duplicate := seen[symbol]; duplicate {
			continue
		}
		seen[symbol] = struct{}{}
		companies = append(companies, domain.Company{
			Symbol: symbol,
			Name:   strings.TrimSpace(identification.Company.CompanyName),
		})
	}
	return companies
}

// toDividends converts the dividends BVB reported for one company.
func toDividends(company domain.Company, infos []dividendInfo, log *slog.Logger) []domain.Dividend {
	dividends := make([]domain.Dividend, 0, len(infos))
	for _, info := range infos {
		dividends = append(dividends, toDividend(company, info, log))
	}
	return dividends
}

// toDividend converts one reported dividend, deriving its stable identifier.
func toDividend(company domain.Company, info dividendInfo, log *slog.Logger) domain.Dividend {
	dividendType := strings.TrimSpace(info.DividendType)

	dividend := domain.Dividend{
		Company: company.Symbol,
		Year:    info.Year,
		Type:    dividendType,
		Amounts: domain.Amounts{
			GrossPerShareNaturalPerson: parseAmount(info.DividendForNaturalPersons, "DividendForNaturalPersons", company, log),
			GrossPerShareLegalPerson:   parseAmount(info.DividendForLegalPersons, "DividendForLegalPersons", company, log),
			Total:                      parseAmount(info.DividendsTotal, "DividendsTotal", company, log),
		},
		Schedule: domain.Schedule{
			AnnouncementDate: parseDate(info.AnnouncementDate, "AnnouncementDate", company, log),
			GMSReferenceDate: parseDate(info.ReferenceDateForGMS, "ReferenceDateForGMS", company, log),
			GMSDate:          parseDate(info.GMSDate, "GMSDate", company, log),
			RecordDate:       parseDate(info.RecordDate, "RecordDate", company, log),
			ExDividendDate:   parseDate(info.ExDividendDate, "ExDividendDate", company, log),
			PaymentStartDate: parseDate(info.StartPaymentDate, "StartPaymentDate", company, log),
			PaymentEndDate:   parseDate(info.EndPaymentDate, "EndPaymentDate", company, log),
		},
		DistributionMethod: strings.TrimSpace(info.MethodOfDividendDistribution),
	}
	dividend.ID = domain.NewID(dividend.NaturalKey())
	return dividend
}

// parseAmount reads a reported decimal, treating an absent or unparseable
// value as "not reported" rather than as zero.
func parseAmount(raw *string, field string, company domain.Company, log *slog.Logger) domain.Amount {
	if raw == nil {
		return domain.NoAmount
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return domain.NoAmount
	}
	amount, err := domain.ParseAmount(text)
	if err != nil {
		log.Warn("ignoring an amount BVB reported in an unexpected form",
			slog.String("company", company.Symbol.String()),
			slog.String("field", field),
			slog.String("value", text))
		return domain.NoAmount
	}
	return amount
}

// parseDate reads a reported xsd:dateTime as a calendar date.
func parseDate(raw *string, field string, company domain.Company, log *slog.Logger) domain.Date {
	if raw == nil {
		return domain.NoDate
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return domain.NoDate
	}
	for _, layout := range dateTimeLayouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			return domain.DateOf(parsed)
		}
	}
	log.Warn("ignoring a date BVB reported in an unexpected form",
		slog.String("company", company.Symbol.String()),
		slog.String("field", field),
		slog.String("value", text))
	return domain.NoDate
}
