package bvb

import (
	"log/slog"
	"strings"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// toCompanies converts the identifications BVB reported, discarding duplicates
// and anything without a usable symbol.
//
// BVB returns one identification per announced dividend, so a company that
// announced twice in the window appears twice. The Java client called
// .distinct() on objects with no equals method, which deduplicated nothing and
// re-imported the company once per entry.
func toCompanies(identifications []bvbclient.Identification, log *slog.Logger) []*companies.Company {
	result := make([]*companies.Company, 0, len(identifications))
	seen := make(map[common.Symbol]struct{}, len(identifications))

	for _, identification := range identifications {
		symbol, err := common.ParseSymbol(identification.Symbol)
		if err != nil {
			log.Warn("skipping a company with an unusable symbol",
				slog.String("symbol", identification.Symbol),
				slog.Any("err", err))

			continue
		}

		if _, duplicate := seen[symbol]; duplicate {
			continue
		}
		seen[symbol] = struct{}{}

		result = append(result, &companies.Company{
			Symbol:      symbol,
			DisplayName: strings.TrimSpace(identification.CompanyName),
		})
	}

	return result
}

// toCompaniesFromSymbols converts a list of tickers into companies, discarding
// duplicates and anything unusable.
//
// The result carries no display name, because the operation these symbols come
// from reports none.
func toCompaniesFromSymbols(symbols []string, log *slog.Logger) []*companies.Company {
	result := make([]*companies.Company, 0, len(symbols))
	seen := make(map[common.Symbol]struct{}, len(symbols))

	for _, raw := range symbols {
		symbol, err := common.ParseSymbol(raw)
		if err != nil {
			log.Warn("skipping an issuer with an unusable symbol",
				slog.String("symbol", raw),
				slog.Any("err", err))

			continue
		}

		if _, duplicate := seen[symbol]; duplicate {
			continue
		}
		seen[symbol] = struct{}{}

		result = append(result, &companies.Company{Symbol: symbol})
	}

	return result
}
