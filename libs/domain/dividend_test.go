package domain_test

import (
	"testing"
	"time"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

func date(t *testing.T, s string) domain.Date {
	t.Helper()
	if s == "" {
		return domain.NoDate
	}
	d, err := domain.ParseDate(s)
	if err != nil {
		t.Fatalf("ParseDate(%q): %v", s, err)
	}
	return d
}

// The identifier of a dividend must depend only on its natural key, and must
// survive a re-import unchanged. This is the property the Java service lacked:
// it generated random UUIDs and re-created every row daily.
func TestNewIDIsStableAcrossImports(t *testing.T) {
	first := domain.Dividend{
		Company:            "SNP",
		Year:               2025,
		Type:               "cash",
		Schedule:           domain.Schedule{ExDividendDate: date(t, "2026-04-17")},
		DistributionMethod: "bank transfer",
	}
	// The same dividend as re-reported the next day, with a figure now filled
	// in and a different distribution method wording.
	second := domain.Dividend{
		Company:            "SNP",
		Year:               2025,
		Type:               "cash",
		Amounts:            domain.Amounts{Total: mustAmount(t, "1234.5678")},
		Schedule:           domain.Schedule{ExDividendDate: date(t, "2026-04-17"), RecordDate: date(t, "2026-04-18")},
		DistributionMethod: "Bank transfer / Depozitarul Central",
	}

	if got, want := domain.NewID(second.NaturalKey()), domain.NewID(first.NaturalKey()); got != want {
		t.Errorf("ID changed across imports: got %s, want %s", got, want)
	}
}

func TestNewIDDistinguishesDividends(t *testing.T) {
	base := domain.NaturalKey{Company: "SNP", Year: 2025, Type: "cash", ExDividend: date(t, "2026-04-17")}

	tests := map[string]domain.NaturalKey{
		"different company": {Company: "TLV", Year: 2025, Type: "cash", ExDividend: date(t, "2026-04-17")},
		"different year":    {Company: "SNP", Year: 2024, Type: "cash", ExDividend: date(t, "2026-04-17")},
		"different type":    {Company: "SNP", Year: 2025, Type: "stock", ExDividend: date(t, "2026-04-17")},
		"different ex date": {Company: "SNP", Year: 2025, Type: "cash", ExDividend: date(t, "2026-04-18")},
		"absent ex date":    {Company: "SNP", Year: 2025, Type: "cash", ExDividend: domain.NoDate},
	}

	baseID := domain.NewID(base)
	for name, key := range tests {
		t.Run(name, func(t *testing.T) {
			if got := domain.NewID(key); got == baseID {
				t.Errorf("ID collided with the base key: %s", got)
			}
		})
	}
}

// The natural key is encoded with length prefixes so that no combination of
// field values can produce the same encoding as a different combination. Type
// is a free-form string owned by BVB, so it is the field that could collide.
func TestNaturalKeyEncodingIsUnambiguous(t *testing.T) {
	a := domain.NaturalKey{Company: "SNP", Year: 2025, Type: "cash", ExDividend: domain.NoDate}
	b := domain.NaturalKey{Company: "SN", Year: 2025, Type: "Pcash", ExDividend: domain.NoDate}

	if a.String() == b.String() {
		t.Errorf("distinct keys encoded identically: %q", a.String())
	}
	if domain.NewID(a) == domain.NewID(b) {
		t.Error("distinct keys produced the same ID")
	}
}

func TestDividendState(t *testing.T) {
	now := time.Date(2026, time.August, 29, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		schedule domain.Schedule
		want     domain.State
	}{
		{
			name:     "no ex-dividend date is undated",
			schedule: domain.Schedule{RecordDate: date(t, "2026-09-01")},
			want:     domain.StateUndated,
		},
		{
			name:     "ex date in the future is announced",
			schedule: domain.Schedule{ExDividendDate: date(t, "2026-09-15")},
			want:     domain.StateAnnounced,
		},
		{
			// The ex-dividend date is the first day the share trades without
			// entitlement, so on that day the dividend can no longer be earned.
			name:     "ex date today is already passed",
			schedule: domain.Schedule{ExDividendDate: date(t, "2026-08-29")},
			want:     domain.StateExPassed,
		},
		{
			name:     "ex date passed with no payment dates",
			schedule: domain.Schedule{ExDividendDate: date(t, "2026-08-01")},
			want:     domain.StateExPassed,
		},
		{
			name: "payment not yet started",
			schedule: domain.Schedule{
				ExDividendDate:   date(t, "2026-08-01"),
				PaymentStartDate: date(t, "2026-09-10"),
			},
			want: domain.StateExPassed,
		},
		{
			name: "payment window open",
			schedule: domain.Schedule{
				ExDividendDate:   date(t, "2026-08-01"),
				PaymentStartDate: date(t, "2026-08-20"),
				PaymentEndDate:   date(t, "2026-09-20"),
			},
			want: domain.StatePaying,
		},
		{
			name: "payment starts today",
			schedule: domain.Schedule{
				ExDividendDate:   date(t, "2026-08-01"),
				PaymentStartDate: date(t, "2026-08-29"),
			},
			want: domain.StatePaying,
		},
		{
			name: "payment window closed",
			schedule: domain.Schedule{
				ExDividendDate:   date(t, "2026-01-10"),
				PaymentStartDate: date(t, "2026-02-01"),
				PaymentEndDate:   date(t, "2026-03-01"),
			},
			want: domain.StatePaid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := domain.Dividend{Company: "SNP", Year: 2025, Schedule: tt.schedule}
			if got := d.State(now); got != tt.want {
				t.Errorf("State() = %v, want %v", got, tt.want)
			}
			if got, want := d.IsActive(now), tt.want == domain.StateAnnounced; got != want {
				t.Errorf("IsActive() = %v, want %v", got, want)
			}
		})
	}
}

func mustAmount(t *testing.T, s string) domain.Amount {
	t.Helper()
	a, err := domain.ParseAmount(s)
	if err != nil {
		t.Fatalf("ParseAmount(%q): %v", s, err)
	}
	return a
}
