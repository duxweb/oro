package oro

type Source interface {
	sourceAST() SourceAST
}

type TableSource string

func (source TableSource) sourceAST() SourceAST {
	return SourceAST{Table: string(source)}
}

type QuerySource struct {
	query any
	alias string
}

func Query(query any) QuerySource {
	return QuerySource{query: query}
}

func (source QuerySource) As(alias string) QuerySource {
	source.alias = alias
	return source
}

func (source QuerySource) sourceAST() SourceAST {
	return queryastPendingSource(source.alias, source.query)
}

type Join struct {
	ast JoinAST
}

func (join *Join) As(alias string) *Join {
	join.ast.Alias = alias
	return join
}

func (join *Join) OnColumn(left string, args ...string) *Join {
	join.ast.Conditions = append(join.ast.Conditions, buildJoinColumnCondition("and", left, args...))
	return join
}

func (join *Join) OrOnColumn(left string, args ...string) *Join {
	join.ast.Conditions = append(join.ast.Conditions, buildJoinColumnCondition("or", left, args...))
	return join
}

func (join *Join) Where(field string, args ...any) *Join {
	join.ast.Conditions = append(join.ast.Conditions, buildJoinValueCondition("and", field, args...))
	return join
}

func (join *Join) OrWhere(field string, args ...any) *Join {
	join.ast.Conditions = append(join.ast.Conditions, buildJoinValueCondition("or", field, args...))
	return join
}

func (join *Join) WhereGroup(fn func(q *Join)) *Join {
	if fn == nil {
		return join
	}
	group := &Join{}
	fn(group)
	if len(group.ast.Conditions) == 0 {
		return join
	}
	join.ast.Conditions = append(join.ast.Conditions, JoinCondition{
		Bool:  "and",
		Group: group.ast.Conditions,
	})
	return join
}

func buildJoinColumnCondition(boolOp string, left string, args ...string) JoinCondition {
	condition := JoinCondition{Bool: boolOp, Left: left, Op: "=", Column: true}
	if len(args) == 1 {
		condition.Right = args[0]
	}
	if len(args) >= 2 {
		if !IsSafeColumnOperator(args[0]) {
			condition.Err = &Error{Op: "join", Kind: ErrInvalidArgument, Field: left}
			return condition
		}
		condition.Op = args[0]
		condition.Right = args[1]
	}
	return condition
}

func buildJoinValueCondition(boolOp string, field string, args ...any) JoinCondition {
	condition := JoinCondition{Bool: boolOp, Left: field, Op: "="}
	if len(args) == 1 {
		condition.Value = args[0]
	}
	if len(args) >= 2 {
		op, _ := args[0].(string)
		if !IsSafeConditionOperator(op) {
			condition.Err = &Error{Op: "join", Kind: ErrInvalidArgument, Field: field}
			return condition
		}
		condition.Op = op
		condition.Value = args[1]
	}
	return condition
}

func buildJoin(joinType JoinType, source any, fn func(j *Join)) JoinAST {
	join := &Join{ast: JoinAST{Type: joinType}}
	switch typedSource := source.(type) {
	case string:
		join.ast.Table = typedSource
	case Source:
		join.ast.Source = typedSource.sourceAST()
	default:
		join.ast.Err = &Error{Op: "join", Kind: ErrInvalidArgument}
	}
	if fn != nil {
		fn(join)
	}
	return join.ast
}

func As(field string, alias string) FieldExpr {
	return FieldExpr{Name: field, Alias: alias}
}
