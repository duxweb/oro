package oro

import "github.com/duxweb/oro/internal/queryast"

const (
	// JoinInner represents an INNER JOIN.
	JoinInner = queryast.JoinInner
	// JoinLeft represents a LEFT JOIN.
	JoinLeft = queryast.JoinLeft
	// JoinRight represents a RIGHT JOIN.
	JoinRight = queryast.JoinRight
	// JoinFull represents a FULL JOIN.
	JoinFull = queryast.JoinFull
	// JoinCross represents a CROSS JOIN.
	JoinCross = queryast.JoinCross
)

const (
	// LockNone means no row lock is requested.
	LockNone = queryast.LockNone
	// LockUpdate requests an update lock.
	LockUpdate = queryast.LockUpdate
	// LockShare requests a shared lock.
	LockShare = queryast.LockShare
)

// Statement is a query AST statement.
type Statement = queryast.Statement

// SelectAST is a SELECT statement AST.
type SelectAST = queryast.SelectAST

// JoinAST is a JOIN clause AST.
type JoinAST = queryast.JoinAST

// SourceAST is a table, subquery, or raw source AST.
type SourceAST = queryast.SourceAST

// JoinType identifies a SQL join type.
type JoinType = queryast.JoinType

// JoinCondition is a condition in a JOIN clause.
type JoinCondition = queryast.JoinCondition

// InsertAST is an INSERT statement AST.
type InsertAST = queryast.InsertAST

// UpdateAST is an UPDATE statement AST.
type UpdateAST = queryast.UpdateAST

// DeleteAST is a DELETE statement AST.
type DeleteAST = queryast.DeleteAST

// RawSpec stores raw SQL and bound arguments.
type RawSpec = queryast.RawSpec

// CompiledSQL is a dialect-compiled SQL string with bound arguments.
type CompiledSQL = queryast.CompiledSQL

// Condition is a structured query condition.
type Condition = queryast.Condition

// ColumnCondition compares one column to another column.
type ColumnCondition = queryast.ColumnCondition

// CountCondition constrains relation counts.
type CountCondition = queryast.CountCondition

// SelectExpr is one SELECT item.
type SelectExpr = queryast.SelectExpr

// OrderExpr is one ORDER BY item.
type OrderExpr = queryast.OrderExpr

// ConflictSpec describes an upsert conflict action.
type ConflictSpec = queryast.ConflictSpec

// LockMode identifies a row-lock mode.
type LockMode = queryast.LockMode

// LockSpec describes requested row locking.
type LockSpec = queryast.LockSpec

// QuerySpec is the normalized read-query specification used by planners.
type QuerySpec struct {
	Connection string
	ShardGroup string
	Table      string
	Alias      string
	From       SourceAST
	ModelName  string
	Where      []Condition
	Select     []SelectExpr
	SelectErr  error
	Joins      []JoinAST
	Group      []string
	Having     []Condition
	Order      []OrderExpr
	Limit      *int
	Offset     *int
	Lock       LockSpec
	With       []WithSpec
	SkipEvents bool
	UsePrimary bool
	Cache      CacheSpec
	Timeout    int64
	Model      *ModelSchema
	finalized  bool
}

// CacheSpec describes per-query result caching.
type CacheSpec struct {
	Enabled bool
	TTL     int64
	Key     string
	Tags    []string
}

// WriteSpec is the normalized write-query specification used by executors.
type WriteSpec struct {
	QuerySpec
	Values    []Map
	Primary   []string
	Conflict  ConflictSpec
	Returning bool
	Operation string
}

// WithSpec describes an eager-loaded relation.
type WithSpec struct {
	Name     string
	Relation *RelationSchema
	Callback func(*RelationQuery)
}

func queryastPendingSource(alias string, query any) SourceAST {
	return queryast.PendingSource(alias, query)
}

type LockOption interface {
	applyLockOption(*LockSpec)
}

type lockOptionFunc func(*LockSpec)

func (fn lockOptionFunc) applyLockOption(spec *LockSpec) {
	fn(spec)
}

// NoWait requests non-blocking lock acquisition when supported by the driver.
func NoWait() LockOption {
	return lockOptionFunc(func(spec *LockSpec) {
		spec.NoWait = true
	})
}

// SkipLocked skips already locked rows when supported by the driver.
func SkipLocked() LockOption {
	return lockOptionFunc(func(spec *LockSpec) {
		spec.SkipLocked = true
	})
}

func applyLockOptions(mode LockMode, options []LockOption) LockSpec {
	spec := LockSpec{Mode: mode}
	for _, option := range options {
		if option != nil {
			option.applyLockOption(&spec)
		}
	}
	return spec
}
