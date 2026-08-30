package common_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

func TestParseSymbol(t *testing.T) {
	valid := map[string]common.Symbol{
		"SNP":                  "SNP",
		"snp":                  "SNP",
		"  tlv  ":              "TLV",
		"TLV.R":                "TLV.R",
		"H2O":                  "H2O",
		"SIF-5":                "SIF-5",
		"12345678901234567890": "12345678901234567890",
	}
	for input, want := range valid {
		t.Run("valid/"+input, func(t *testing.T) {
			got, err := common.ParseSymbol(input)
			if err != nil {
				t.Fatalf("ParseSymbol(%q): unexpected error: %v", input, err)
			}
			if got != want {
				t.Errorf("ParseSymbol(%q) = %q, want %q", input, got, want)
			}
		})
	}

	invalid := []string{"", "   ", "123456789012345678901", "SN P", "SNP;DROP", "SNP*", "SNP'"}
	for _, input := range invalid {
		t.Run("invalid/"+input, func(t *testing.T) {
			if _, err := common.ParseSymbol(input); !errors.Is(err, common.ErrSymbolInvalid) {
				t.Errorf("ParseSymbol(%q) error = %v, want ErrInvalidArgument", input, err)
			}
		})
	}
}

// Dividend figures are money and must survive a round trip exactly. The Java
// DTO declared them as Double, which silently rounded them.
func TestAmountIsExact(t *testing.T) {
	const exact = "0.1234"
	a, err := common.ParseAmount(exact)
	if err != nil {
		t.Fatalf("ParseAmount: %v", err)
	}
	if got := a.String(); got != exact {
		t.Errorf("Amount.String() = %q, want %q", got, exact)
	}
	if !a.Valid() {
		t.Error("parsed amount reports itself absent")
	}
}

func TestAmountAbsentIsNotZero(t *testing.T) {
	zero, err := common.ParseAmount("0")
	if err != nil {
		t.Fatalf("ParseAmount: %v", err)
	}
	if common.NoAmount.Valid() {
		t.Error("NoAmount reports itself present")
	}
	if common.NoAmount.Equal(zero) {
		t.Error("an unreported amount compared equal to zero")
	}
	if common.NoAmount.String() != "" {
		t.Errorf("NoAmount.String() = %q, want empty", common.NoAmount.String())
	}
}

func TestAmountEqualIgnoresTrailingZeros(t *testing.T) {
	a := mustAmount(t, "0.10")
	b := mustAmount(t, "0.1000")
	if !a.Equal(b) {
		t.Errorf("%s did not compare equal to %s", a, b)
	}
}

func TestDate(t *testing.T) {
	d, err := common.ParseDate("2026-04-17")
	if err != nil {
		t.Fatalf("ParseDate: %v", err)
	}
	if got, want := d.String(), "2026-04-17"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if got, want := d.Year(), 2026; got != want {
		t.Errorf("Year() = %d, want %d", got, want)
	}
	if got, want := d.Time(), time.Date(2026, time.April, 17, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("Time() = %v, want %v", got, want)
	}

	for _, bad := range []string{"", "17/04/2026", "2026-04-17T00:00:00Z", "2026-13-01"} {
		if _, err := common.ParseDate(bad); !errors.Is(err, common.ErrDateInvalid) {
			t.Errorf("ParseDate(%q) error = %v, want ErrDateInvalid", bad, err)
		}
	}
}

// An absent date must never compare as though it were a real one, or dividends
// BVB has not scheduled yet would leak into date-range results.
func TestDateComparisonsWithAbsentDates(t *testing.T) {
	present := date(t, "2026-04-17")
	absent := common.NoDate

	if absent.Before(present) || absent.After(present) {
		t.Error("an absent date compared as ordered against a present one")
	}
	if present.Before(absent) || present.After(absent) {
		t.Error("a present date compared as ordered against an absent one")
	}
	if !absent.Equal(common.NoDate) {
		t.Error("two absent dates did not compare equal")
	}
	if absent.Equal(present) {
		t.Error("an absent date compared equal to a present one")
	}
	if absent.Valid() {
		t.Error("NoDate reports itself present")
	}
}

func TestDateOfUsesCalendarDayNotInstant(t *testing.T) {
	// Late evening in Bucharest is still the same calendar day there, and BVB
	// reports its dates in local time.
	bucharest := time.FixedZone("EEST", 3*60*60)
	evening := time.Date(2026, time.April, 17, 23, 30, 0, 0, bucharest)

	if got, want := common.DateOf(evening).String(), "2026-04-17"; got != want {
		t.Errorf("DateOf(%v) = %q, want %q", evening, got, want)
	}
}

func date(t *testing.T, s string) common.Date {
	t.Helper()

	if s == "" {
		return common.NoDate
	}

	d, err := common.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}

	return d
}

func mustAmount(t *testing.T, s string) common.Amount {
	t.Helper()

	a, err := common.ParseAmount(s)
	if err != nil {
		t.Fatalf("ParseAmount(%q): %v", s, err)
	}

	return a
}
