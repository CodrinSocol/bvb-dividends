// Package companies contains the companies subdomain.
//
// A company is a Bucharest Stock Exchange listing that has announced at least
// one dividend. Companies are discovered by importing from BVB and are never
// authored, so the slice owns three things: the entity, the import that
// discovers it, and the read side the API exposes.
//
// The slice may not import another slice. Where the dividends slice needs
// something from here — whether a company exists — it declares an interface of
// its own and the composition root in cmd/ satisfies it with this service.
package companies
