package bvb_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/companies/infra/bvb"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// fakeClient answers with a recorded response rather than calling BVB.
type fakeClient struct {
	identifications []bvbclient.Identification
	err             error
	days            int
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
