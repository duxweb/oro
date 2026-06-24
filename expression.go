package oro

type FieldExpr struct {
	Name  string
	Alias string
}

func Field(name string) FieldExpr {
	return FieldExpr{Name: name}
}

func Column(name string) FieldExpr {
	return FieldExpr{Name: name}
}

func (field FieldExpr) Eq(value any) Condition {
	return Condition{Field: field.Name, Op: "=", Value: value}
}

func (field FieldExpr) NotEq(value any) Condition {
	return Condition{Field: field.Name, Op: "!=", Value: value}
}

func (field FieldExpr) Gt(value any) Condition {
	return Condition{Field: field.Name, Op: ">", Value: value}
}

func (field FieldExpr) Gte(value any) Condition {
	return Condition{Field: field.Name, Op: ">=", Value: value}
}

func (field FieldExpr) Lt(value any) Condition {
	return Condition{Field: field.Name, Op: "<", Value: value}
}

func (field FieldExpr) Lte(value any) Condition {
	return Condition{Field: field.Name, Op: "<=", Value: value}
}

func (field FieldExpr) Like(value any) Condition {
	return Condition{Field: field.Name, Op: "like", Value: value}
}

func (field FieldExpr) NotLike(value any) Condition {
	return Condition{Field: field.Name, Op: "not like", Value: value}
}

func (field FieldExpr) In(values ...any) Condition {
	return Condition{Field: field.Name, Op: "in_values", Value: append([]any(nil), values...)}
}

func (field FieldExpr) NotIn(values ...any) Condition {
	return Condition{Field: field.Name, Op: "not_in_values", Value: append([]any(nil), values...)}
}

func (field FieldExpr) Between(start any, end any) Condition {
	return Condition{Field: field.Name, Op: "between", Value: []any{start, end}}
}

func (field FieldExpr) IsNull() Condition {
	return isNullCondition(field.Name)
}

func (field FieldExpr) IsNotNull() Condition {
	return isNotNullCondition(field.Name)
}

func (field FieldExpr) EqCol(right string) Condition {
	return buildColumnCondition(field.Name, "=", right)
}

func (field FieldExpr) NotEqCol(right string) Condition {
	return buildColumnCondition(field.Name, "!=", right)
}

func (field FieldExpr) GtCol(right string) Condition {
	return buildColumnCondition(field.Name, ">", right)
}

func (field FieldExpr) GteCol(right string) Condition {
	return buildColumnCondition(field.Name, ">=", right)
}

func (field FieldExpr) LtCol(right string) Condition {
	return buildColumnCondition(field.Name, "<", right)
}

func (field FieldExpr) LteCol(right string) Condition {
	return buildColumnCondition(field.Name, "<=", right)
}

type RawExpr struct {
	SQL  string
	Args []any
}

func Raw(sql string, args ...any) RawExpr {
	return RawExpr{SQL: sql, Args: args}
}

type IncrementExpr struct {
	Value any
}

func Increment(value any) IncrementExpr {
	return IncrementExpr{Value: value}
}

type DecrementExpr struct {
	Value any
}

func Decrement(value any) DecrementExpr {
	return DecrementExpr{Value: value}
}

func selectExprs(items []any) ([]SelectExpr, error) {
	exprs := make([]SelectExpr, 0, len(items))
	for _, item := range items {
		switch typedItem := item.(type) {
		case string:
			exprs = append(exprs, SelectExpr{Expr: typedItem})
		case FieldExpr:
			exprs = append(exprs, SelectExpr{Expr: typedItem.Name, Alias: typedItem.Alias})
		case RawExpr:
			exprs = append(exprs, SelectExpr{Expr: typedItem.SQL, Raw: true, Args: typedItem.Args})
		case AggregateExpr:
			exprs = append(exprs, SelectExpr{Expr: aggregateSQL(typedItem.Func, typedItem.Field), Alias: typedItem.Alias, Raw: true})
		case RelationAggregateExpr:
			exprs = append(exprs, SelectExpr{Expr: "__oro_relation_aggregate__", Alias: typedItem.Alias, Raw: true, Args: []any{typedItem}})
		case FullTextExpr:
			exprs = append(exprs, SelectExpr{Expr: "__oro_fulltext_score__", Alias: typedItem.Alias, Raw: true, Args: []any{typedItem}})
		case QuerySource:
			source := typedItem.sourceAST()
			exprs = append(exprs, SelectExpr{Alias: source.Alias, Source: &source})
		default:
			return nil, &Error{Op: "select", Kind: ErrInvalidArgument}
		}
	}
	return exprs, nil
}

func orderExprs(desc bool, fields []string) []OrderExpr {
	exprs := make([]OrderExpr, 0, len(fields))
	for _, field := range fields {
		exprs = append(exprs, OrderExpr{Expr: field, Desc: desc})
	}
	return exprs
}

type JSONField struct {
	Field string
}

type JSONPath struct {
	Field string
	Parts []string
}

type JSONCondition struct {
	Field string
	Parts []string
	Op    string
	Value any
}

func JSON(field string) JSONField {
	return JSONField{Field: field}
}

func (field JSONField) Path(parts ...string) JSONPath {
	return JSONPath{Field: field.Field, Parts: append([]string(nil), parts...)}
}

func (path JSONPath) Eq(value any) Condition {
	return jsonCondition(path, "=", value)
}

func (path JSONPath) NotEq(value any) Condition {
	return jsonCondition(path, "!=", value)
}

func (path JSONPath) IsNull() Condition {
	return jsonCondition(path, "is null", nil)
}

func (path JSONPath) IsNotNull() Condition {
	return jsonCondition(path, "is not null", nil)
}

func (path JSONPath) Exists() Condition {
	return jsonCondition(path, "exists", nil)
}

func (path JSONPath) Contains(value any) Condition {
	return jsonCondition(path, "contains", value)
}

func jsonCondition(path JSONPath, op string, value any) Condition {
	return Condition{
		Field: path.Field,
		Op:    "json",
		Value: JSONCondition{
			Field: path.Field,
			Parts: append([]string(nil), path.Parts...),
			Op:    op,
			Value: value,
		},
	}
}

type FullTextExpr struct {
	Fields  []string
	Query   string
	Alias   string
	IsScore bool
}

func FullText(fields ...string) FullTextExpr {
	return FullTextExpr{Fields: append([]string(nil), fields...)}
}

func (expr FullTextExpr) Match(query string) Condition {
	return Condition{
		Op: "fulltext",
		Value: FullTextExpr{
			Fields: append([]string(nil), expr.Fields...),
			Query:  query,
		},
	}
}

func (expr FullTextExpr) Score(query string) FullTextExpr {
	expr.Query = query
	expr.IsScore = true
	return expr
}

func (expr FullTextExpr) As(alias string) FullTextExpr {
	expr.Alias = alias
	return expr
}
