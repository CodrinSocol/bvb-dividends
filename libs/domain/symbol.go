package domain

import (
	"fmt"
	"strings"
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
		return "", fmt.Errorf("%w: symbol is empty", ErrInvalidArgument)
	}
	if len(normalised) > MaxSymbolLen {
		return "", fmt.Errorf("%w: symbol %q is longer than %d characters",
			ErrInvalidArgument, s, MaxSymbolLen)
	}
	for _, r := range normalised {
		if !isSymbolRune(r) {
			return "", fmt.Errorf("%w: symbol %q contains invalid character %q",
				ErrInvalidArgument, s, r)
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
