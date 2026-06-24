package queryast

import internaltypes "github.com/duxweb/oro/internal/types"

type Statement interface {
	statementNode()
}

type SelectAST struct {
	Table  string
	Alias  string
	From   SourceAST
	Joins  []JoinAST
	Where  []Condition
	Select []SelectExpr
	Group  []string
	Having []Condition
	Order  []OrderExpr
	Limit  *int
	Offset *int
	Lock   LockSpec
}

func (SelectAST) statementNode() {}

type JoinAST struct {
	Type       JoinType
	Table      string
	Alias      string
	Source     SourceAST
	Raw        *RawSpec
	Err        error
	Conditions []JoinCondition
}

type SourceAST struct {
	Table string
	Alias string
	Query *SelectAST
	Raw   *RawSpec
	query any
}

func PendingSource(alias string, query any) SourceAST {
	return SourceAST{Alias: alias, query: query}
}

func (source SourceAST) PendingQuery() any {
	return source.query
}

func (source *SourceAST) Resolve(resolved SourceAST) {
	if source == nil {
		return
	}
	source.Query = resolved.Query
	source.Raw = resolved.Raw
	source.Table = resolved.Table
	source.query = nil
}

type JoinType int

const (
	JoinInner JoinType = iota + 1
	JoinLeft
	JoinRight
	JoinFull
	JoinCross
)

type JoinCondition struct {
	Bool   string
	Left   string
	Op     string
	Right  string
	Value  any
	Column bool
	Group  []JoinCondition
}

type InsertAST struct {
	Table     string
	Values    []internaltypes.Map
	Conflict  ConflictSpec
	Returning bool
}

func (InsertAST) statementNode() {}

type UpdateAST struct {
	Table  string
	Values internaltypes.Map
	Where  []Condition
}

func (UpdateAST) statementNode() {}

type DeleteAST struct {
	Table string
	Where []Condition
}

func (DeleteAST) statementNode() {}

type RawSpec struct {
	SQL  string
	Args []any
}

type CompiledSQL struct {
	SQL  string
	Args []any
}

type Condition struct {
	Bool       string
	Field      string
	Op         string
	Value      any
	Conditions []Condition
}

type ColumnCondition struct {
	Op    string
	Right string
}

type CountCondition struct {
	Source *SourceAST
	Op     string
	Value  int64
}

type SelectExpr struct {
	Expr   string
	Alias  string
	Raw    bool
	Args   []any
	Source *SourceAST
}

type OrderExpr struct {
	Expr string
	Desc bool
	Raw  bool
	Args []any
}

type ConflictSpec struct {
	Columns   []string
	DoNothing bool
	UpdateAll bool
	Update    []string
	UpdateMap internaltypes.Map
}

type LockMode int

const (
	LockNone LockMode = iota
	LockUpdate
	LockShare
)

type LockSpec struct {
	Mode       LockMode
	NoWait     bool
	SkipLocked bool
}
