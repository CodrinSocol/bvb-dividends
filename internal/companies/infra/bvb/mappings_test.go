package bvb_test

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/bvb"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// fakeClient answers with a recorded response rather than calling BVB.
type fakeClient struct {
	identifications []bvbclient.Identification
	balances        map[int][]bvbclient.SymbolBalance
	years           []int
	reportType      bvbclient.ReportType
	err             error
	days            int
}

func (f *fakeClient) GetAvailableBalances(
	_ context.Context,
	year int,
	reportType bvbclient.ReportType,
) ([]bvbclient.SymbolBalance, error) {
	f.years = append(f.years, year)
	f.reportType = reportType

	return f.balances[year], f.err
}

func (f *fakeClient) GetLastDividends(_ context.Context, days int) ([]bvbclient.Identification, error) {
	f.days = days

	return f.identifications, f.err
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// BVB reports one entry per announcement rather than per company, in whatever
// casing it happens to hold, and sometimes with no usable symbol at all. The
// Java client called .distinct() on objects with no equals method, so it
// deduplicated nothing and re-imported a company once per announcement.
func TestRecentlyAnnouncing(t *testing.T) {
	t.Parallel()

	client := &fakeClient{identifications: []bvbclient.Identification{
		{Symbol: "SNP", CompanyName: "OMV PETROM S.A."},
		{Symbol: "TLV", CompanyName: "  BANCA TRANSILVANIA S.A.  "},
		{Symbol: "snp", CompanyName: "OMV PETROM S.A."},
		{Symbol: "", CompanyName: "UNKNOWN ISSUER"},
		{Symbol: "SN P", CompanyName: "MALFORMED"},
	}}

	source := bvb.NewSource(quietLogger(), client)

	result, err := source.RecentlyAnnouncing(t.Context(), 20)
	if err != nil {
		t.Fatalf("RecentlyAnnouncing: %v", err)
	}

	if client.days != 20 {
		t.Errorf("asked BVB for %d days, want 20", client.days)
	}

	want := []companies.Company{
		{Symbol: "SNP", DisplayName: "OMV PETROM S.A."},
		{Symbol: "TLV", DisplayName: "BANCA TRANSILVANIA S.A."},
	}
	if len(result) != len(want) {
		t.Fatalf("got %d companies, want %d: %+v", len(result), len(want), result)
	}
	for i, company := range result {
		if company.Symbol != want[i].Symbol {
			t.Errorf("company %d symbol = %q, want %q", i, company.Symbol, want[i].Symbol)
		}
		if company.DisplayName != want[i].DisplayName {
			t.Errorf("company %d name = %q, want %q", i, company.DisplayName, want[i].DisplayName)
		}
	}
}

func TestRecentlyAnnouncingPropagatesAFailure(t *testing.T) {
	t.Parallel()

	client := &fakeClient{err: bvbclient.ErrInvalidArgument}
	source := bvb.NewSource(quietLogger(), client)

	if _, err := source.RecentlyAnnouncing(t.Context(), 0); err == nil {
		t.Fatal("a failed call was reported as success")
	}
}

// The service has no operation that lists companies, so the whole market is
// read from the issuers that filed a balance. Several years are asked for and
// unioned: the current year returns nothing until the filing season reaches it,
// and a company that has since delisted only appears in the years it filed.
func TestAll(t *testing.T) {
	t.Parallel()

	client := &fakeClient{balances: map[int][]bvbclient.SymbolBalance{
		// The current year, before the filing season.
		2026: {},
		2025: {{Symbol: "SNP"}, {Symbol: "TLV"}},
		// TLV again, and one that stopped filing after 2024.
		2024: {{Symbol: "TLV"}, {Symbol: "brd"}, {Symbol: ""}},
	}}

	source := bvb.NewSource(quietLogger(), client)

	result, err := source.All(t.Context())
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	got := make([]common.Symbol, len(result))
	for i, company := range result {
		got[i] = company.Symbol
	}
	if want := []common.Symbol{"SNP", "TLV", "BRD"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if client.reportType != bvbclient.ReportTypeAnnual {
		t.Errorf("asked for %q balances, want annual", client.reportType)
	}
	if len(client.years) != 3 {
		t.Errorf("asked about %d years, want 3: %v", len(client.years), client.years)
	}

	// The listing reports a ticker and nothing else, so the name is left for
	// the dividend import to fill in rather than being invented here.
	if result[0].DisplayName != "" {
		t.Errorf("display name = %q, want it left empty", result[0].DisplayName)
	}
}

func TestAllPropagatesAFailure(t *testing.T) {
	t.Parallel()

	source := bvb.NewSource(quietLogger(), &fakeClient{err: bvbclient.ErrInvalidArgument})

	if _, err := source.All(t.Context()); err == nil {
		t.Fatal("a failed call was reported as success")
	}
}
