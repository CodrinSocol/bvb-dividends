package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

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

// NoAmount is the zero Amount, representing a value BVB has not reported.
var NoAmount = Amount{}

// NewAmount returns a present Amount holding d.
func NewAmount(d decimal.Decimal) Amount {
	return Amount{value: d, valid: true}
}

// ParseAmount parses a decimal string such as "0.1234".
func ParseAmount(s string) (Amount, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return NoAmount, fmt.Errorf("%w: %q is not a decimal: %w", ErrInvalidArgument, s, err)
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
