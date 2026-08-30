package domain

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// idNamespace seeds the UUIDv5 derivation of dividend identifiers.
//
// It must never change: every dividend's identity, and therefore every
// resource name this API has ever handed out, is derived from it.
var idNamespace = uuid.MustParse("8bac01cb-0156-4b9d-9c9b-8f2ccc00da6e")

// ID identifies a dividend.
//
// IDs are derived from the dividend's NaturalKey rather than generated, so
// re-importing the same dividend produces the same ID. The Java service
// generated random UUIDs and deleted-then-reinserted every company's dividends
// on each daily run, which changed every ID every day and broke any link
// anyone had saved.
type ID uuid.UUID

// ParseID parses the canonical UUID form of an ID.
func ParseID(s string) (ID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return ID{}, ErrInvalidArgument
	}
	return ID(parsed), nil
}

// String returns the canonical UUID form.
func (id ID) String() string { return uuid.UUID(id).String() }

// NaturalKey is what makes a dividend the same dividend across imports.
//
// BVB has no stable identifier of its own for a dividend, but a company
// declares at most one dividend of a given type, for a given fiscal year, going
// ex on a given date. That tuple is the identity.
type NaturalKey struct {
	Company    Symbol
	Year       int
	Type       string
	ExDividend Date
}

// NaturalKey returns the key identifying this dividend across imports.
func (d Dividend) NaturalKey() NaturalKey {
	return NaturalKey{
		Company:    d.Company,
		Year:       d.Year,
		Type:       d.Type,
		ExDividend: d.Schedule.ExDividendDate,
	}
}

// String returns an unambiguous encoding of the key.
//
// Each component is length-prefixed rather than delimited, because Type is a
// free-form string owned by BVB: with a plain separator, a crafted or merely
// unusual type could make two different keys encode identically and collapse
// two dividends into one row.
func (k NaturalKey) String() string {
	var b strings.Builder
	for _, field := range []string{
		string(k.Company),
		strconv.Itoa(k.Year),
		k.Type,
		k.ExDividend.String(),
	} {
		b.WriteString(strconv.Itoa(len(field)))
		b.WriteByte(':')
		b.WriteString(field)
	}
	return b.String()
}

// NewID derives the stable identifier of the dividend named by k.
func NewID(k NaturalKey) ID {
	return ID(uuid.NewSHA1(idNamespace, []byte(k.String())))
}

// State is the stage a dividend has reached in its lifecycle.
//
// It is derived from the schedule at read time rather than stored, so it can
// never go stale.
type State uint8

const (
	// StateUnspecified is the zero value and is never returned by State.
	StateUnspecified State = iota

	// StateAnnounced means the ex-dividend date is still in the future, so
	// buying the share today still earns this dividend.
	StateAnnounced

	// StateExPassed means the ex-dividend date has passed but payment has not
	// started.
	StateExPassed

	// StatePaying means the payment window is open.
	StatePaying

	// StatePaid means the payment window has closed.
	StatePaid

	// StateUndated means BVB reported no ex-dividend date, so the stage cannot
	// be derived.
	StateUndated
)

// String returns a short lower-case name for the state.
func (s State) String() string {
	switch s {
	case StateAnnounced:
		return "announced"
	case StateExPassed:
		return "ex_passed"
	case StatePaying:
		return "paying"
	case StatePaid:
		return "paid"
	case StateUndated:
		return "undated"
	default:
		return "unspecified"
	}
}

// Amounts holds the money figures BVB reports for a dividend. Any of them may
// be absent while the distribution is still being approved.
type Amounts struct {
	// GrossPerShareNaturalPerson is the gross per-share amount payable to
	// natural persons, before withholding tax.
	GrossPerShareNaturalPerson Amount

	// GrossPerShareLegalPerson is the gross per-share amount payable to legal
	// persons, before withholding tax.
	GrossPerShareLegalPerson Amount

	// Total is the total amount distributed across all shares.
	Total Amount
}

// Schedule holds the key dates of a dividend. Any of them may be absent.
type Schedule struct {
	AnnouncementDate Date
	GMSReferenceDate Date
	GMSDate          Date
	RecordDate       Date
	ExDividendDate   Date
	PaymentStartDate Date
	PaymentEndDate   Date
}

// Dividend is a dividend announced by a listed company.
type Dividend struct {
	// ID is derived from NaturalKey; see NewID.
	ID ID

	// Company is the ticker of the company that declared the dividend.
	Company Symbol

	// Year is the fiscal year the dividend relates to.
	Year int

	// Type is the dividend type as reported by BVB, for example "cash". It is
	// an open set owned by BVB, so it is carried through unmodified rather
	// than mapped onto an enum this project would have to guess at.
	Type string

	// Amounts holds the reported money figures.
	Amounts Amounts

	// Schedule holds the reported dates.
	Schedule Schedule

	// DistributionMethod describes how the dividend is paid out, as reported
	// by BVB.
	DistributionMethod string

	CreatedAt time.Time
	UpdatedAt time.Time
}

// State returns the stage this dividend has reached as of now.
func (d Dividend) State(now time.Time) State {
	ex := d.Schedule.ExDividendDate
	if !ex.Valid() {
		return StateUndated
	}
	today := DateOf(now)

	// The ex-dividend date is the first day the share trades without
	// entitlement, so a dividend stops being claimable on that date, not after
	// it.
	if ex.After(today) {
		return StateAnnounced
	}

	start, end := d.Schedule.PaymentStartDate, d.Schedule.PaymentEndDate
	if end.Valid() && end.Before(today) {
		return StatePaid
	}
	if start.Valid() && !start.After(today) {
		return StatePaying
	}
	return StateExPassed
}

// IsActive reports whether the dividend can still be earned by buying the
// share, which is what the Java service's /active-dividends endpoint meant.
func (d Dividend) IsActive(now time.Time) bool {
	return d.State(now) == StateAnnounced
}

// Validate reports whether the dividend is well formed enough to persist.
func (d Dividend) Validate() error {
	if _, err := ParseSymbol(string(d.Company)); err != nil {
		return err
	}
	return nil
}
