package aipquery

import (
	"go.einride.tech/aip/ordering"
	"go.einride.tech/aip/pagination"

	"github.com/CodrinSocol/bvb-dividends-ro/libs/domain"
)

// Page size bounds, as documented on the request messages.
const (
	// DefaultPageSize is used when the caller does not ask for a size.
	DefaultPageSize = 50
	// MaxPageSize is the largest page the service will return. A larger
	// request is coerced down rather than rejected, as AIP-158 requires.
	MaxPageSize = 1000
)

// CompileOrderBy parses an AIP-132 order_by expression.
//
// An empty expression returns no terms, which leaves the repository's default
// ordering in place.
func (c *Compiler) CompileOrderBy(orderBy string) ([]domain.OrderTerm, error) {
	var parsed ordering.OrderBy
	if err := parsed.UnmarshalString(orderBy); err != nil {
		return nil, invalidf("order_by is not valid: %v", err)
	}

	terms := make([]domain.OrderTerm, 0, len(parsed.Fields))
	for _, sortField := range parsed.Fields {
		f, ok := c.byPath[sortField.Path]
		if !ok {
			return nil, invalidf("field %q cannot be sorted on", sortField.Path)
		}
		terms = append(terms, domain.OrderTerm{Field: f.target, Descending: sortField.Desc})
	}
	return terms, nil
}

// Page resolves the AIP-158 pagination parameters of a list request.
//
// The returned token is the one to continue from; call Next on it to produce
// the next_page_token for the response. The token carries a checksum of every
// other field of the request, so continuing a listing with a different filter
// or ordering fails here rather than silently returning a page from a
// different result set.
func Page(request pagination.Request) (domain.PageRequest, pagination.PageToken, error) {
	token, err := pagination.ParsePageToken(request)
	if err != nil {
		return domain.PageRequest{}, pagination.PageToken{},
			invalidf("page_token is not valid for this request; it must be used with the same filter and ordering")
	}

	size := int(request.GetPageSize())
	switch {
	case size <= 0:
		size = DefaultPageSize
	case size > MaxPageSize:
		size = MaxPageSize
	}

	return domain.PageRequest{Size: size, Offset: token.Offset}, token, nil
}

// NextPageToken returns the token for the page after the one just returned, or
// an empty string when this page was the last.
func NextPageToken(token pagination.PageToken, request pagination.Request, page domain.PageRequest, total int64) string {
	if page.Offset+int64(page.Size) >= total {
		return ""
	}
	return token.Next(request).String()
}
