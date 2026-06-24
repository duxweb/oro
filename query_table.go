package oro

import (
	"context"
	"time"
)

type TableQuery struct {
	db        *DB
	spec      QuerySpec
	shard     Map
	allShards bool
}

func (query *TableQuery) Where(field any, args ...any) *TableQuery {
	clone := *query
	conditions, err := appendWhereCondition(clone.spec.Where, "and", field, args...)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Where = conditions
	return &clone
}

func (query *TableQuery) OrWhere(field any, args ...any) *TableQuery {
	clone := *query
	conditions, err := appendWhereCondition(clone.spec.Where, "or", field, args...)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Where = conditions
	return &clone
}

func (query *TableQuery) WhereGroup(fn func(w *WhereBuilder)) *TableQuery {
	clone := *query
	condition := buildWhereGroup("and", fn)
	if condition.Op == "empty_group" {
		return &clone
	}
	clone.spec.Where = append(clone.spec.Where, condition)
	return &clone
}

func (query *TableQuery) OrWhereGroup(fn func(w *WhereBuilder)) *TableQuery {
	clone := *query
	condition := buildWhereGroup("or", fn)
	if condition.Op == "empty_group" {
		return &clone
	}
	clone.spec.Where = append(clone.spec.Where, condition)
	return &clone
}

func (query *TableQuery) WhereWhen(condition bool, fn func(w *WhereBuilder)) *TableQuery {
	if !condition {
		return query
	}
	return query.WhereGroup(fn)
}

func (query *TableQuery) WhereRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", RawCondition(sql, args...)))
	return &clone
}

func (query *TableQuery) WhereColumn(left string, args ...string) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildColumnCondition(left, args...)))
	return &clone
}

func (query *TableQuery) WhereIn(field string, source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildInCondition(field, source)))
	return &clone
}

func (query *TableQuery) WhereExists(source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildExistsCondition(source)))
	return &clone
}

func (query *TableQuery) Select(items ...any) *TableQuery {
	clone := *query
	exprs, err := selectExprs(items)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Select = append(clone.spec.Select, exprs...)
	return &clone
}

func (query *TableQuery) As(alias string) *TableQuery {
	clone := *query
	clone.spec.Alias = alias
	return &clone
}

func (query *TableQuery) UsePrimary() *TableQuery {
	clone := *query
	clone.spec.UsePrimary = true
	return &clone
}

func (query *TableQuery) Cache(ttl time.Duration) *TableQuery {
	clone := *query
	clone.spec.Cache.Enabled = true
	clone.spec.Cache.TTL = int64(ttl)
	if ttl <= 0 {
		clone.spec.SelectErr = &Error{Op: "cache", Kind: ErrInvalidArgument}
	}
	return &clone
}

func (query *TableQuery) CacheKey(key string) *TableQuery {
	clone := *query
	clone.spec.Cache.Key = key
	return &clone
}

func (query *TableQuery) CacheTags(tags ...string) *TableQuery {
	clone := *query
	clone.spec.Cache.Tags = append(clone.spec.Cache.Tags, tags...)
	return &clone
}

func (query *TableQuery) Timeout(timeout time.Duration) *TableQuery {
	clone := *query
	clone.spec.Timeout = int64(timeout)
	return &clone
}

func (query *TableQuery) Join(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinInner, source, fn))
	return &clone
}

func (query *TableQuery) LeftJoin(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinLeft, source, fn))
	return &clone
}

func (query *TableQuery) RightJoin(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinRight, source, fn))
	return &clone
}

func (query *TableQuery) FullJoin(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinFull, source, fn))
	return &clone
}

func (query *TableQuery) CrossJoin(table string) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinCross, table, nil))
	return &clone
}

func (query *TableQuery) JoinRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, JoinAST{
		Raw: &RawSpec{SQL: sql, Args: args},
	})
	return &clone
}

func (query *TableQuery) OrderBy(fields ...string) *TableQuery {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, orderExprs(false, fields)...)
	return &clone
}

func (query *TableQuery) OrderByDesc(fields ...string) *TableQuery {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, orderExprs(true, fields)...)
	return &clone
}

func (query *TableQuery) OrderByRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, OrderExpr{Expr: sql, Raw: true, Args: args})
	return &clone
}

func (query *TableQuery) GroupBy(fields ...string) *TableQuery {
	clone := *query
	clone.spec.Group = append(clone.spec.Group, fields...)
	return &clone
}

func (query *TableQuery) Having(field string, args ...any) *TableQuery {
	clone := *query
	conditions, err := buildConditions(field, args...)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Having = append(clone.spec.Having, conditionsWithBool("and", conditions)...)
	return &clone
}

func (query *TableQuery) HavingColumn(left string, args ...string) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildColumnCondition(left, args...)))
	return &clone
}

func (query *TableQuery) HavingIn(field string, source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildInCondition(field, source)))
	return &clone
}

func (query *TableQuery) HavingExists(source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildExistsCondition(source)))
	return &clone
}

func (query *TableQuery) HavingRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", RawCondition(sql, args...)))
	return &clone
}

func (query *TableQuery) Limit(limit int) *TableQuery {
	clone := *query
	clone.spec.Limit = &limit
	return &clone
}

func (query *TableQuery) Offset(offset int) *TableQuery {
	clone := *query
	clone.spec.Offset = &offset
	return &clone
}

func (query *TableQuery) LockForUpdate(options ...LockOption) *TableQuery {
	clone := *query
	clone.spec.Lock = applyLockOptions(LockUpdate, options)
	return &clone
}

func (query *TableQuery) LockForShare(options ...LockOption) *TableQuery {
	clone := *query
	clone.spec.Lock = applyLockOptions(LockShare, options)
	return &clone
}

func (query *TableQuery) Shard(group string, values Map) *TableQuery {
	clone := *query
	clone.spec.ShardGroup = group
	clone.shard = copyMap(values)
	clone.allShards = false
	return &clone
}

func (query *TableQuery) AllShards(group string) *TableQuery {
	clone := *query
	clone.spec.ShardGroup = group
	clone.shard = nil
	clone.allShards = true
	return &clone
}

func (query *TableQuery) First(ctx context.Context) (Map, error) {
	if query.allShards {
		if len(query.spec.Order) == 0 {
			return nil, &Error{Op: "first", Kind: ErrOrderRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
		}
		rows, err := query.Limit(1).Get(ctx)
		if err != nil || len(rows) == 0 {
			return nil, err
		}
		return rows[0], nil
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return nil, err
	}
	return queryFirstRowPrepared(ctx, query.db, spec)
}

func (query *TableQuery) Get(ctx context.Context) ([]Map, error) {
	if query.allShards {
		return queryAllShardRows(ctx, query)
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return nil, err
	}
	return queryRowsPrepared(ctx, query.db, spec)
}

func (query *TableQuery) Stream(ctx context.Context) (Stream[Map], error) {
	if query.allShards {
		return nil, &Error{Op: "stream", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return nil, err
	}
	rows, err := streamQueryPrepared(ctx, query.db, spec)
	if err != nil {
		return nil, err
	}
	return &mapStream{rows: rows}, nil
}

func (query *TableQuery) Chunk(ctx context.Context, size int, fn func([]Map) error) error {
	if query.allShards {
		return &Error{Op: "chunk", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := chunkSpecError(query.spec); err != nil {
		return err
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return err
	}
	spec, err = applyTableChunkOrder(ctx, query.db, spec)
	if err != nil {
		return err
	}
	spec.Limit = &size
	return chunkMaps(ctx, spec, func(chunkSpec QuerySpec) ([]Map, error) {
		return queryRowsPrepared(ctx, query.db, chunkSpec)
	}, fn)
}

func (query *TableQuery) Each(ctx context.Context, fn func(Map) error) error {
	if fn == nil {
		return &Error{Op: "each", Kind: ErrInvalidArgument}
	}
	return query.Chunk(ctx, eachSize(query.db.runtime.Config), func(rows []Map) error {
		for _, row := range rows {
			if err := fn(row); err != nil {
				return err
			}
		}
		return nil
	})
}

func (query *TableQuery) Paginate(size int) *Paginator[Map] {
	specErr := paginateSpecError(query.spec)
	return &Paginator[Map]{
		size: size,
		err:  specErr,
		count: func(ctx context.Context) (int64, error) {
			return query.Count(ctx)
		},
		items: func(ctx context.Context, limit int, offset int) ([]Map, error) {
			return query.Limit(limit).Offset(offset).Get(ctx)
		},
	}
}

func (query *TableQuery) Count(ctx context.Context) (int64, error) {
	if query.allShards {
		rows, err := queryAllShardCounts(ctx, query)
		if err != nil {
			return 0, err
		}
		var total int64
		for _, row := range rows {
			value, err := rowInt64(row, "total")
			if err != nil {
				return 0, err
			}
			total += value
		}
		return total, nil
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return 0, err
	}
	spec.Select = []SelectExpr{{Expr: "count(*)", Alias: "total", Raw: true}}
	spec.Order = nil
	spec.Limit = nil
	spec.Offset = nil

	row, err := queryFirstRowPrepared(ctx, query.db, spec)
	if err != nil || row == nil {
		return 0, err
	}
	return rowInt64(row, "total")
}

func (query *TableQuery) Exists(ctx context.Context) (bool, error) {
	if query.allShards {
		rows, err := queryAllShardExists(ctx, query)
		if err != nil {
			return false, err
		}
		return len(rows) > 0, nil
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return false, err
	}
	spec.Select = []SelectExpr{{Expr: "1", Raw: true}}
	spec.Order = nil
	limit := 1
	spec.Limit = &limit
	spec.Offset = nil

	row, err := queryFirstRowPrepared(ctx, query.db, spec)
	if err != nil {
		return false, err
	}
	return row != nil, nil
}

func (query *TableQuery) Sum(ctx context.Context, field string) (Decimal, error) {
	if query.allShards {
		return Decimal("0"), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return Decimal("0"), err
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return Decimal("0"), err
	}
	return aggregateDecimal(ctx, query.db, spec, "sum", field)
}

func (query *TableQuery) Avg(ctx context.Context, field string) (Decimal, error) {
	if query.allShards {
		return Decimal("0"), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return Decimal("0"), err
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return Decimal("0"), err
	}
	return aggregateDecimal(ctx, query.db, spec, "avg", field)
}

func (query *TableQuery) Min[T any](ctx context.Context, field string) (Null[T], error) {
	if query.allShards {
		return NullZero[T](), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return NullZero[T](), err
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return NullZero[T](), err
	}
	return aggregateNull[T](ctx, query.db, spec, "min", field)
}

func (query *TableQuery) Max[T any](ctx context.Context, field string) (Null[T], error) {
	if query.allShards {
		return NullZero[T](), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return NullZero[T](), err
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return NullZero[T](), err
	}
	return aggregateNull[T](ctx, query.db, spec, "max", field)
}

func (query *TableQuery) Create(ctx context.Context, values Map, options ...WriteOption) (Map, error) {
	if query.allShards {
		return nil, &Error{Op: "create", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: query.spec.Table}
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return nil, err
	}
	rows, err := createRows(ctx, query.db, WriteSpec{
		QuerySpec: spec,
		Values:    []Map{values},
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &Error{Op: "create", Kind: ErrScan, Table: query.spec.Table}
	}
	return rows[0], nil
}

func (query *TableQuery) Upsert(ctx context.Context, values Map, options ...WriteOption) (Map, error) {
	if query.allShards {
		return nil, &Error{Op: "upsert", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Table: query.spec.Table}
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return nil, err
	}
	writeOptions := applyWriteOptions(options)
	if writeOptions.conflict == nil {
		return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Table: query.spec.Table}
	}
	rows, err := upsertRows(ctx, query.db, WriteSpec{
		QuerySpec: spec,
		Values:    []Map{values},
		Conflict:  *writeOptions.conflict,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &Error{Op: "upsert", Kind: ErrScan, Table: query.spec.Table}
	}
	return rows[0], nil
}

func (query *TableQuery) CreateMany(ctx context.Context, values []Map, options ...WriteOption) ([]Map, error) {
	if len(values) == 0 {
		return []Map{}, nil
	}

	rows := make([]Map, 0, len(values))
	for _, value := range values {
		if len(value) == 0 {
			return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: query.spec.Table}
		}
		created, err := query.Create(ctx, value, options...)
		if err != nil {
			return nil, err
		}
		rows = append(rows, created)
	}
	return rows, nil
}

func (query *TableQuery) UpsertMany(ctx context.Context, values []Map, options ...WriteOption) ([]Map, error) {
	if len(values) == 0 {
		return []Map{}, nil
	}

	rows := make([]Map, 0, len(values))
	for _, value := range values {
		if len(value) == 0 {
			return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Table: query.spec.Table}
		}
		upserted, err := query.Upsert(ctx, value, options...)
		if err != nil {
			return nil, err
		}
		rows = append(rows, upserted)
	}
	return rows, nil
}

func (query *TableQuery) Update(ctx context.Context, values Map, options ...WriteOption) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "update", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return 0, &Error{Op: "update", Kind: ErrInvalidArgument, Table: query.spec.Table}
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return 0, err
	}
	return updateRows(ctx, query.db, WriteSpec{
		QuerySpec: spec,
		Values:    []Map{values},
	})
}

func (query *TableQuery) Delete(ctx context.Context) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "delete", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	spec, err := tableShardSpec(query)
	if err != nil {
		return 0, err
	}
	return deleteRows(ctx, query.db, WriteSpec{QuerySpec: spec})
}
