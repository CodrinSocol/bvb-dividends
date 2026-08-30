// Package dividends contains the dividends subdomain.
//
// A dividend belongs to the company that declared it, and is identified by a
// key derived from what BVB reports about it rather than by a number this
// service generates, so a re-import updates the row it wrote the first time and
// a resource name stays valid.
//
// The slice fetches from BVB itself, against the list of companies the
// companies import produced; that list is the only thing that passes between
// the two slices.
package dividends
