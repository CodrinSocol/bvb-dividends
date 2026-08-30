package domain

import (
	"fmt"
	"time"
)

// dateLayout is the only textual form a Date accepts or produces.
const dateLayout = "2006-01-02"

// Date is a calendar date, or the absence of one.
//
// Every date on a dividend is genuinely a calendar date: an ex-dividend date
// falls on a trading day, not at an instant. BVB transmits them as xsd:dateTime
// with a meaningless time component, and the Java service stored them as
// LocalDateTime, which is why its queries had to invent start-of-day boundaries
// to compare them. Modelling the absence explicitly matters too, because BVB
// publishes these dates progressively as a dividend moves from announcement to
// payment, so most are unset early on.
type Date struct {
	year  int
	month time.Month
	day   int
	valid bool
}

// NoDate is the zero Date, representing a date BVB has not reported.
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
		return NoDate, fmt.Errorf("%w: %q is not a date in YYYY-MM-DD form", ErrInvalidArgument, s)
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

// Time returns the date as midnight UTC. It returns the zero time when the
// date is absent.
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
