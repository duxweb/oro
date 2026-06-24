package oro

import "time"

type Scope[T any] func(q *ScopeQuery[T])

type TableScope func(q *TableScopeQuery)

type ScopeQuery[T any] struct {
	query *ModelQuery[T]
}

type TableScopeQuery struct {
	query *TableQuery
}

func (query *ModelQuery[T]) Scope(scopes ...Scope[T]) *ModelQuery[T] {
	next := query
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		builder := &ScopeQuery[T]{query: next}
		scope(builder)
		if builder.query != nil {
			next = builder.query
		}
	}
	return next
}

func (query *ModelQuery[T]) ScopeWhen(condition bool, scopes ...Scope[T]) *ModelQuery[T] {
	if !condition {
		return query
	}
	return query.Scope(scopes...)
}

func (query *TableQuery) Scope(scopes ...TableScope) *TableQuery {
	next := query
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		builder := &TableScopeQuery{query: next}
		scope(builder)
		if builder.query != nil {
			next = builder.query
		}
	}
	return next
}

func (query *TableQuery) ScopeWhen(condition bool, scopes ...TableScope) *TableQuery {
	if !condition {
		return query
	}
	return query.Scope(scopes...)
}

func (query *ScopeQuery[T]) Where(field any, args ...any) *ScopeQuery[T] {
	query.query = query.query.Where(field, args...)
	return query
}

func (query *ScopeQuery[T]) OrWhere(field any, args ...any) *ScopeQuery[T] {
	query.query = query.query.OrWhere(field, args...)
	return query
}

func (query *ScopeQuery[T]) WhereGroup(fn func(w *WhereBuilder)) *ScopeQuery[T] {
	query.query = query.query.WhereGroup(fn)
	return query
}

func (query *ScopeQuery[T]) OrWhereGroup(fn func(w *WhereBuilder)) *ScopeQuery[T] {
	query.query = query.query.OrWhereGroup(fn)
	return query
}

func (query *ScopeQuery[T]) WhereWhen(condition bool, fn func(w *WhereBuilder)) *ScopeQuery[T] {
	query.query = query.query.WhereWhen(condition, fn)
	return query
}

func (query *ScopeQuery[T]) WhereRaw(sql string, args ...any) *ScopeQuery[T] {
	query.query = query.query.WhereRaw(sql, args...)
	return query
}

func (query *ScopeQuery[T]) WhereColumn(left string, args ...string) *ScopeQuery[T] {
	query.query = query.query.WhereColumn(left, args...)
	return query
}

func (query *ScopeQuery[T]) WhereIn(field string, source QuerySource) *ScopeQuery[T] {
	query.query = query.query.WhereIn(field, source)
	return query
}

func (query *ScopeQuery[T]) WhereExists(source QuerySource) *ScopeQuery[T] {
	query.query = query.query.WhereExists(source)
	return query
}

func (query *ScopeQuery[T]) Select(items ...any) *ScopeQuery[T] {
	query.query = query.query.Select(items...)
	return query
}

func (query *ScopeQuery[T]) With(relation any, callbacks ...func(*RelationQuery)) *ScopeQuery[T] {
	query.query = query.query.With(relation, callbacks...)
	return query
}

func (query *ScopeQuery[T]) For(relation Relation) *ScopeQuery[T] {
	query.query = query.query.For(relation)
	return query
}

func (query *ScopeQuery[T]) As(alias string) *ScopeQuery[T] {
	query.query = query.query.As(alias)
	return query
}

func (query *ScopeQuery[T]) SelectHidden(fields ...string) *ScopeQuery[T] {
	query.query = query.query.SelectHidden(fields...)
	return query
}

func (query *ScopeQuery[T]) SkipHooks() *ScopeQuery[T] {
	query.query = query.query.SkipHooks()
	return query
}

func (query *ScopeQuery[T]) SkipEvents() *ScopeQuery[T] {
	query.query = query.query.SkipEvents()
	return query
}

func (query *ScopeQuery[T]) UsePrimary() *ScopeQuery[T] {
	query.query = query.query.UsePrimary()
	return query
}

func (query *ScopeQuery[T]) Cache(ttl time.Duration) *ScopeQuery[T] {
	query.query = query.query.Cache(ttl)
	return query
}

func (query *ScopeQuery[T]) CacheKey(key string) *ScopeQuery[T] {
	query.query = query.query.CacheKey(key)
	return query
}

func (query *ScopeQuery[T]) CacheTags(tags ...string) *ScopeQuery[T] {
	query.query = query.query.CacheTags(tags...)
	return query
}

func (query *ScopeQuery[T]) Timeout(timeout time.Duration) *ScopeQuery[T] {
	query.query = query.query.Timeout(timeout)
	return query
}

func (query *ScopeQuery[T]) Join(source any, fn func(j *Join)) *ScopeQuery[T] {
	query.query = query.query.Join(source, fn)
	return query
}

func (query *ScopeQuery[T]) LeftJoin(source any, fn func(j *Join)) *ScopeQuery[T] {
	query.query = query.query.LeftJoin(source, fn)
	return query
}

func (query *ScopeQuery[T]) RightJoin(source any, fn func(j *Join)) *ScopeQuery[T] {
	query.query = query.query.RightJoin(source, fn)
	return query
}

func (query *ScopeQuery[T]) FullJoin(source any, fn func(j *Join)) *ScopeQuery[T] {
	query.query = query.query.FullJoin(source, fn)
	return query
}

func (query *ScopeQuery[T]) CrossJoin(table string) *ScopeQuery[T] {
	query.query = query.query.CrossJoin(table)
	return query
}

func (query *ScopeQuery[T]) JoinRaw(sql string, args ...any) *ScopeQuery[T] {
	query.query = query.query.JoinRaw(sql, args...)
	return query
}

func (query *ScopeQuery[T]) OrderBy(fields ...string) *ScopeQuery[T] {
	query.query = query.query.OrderBy(fields...)
	return query
}

func (query *ScopeQuery[T]) OrderByDesc(fields ...string) *ScopeQuery[T] {
	query.query = query.query.OrderByDesc(fields...)
	return query
}

func (query *ScopeQuery[T]) OrderByRaw(sql string, args ...any) *ScopeQuery[T] {
	query.query = query.query.OrderByRaw(sql, args...)
	return query
}

func (query *ScopeQuery[T]) GroupBy(fields ...string) *ScopeQuery[T] {
	query.query = query.query.GroupBy(fields...)
	return query
}

func (query *ScopeQuery[T]) Having(field string, args ...any) *ScopeQuery[T] {
	query.query = query.query.Having(field, args...)
	return query
}

func (query *ScopeQuery[T]) HavingColumn(left string, args ...string) *ScopeQuery[T] {
	query.query = query.query.HavingColumn(left, args...)
	return query
}

func (query *ScopeQuery[T]) HavingIn(field string, source QuerySource) *ScopeQuery[T] {
	query.query = query.query.HavingIn(field, source)
	return query
}

func (query *ScopeQuery[T]) HavingExists(source QuerySource) *ScopeQuery[T] {
	query.query = query.query.HavingExists(source)
	return query
}

func (query *ScopeQuery[T]) HavingRaw(sql string, args ...any) *ScopeQuery[T] {
	query.query = query.query.HavingRaw(sql, args...)
	return query
}

func (query *ScopeQuery[T]) Limit(limit int) *ScopeQuery[T] {
	query.query = query.query.Limit(limit)
	return query
}

func (query *ScopeQuery[T]) Offset(offset int) *ScopeQuery[T] {
	query.query = query.query.Offset(offset)
	return query
}

func (query *ScopeQuery[T]) LockForUpdate(options ...LockOption) *ScopeQuery[T] {
	query.query = query.query.LockForUpdate(options...)
	return query
}

func (query *ScopeQuery[T]) LockForShare(options ...LockOption) *ScopeQuery[T] {
	query.query = query.query.LockForShare(options...)
	return query
}

func (query *ScopeQuery[T]) WithDeleted() *ScopeQuery[T] {
	query.query = query.query.WithDeleted()
	return query
}

func (query *ScopeQuery[T]) OnlyDeleted() *ScopeQuery[T] {
	query.query = query.query.OnlyDeleted()
	return query
}

func (query *ScopeQuery[T]) Shard(values Map) *ScopeQuery[T] {
	query.query = query.query.Shard(values)
	return query
}

func (query *ScopeQuery[T]) AllShards() *ScopeQuery[T] {
	query.query = query.query.AllShards()
	return query
}

func (query *TableScopeQuery) Where(field any, args ...any) *TableScopeQuery {
	query.query = query.query.Where(field, args...)
	return query
}

func (query *TableScopeQuery) OrWhere(field any, args ...any) *TableScopeQuery {
	query.query = query.query.OrWhere(field, args...)
	return query
}

func (query *TableScopeQuery) WhereGroup(fn func(w *WhereBuilder)) *TableScopeQuery {
	query.query = query.query.WhereGroup(fn)
	return query
}

func (query *TableScopeQuery) OrWhereGroup(fn func(w *WhereBuilder)) *TableScopeQuery {
	query.query = query.query.OrWhereGroup(fn)
	return query
}

func (query *TableScopeQuery) WhereWhen(condition bool, fn func(w *WhereBuilder)) *TableScopeQuery {
	query.query = query.query.WhereWhen(condition, fn)
	return query
}

func (query *TableScopeQuery) WhereRaw(sql string, args ...any) *TableScopeQuery {
	query.query = query.query.WhereRaw(sql, args...)
	return query
}

func (query *TableScopeQuery) WhereColumn(left string, args ...string) *TableScopeQuery {
	query.query = query.query.WhereColumn(left, args...)
	return query
}

func (query *TableScopeQuery) WhereIn(field string, source QuerySource) *TableScopeQuery {
	query.query = query.query.WhereIn(field, source)
	return query
}

func (query *TableScopeQuery) WhereExists(source QuerySource) *TableScopeQuery {
	query.query = query.query.WhereExists(source)
	return query
}

func (query *TableScopeQuery) Select(items ...any) *TableScopeQuery {
	query.query = query.query.Select(items...)
	return query
}

func (query *TableScopeQuery) As(alias string) *TableScopeQuery {
	query.query = query.query.As(alias)
	return query
}

func (query *TableScopeQuery) UsePrimary() *TableScopeQuery {
	query.query = query.query.UsePrimary()
	return query
}

func (query *TableScopeQuery) Cache(ttl time.Duration) *TableScopeQuery {
	query.query = query.query.Cache(ttl)
	return query
}

func (query *TableScopeQuery) CacheKey(key string) *TableScopeQuery {
	query.query = query.query.CacheKey(key)
	return query
}

func (query *TableScopeQuery) CacheTags(tags ...string) *TableScopeQuery {
	query.query = query.query.CacheTags(tags...)
	return query
}

func (query *TableScopeQuery) Timeout(timeout time.Duration) *TableScopeQuery {
	query.query = query.query.Timeout(timeout)
	return query
}

func (query *TableScopeQuery) Join(source any, fn func(j *Join)) *TableScopeQuery {
	query.query = query.query.Join(source, fn)
	return query
}

func (query *TableScopeQuery) LeftJoin(source any, fn func(j *Join)) *TableScopeQuery {
	query.query = query.query.LeftJoin(source, fn)
	return query
}

func (query *TableScopeQuery) RightJoin(source any, fn func(j *Join)) *TableScopeQuery {
	query.query = query.query.RightJoin(source, fn)
	return query
}

func (query *TableScopeQuery) FullJoin(source any, fn func(j *Join)) *TableScopeQuery {
	query.query = query.query.FullJoin(source, fn)
	return query
}

func (query *TableScopeQuery) CrossJoin(table string) *TableScopeQuery {
	query.query = query.query.CrossJoin(table)
	return query
}

func (query *TableScopeQuery) JoinRaw(sql string, args ...any) *TableScopeQuery {
	query.query = query.query.JoinRaw(sql, args...)
	return query
}

func (query *TableScopeQuery) OrderBy(fields ...string) *TableScopeQuery {
	query.query = query.query.OrderBy(fields...)
	return query
}

func (query *TableScopeQuery) OrderByDesc(fields ...string) *TableScopeQuery {
	query.query = query.query.OrderByDesc(fields...)
	return query
}

func (query *TableScopeQuery) OrderByRaw(sql string, args ...any) *TableScopeQuery {
	query.query = query.query.OrderByRaw(sql, args...)
	return query
}

func (query *TableScopeQuery) GroupBy(fields ...string) *TableScopeQuery {
	query.query = query.query.GroupBy(fields...)
	return query
}

func (query *TableScopeQuery) Having(field string, args ...any) *TableScopeQuery {
	query.query = query.query.Having(field, args...)
	return query
}

func (query *TableScopeQuery) HavingColumn(left string, args ...string) *TableScopeQuery {
	query.query = query.query.HavingColumn(left, args...)
	return query
}

func (query *TableScopeQuery) HavingIn(field string, source QuerySource) *TableScopeQuery {
	query.query = query.query.HavingIn(field, source)
	return query
}

func (query *TableScopeQuery) HavingExists(source QuerySource) *TableScopeQuery {
	query.query = query.query.HavingExists(source)
	return query
}

func (query *TableScopeQuery) HavingRaw(sql string, args ...any) *TableScopeQuery {
	query.query = query.query.HavingRaw(sql, args...)
	return query
}

func (query *TableScopeQuery) Limit(limit int) *TableScopeQuery {
	query.query = query.query.Limit(limit)
	return query
}

func (query *TableScopeQuery) Offset(offset int) *TableScopeQuery {
	query.query = query.query.Offset(offset)
	return query
}

func (query *TableScopeQuery) LockForUpdate(options ...LockOption) *TableScopeQuery {
	query.query = query.query.LockForUpdate(options...)
	return query
}

func (query *TableScopeQuery) LockForShare(options ...LockOption) *TableScopeQuery {
	query.query = query.query.LockForShare(options...)
	return query
}

func (query *TableScopeQuery) Shard(group string, values Map) *TableScopeQuery {
	query.query = query.query.Shard(group, values)
	return query
}

func (query *TableScopeQuery) AllShards(group string) *TableScopeQuery {
	query.query = query.query.AllShards(group)
	return query
}
