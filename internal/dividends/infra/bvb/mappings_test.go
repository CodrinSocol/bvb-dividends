package bvb_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends/infra/bvb"
	bvbclient "github.com/CodrinSocol/bvb-dividends-ro/libs/go/bvb-client"
)

// fakeClient answers with recorded values rather than calling BVB.
type fakeClient struct {
	infos  []bvbclient.DividendInfo
	err    error
	symbol string
}

func (f *fakeClient) GetDividends(_ context.Context, symbol string) ([]bvbclient.DividendInfo, error) {
	f.symbol = symbol

	return f.infos, f.err
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func fetch(t *testing.T, infos []bvbclient.DividendInfo) []*dividends.Dividend {
	t.Helper()

	source := bvb.NewSource(quietLogger(), &fakeClient{infos: infos})

	result, err := source.DividendsFor(t.Context(), "SNP")
	if err != nil {
		t.Fatalf("DividendsFor: %v", err)
	}

	return result
}

// A fully reported dividend must arrive with every field read, and the money
// figures exact to the last digit BVB sent: the previous service declared
// them as a floating-point type, which silently rounded them.
func TestFullyReportedDividend(t *testing.T) {
	t.Parallel()

	result := fetch(t, []bvbclient.DividendInfo{{
		Year:                         2024,
		DividendForNaturalPersons:    "0.0345",
		DividendForLegalPersons:      "0.0345",
		DividendsTotal:               "2085123456.7890",
		DividendType:                 " cash ",
		ReferenceDateForGMS:          "2025-03-14T00:00:00",
		GMSDate:                      "2025-04-24T00:00:00",
		RecordDate:                   "2025-06-11T00:00:00",
		ExDividendDate:               "2025-06-10T00:00:00",
		AnnouncementDate:             "2025-03-01T00:00:00",
		StartPaymentDate:             "2025-06-25T00:00:00",
		EndPaymentDate:               "2025-12-31T00:00:00",
		MethodOfDividendDistribution: "Bank transfer / Depozitarul Central",
	}})

	if len(result) != 1 {
		t.Fatalf("got %d dividends, want 1", len(result))
	}

	dividend := result[0]
	if dividend.Year != 2024 {
		t.Errorf("year = %d, want 2024", dividend.Year)
	}
	if dividend.Type != "cash" {
		t.Errorf("type = %q, want %q; the surrounding space was not trimmed", dividend.Type, "cash")
	}
	// Compared numerically: the decimal drops a trailing zero when it prints,
	// but not a digit of value, which a float would have.
	wantTotal, err := common.ParseAmount("2085123456.7890")
	if err != nil {
		t.Fatalf("ParseAmount: %v", err)
	}
	if !dividend.Amounts.Total.Equal(wantTotal) {
		t.Errorf("total = %q, want %q exactly", dividend.Amounts.Total, wantTotal)
	}
	if got := dividend.Schedule.ExDividendDate.String(); got != "2025-06-10" {
		t.Errorf("ex-dividend date = %q, want the calendar day without the time", got)
	}
	if got := dividend.Schedule.PaymentEndDate.String(); got != "2025-12-31" {
		t.Errorf("payment end date = %q", got)
	}
	if dividend.DistributionMethod != "Bank transfer / Depozitarul Central" {
		t.Errorf("distribution method = %q", dividend.DistributionMethod)
	}
	if dividend.ID != dividends.NewID(dividend.NaturalKey()) {
		t.Error("the identifier was not derived from the natural key")
	}
}

// BVB publishes the figures and dates progressively, so an early-stage dividend
// has most of them unset. Absent must stay distinguishable from zero, or an
// unannounced dividend reads the same as one that pays nothing.
func TestAnnouncedButUnscheduledDividend(t *testing.T) {
	t.Parallel()

	result := fetch(t, []bvbclient.DividendInfo{{
		Year:             2025,
		DividendType:     "cash",
		AnnouncementDate: "2026-02-18T00:00:00",
	}})

	dividend := result[0]
	if dividend.Amounts.Total.Valid() {
		t.Error("an unreported total was read as a value")
	}
	if dividend.Schedule.ExDividendDate.Valid() {
		t.Error("an unreported ex-dividend date was read as a value")
	}
	if got := dividend.Schedule.AnnouncementDate.String(); got != "2026-02-18" {
		t.Errorf("announcement date = %q, want 2026-02-18", got)
	}
	if got := dividend.StateAt(dividend.Schedule.AnnouncementDate.Time()); got != dividends.StateUndated {
		t.Errorf("state = %v, want undated", got)
	}
}

// One unreadable value must cost only its own field. The dividend, and every
// other field of it, still has to arrive.
func TestAnUnreadableValueSpoilsOnlyItsOwnField(t *testing.T) {
	t.Parallel()

	result := fetch(t, []bvbclient.DividendInfo{{
		Year:                         2023,
		DividendForNaturalPersons:    "not reported",
		DividendsTotal:               "1500000",
		DividendType:                 "stock",
		ExDividendDate:               "11/06/2024",
		RecordDate:                   "2024-06-12T00:00:00",
		MethodOfDividendDistribution: "Share allotment",
	}})

	dividend := result[0]
	if dividend.Amounts.GrossPerShareNaturalPerson.Valid() {
		t.Error("an unparseable amount was read as a value")
	}
	if dividend.Schedule.ExDividendDate.Valid() {
		t.Error("an unparseable date was read as a value")
	}
	if got := dividend.Amounts.Total.String(); got != "1500000" {
		t.Errorf("total = %q; a bad sibling field spoiled it", got)
	}
	if got := dividend.Schedule.RecordDate.String(); got != "2024-06-12" {
		t.Errorf("record date = %q; a bad sibling field spoiled it", got)
	}
}

// BVB has been seen sending its timestamps in more than one shape.
func TestDateLayouts(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"2025-06-10T00:00:00",
		"2025-06-10T00:00:00Z",
		"2025-06-10T03:00:00+03:00",
		"2025-06-10T00:00:00.123456",
		"2025-06-10",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			result := fetch(t, []bvbclient.DividendInfo{{Year: 2024, ExDividendDate: raw}})
			if got := result[0].Schedule.ExDividendDate.String(); got != "2025-06-10" {
				t.Errorf("%q read as %q, want 2025-06-10", raw, got)
			}
		})
	}
}

func TestDividendsForRefusesAnUnusableSymbol(t *testing.T) {
	t.Parallel()

	client := &fakeClient{}
	source := bvb.NewSource(quietLogger(), client)

	if _, err := source.DividendsFor(t.Context(), common.Symbol("")); err == nil {
		t.Fatal("an empty symbol was sent to BVB")
	}
	if client.symbol != "" {
		t.Errorf("BVB was called with %q", client.symbol)
	}
}
