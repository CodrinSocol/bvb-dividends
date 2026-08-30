// Package common contains what the vertical slices share.
//
// Its sub-packages are the application's infrastructure — configuration,
// logging, the database, the ConnectRPC server, the AIP interceptors — and this
// package holds the small vocabulary the slices have in common: the value
// objects a ticker symbol, a calendar date and a money amount are modelled
// with, the error type they report failures through, and the shapes of a list
// query and its result.
//
// Nothing here may import a slice. The dependency runs one way: a slice depends
// on common, and two slices that need to talk do so through an interface the
// consumer declares and the composition root in cmd/ satisfies.
package common
