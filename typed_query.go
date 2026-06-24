package oro

import (
	"context"
	"time"
)

type TypedTableQuery[T any] struct {
	query *TableQuery
}

type TypedRawQuery[T any] struct {
	query *RawQuery
}

func (query *TableQuery) MapTo[T any]() *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query}
}

func (query *RawQuery) MapTo[T any]() *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query}
}

func (query *TypedRawQuery[T]) Cache(ttl time.Duration) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.Cache(ttl)}
}

func (query *TypedRawQuery[T]) CacheKey(key string) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.CacheKey(key)}
}

func (query *TypedRawQuery[T]) CacheTags(tags ...string) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.CacheTags(tags...)}
}

func (query *TypedRawQuery[T]) Timeout(timeout time.Duration) *TypedRawQuery[T] {
	return &TypedRawQuery[T]{query: query.query.Timeout(timeout)}
}

func (query *TypedTableQuery[T]) First(ctx context.Context) (*T, error) {
	if structRowsDirectAvailable(query.query.db, query.query.spec) && !query.query.allShards {
		spec, schema, err := typedTableScanSpec[T](query.query)
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

func (query *TypedTableQuery[T]) Get(ctx context.Context) ([]*T, error) {
	if structRowsDirectAvailable(query.query.db, query.query.spec) && !query.query.allShards {
		spec, schema, err := typedTableScanSpec[T](query.query)
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

func (query *TypedTableQuery[T]) Sum(ctx context.Context, field string) (Decimal, error) {
	return query.query.Sum(ctx, field)
}

func (query *TypedTableQuery[T]) Avg(ctx context.Context, field string) (Decimal, error) {
	return query.query.Avg(ctx, field)
}

func (query *TypedTableQuery[T]) Min[V any](ctx context.Context, field string) (Null[V], error) {
	return query.query.Min[V](ctx, field)
}

func (query *TypedTableQuery[T]) Max[V any](ctx context.Context, field string) (Null[V], error) {
	return query.query.Max[V](ctx, field)
}

func (query *TypedTableQuery[T]) Where(field any, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Where(field, args...)}
}

func (query *TypedTableQuery[T]) OrWhere(field any, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrWhere(field, args...)}
}

func (query *TypedTableQuery[T]) WhereGroup(fn func(w *WhereBuilder)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereGroup(fn)}
}

func (query *TypedTableQuery[T]) OrWhereGroup(fn func(w *WhereBuilder)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrWhereGroup(fn)}
}

func (query *TypedTableQuery[T]) WhereWhen(condition bool, fn func(w *WhereBuilder)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereWhen(condition, fn)}
}

func (query *TypedTableQuery[T]) WhereRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereRaw(sql, args...)}
}

func (query *TypedTableQuery[T]) WhereColumn(left string, args ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereColumn(left, args...)}
}

func (query *TypedTableQuery[T]) WhereIn(field string, source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereIn(field, source)}
}

func (query *TypedTableQuery[T]) WhereExists(source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.WhereExists(source)}
}

func (query *TypedTableQuery[T]) Select(items ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Select(items...)}
}

func (query *TypedTableQuery[T]) Cache(ttl time.Duration) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Cache(ttl)}
}

func (query *TypedTableQuery[T]) CacheKey(key string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.CacheKey(key)}
}

func (query *TypedTableQuery[T]) CacheTags(tags ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.CacheTags(tags...)}
}

func (query *TypedTableQuery[T]) Timeout(timeout time.Duration) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Timeout(timeout)}
}

func (query *TypedTableQuery[T]) As(alias string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.As(alias)}
}

func (query *TypedTableQuery[T]) Join(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Join(source, fn)}
}

func (query *TypedTableQuery[T]) LeftJoin(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.LeftJoin(source, fn)}
}

func (query *TypedTableQuery[T]) RightJoin(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.RightJoin(source, fn)}
}

func (query *TypedTableQuery[T]) FullJoin(source any, fn func(j *Join)) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.FullJoin(source, fn)}
}

func (query *TypedTableQuery[T]) CrossJoin(table string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.CrossJoin(table)}
}

func (query *TypedTableQuery[T]) JoinRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.JoinRaw(sql, args...)}
}

func (query *TypedTableQuery[T]) OrderBy(fields ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrderBy(fields...)}
}

func (query *TypedTableQuery[T]) OrderByDesc(fields ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrderByDesc(fields...)}
}

func (query *TypedTableQuery[T]) OrderByRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.OrderByRaw(sql, args...)}
}

func (query *TypedTableQuery[T]) GroupBy(fields ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.GroupBy(fields...)}
}

func (query *TypedTableQuery[T]) Having(field string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Having(field, args...)}
}

func (query *TypedTableQuery[T]) HavingColumn(left string, args ...string) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingColumn(left, args...)}
}

func (query *TypedTableQuery[T]) HavingIn(field string, source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingIn(field, source)}
}

func (query *TypedTableQuery[T]) HavingExists(source QuerySource) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingExists(source)}
}

func (query *TypedTableQuery[T]) HavingRaw(sql string, args ...any) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.HavingRaw(sql, args...)}
}

func (query *TypedTableQuery[T]) Limit(limit int) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Limit(limit)}
}

func (query *TypedTableQuery[T]) Offset(offset int) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.Offset(offset)}
}

func (query *TypedTableQuery[T]) LockForUpdate(options ...LockOption) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.LockForUpdate(options...)}
}

func (query *TypedTableQuery[T]) LockForShare(options ...LockOption) *TypedTableQuery[T] {
	return &TypedTableQuery[T]{query: query.query.LockForShare(options...)}
}

func (query *TypedTableQuery[T]) Create(ctx context.Context, values Map, options ...WriteOption) (*T, error) {
	row, err := query.query.Create(ctx, values, options...)
	if err != nil || row == nil {
		return nil, err
	}
	return mapDTO[T](query.query.db, row)
}

func (query *TypedTableQuery[T]) Upsert(ctx context.Context, values Map, options ...WriteOption) (*T, error) {
	row, err := query.query.Upsert(ctx, values, options...)
	if err != nil || row == nil {
		return nil, err
	}
	return mapDTO[T](query.query.db, row)
}

func (query *TypedTableQuery[T]) CreateMany(ctx context.Context, values []Map, options ...WriteOption) (*CreateResult, error) {
	return query.query.CreateMany(ctx, values, options...)
}

func (query *TypedTableQuery[T]) CreateManyResult(ctx context.Context, values []Map, options ...WriteOption) ([]*T, error) {
	rows, err := query.query.CreateManyResult(ctx, values, options...)
	if err != nil {
		return nil, err
	}
	return mapDTOs[T](query.query.db, rows)
}

func (query *TypedTableQuery[T]) UpsertMany(ctx context.Context, values []Map, options ...WriteOption) ([]*T, error) {
	rows, err := query.query.UpsertMany(ctx, values, options...)
	if err != nil {
		return nil, err
	}
	return mapDTOs[T](query.query.db, rows)
}

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

func typedTableScanSpec[T any](query *TableQuery) (QuerySpec, *ModelSchema, error) {
	spec, err := tableShardSpec(query)
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
