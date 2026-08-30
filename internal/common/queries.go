package common

import (
	"context"
	"iter"

	"github.com/cockroachdb/errors"
	"github.com/google/cel-go/cel"
)

var (
	// ErrPageTokenInvalid is returned when an invalid page token is given.
	ErrPageTokenInvalid = NewError(ErrorCodeInvalidArgument, "invalid page token")
	// ErrPaginationQueryModified is returned when the query parameters change
	// part-way through a paginated listing.
	ErrPaginationQueryModified = NewError(ErrorCodeInvalidArgument, "pagination query modified")
	// ErrFilterInvalid is returned when an invalid filter is given.
	ErrFilterInvalid = NewError(ErrorCodeInvalidArgument, "invalid filter")
	// ErrOrderByInvalid is returned when an invalid order by value is given.
	ErrOrderByInvalid = NewError(ErrorCodeInvalidArgument, "invalid order by")
)

// Filter is a parsed and type-checked AIP-160 filter expression.
//
// It is carried as a CEL AST rather than as text: the expression has already
// been checked against the fields the resource actually declares by the time a
// repository sees it, so the repository translates a tree it can trust instead
// of parsing a string a caller wrote.
type Filter struct {
	Ast *cel.Ast
}

// NewFilterEmpty creates a new, empty instance of [Filter].
func NewFilterEmpty() Filter {
	return Filter{Ast: nil}
}

// Empty returns true if the [Filter] is empty.
func (f Filter) Empty() bool { return f.Ast == nil }

// NotEmpty returns true if the [Filter] is not empty.
func (f Filter) NotEmpty() bool { return !f.Empty() }

// OrderDirection defines the direction of ordering.
type OrderDirection int

const (
	// OrderDirectionAsc indicates ordering in ascending direction.
	OrderDirectionAsc OrderDirection = iota
	// OrderDirectionDesc indicates ordering in descending direction.
	OrderDirectionDesc
)

// String returns the string representation of an [OrderDirection].
func (obd OrderDirection) String() string {
	if obd == OrderDirectionDesc {
		return "DESC"
	}

	return "ASC"
}

// OrderByClause orders a collection by one path.
type OrderByClause struct {
	Path      string
	Direction OrderDirection
}

// Equal returns true if a given [OrderByClause] is equal to this one.
func (obc OrderByClause) Equal(other OrderByClause) bool {
	return obc.Path == other.Path && obc.Direction == other.Direction
}

// OrderBy is every [OrderByClause] to apply to a collection, in order.
type OrderBy []OrderByClause

// ContainsClause returns true if the given clause is present.
func (ob OrderBy) ContainsClause(path string, direction OrderDirection) bool {
	clause := OrderByClause{Path: path, Direction: direction}
	for _, c := range ob {
		if c.Equal(clause) {
			return true
		}
	}

	return false
}

// ContainsPath returns true if a clause with the given path is present.
func (ob OrderBy) ContainsPath(path string) bool {
	return ob.ContainsClause(path, OrderDirectionAsc) || ob.ContainsClause(path, OrderDirectionDesc)
}

// ListQuery is everything needed to read one page of a collection.
type ListQuery struct {
	PageSize  int32
	PageToken string
	Filter    Filter
	OrderBy   OrderBy
}

// ListResult is the outcome of a [ListQuery].
//
// TotalSize is a pointer because counting the whole collection costs a second
// query: a nil total means the caller did not ask for one, which is different
// from a total of zero.
type ListResult[T any] struct {
	Items         []T
	NextPageToken string
	TotalSize     *uint32
}

// ListQueryPages iterates the pages a [ListQuery] returns, following the next
// page token until the listing is exhausted.
func ListQueryPages[T any](
	ctx context.Context,
	qryFn func(context.Context, ListQuery) (ListResult[T], error),
	qry ListQuery,
) iter.Seq2[[]T, error] {
	return func(yield func([]T, error) bool) {
		for {
			res, err := qryFn(ctx, qry)
			if !yield(res.Items, errors.WithStack(err)) {
				return
			}

			qry.PageToken = res.NextPageToken
			if qry.PageToken == "" {
				break
			}
		}
	}
}
