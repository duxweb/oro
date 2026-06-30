package oro

import (
	"context"
	"time"
)

// TableQuery is a clone-on-write query builder for direct table access.
type TableQuery struct {
	db        *DB
	spec      QuerySpec
	shard     Map
	allShards bool
}

// Where adds AND conditions to the table query.
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

// OrWhere adds OR conditions to the table query.
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

// WhereGroup adds a grouped AND condition built by fn.
func (query *TableQuery) WhereGroup(fn func(w *WhereBuilder)) *TableQuery {
	clone := *query
	condition := buildWhereGroup("and", fn)
	if condition.Op == "empty_group" {
		return &clone
	}
	clone.spec.Where = append(clone.spec.Where, condition)
	return &clone
}

// OrWhereGroup adds a grouped OR condition built by fn.
func (query *TableQuery) OrWhereGroup(fn func(w *WhereBuilder)) *TableQuery {
	clone := *query
	condition := buildWhereGroup("or", fn)
	if condition.Op == "empty_group" {
		return &clone
	}
	clone.spec.Where = append(clone.spec.Where, condition)
	return &clone
}

// WhereWhen applies fn only when condition is true.
func (query *TableQuery) WhereWhen(condition bool, fn func(w *WhereBuilder)) *TableQuery {
	if !condition {
		return query
	}
	return query.WhereGroup(fn)
}

// WhereRaw adds a raw SQL WHERE fragment with bound arguments.
func (query *TableQuery) WhereRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", RawCondition(sql, args...)))
	return &clone
}

// WhereColumn compares two columns in the WHERE clause.
func (query *TableQuery) WhereColumn(left string, args ...string) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildColumnCondition(left, args...)))
	return &clone
}

// WhereIn adds an IN subquery condition.
func (query *TableQuery) WhereIn(field string, source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildInCondition(field, source)))
	return &clone
}

// WhereExists adds an EXISTS subquery condition.
func (query *TableQuery) WhereExists(source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildExistsCondition(source)))
	return &clone
}

// Select sets explicit columns, expressions, or aggregates to read.
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

// As sets a table alias for the query.
func (query *TableQuery) As(alias string) *TableQuery {
	clone := *query
	clone.spec.Alias = alias
	return &clone
}

// UsePrimary forces reads to use the primary connection.
func (query *TableQuery) UsePrimary() *TableQuery {
	clone := *query
	clone.spec.UsePrimary = true
	return &clone
}

// Cache enables query result caching for ttl.
func (query *TableQuery) Cache(ttl time.Duration) *TableQuery {
	clone := *query
	clone.spec.Cache.Enabled = true
	clone.spec.Cache.TTL = int64(ttl)
	if ttl <= 0 {
		clone.spec.SelectErr = &Error{Op: "cache", Kind: ErrInvalidArgument}
	}
	return &clone
}

// CacheKey sets an explicit cache key.
func (query *TableQuery) CacheKey(key string) *TableQuery {
	clone := *query
	clone.spec.Cache.Key = key
	return &clone
}

// CacheTags adds cache invalidation tags.
func (query *TableQuery) CacheTags(tags ...string) *TableQuery {
	clone := *query
	clone.spec.Cache.Tags = append(clone.spec.Cache.Tags, tags...)
	return &clone
}

// Timeout sets a per-query timeout.
func (query *TableQuery) Timeout(timeout time.Duration) *TableQuery {
	clone := *query
	clone.spec.Timeout = int64(timeout)
	return &clone
}

// Join adds an inner join.
func (query *TableQuery) Join(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinInner, source, fn))
	return &clone
}

// LeftJoin adds a left join.
func (query *TableQuery) LeftJoin(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinLeft, source, fn))
	return &clone
}

// RightJoin adds a right join.
func (query *TableQuery) RightJoin(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinRight, source, fn))
	return &clone
}

// FullJoin adds a full join.
func (query *TableQuery) FullJoin(source any, fn func(j *Join)) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinFull, source, fn))
	return &clone
}

// CrossJoin adds a cross join.
func (query *TableQuery) CrossJoin(table string) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinCross, table, nil))
	return &clone
}

// JoinRaw adds a raw join fragment.
func (query *TableQuery) JoinRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, JoinAST{
		Raw: &RawSpec{SQL: sql, Args: args},
	})
	return &clone
}

// OrderBy orders by columns ascending.
func (query *TableQuery) OrderBy(fields ...string) *TableQuery {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, orderExprs(false, fields)...)
	return &clone
}

// OrderByDesc orders by columns descending.
func (query *TableQuery) OrderByDesc(fields ...string) *TableQuery {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, orderExprs(true, fields)...)
	return &clone
}

// OrderByRaw adds a raw ORDER BY expression.
func (query *TableQuery) OrderByRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, OrderExpr{Expr: sql, Raw: true, Args: args})
	return &clone
}

// GroupBy groups results by columns.
func (query *TableQuery) GroupBy(fields ...string) *TableQuery {
	clone := *query
	clone.spec.Group = append(clone.spec.Group, fields...)
	return &clone
}

// Having adds AND conditions to the HAVING clause.
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

// HavingColumn compares two columns in the HAVING clause.
func (query *TableQuery) HavingColumn(left string, args ...string) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildColumnCondition(left, args...)))
	return &clone
}

// HavingIn adds an IN subquery to the HAVING clause.
func (query *TableQuery) HavingIn(field string, source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildInCondition(field, source)))
	return &clone
}

// HavingExists adds an EXISTS subquery to the HAVING clause.
func (query *TableQuery) HavingExists(source QuerySource) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildExistsCondition(source)))
	return &clone
}

// HavingRaw adds a raw HAVING fragment with bound arguments.
func (query *TableQuery) HavingRaw(sql string, args ...any) *TableQuery {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", RawCondition(sql, args...)))
	return &clone
}

// Limit sets the maximum number of rows.
func (query *TableQuery) Limit(limit int) *TableQuery {
	clone := *query
	clone.spec.Limit = &limit
	return &clone
}

// Offset sets the number of rows to skip.
func (query *TableQuery) Offset(offset int) *TableQuery {
	clone := *query
	clone.spec.Offset = &offset
	return &clone
}

// LockForUpdate adds a FOR UPDATE lock.
func (query *TableQuery) LockForUpdate(options ...LockOption) *TableQuery {
	clone := *query
	clone.spec.Lock = applyLockOptions(LockUpdate, options)
	return &clone
}

// LockForShare adds a FOR SHARE lock.
func (query *TableQuery) LockForShare(options ...LockOption) *TableQuery {
	clone := *query
	clone.spec.Lock = applyLockOptions(LockShare, options)
	return &clone
}

// Shard pins the query to a shard group resolved from values.
func (query *TableQuery) Shard(group string, values Map) *TableQuery {
	clone := *query
	clone.spec.ShardGroup = group
	clone.shard = copyMap(values)
	clone.allShards = false
	return &clone
}

// AllShards runs the read across all shards in group.
func (query *TableQuery) AllShards(group string) *TableQuery {
	clone := *query
	clone.spec.ShardGroup = group
	clone.shard = nil
	clone.allShards = true
	return &clone
}

// First returns the first matching row or nil when no row is found.
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
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return nil, err
	}
	return queryFirstRowPrepared(ctx, query.db, spec)
}

// Get returns all matching rows as Map values.
func (query *TableQuery) Get(ctx context.Context) ([]Map, error) {
	if query.allShards {
		return queryAllShardRows(ctx, query)
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return nil, err
	}
	return queryRowsPrepared(ctx, query.db, spec)
}

// Stream opens a streaming iterator for matching rows.
func (query *TableQuery) Stream(ctx context.Context) (Stream[Map], error) {
	if query.allShards {
		return nil, &Error{Op: "stream", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return nil, err
	}
	rows, err := streamQueryPrepared(ctx, query.db, spec)
	if err != nil {
		return nil, err
	}
	return &mapStream{rows: rows}, nil
}

// Chunk iterates matching rows in batches.
func (query *TableQuery) Chunk(ctx context.Context, size int, fn func([]Map) error) error {
	if query.allShards {
		return &Error{Op: "chunk", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := chunkSpecError(query.spec); err != nil {
		return err
	}
	spec, err := tableShardSpec(ctx, query)
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

// Each calls fn for every matching row.
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

// Paginate creates a paginator for the query.
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

// Count returns the number of matching rows or groups.
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
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return 0, err
	}
	// Finalize before wrapping so scoping (tenant/soft-delete) lands inside the
	// grouped-count subquery instead of the discarded outer wrapper.
	if err := finalizeReadSpec(ctx, query.db, &spec); err != nil {
		return 0, err
	}
	spec, err = countQuerySpec(spec)
	if err != nil {
		return 0, err
	}

	row, err := queryFirstRowPrepared(ctx, query.db, spec)
	if err != nil || row == nil {
		return 0, err
	}
	return rowInt64(row, "total")
}

// Exists reports whether the query has at least one matching row.
func (query *TableQuery) Exists(ctx context.Context) (bool, error) {
	if query.allShards {
		rows, err := queryAllShardExists(ctx, query)
		if err != nil {
			return false, err
		}
		return len(rows) > 0, nil
	}
	spec, err := tableShardSpec(ctx, query)
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

// Sum returns the decimal sum of field.
func (query *TableQuery) Sum(ctx context.Context, field string) (Decimal, error) {
	if query.allShards {
		return Decimal("0"), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return Decimal("0"), err
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return Decimal("0"), err
	}
	return aggregateDecimal(ctx, query.db, spec, "sum", field)
}

// Avg returns the decimal average of field.
func (query *TableQuery) Avg(ctx context.Context, field string) (Decimal, error) {
	if query.allShards {
		return Decimal("0"), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return Decimal("0"), err
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return Decimal("0"), err
	}
	return aggregateDecimal(ctx, query.db, spec, "avg", field)
}

// Min returns the nullable minimum value of field.
func (query *TableQuery) Min[T any](ctx context.Context, field string) (Null[T], error) {
	if query.allShards {
		return NullZero[T](), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return NullZero[T](), err
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return NullZero[T](), err
	}
	return aggregateNull[T](ctx, query.db, spec, "min", field)
}

// Max returns the nullable maximum value of field.
func (query *TableQuery) Max[T any](ctx context.Context, field string) (Null[T], error) {
	if query.allShards {
		return NullZero[T](), &Error{Op: "aggregate", Kind: ErrUnsupported, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if err := ensureAggregateSpec(query.spec); err != nil {
		return NullZero[T](), err
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return NullZero[T](), err
	}
	return aggregateNull[T](ctx, query.db, spec, "max", field)
}

// Create inserts one row and returns generated values.
func (query *TableQuery) Create(ctx context.Context, values Map, options ...WriteOption) (Map, error) {
	if query.allShards {
		return nil, &Error{Op: "create", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: query.spec.Table}
	}
	spec, err := tableShardSpec(ctx, query)
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

// Upsert inserts or updates one row using a conflict option.
func (query *TableQuery) Upsert(ctx context.Context, values Map, options ...WriteOption) (Map, error) {
	if query.allShards {
		return nil, &Error{Op: "upsert", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Table: query.spec.Table}
	}
	spec, err := tableShardSpec(ctx, query)
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

// CreateMany inserts rows in batches and returns generated primary IDs.
func (query *TableQuery) CreateMany(ctx context.Context, values []Map, options ...WriteOption) (*CreateResult, error) {
	if query.allShards {
		return nil, &Error{Op: "create", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return &CreateResult{}, nil
	}

	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return nil, err
	}
	writeOptions := applyWriteOptions(options)
	result := &CreateResult{}
	for _, chunk := range chunkMapsForCreate(values, createBatchSize(query.db.runtime.Config, writeOptions)) {
		for _, value := range chunk {
			if len(value) == 0 {
				return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: query.spec.Table}
			}
		}
		if !mapsHaveSameKeys(chunk) {
			for _, value := range chunk {
				created, err := query.Create(ctx, value, options...)
				if err != nil {
					return nil, err
				}
				if result.PrimaryKey == "" {
					result.PrimaryKey, _ = tablePrimaryKey(ctx, query.db, spec)
				}
				if result.PrimaryKey != "" {
					result.primaryIDs = appendPrimaryIDs(result.primaryIDs, []any{created[result.PrimaryKey]})
				}
				result.RowsAffected++
			}
			continue
		}
		created, err := createResultRows(ctx, query.db, WriteSpec{
			QuerySpec: spec,
			Values:    chunk,
		})
		if err != nil {
			return nil, err
		}
		if result.PrimaryKey == "" {
			result.PrimaryKey = created.PrimaryKey
		}
		result.primaryIDs = appendPrimaryIDs(result.primaryIDs, created.primaryIDs)
		result.RowsAffected += created.RowsAffected
	}
	return result, nil
}

// CreateManyResult inserts rows in batches and returns generated rows.
func (query *TableQuery) CreateManyResult(ctx context.Context, values []Map, options ...WriteOption) ([]Map, error) {
	if query.allShards {
		return nil, &Error{Op: "create", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return []Map{}, nil
	}

	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return nil, err
	}
	writeOptions := applyWriteOptions(options)
	rows := make([]Map, 0, len(values))
	for _, chunk := range chunkMapsForCreate(values, createBatchSize(query.db.runtime.Config, writeOptions)) {
		for _, value := range chunk {
			if len(value) == 0 {
				return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: query.spec.Table}
			}
		}
		if !mapsHaveSameKeys(chunk) {
			for _, value := range chunk {
				created, err := query.Create(ctx, value, options...)
				if err != nil {
					return nil, err
				}
				rows = append(rows, created)
			}
			continue
		}
		created, err := createRows(ctx, query.db, WriteSpec{
			QuerySpec: spec,
			Values:    chunk,
		})
		if err != nil {
			return nil, err
		}
		rows = append(rows, created...)
	}
	return rows, nil
}

// UpsertMany upserts rows one by one with the configured conflict option.
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

// Update updates matching rows with explicit Map values.
func (query *TableQuery) Update(ctx context.Context, values Map, options ...WriteOption) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "update", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	if len(values) == 0 {
		return 0, &Error{Op: "update", Kind: ErrInvalidArgument, Table: query.spec.Table}
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return 0, err
	}
	return updateRows(ctx, query.db, WriteSpec{
		QuerySpec: spec,
		Values:    []Map{values},
	})
}

// Delete deletes matching rows.
func (query *TableQuery) Delete(ctx context.Context) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "delete", Kind: ErrShardRequired, Table: query.spec.Table, Field: query.spec.ShardGroup}
	}
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return 0, err
	}
	return deleteRows(ctx, query.db, WriteSpec{QuerySpec: spec})
}
