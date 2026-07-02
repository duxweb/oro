package oro

import (
	"context"
	"time"
)

// TypedTableQuery maps table rows into DTO values of type T.
type TypedTableQuery[T any] struct {
	query *TableQuery
}

// TypedRawQuery maps raw SQL rows into DTO values of type T.
type TypedRawQuery[T any] struct {
	query *RawQuery
}

// MapTo maps table query results into DTO values of type T.
func (query *TableQuery) MapTo[T any]() *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query}
}

// MapTo maps raw query results into DTO values of type T.
func (query *RawQuery) MapTo[T any]() *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query}
}

// Cache enables result caching for the typed raw query.
func (query *TypedRawQuery[T]) Cache(ttl time.Duration) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.Cache(ttl)}
}

// CacheKey sets an explicit cache key for the typed raw query.
func (query *TypedRawQuery[T]) CacheKey(key string) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.CacheKey(key)}
}

// CacheTags adds cache invalidation tags to the typed raw query.
func (query *TypedRawQuery[T]) CacheTags(tags ...string) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.CacheTags(tags...)}
}

// Timeout sets a per-query timeout for the typed raw query.
func (query *TypedRawQuery[T]) Timeout(timeout time.Duration) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.Timeout(timeout)}
}

// First returns the first mapped DTO or nil when no row is found.
func (query *TypedTableQuery[T]) First(ctx context.Context) (*T, error) {
	if structRowsDirectAvailable(query.query.db, query.query.spec) && !query.query.allShards {
		spec, schema, err := typedTableScanSpec[T](ctx, query.query)
		if err != nil {
			return nil, err
		}
		return queryStructFirstDirect[T](ctx, query.query.db, spec, schema)
	}
	row, err := query.query.First(ctx)
	if err != nil || row == nil {
		return nil, err
	}
	return mapDTO[T](query.query.db, row)
}

// Get returns all matching rows mapped into DTO values.
func (query *TypedTableQuery[T]) Get(ctx context.Context) ([]*T, error) {
	if structRowsDirectAvailable(query.query.db, query.query.spec) && !query.query.allShards {
		spec, schema, err := typedTableScanSpec[T](ctx, query.query)
		if err != nil {
			return nil, err
		}
		return queryStructRowsDirect[T](ctx, query.query.db, spec, schema)
	}
	rows, err := query.query.Get(ctx)
	if err != nil {
		return nil, err
	}
	return mapDTOs[T](query.query.db, rows)
}

// Stream opens a streaming iterator for mapped DTO values.
func (query *TypedTableQuery[T]) Stream(ctx context.Context) (Stream[*T], error) {
	rows, err := streamQuery(ctx, query.query.db, query.query.spec)
	if err != nil {
		return nil, err
	}
	return &mappedStream[*T]{
		rows: rows,
		mapFn: func(row Map) (*T, error) {
			return mapDTO[T](query.query.db, row)
		},
	}, nil
}

// Chunk iterates mapped DTO values in batches.
func (query *TypedTableQuery[T]) Chunk(ctx context.Context, size int, fn func([]*T) error) error {
	if fn == nil {
		return &Error{Op: "chunk", Kind: ErrInvalidArgument}
	}
	return query.query.Chunk(ctx, size, func(rows []Map) error {
		values, err := mapDTOs[T](query.query.db, rows)
		if err != nil {
			return err
		}
		return fn(values)
	})
}

// Each calls fn for every mapped DTO.
func (query *TypedTableQuery[T]) Each(ctx context.Context, fn func(*T) error) error {
	if fn == nil {
		return &Error{Op: "each", Kind: ErrInvalidArgument}
	}
	return query.Chunk(ctx, eachSize(query.query.db.runtime.Config), func(values []*T) error {
		for _, value := range values {
			if err := fn(value); err != nil {
				return err
			}
		}
		return nil
	})
}

// Paginate creates a paginator for mapped DTO values.
func (query *TypedTableQuery[T]) Paginate(size int) *Paginator[*T] {
	specErr := paginateSpecError(query.query.spec)
	return &Paginator[*T]{
		size: size,
		err:  specErr,
		count: func(ctx context.Context) (int64, error) {
			return query.query.Count(ctx)
		},
		items: func(ctx context.Context, limit int, offset int) ([]*T, error) {
			return query.Limit(limit).Offset(offset).Get(ctx)
		},
	}
}

// Sum returns the decimal sum of field.
func (query *TypedTableQuery[T]) Sum(ctx context.Context, field string) (Decimal, error) {
	return query.query.Sum(ctx, field)
}

// Avg returns the decimal average of field.
func (query *TypedTableQuery[T]) Avg(ctx context.Context, field string) (Decimal, error) {
	return query.query.Avg(ctx, field)
}

// Min returns the nullable minimum value of field.
func (query *TypedTableQuery[T]) Min[V any](ctx context.Context, field string) (Null[V], error) {
	return query.query.Min[V](ctx, field)
}

// Max returns the nullable maximum value of field.
func (query *TypedTableQuery[T]) Max[V any](ctx context.Context, field string) (Null[V], error) {
	return query.query.Max[V](ctx, field)
}

// Where adds AND conditions to the typed table query.
func (query *TypedTableQuery[T]) Where(field any, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Where(field, args...)}
}

// OrWhere adds OR conditions to the typed table query.
func (query *TypedTableQuery[T]) OrWhere(field any, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrWhere(field, args...)}
}

// WhereGroup adds a grouped AND condition built by fn.
func (query *TypedTableQuery[T]) WhereGroup(fn func(w *WhereBuilder)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereGroup(fn)}
}

// OrWhereGroup adds a grouped OR condition built by fn.
func (query *TypedTableQuery[T]) OrWhereGroup(fn func(w *WhereBuilder)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrWhereGroup(fn)}
}

// WhereWhen applies fn only when condition is true.
func (query *TypedTableQuery[T]) WhereWhen(condition bool, fn func(w *WhereBuilder)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereWhen(condition, fn)}
}

// WhereRaw adds a raw SQL WHERE fragment with bound arguments.
func (query *TypedTableQuery[T]) WhereRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereRaw(sql, args...)}
}

// WhereColumn compares two columns in the WHERE clause.
func (query *TypedTableQuery[T]) WhereColumn(left string, args ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereColumn(left, args...)}
}

// WhereIn adds an IN subquery condition.
func (query *TypedTableQuery[T]) WhereIn(field string, source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereIn(field, source)}
}

// WhereExists adds an EXISTS subquery condition.
func (query *TypedTableQuery[T]) WhereExists(source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereExists(source)}
}

// Select sets explicit columns, expressions, or aggregates to read.
func (query *TypedTableQuery[T]) Select(items ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Select(items...)}
}

// Cache enables query result caching for ttl.
func (query *TypedTableQuery[T]) Cache(ttl time.Duration) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Cache(ttl)}
}

// CacheKey sets an explicit cache key.
func (query *TypedTableQuery[T]) CacheKey(key string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.CacheKey(key)}
}

// CacheTags adds cache invalidation tags.
func (query *TypedTableQuery[T]) CacheTags(tags ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.CacheTags(tags...)}
}

// Timeout sets a per-query timeout.
func (query *TypedTableQuery[T]) Timeout(timeout time.Duration) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Timeout(timeout)}
}

// As sets a table alias for the query.
func (query *TypedTableQuery[T]) As(alias string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.As(alias)}
}

// Join adds an inner join.
func (query *TypedTableQuery[T]) Join(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Join(source, fn)}
}

// LeftJoin adds a left join.
func (query *TypedTableQuery[T]) LeftJoin(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.LeftJoin(source, fn)}
}

// RightJoin adds a right join.
func (query *TypedTableQuery[T]) RightJoin(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.RightJoin(source, fn)}
}

// FullJoin adds a full join.
func (query *TypedTableQuery[T]) FullJoin(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.FullJoin(source, fn)}
}

// CrossJoin adds a cross join.
func (query *TypedTableQuery[T]) CrossJoin(table string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.CrossJoin(table)}
}

// JoinRaw adds a raw join fragment.
func (query *TypedTableQuery[T]) JoinRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.JoinRaw(sql, args...)}
}

// OrderBy orders by columns ascending.
func (query *TypedTableQuery[T]) OrderBy(fields ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrderBy(fields...)}
}

// OrderByDesc orders by columns descending.
func (query *TypedTableQuery[T]) OrderByDesc(fields ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrderByDesc(fields...)}
}

// OrderByRaw adds a raw ORDER BY expression.
func (query *TypedTableQuery[T]) OrderByRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrderByRaw(sql, args...)}
}

// GroupBy groups results by columns.
func (query *TypedTableQuery[T]) GroupBy(fields ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.GroupBy(fields...)}
}

// Having adds AND conditions to the HAVING clause.
func (query *TypedTableQuery[T]) Having(field string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Having(field, args...)}
}

// HavingColumn compares two columns in the HAVING clause.
func (query *TypedTableQuery[T]) HavingColumn(left string, args ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingColumn(left, args...)}
}

// HavingIn adds an IN subquery to the HAVING clause.
func (query *TypedTableQuery[T]) HavingIn(field string, source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingIn(field, source)}
}

// HavingExists adds an EXISTS subquery to the HAVING clause.
func (query *TypedTableQuery[T]) HavingExists(source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingExists(source)}
}

// HavingRaw adds a raw HAVING fragment with bound arguments.
func (query *TypedTableQuery[T]) HavingRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingRaw(sql, args...)}
}

// Limit sets the maximum number of rows.
func (query *TypedTableQuery[T]) Limit(limit int) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Limit(limit)}
}

// Offset sets the number of rows to skip.
func (query *TypedTableQuery[T]) Offset(offset int) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Offset(offset)}
}

// LockForUpdate adds a FOR UPDATE lock.
func (query *TypedTableQuery[T]) LockForUpdate(options ...LockOption) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.LockForUpdate(options...)}
}

// LockForShare adds a FOR SHARE lock.
func (query *TypedTableQuery[T]) LockForShare(options ...LockOption) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.LockForShare(options...)}
}

// Create inserts one row and maps generated values into T.
func (query *TypedTableQuery[T]) Create(ctx context.Context, values Map, options ...WriteOption) (*T, error) {
	row, err := query.query.Create(ctx, values, options...)
	if err != nil || row == nil {
		return nil, err
	}
	return mapDTO[T](query.query.db, row)
}

// Upsert inserts or updates one row and maps generated values into T.
func (query *TypedTableQuery[T]) Upsert(ctx context.Context, values Map, options ...WriteOption) (*T, error) {
	row, err := query.query.Upsert(ctx, values, options...)
	if err != nil || row == nil {
		return nil, err
	}
	return mapDTO[T](query.query.db, row)
}

// CreateMany inserts rows in batches and returns generated primary IDs.
func (query *TypedTableQuery[T]) CreateMany(ctx context.Context, values []Map, options ...WriteOption) (*CreateResult, error) {
	return query.query.CreateMany(ctx, values, options...)
}

// CreateManyResult inserts rows in batches and returns mapped DTO values.
func (query *TypedTableQuery[T]) CreateManyResult(ctx context.Context, values []Map, options ...WriteOption) ([]*T, error) {
	rows, err := query.query.CreateManyResult(ctx, values, options...)
	if err != nil {
		return nil, err
	}
	return mapDTOs[T](query.query.db, rows)
}

// UpsertMany upserts rows in multi-row batches and returns affected rows.
func (query *TypedTableQuery[T]) UpsertMany(ctx context.Context, values []Map, options ...WriteOption) (int64, error) {
	return query.query.UpsertMany(ctx, values, options...)
}

// First returns the first raw row mapped into T or nil when no row is found.
func (query *TypedRawQuery[T]) First(ctx context.Context) (*T, error) {
	if structRowsDirectAvailable(query.query.db, QuerySpec{Cache: query.query.cache}) {
		schema := typedSchema[T](query.query.db)
		return queryRawStructFirstDirect[T](ctx, query.query.db, query.query.raw, query.query.timeout, schema)
	}
	row, err := query.query.First(ctx)
	if err != nil || row == nil {
		return nil, err
	}
	return mapDTO[T](query.query.db, row)
}

// Get returns all raw rows mapped into T.
func (query *TypedRawQuery[T]) Get(ctx context.Context) ([]*T, error) {
	if structRowsDirectAvailable(query.query.db, QuerySpec{Cache: query.query.cache}) {
		schema := typedSchema[T](query.query.db)
		return queryRawStructRowsDirect[T](ctx, query.query.db, query.query.raw, query.query.timeout, schema)
	}
	rows, err := query.query.Get(ctx)
	if err != nil {
		return nil, err
	}
	return mapDTOs[T](query.query.db, rows)
}

// Stream opens a streaming iterator for raw rows mapped into T.
func (query *TypedRawQuery[T]) Stream(ctx context.Context) (Stream[*T], error) {
	rows, err := streamRaw(ctx, query.query.db, query.query.raw, query.query.timeout)
	if err != nil {
		return nil, err
	}
	return &mappedStream[*T]{
		rows: rows,
		mapFn: func(row Map) (*T, error) {
			return mapDTO[T](query.query.db, row)
		},
	}, nil
}

func mapDTO[T any](db *DB, row Map) (*T, error) {
	value := new(T)
	if err := db.runtime.Mapper.MapDTO(row, value); err != nil {
		return nil, err
	}
	return value, nil
}

func mapDTOs[T any](db *DB, rows []Map) ([]*T, error) {
	values := make([]*T, 0, len(rows))
	for _, row := range rows {
		value, err := mapDTO[T](db, row)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func typedTableScanSpec[T any](ctx context.Context, query *TableQuery) (QuerySpec, *ModelSchema, error) {
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return QuerySpec{}, nil, err
	}
	return spec, typedSchema[T](query.db), nil
}

func typedSchema[T any](db *DB) *ModelSchema {
	if db == nil || db.runtime == nil || db.runtime.Registry == nil {
		return nil
	}
	destType, err := structTypeOfGeneric[T]()
	if err != nil {
		return nil
	}
	schema, _ := db.runtime.Registry.GetType(destType)
	return schema
}
