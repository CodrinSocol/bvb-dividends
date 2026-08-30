// Package app wires the application together.
//
// Everything a slice needs — the logger, the database, the HTTP server, the
// ConnectRPC server, the scheduler — is provided here, once, and slices
// contribute to it through fx groups: a schema through `pgnamespaces`, a
// protocol interface through `interfaces`, a scheduled import through `jobs`.
// The composition root in cmd/ therefore reads as a list of slices rather than
// as a construction order.
package app
