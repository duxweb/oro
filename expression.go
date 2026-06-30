package oro

import "strings"

// FieldExpr describes a structured field condition target.
type FieldExpr struct {
	Name  string
	Alias string
}

// Field creates a field expression. Model queries resolve name as a Go field;
// table queries resolve name as a database column.
func Field(name string) FieldExpr {
	return FieldExpr{Name: name}
}

// Column is a semantic alias for Field when the call site wants column wording.
func Column(name string) FieldExpr {
	return FieldExpr{Name: name}
}

// Eq returns a field = value condition.
func (field FieldExpr) Eq(value any) Condition {
	return Condition{Field: field.Name, Op: "=", Value: value}
}

// NotEq returns a field != value condition.
func (field FieldExpr) NotEq(value any) Condition {
	return Condition{Field: field.Name, Op: "!=", Value: value}
}

// Gt returns a field > value condition.
func (field FieldExpr) Gt(value any) Condition {
	return Condition{Field: field.Name, Op: ">", Value: value}
}

// Gte returns a field >= value condition.
func (field FieldExpr) Gte(value any) Condition {
	return Condition{Field: field.Name, Op: ">=", Value: value}
}

// Lt returns a field < value condition.
func (field FieldExpr) Lt(value any) Condition {
	return Condition{Field: field.Name, Op: "<", Value: value}
}

// Lte returns a field <= value condition.
func (field FieldExpr) Lte(value any) Condition {
	return Condition{Field: field.Name, Op: "<=", Value: value}
}

// Like returns a LIKE condition without escaping wildcard characters.
func (field FieldExpr) Like(value any) Condition {
	return Condition{Field: field.Name, Op: "like", Value: value}
}

// NotLike returns a NOT LIKE condition without escaping wildcard characters.
func (field FieldExpr) NotLike(value any) Condition {
	return Condition{Field: field.Name, Op: "not like", Value: value}
}

// Contains returns an escaped literal substring LIKE condition.
func (field FieldExpr) Contains(value string) Condition {
	return Condition{Field: field.Name, Op: "like", Value: "%" + EscapeLike(value) + "%", Escape: `\`}
}

// StartsWith returns an escaped literal prefix LIKE condition.
func (field FieldExpr) StartsWith(value string) Condition {
	return Condition{Field: field.Name, Op: "like", Value: EscapeLike(value) + "%", Escape: `\`}
}

// EndsWith returns an escaped literal suffix LIKE condition.
func (field FieldExpr) EndsWith(value string) Condition {
	return Condition{Field: field.Name, Op: "like", Value: "%" + EscapeLike(value), Escape: `\`}
}

// In returns an IN condition for a list of values.
func (field FieldExpr) In(values ...any) Condition {
	return Condition{Field: field.Name, Op: "in_values", Value: append([]any(nil), values...)}
}

// NotIn returns a NOT IN condition for a list of values.
func (field FieldExpr) NotIn(values ...any) Condition {
	return Condition{Field: field.Name, Op: "not_in_values", Value: append([]any(nil), values...)}
}

// Between returns a closed BETWEEN condition.
func (field FieldExpr) Between(start any, end any) Condition {
	return Condition{Field: field.Name, Op: "between", Value: []any{start, end}}
}

// NotBetween returns the negation of a closed BETWEEN condition.
func (field FieldExpr) NotBetween(start any, end any) Condition {
	return Not(field.Between(start, end))
}

// IsNull returns an IS NULL condition.
func (field FieldExpr) IsNull() Condition {
	return isNullCondition(field.Name)
}

// IsNotNull returns an IS NOT NULL condition.
func (field FieldExpr) IsNotNull() Condition {
	return isNotNullCondition(field.Name)
}

// EqCol returns a field = right-column condition.
func (field FieldExpr) EqCol(right string) Condition {
	return buildColumnCondition(field.Name, "=", right)
}

// NotEqCol returns a field != right-column condition.
func (field FieldExpr) NotEqCol(right string) Condition {
	return buildColumnCondition(field.Name, "!=", right)
}

// GtCol returns a field > right-column condition.
func (field FieldExpr) GtCol(right string) Condition {
	return buildColumnCondition(field.Name, ">", right)
}

// GteCol returns a field >= right-column condition.
func (field FieldExpr) GteCol(right string) Condition {
	return buildColumnCondition(field.Name, ">=", right)
}

// LtCol returns a field < right-column condition.
func (field FieldExpr) LtCol(right string) Condition {
	return buildColumnCondition(field.Name, "<", right)
}

// LteCol returns a field <= right-column condition.
func (field FieldExpr) LteCol(right string) Condition {
	return buildColumnCondition(field.Name, "<=", right)
}

// RawExpr is a raw SQL expression with bound arguments.
type RawExpr struct {
	SQL  string
	Args []any
}

// Raw creates a raw SQL expression. When used as db.Raw it starts a raw query;
// when used in Select or Where it acts as a structured raw expression.
func Raw(sql string, args ...any) RawExpr {
	return RawExpr{SQL: sql, Args: args}
}

// EscapeLike escapes \, %, and _ for literal LIKE matching.
func EscapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

// IncrementExpr represents an arithmetic increment write expression.
type IncrementExpr struct {
	Value any
}

// Increment creates an increment write expression.
func Increment(value any) IncrementExpr {
	return IncrementExpr{Value: value}
}

// DecrementExpr represents an arithmetic decrement write expression.
type DecrementExpr struct {
	Value any
}

// Decrement creates a decrement write expression.
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
			exprs = append(exprs, SelectExpr{Expr: "__oro_aggregate__", Alias: typedItem.Alias, Args: []any{typedItem}})
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

// JSONField describes a JSON column for path conditions.
type JSONField struct {
	Field string
}

// JSONPath describes a JSON column path condition target.
type JSONPath struct {
	Field string
	Parts []string
}

// JSONCondition is the structured payload for JSON path comparisons.
type JSONCondition struct {
	Field string
	Parts []string
	Op    string
	Value any
}

// JSON creates a JSON field expression for path-based conditions.
func JSON(field string) JSONField {
	return JSONField{Field: field}
}

// Path selects a nested JSON path under the JSON field.
func (field JSONField) Path(parts ...string) JSONPath {
	return JSONPath{Field: field.Field, Parts: append([]string(nil), parts...)}
}

// Eq returns a JSON path equality condition.
func (path JSONPath) Eq(value any) Condition {
	return jsonCondition(path, "=", value)
}

// NotEq returns a JSON path inequality condition.
func (path JSONPath) NotEq(value any) Condition {
	return jsonCondition(path, "!=", value)
}

// IsNull returns a JSON path IS NULL condition.
func (path JSONPath) IsNull() Condition {
	return jsonCondition(path, "is null", nil)
}

// IsNotNull returns a JSON path IS NOT NULL condition.
func (path JSONPath) IsNotNull() Condition {
	return jsonCondition(path, "is not null", nil)
}

// Exists returns a JSON path existence condition.
func (path JSONPath) Exists() Condition {
	return jsonCondition(path, "exists", nil)
}

// Contains returns a JSON path containment condition.
func (path JSONPath) Contains(value any) Condition {
	return jsonCondition(path, "contains", value)
}

// Like returns a JSON path LIKE condition.
func (path JSONPath) Like(value any) Condition {
	return jsonCondition(path, "like", value)
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

// FullTextExpr describes a full-text match condition over one or more fields.
type FullTextExpr struct {
	Fields  []string
	Query   string
	Alias   string
	IsScore bool
}

// FullText creates a full-text expression over fields.
func FullText(fields ...string) FullTextExpr {
	return FullTextExpr{Fields: append([]string(nil), fields...)}
}

// Match returns a full-text match condition.
func (expr FullTextExpr) Match(query string) Condition {
	return Condition{
		Op: "fulltext",
		Value: FullTextExpr{
			Fields: append([]string(nil), expr.Fields...),
			Query:  query,
		},
	}
}

// Score returns a full-text score expression for SELECT lists.
func (expr FullTextExpr) Score(query string) FullTextExpr {
	expr.Query = query
	expr.IsScore = true
	return expr
}

// As aliases a full-text score expression.
func (expr FullTextExpr) As(alias string) FullTextExpr {
	expr.Alias = alias
	return expr
}
