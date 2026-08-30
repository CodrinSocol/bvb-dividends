package domain

import "errors"

// Sentinel errors returned by the domain and its adapters.
//
// Callers match on these with errors.Is; the transport layer maps them to
// status codes in exactly one place, so handlers never build errors by hand.
var (
	// ErrNotFound means the requested resource does not exist.
	ErrNotFound = errors.New("not found")

	// ErrInvalidArgument means the caller supplied something the domain
	// refuses to represent, such as a malformed ticker symbol.
	ErrInvalidArgument = errors.New("invalid argument")
)
