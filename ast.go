package oro

import "github.com/duxweb/oro/internal/queryast"

const (
	JoinInner = queryast.JoinInner
	JoinLeft  = queryast.JoinLeft
	JoinRight = queryast.JoinRight
	JoinFull  = queryast.JoinFull
	JoinCross = queryast.JoinCross
)

const (
	LockNone   = queryast.LockNone
	LockUpdate = queryast.LockUpdate
	LockShare  = queryast.LockShare
)

type Statement = queryast.Statement
type SelectAST = queryast.SelectAST
type JoinAST = queryast.JoinAST
type SourceAST = queryast.SourceAST
type JoinType = queryast.JoinType
type JoinCondition = queryast.JoinCondition
type InsertAST = queryast.InsertAST
type UpdateAST = queryast.UpdateAST
type DeleteAST = queryast.DeleteAST
type RawSpec = queryast.RawSpec
type CompiledSQL = queryast.CompiledSQL
type Condition = queryast.Condition
type ColumnCondition = queryast.ColumnCondition
type CountCondition = queryast.CountCondition
type SelectExpr = queryast.SelectExpr
type OrderExpr = queryast.OrderExpr
type ConflictSpec = queryast.ConflictSpec
type LockMode = queryast.LockMode
type LockSpec = queryast.LockSpec

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
}

type CacheSpec struct {
	Enabled bool
	TTL     int64
	Key     string
	Tags    []string
}

type WriteSpec struct {
	QuerySpec
	Values    []Map
	Primary   []string
	Conflict  ConflictSpec
	Returning bool
}

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

func NoWait() LockOption {
	return lockOptionFunc(func(spec *LockSpec) {
		spec.NoWait = true
	})
}

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
