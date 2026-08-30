package common

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	// ErrSymbolInvalid is returned when a string cannot be a ticker symbol.
	ErrSymbolInvalid = NewError(ErrorCodeInvalidArgument, "invalid ticker symbol")
	// ErrDateInvalid is returned when a string cannot be a calendar date.
	ErrDateInvalid = NewError(ErrorCodeInvalidArgument, "invalid date")
	// ErrAmountInvalid is returned when a string cannot be a money amount.
	ErrAmountInvalid = NewError(ErrorCodeInvalidArgument, "invalid amount")
)

// MaxSymbolLen is the widest ticker the database column accepts.
const MaxSymbolLen = 20

// Symbol is a Bucharest Stock Exchange ticker, such as "SNP" or "TLV".
//
// Symbols are always upper-case: BVB reports them inconsistently, and the
// symbol doubles as a primary key and as a resource name segment, so
// normalising once at the edge keeps lookups and URLs stable.
type Symbol string

// ParseSymbol normalises and validates a ticker symbol.
func ParseSymbol(s string) (Symbol, error) {
	normalised := strings.ToUpper(strings.TrimSpace(s))
	if normalised == "" {
		return "", ErrSymbolInvalid.WithProblem("symbol", "is empty")
	}
	if len(normalised) > MaxSymbolLen {
		return "", ErrSymbolInvalid.WithProblem("symbol", "is longer than 20 characters")
	}
	for _, r := range normalised {
		if !isSymbolRune(r) {
			return "", ErrSymbolInvalid.WithProblem("symbol", "contains "+string(r))
		}
	}

	return Symbol(normalised), nil
}

// isSymbolRune reports whether r may appear in a ticker. BVB uses plain
// alphanumerics for shares and adds separators for rights and other
// instruments, for example "SNP" and "TLV.R".
func isSymbolRune(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '.', r == '-', r == '_':
		return true
	default:
		return false
	}
}

// String returns the symbol as written, for example "SNP".
func (s Symbol) String() string { return string(s) }

// dateLayout is the only textual form a [Date] accepts or produces.
const dateLayout = "2006-01-02"

// Date is a calendar date, or the absence of one.
//
// Every date on a dividend is genuinely a calendar date: an ex-dividend date
// falls on a trading day, not at an instant. BVB transmits them as xsd:dateTime
// with a meaningless time component, and the previous service stored them as
// a datetime, which is why its queries had to invent start-of-day boundaries
// to compare them. Modelling the absence explicitly matters too, because BVB
// publishes these dates progressively as a dividend moves from announcement to
// payment, so most are unset early on.
type Date struct {
	year  int
	month time.Month
	day   int
	valid bool
}

// NoDate is the zero [Date], representing a date BVB has not reported.
var NoDate = Date{}

// NewDate returns a present Date. The components are normalised the same way
// time.Date normalises them, so NewDate(2026, 13, 1) is January 2027.
func NewDate(year int, month time.Month, day int) Date {
	t := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	return Date{year: t.Year(), month: t.Month(), day: t.Day(), valid: true}
}

// DateOf returns the calendar date of t in t's own location.
func DateOf(t time.Time) Date {
	return Date{year: t.Year(), month: t.Month(), day: t.Day(), valid: true}
}

// ParseDate parses a date in YYYY-MM-DD form.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return NoDate, ErrDateInvalid.WithProblem("date", s+" is not in YYYY-MM-DD form").WithUnderlying(err)
	}

	return DateOf(t), nil
}

// Valid reports whether a date was reported at all.
func (d Date) Valid() bool { return d.valid }

// Year returns the year of a present date, or zero for an absent one.
func (d Date) Year() int { return d.year }

// Month returns the month of a present date, or zero for an absent one.
func (d Date) Month() time.Month { return d.month }

// Day returns the day of month of a present date, or zero for an absent one.
func (d Date) Day() int { return d.day }

// Time returns the date as midnight UTC, or the zero time when it is absent.
func (d Date) Time() time.Time {
	if !d.valid {
		return time.Time{}
	}

	return time.Date(d.year, d.month, d.day, 0, 0, 0, 0, time.UTC)
}

// String returns the date in YYYY-MM-DD form, or "" when absent.
func (d Date) String() string {
	if !d.valid {
		return ""
	}

	return d.Time().Format(dateLayout)
}

// Compare orders two present dates chronologically, returning a negative
// number, zero, or a positive number as d is before, equal to, or after other.
//
// An absent date sorts before every present date, and two absent dates are
// equal. Callers that must distinguish "unknown" from "earliest" should check
// Valid rather than relying on this ordering.
func (d Date) Compare(other Date) int {
	if !d.valid || !other.valid {
		switch {
		case d.valid == other.valid:
			return 0
		case !d.valid:
			return -1
		default:
			return 1
		}
	}

	return d.Time().Compare(other.Time())
}

// Before reports whether d is a present date strictly before other.
func (d Date) Before(other Date) bool {
	return d.valid && other.valid && d.Compare(other) < 0
}

// After reports whether d is a present date strictly after other.
func (d Date) After(other Date) bool {
	return d.valid && other.valid && d.Compare(other) > 0
}

// Equal reports whether two dates are both absent, or the same calendar day.
func (d Date) Equal(other Date) bool {
	return d.valid == other.valid && d.Compare(other) == 0
}

// Currency is the ISO 4217 code every amount in this domain is denominated in.
//
// BVB is a single-currency market, so amounts carry no currency of their own;
// the code is attached once, at the API boundary.
const Currency = "RON"

// Amount is an exact monetary value, or the absence of one.
//
// BVB leaves dividend amounts unset while a distribution is still being
// approved, so "no amount" is a distinct state from zero: a zero dividend and
// an unannounced dividend must not read the same. The value is a decimal
// rather than a float because these are money figures that round-trip through
// a numeric(20,4) column and are compared for equality.
type Amount struct {
	value decimal.Decimal
	valid bool
}

// NoAmount is the zero [Amount], representing a value BVB has not reported.
var NoAmount = Amount{}

// NewAmount returns a present Amount holding d.
func NewAmount(d decimal.Decimal) Amount {
	return Amount{value: d, valid: true}
}

// ParseAmount parses a decimal string such as "0.1234".
func ParseAmount(s string) (Amount, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return NoAmount, ErrAmountInvalid.WithProblem("amount", s+" is not a decimal").WithUnderlying(err)
	}

	return NewAmount(d), nil
}

// Valid reports whether an amount was reported at all.
func (a Amount) Valid() bool { return a.valid }

// Decimal returns the underlying value. It is the zero decimal when the amount
// is absent, so check Valid first where the difference matters.
func (a Amount) Decimal() decimal.Decimal { return a.value }

// String returns the exact decimal representation, or "" when absent.
func (a Amount) String() string {
	if !a.valid {
		return ""
	}

	return a.value.String()
}

// Equal reports whether two amounts are both absent, or both present and
// numerically equal. It deliberately ignores trailing-zero differences, so
// "0.10" and "0.1000" compare equal.
func (a Amount) Equal(other Amount) bool {
	if a.valid != other.valid {
		return false
	}

	return !a.valid || a.value.Equal(other.value)
}
