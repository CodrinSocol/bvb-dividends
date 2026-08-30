package dividends_test

import (
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
	"github.com/CodrinSocol/bvb-dividends-ro/internal/dividends"
)

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

// The identifier of a dividend must depend only on its natural key, and must
// survive a re-import unchanged. This is the property the Java service lacked:
// it generated random UUIDs and re-created every row daily.
func TestNewIDIsStableAcrossImports(t *testing.T) {
	first := dividends.Dividend{
		Company:            "SNP",
		Year:               2025,
		Type:               "cash",
		Schedule:           dividends.Schedule{ExDividendDate: date(t, "2026-04-17")},
		DistributionMethod: "bank transfer",
	}
	// The same dividend as re-reported the next day, with a figure now filled
	// in and a different distribution method wording.
	second := dividends.Dividend{
		Company:            "SNP",
		Year:               2025,
		Type:               "cash",
		Amounts:            dividends.Amounts{Total: mustAmount(t, "1234.5678")},
		Schedule:           dividends.Schedule{ExDividendDate: date(t, "2026-04-17"), RecordDate: date(t, "2026-04-18")},
		DistributionMethod: "Bank transfer / Depozitarul Central",
	}

	if got, want := dividends.NewID(second.NaturalKey()), dividends.NewID(first.NaturalKey()); got != want {
		t.Errorf("ID changed across imports: got %s, want %s", got, want)
	}
}

func TestNewIDDistinguishesDividends(t *testing.T) {
	base := dividends.NaturalKey{Company: "SNP", Year: 2025, Type: "cash", ExDividend: date(t, "2026-04-17")}

	tests := map[string]dividends.NaturalKey{
		"different company": {Company: "TLV", Year: 2025, Type: "cash", ExDividend: date(t, "2026-04-17")},
		"different year":    {Company: "SNP", Year: 2024, Type: "cash", ExDividend: date(t, "2026-04-17")},
		"different type":    {Company: "SNP", Year: 2025, Type: "stock", ExDividend: date(t, "2026-04-17")},
		"different ex date": {Company: "SNP", Year: 2025, Type: "cash", ExDividend: date(t, "2026-04-18")},
		"absent ex date":    {Company: "SNP", Year: 2025, Type: "cash", ExDividend: common.NoDate},
	}

	baseID := dividends.NewID(base)
	for name, key := range tests {
		t.Run(name, func(t *testing.T) {
			if got := dividends.NewID(key); got == baseID {
				t.Errorf("ID collided with the base key: %s", got)
			}
		})
	}
}

// The natural key is encoded with length prefixes so that no combination of
// field values can produce the same encoding as a different combination. Type
// is a free-form string owned by BVB, so it is the field that could collide.
func TestNaturalKeyEncodingIsUnambiguous(t *testing.T) {
	a := dividends.NaturalKey{Company: "SNP", Year: 2025, Type: "cash", ExDividend: common.NoDate}
	b := dividends.NaturalKey{Company: "SN", Year: 2025, Type: "Pcash", ExDividend: common.NoDate}

	if a.String() == b.String() {
		t.Errorf("distinct keys encoded identically: %q", a.String())
	}
	if dividends.NewID(a) == dividends.NewID(b) {
		t.Error("distinct keys produced the same ID")
	}
}

func TestDividendState(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		schedule dividends.Schedule
		want     dividends.State
	}{
		{
			name:     "no ex-dividend date is undated",
			schedule: dividends.Schedule{RecordDate: date(t, "2026-09-01")},
			want:     dividends.StateUndated,
		},
		{
			name:     "ex date in the future is announced",
			schedule: dividends.Schedule{ExDividendDate: date(t, "2026-09-15")},
			want:     dividends.StateAnnounced,
		},
		{
			// The ex-dividend date is the first day the share trades without
			// entitlement, so on that day the dividend can no longer be earned.
			name:     "ex date today is already passed",
			schedule: dividends.Schedule{ExDividendDate: date(t, "2026-08-29")},
			want:     dividends.StateExPassed,
		},
		{
			name:     "ex date passed with no payment dates",
			schedule: dividends.Schedule{ExDividendDate: date(t, "2026-08-01")},
			want:     dividends.StateExPassed,
		},
		{
			name: "payment not yet started",
			schedule: dividends.Schedule{
				ExDividendDate:   date(t, "2026-08-01"),
				PaymentStartDate: date(t, "2026-09-10"),
			},
			want: dividends.StateExPassed,
		},
		{
			name: "payment window open",
			schedule: dividends.Schedule{
				ExDividendDate:   date(t, "2026-08-01"),
				PaymentStartDate: date(t, "2026-08-20"),
				PaymentEndDate:   date(t, "2026-09-20"),
			},
			want: dividends.StatePaying,
		},
		{
			name: "payment starts today",
			schedule: dividends.Schedule{
				ExDividendDate:   date(t, "2026-08-01"),
				PaymentStartDate: date(t, "2026-08-29"),
			},
			want: dividends.StatePaying,
		},
		{
			name: "payment window closed",
			schedule: dividends.Schedule{
				ExDividendDate:   date(t, "2026-01-10"),
				PaymentStartDate: date(t, "2026-02-01"),
				PaymentEndDate:   date(t, "2026-03-01"),
			},
			want: dividends.StatePaid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dividends.Dividend{Company: "SNP", Year: 2025, Schedule: tt.schedule}
			if got := d.StateAt(now); got != tt.want {
				t.Errorf("StateAt() = %v, want %v", got, tt.want)
			}
			if got, want := d.IsActive(now), tt.want == dividends.StateAnnounced; got != want {
				t.Errorf("IsActive() = %v, want %v", got, want)
			}
		})
	}
}

func mustAmount(t *testing.T, s string) common.Amount {
	t.Helper()
	a, err := common.ParseAmount(s)
	if err != nil {
		t.Fatalf("ParseAmount(%q): %v", s, err)
	}
	return a
}
