package common

var (
	// ErrEntityInvalid is returned when a given entity is invalid.
	ErrEntityInvalid = NewError(ErrorCodeInvalidArgument, "entity invalid")
	// ErrEntityNotFound is returned when a requested entity could not be found.
	ErrEntityNotFound = NewError(ErrorCodeNotFound, "entity not found")
	// ErrEntityConstraintViolation is returned when a specific, entity-related
	// constraint is violated.
	ErrEntityConstraintViolation = NewError(ErrorCodeInvalidArgument, "entity constraint violation")
)
