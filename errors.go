package oro

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidArgument       = errors.New("oro: invalid argument")
	ErrInvalidQuery          = errors.New("oro: invalid query")
	ErrUnknownField          = errors.New("oro: unknown field")
	ErrUnknownRelation       = errors.New("oro: unknown relation")
	ErrRelationNotLoaded     = errors.New("oro: relation not loaded")
	ErrUnsafeUpdate          = errors.New("oro: unsafe update")
	ErrUnsafeDelete          = errors.New("oro: unsafe delete")
	ErrScan                  = errors.New("oro: scan error")
	ErrHook                  = errors.New("oro: hook error")
	ErrEvent                 = errors.New("oro: event error")
	ErrConflict              = errors.New("oro: conflict")
	ErrConstraint            = errors.New("oro: constraint violation")
	ErrTransactionRequired   = errors.New("oro: transaction required")
	ErrTransactionConnection = errors.New("oro: transaction connection mismatch")
	ErrStaleData             = errors.New("oro: stale data")
	ErrUnknownConnection     = errors.New("oro: unknown connection")
	ErrCrossConnectionQuery  = errors.New("oro: cross connection query")
	ErrTenantRequired        = errors.New("oro: tenant required")
	ErrUnknownTenant         = errors.New("oro: unknown tenant")
	ErrShardRequired         = errors.New("oro: shard required")
	ErrShardNotFound         = errors.New("oro: shard not found")
	ErrShardConflict         = errors.New("oro: shard conflict")
	ErrCrossShardJoin        = errors.New("oro: cross shard join")
	ErrCrossShardTransaction = errors.New("oro: cross shard transaction")
	ErrCacheStoreRequired    = errors.New("oro: cache store required")
	ErrCacheKeyRequired      = errors.New("oro: cache key required")
	ErrOrderRequired         = errors.New("oro: order required")
	ErrClosed                = errors.New("oro: closed")
	ErrDeadlock              = errors.New("oro: deadlock")
	ErrSerializationFailure  = errors.New("oro: serialization failure")
	ErrUnsafeSchemaChange    = errors.New("oro: unsafe schema change")
	ErrAmbiguousSchemaChange = errors.New("oro: ambiguous schema change")
	ErrUnsupported           = errors.New("oro: unsupported")
)

type Error struct {
	Op       string
	Kind     error
	Model    string
	Table    string
	Field    string
	Relation string
	SQL      string
	Args     []any
	Cause    error
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.Cause != nil {
		return fmt.Sprintf("%s: %v: %v", err.Op, err.Kind, err.Cause)
	}
	if err.Kind != nil {
		return fmt.Sprintf("%s: %v", err.Op, err.Kind)
	}
	return err.Op
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func (err *Error) Is(target error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err.Kind, target)
}

func wrapError(op string, kind error, cause error) error {
	if cause == nil && kind == nil {
		return nil
	}
	return &Error{Op: op, Kind: kind, Cause: cause}
}
