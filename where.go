package oro

func buildCondition(field any, args ...any) Condition {
	conditions, err := buildConditions(field, args...)
	if err != nil {
		return invalidCondition("where", field)
	}
	if len(conditions) == 1 {
		return conditions[0]
	}
	return And(conditions...)
}

func buildConditions(field any, args ...any) ([]Condition, error) {
	if condition, ok := field.(Condition); ok && len(args) == 0 {
		return []Condition{condition}, nil
	}
	if condition, ok := field.(Condition); ok {
		conditions := []Condition{condition}
		for _, arg := range args {
			nextCondition, ok := arg.(Condition)
			if !ok {
				return nil, &Error{Op: "where", Kind: ErrInvalidArgument}
			}
			conditions = append(conditions, nextCondition)
		}
		return conditions, nil
	}
	fieldName, ok := field.(string)
	if !ok || fieldName == "" {
		return nil, &Error{Op: "where", Kind: ErrInvalidArgument}
	}
	if len(args) == 1 {
		return []Condition{{Field: fieldName, Op: "=", Value: args[0]}}, nil
	}
	if len(args) == 2 {
		op, ok := args[0].(string)
		if !ok || op == "" {
			return nil, &Error{Op: "where", Kind: ErrInvalidArgument, Field: fieldName}
		}
		return []Condition{{Field: fieldName, Op: op, Value: args[1]}}, nil
	}
	return nil, &Error{Op: "where", Kind: ErrInvalidArgument, Field: fieldName}
}

func buildSingleCondition(field any, args ...any) (Condition, bool, error) {
	if condition, ok := field.(Condition); ok && len(args) == 0 {
		return condition, true, nil
	}
	if _, ok := field.(Condition); ok {
		return Condition{}, false, nil
	}
	fieldName, ok := field.(string)
	if !ok || fieldName == "" {
		return Condition{}, false, &Error{Op: "where", Kind: ErrInvalidArgument}
	}
	if len(args) == 1 {
		return Condition{Field: fieldName, Op: "=", Value: args[0]}, true, nil
	}
	if len(args) == 2 {
		op, ok := args[0].(string)
		if !ok || op == "" {
			return Condition{}, false, &Error{Op: "where", Kind: ErrInvalidArgument, Field: fieldName}
		}
		return Condition{Field: fieldName, Op: op, Value: args[1]}, true, nil
	}
	return Condition{}, false, nil
}

func appendWhereCondition(conditions []Condition, boolOp string, field any, args ...any) ([]Condition, error) {
	if condition, ok := buildConditionFromRawOrCondition(field, args...); ok {
		return append(conditions, withBool(boolOp, condition)), nil
	}
	if condition, ok, err := buildSingleCondition(field, args...); ok || err != nil {
		if err != nil {
			return conditions, err
		}
		return append(conditions, withBool(boolOp, condition)), nil
	}
	built, err := buildConditions(field, args...)
	if err != nil {
		return conditions, err
	}
	return append(conditions, conditionsWithBool(boolOp, built)...), nil
}

func invalidCondition(op string, field any) Condition {
	fieldName, _ := field.(string)
	return Condition{Op: "invalid", Value: &Error{Op: op, Kind: ErrInvalidArgument, Field: fieldName}}
}

func conditionsWithBool(boolOp string, conditions []Condition) []Condition {
	if boolOp == "" {
		boolOp = "and"
	}
	next := make([]Condition, 0, len(conditions))
	for _, condition := range conditions {
		if condition.Bool == "" {
			condition.Bool = boolOp
		}
		next = append(next, condition)
	}
	return next
}

func withBool(boolOp string, condition Condition) Condition {
	if condition.Bool == "" {
		condition.Bool = boolOp
	}
	return condition
}

func And(conditions ...Condition) Condition {
	return Condition{Op: "group", Conditions: conditionsWithBool("and", conditions)}
}

func Or(conditions ...Condition) Condition {
	return Condition{Op: "group", Conditions: conditionsWithBool("or", conditions)}
}

func Not(condition Condition) Condition {
	return Condition{Op: "not", Conditions: []Condition{condition}}
}

func Exists(query QuerySource) Condition {
	return buildExistsCondition(query)
}

func RawCondition(sql string, args ...any) Condition {
	return Condition{Field: sql, Op: "raw", Value: args}
}

func conditionRaw(raw RawExpr) Condition {
	return RawCondition(raw.SQL, raw.Args...)
}

func buildConditionFromRawOrCondition(field any, args ...any) (Condition, bool) {
	if raw, ok := field.(RawExpr); ok && len(args) == 0 {
		return conditionRaw(raw), true
	}
	if condition, ok := field.(Condition); ok && len(args) == 0 {
		return condition, true
	}
	return Condition{}, false
}

func buildColumnCondition(left string, args ...string) Condition {
	condition := Condition{
		Field: left,
		Op:    "column",
		Value: ColumnCondition{
			Op: "=",
		},
	}
	columnCondition := condition.Value.(ColumnCondition)
	if len(args) == 1 {
		columnCondition.Right = args[0]
	}
	if len(args) >= 2 {
		columnCondition.Op = args[0]
		columnCondition.Right = args[1]
	}
	condition.Value = columnCondition
	return condition
}

func buildInCondition(field string, source QuerySource) Condition {
	sourceAST := source.sourceAST()
	return Condition{Field: field, Op: "in", Value: &sourceAST}
}

func buildExistsCondition(query QuerySource) Condition {
	source := query.sourceAST()
	return Condition{Op: "exists", Value: &source}
}

func isNullCondition(field string) Condition {
	return Condition{Field: field, Op: "is null"}
}

func isNotNullCondition(field string) Condition {
	return Condition{Field: field, Op: "is not null"}
}

type WhereBuilder struct {
	conditions []Condition
	err        error
}

func (builder *WhereBuilder) Where(field any, args ...any) *WhereBuilder {
	conditions, err := appendWhereCondition(builder.conditions, "and", field, args...)
	if err != nil {
		builder.err = err
		return builder
	}
	builder.conditions = conditions
	return builder
}

func (builder *WhereBuilder) OrWhere(field any, args ...any) *WhereBuilder {
	conditions, err := appendWhereCondition(builder.conditions, "or", field, args...)
	if err != nil {
		builder.err = err
		return builder
	}
	builder.conditions = conditions
	return builder
}

func (builder *WhereBuilder) WhereGroup(fn func(w *WhereBuilder)) *WhereBuilder {
	return builder.whereGroup("and", fn)
}

func (builder *WhereBuilder) OrWhereGroup(fn func(w *WhereBuilder)) *WhereBuilder {
	return builder.whereGroup("or", fn)
}

func (builder *WhereBuilder) WhereWhen(condition bool, fn func(w *WhereBuilder)) *WhereBuilder {
	if !condition {
		return builder
	}
	return builder.WhereGroup(fn)
}

func (builder *WhereBuilder) WhereColumn(left string, args ...string) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("and", buildColumnCondition(left, args...)))
	return builder
}

func (builder *WhereBuilder) OrWhereColumn(left string, args ...string) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("or", buildColumnCondition(left, args...)))
	return builder
}

func (builder *WhereBuilder) WhereExists(source QuerySource) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("and", buildExistsCondition(source)))
	return builder
}

func (builder *WhereBuilder) OrWhereExists(source QuerySource) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("or", buildExistsCondition(source)))
	return builder
}

func (builder *WhereBuilder) WhereIn(field string, source QuerySource) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("and", buildInCondition(field, source)))
	return builder
}

func (builder *WhereBuilder) OrWhereIn(field string, source QuerySource) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("or", buildInCondition(field, source)))
	return builder
}

func (builder *WhereBuilder) WhereRaw(sql string, args ...any) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("and", RawCondition(sql, args...)))
	return builder
}

func (builder *WhereBuilder) OrWhereRaw(sql string, args ...any) *WhereBuilder {
	builder.conditions = append(builder.conditions, withBool("or", RawCondition(sql, args...)))
	return builder
}

func (builder *WhereBuilder) whereGroup(boolOp string, fn func(w *WhereBuilder)) *WhereBuilder {
	if fn == nil {
		return builder
	}
	group := &WhereBuilder{}
	fn(group)
	if group.err != nil {
		builder.err = group.err
		return builder
	}
	if len(group.conditions) == 0 {
		return builder
	}
	builder.conditions = append(builder.conditions, Condition{
		Bool:       boolOp,
		Op:         "group",
		Conditions: group.conditions,
	})
	return builder
}

func buildWhereGroup(op string, fn func(w *WhereBuilder)) Condition {
	builder := &WhereBuilder{}
	if fn != nil {
		fn(builder)
	}
	if builder.err != nil {
		return Condition{Op: "invalid", Value: builder.err}
	}
	if len(builder.conditions) == 0 {
		return Condition{Op: "empty_group"}
	}
	return Condition{Bool: op, Op: "group", Conditions: builder.conditions}
}
