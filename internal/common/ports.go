package common

import "context"

// EntityReader reads entities from persistence.
type EntityReader[ID, ENT any] interface {
	List(ctx context.Context, qry ListQuery) (ListResult[ENT], error)

	Get(ctx context.Context, id ID) (ENT, error)
}

// EntityUpserter writes entities to persistence, returning how many rows it
// actually changed.
//
// There is no Create, Update or Delete here, because nothing in this domain is
// authored: every company and every dividend is discovered by importing from
// BVB, and a re-import of the same thing must update the row it already wrote
// rather than create a second one. Upsert is the only write the domain has.
type EntityUpserter[ENT any] interface {
	Upsert(ctx context.Context, ents ...ENT) (int, error)
}

// Repository is the interface a [repository] backing an entity implements.
//
// [repository]: https://martinfowler.com/eaaCatalog/repository.html
type Repository[ID any, ENT any] interface {
	EntityReader[ID, ENT]
	EntityUpserter[ENT]
}

// ChildEntityReader reads entities that belong to a parent resource, which is
// how AIP-122 models a dividend: it is always read within the company that
// declared it.
type ChildEntityReader[ParentID, ID, ENT any] interface {
	List(ctx context.Context, parentID ParentID, qry ListQuery) (ListResult[ENT], error)

	Get(ctx context.Context, parentID ParentID, id ID) (ENT, error)
}
