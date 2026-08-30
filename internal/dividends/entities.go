package dividends

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

// idNamespace seeds the UUIDv5 derivation of dividend identifiers.
//
// It must never change: every dividend's identity, and therefore every resource
// name this API has ever handed out, is derived from it.
var idNamespace = uuid.MustParse("8bac01cb-0156-4b9d-9c9b-8f2ccc00da6e")

// ErrIDInvalid is returned when a string cannot be a dividend identifier.
var ErrIDInvalid = common.NewError(common.ErrorCodeInvalidArgument, "invalid dividend identifier")

// ID identifies a dividend.
//
// IDs are derived from the dividend's [NaturalKey] rather than generated, so
// re-importing the same dividend produces the same ID. The previous service
// generated random UUIDs and deleted-then-reinserted every company's
// dividends on each daily run, which changed every ID every day and broke any
// link anyone had saved.
type ID uuid.UUID

// ParseID parses the canonical UUID form of an ID.
func ParseID(s string) (ID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return ID{}, ErrIDInvalid.WithProblem("id", s+" is not a UUID").WithUnderlying(err)
	}

	return ID(parsed), nil
}

// String returns the canonical UUID form.
func (id ID) String() string { return uuid.UUID(id).String() }

// UUID returns the identifier as a plain UUID, for the database.
func (id ID) UUID() uuid.UUID { return uuid.UUID(id) }

// NaturalKey is what makes a dividend the same dividend across imports.
//
// BVB has no stable identifier of its own for a dividend, but a company
// declares at most one dividend of a given type, for a given fiscal year, going
// ex on a given date. That tuple is the identity.
type NaturalKey struct {
	Company    common.Symbol
	Year       int
	Type       string
	ExDividend common.Date
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
// never go stale. The names are the ones the API enum uses, because the
// database derives the same value under the same names for filtering.
type State string

// The stages a dividend passes through.
const (
	// StateUnspecified is the zero value and is never returned by State.
	StateUnspecified State = "STATE_UNSPECIFIED"

	// StateAnnounced means the ex-dividend date is still in the future, so
	// buying the share today still earns this dividend.
	StateAnnounced State = "STATE_ANNOUNCED"

	// StateExPassed means the ex-dividend date has passed but payment has not
	// started.
	//nolint:gosec // G101 reads "PASSED" as a credential; it is a lifecycle stage.
	StateExPassed State = "STATE_EX_PASSED"

	// StatePaying means the payment window is open.
	StatePaying State = "STATE_PAYING"

	// StatePaid means the payment window has closed.
	StatePaid State = "STATE_PAID"

	// StateUndated means BVB reported no ex-dividend date, so the stage cannot
	// be derived.
	StateUndated State = "STATE_UNDATED"
)

// String returns the state as the API names it.
func (s State) String() string { return string(s) }

// Amounts holds the money figures BVB reports for a dividend. Any of them may
// be absent while the distribution is still being approved.
type Amounts struct {
	// GrossPerShareNaturalPerson is the gross per-share amount payable to
	// natural persons, before withholding tax.
	GrossPerShareNaturalPerson common.Amount

	// GrossPerShareLegalPerson is the gross per-share amount payable to legal
	// persons, before withholding tax.
	GrossPerShareLegalPerson common.Amount

	// Total is the total amount distributed across all shares.
	Total common.Amount
}

// Schedule holds the key dates of a dividend. Any of them may be absent.
type Schedule struct {
	AnnouncementDate common.Date
	GMSReferenceDate common.Date
	GMSDate          common.Date
	RecordDate       common.Date
	ExDividendDate   common.Date
	PaymentStartDate common.Date
	PaymentEndDate   common.Date
}

// Dividend is a dividend announced by a listed company.
type Dividend struct {
	// ID is derived from [NaturalKey]; see [NewID].
	ID ID

	// Company is the ticker of the company that declared the dividend.
	Company common.Symbol

	// Year is the fiscal year the dividend relates to.
	Year int

	// Type is the dividend type as reported by BVB, for example "cash". It is
	// an open set owned by BVB, so it is carried through unmodified rather than
	// mapped onto an enum this project would have to guess at.
	Type string

	// Amounts holds the reported money figures.
	Amounts Amounts

	// Schedule holds the reported dates.
	Schedule Schedule

	// DistributionMethod describes how the dividend is paid out, as reported by
	// BVB.
	DistributionMethod string

	// State is the stage the dividend had reached when it was read. It is
	// derived rather than stored; see [Dividend.StateAt].
	State State

	CreateTime time.Time
	UpdateTime time.Time
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

// StateAt returns the stage this dividend has reached as of now.
//
// The same rule is written as a SQL expression in the slice's filterable view,
// so that filtering on the state and reading it back cannot disagree; the
// integration tests assert that the two agree for every stage.
func (d Dividend) StateAt(now time.Time) State {
	ex := d.Schedule.ExDividendDate
	if !ex.Valid() {
		return StateUndated
	}

	today := common.DateOf(now)

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
// share, which is what the previous service's /active-dividends endpoint
// meant.
func (d Dividend) IsActive(now time.Time) bool {
	return d.StateAt(now) == StateAnnounced
}

// Validate reports whether the dividend is well formed enough to persist.
func (d Dividend) Validate() error {
	if _, err := common.ParseSymbol(d.Company.String()); err != nil {
		return err
	}

	return nil
}
