package oro

import "strings"

type RelationQuery struct {
	db             *DB
	schema         *ModelSchema
	count          *CountCondition
	spec           QuerySpec
	shard          Map
	allShards      bool
	softDeleteMode softDeleteMode
}

func (query *RelationQuery) Where(field any, args ...any) *RelationQuery {
	conditions, err := appendWhereCondition(query.spec.Where, "and", field, args...)
	if err != nil {
		query.spec.SelectErr = err
		return query
	}
	query.spec.Where = conditions
	return query
}

func (query *RelationQuery) OrWhere(field any, args ...any) *RelationQuery {
	conditions, err := appendWhereCondition(query.spec.Where, "or", field, args...)
	if err != nil {
		query.spec.SelectErr = err
		return query
	}
	query.spec.Where = conditions
	return query
}

func (query *RelationQuery) WhereGroup(fn func(w *WhereBuilder)) *RelationQuery {
	condition := buildWhereGroup("and", fn)
	if condition.Op != "empty_group" {
		query.spec.Where = append(query.spec.Where, condition)
	}
	return query
}

func (query *RelationQuery) OrWhereGroup(fn func(w *WhereBuilder)) *RelationQuery {
	condition := buildWhereGroup("or", fn)
	if condition.Op != "empty_group" {
		query.spec.Where = append(query.spec.Where, condition)
	}
	return query
}

func (query *RelationQuery) WhereWhen(condition bool, fn func(w *WhereBuilder)) *RelationQuery {
	if !condition {
		return query
	}
	return query.WhereGroup(fn)
}

func (query *RelationQuery) WhereRaw(sql string, args ...any) *RelationQuery {
	query.spec.Where = append(query.spec.Where, withBool("and", RawCondition(sql, args...)))
	return query
}

func (query *RelationQuery) WhereColumn(left string, args ...string) *RelationQuery {
	query.spec.Where = append(query.spec.Where, withBool("and", buildColumnCondition(left, args...)))
	return query
}

func (query *RelationQuery) OrderBy(fields ...string) *RelationQuery {
	query.spec.Order = append(query.spec.Order, orderExprs(false, fields)...)
	return query
}

func (query *RelationQuery) OrderByDesc(fields ...string) *RelationQuery {
	query.spec.Order = append(query.spec.Order, orderExprs(true, fields)...)
	return query
}

func (query *RelationQuery) Limit(limit int) *RelationQuery {
	query.spec.Limit = &limit
	return query
}

func (query *RelationQuery) With(relation any, callbacks ...func(*RelationQuery)) *RelationQuery {
	with, err := buildWithSpec(relation, callbacks)
	if err != nil {
		query.spec.SelectErr = err
		return query
	}
	query.spec.With = append(query.spec.With, with)
	return query
}

func (query *RelationQuery) Shard(values Map) *RelationQuery {
	query.shard = copyMap(values)
	query.allShards = false
	return query
}

func (query *RelationQuery) AllShards() *RelationQuery {
	query.shard = nil
	query.allShards = true
	return query
}

func (query *RelationQuery) WithDeleted() *RelationQuery {
	query.softDeleteMode = softDeleteWith
	return query
}

func (query *RelationQuery) OnlyDeleted() *RelationQuery {
	query.softDeleteMode = softDeleteOnly
	return query
}

func (query *RelationQuery) Count(op string, value int64) *RelationQuery {
	if !IsSafeColumnOperator(op) {
		query.spec.SelectErr = &Error{Op: "where_has.count", Kind: ErrInvalidArgument}
		return query
	}
	query.count = &CountCondition{Op: NormalizeConditionOperator(op), Value: value}
	return query
}

func buildWithSpec(relation any, callbacks []func(*RelationQuery)) (WithSpec, error) {
	with := WithSpec{}
	switch typedRelation := relation.(type) {
	case Relation:
		schema := typedRelation.relationSchema()
		with.Name = schema.Name
		with.Relation = &schema
	case string:
		if typedRelation == "" {
			return WithSpec{}, &Error{Op: "with", Kind: ErrInvalidArgument}
		}
		parts := strings.Split(typedRelation, ".")
		with.Name = parts[0]
		if len(parts) > 1 {
			nextPath := strings.Join(parts[1:], ".")
			with.Callback = func(q *RelationQuery) {
				q.With(nextPath)
			}
		}
	default:
		return WithSpec{}, &Error{Op: "with", Kind: ErrInvalidArgument}
	}
	if len(callbacks) > 0 {
		previous := with.Callback
		callback := callbacks[0]
		with.Callback = func(q *RelationQuery) {
			if previous != nil {
				previous(q)
			}
			if callback != nil {
				callback(q)
			}
		}
	}
	return with, nil
}
