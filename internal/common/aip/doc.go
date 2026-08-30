// Package aip contains utilities for working with Google's [API Improvement Proposals].
//
// The standard list parameters are handled once, as ConnectRPC interceptors,
// rather than in every method: the AIP-160 filter and the AIP-132 ordering are
// parsed and type-checked against the resource's own proto message and put on
// the context, and the AIP-158 page token is wrapped in an envelope carrying a
// checksum of the rest of the request. A slice therefore reads a filter it can
// trust, over fields the proto actually declares, and cannot be handed a page
// token belonging to a different query.
//
// [API Improvement Proposals]: https://google.aip.dev/
package aip
