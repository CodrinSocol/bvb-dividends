// Package bvbclient talks to the Bucharest Stock Exchange financials web
// service.
//
// It is a client library and nothing more: it knows that BVB speaks SOAP over
// HTTP, and it returns what BVB reports in BVB's own shape. Deciding what those
// values mean — which of them are dates, which are money, which company they
// belong to — is the caller's business, and in this repository it is done by
// the vertical slice that owns the resource.
//
// # On the wire format
//
// BVB publishes a WSDL at https://ws.bvb.ro/BVBFinancialsWS/Financials.asmx?WSDL
// and the service this project replaces generated its request and response
// classes from it at build time, so the element names were never written down
// in that repository either. The types in types.go are reconstructed from how
// that generated code was used, which pins every field, and are covered by the
// recorded responses in testdata/. Where BVB's real casing turns out to
// differ, types.go is the only file that needs to change.
//
// # Errors
//
// This package uses the standard library's errors rather than the error
// package the application uses, so that depending on it does not oblige a
// caller to adopt one. Faults carry BVB's own message; see [ErrInvalidArgument]
// for the one error it raises on its own.
package bvbclient
