package oro

import (
	"context"
	"strings"
	"time"
)

type ModelQuery[T any] struct {
	db             *DB
	spec           QuerySpec
	shard          Map
	allShards      bool
	softDeleteMode softDeleteMode
	selectHidden   []string
	skipHooks      bool
	skipEvents     bool
}

type softDeleteMode int

const (
	softDeleteDefault softDeleteMode = iota
	softDeleteWith
	softDeleteOnly
)

func (query *ModelQuery[T]) Where(field any, args ...any) *ModelQuery[T] {
	clone := *query
	conditions, err := appendWhereCondition(clone.spec.Where, "and", field, args...)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Where = conditions
	return &clone
}

func (query *ModelQuery[T]) OrWhere(field any, args ...any) *ModelQuery[T] {
	clone := *query
	conditions, err := appendWhereCondition(clone.spec.Where, "or", field, args...)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Where = conditions
	return &clone
}

func (query *ModelQuery[T]) WhereGroup(fn func(w *WhereBuilder)) *ModelQuery[T] {
	clone := *query
	condition := buildWhereGroup("and", fn)
	if condition.Op == "empty_group" {
		return &clone
	}
	clone.spec.Where = append(clone.spec.Where, condition)
	return &clone
}

func (query *ModelQuery[T]) OrWhereGroup(fn func(w *WhereBuilder)) *ModelQuery[T] {
	clone := *query
	condition := buildWhereGroup("or", fn)
	if condition.Op == "empty_group" {
		return &clone
	}
	clone.spec.Where = append(clone.spec.Where, condition)
	return &clone
}

func (query *ModelQuery[T]) WhereWhen(condition bool, fn func(w *WhereBuilder)) *ModelQuery[T] {
	if !condition {
		return query
	}
	return query.WhereGroup(fn)
}

func (query *ModelQuery[T]) WhereRaw(sql string, args ...any) *ModelQuery[T] {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", RawCondition(sql, args...)))
	return &clone
}

func (query *ModelQuery[T]) WhereColumn(left string, args ...string) *ModelQuery[T] {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildColumnCondition(left, args...)))
	return &clone
}

func (query *ModelQuery[T]) WhereIn(field string, source QuerySource) *ModelQuery[T] {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildInCondition(field, source)))
	return &clone
}

func (query *ModelQuery[T]) WhereExists(source QuerySource) *ModelQuery[T] {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, withBool("and", buildExistsCondition(source)))
	return &clone
}

func (query *ModelQuery[T]) Select(items ...any) *ModelQuery[T] {
	clone := *query
	exprs, err := selectExprs(items)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Select = append(clone.spec.Select, exprs...)
	return &clone
}

func (query *ModelQuery[T]) With(relation any, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	clone := *query
	with, err := buildWithSpec(relation, callbacks)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.With = append(clone.spec.With, with)
	return &clone
}

func (query *ModelQuery[T]) For(relation Relation) *ModelQuery[T] {
	clone := *query
	schema, err := schemaForModel[T](query.db)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	conditions, err := relationForConditions(relation, schema, query.db)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Where = append(clone.spec.Where, conditions...)
	return &clone
}

func (query *ModelQuery[T]) As(alias string) *ModelQuery[T] {
	clone := *query
	clone.spec.Alias = alias
	return &clone
}

func (query *ModelQuery[T]) SelectHidden(fields ...string) *ModelQuery[T] {
	clone := *query
	clone.selectHidden = append(clone.selectHidden, fields...)
	return &clone
}

func (query *ModelQuery[T]) SkipHooks() *ModelQuery[T] {
	clone := *query
	clone.skipHooks = true
	return &clone
}

func (query *ModelQuery[T]) SkipEvents() *ModelQuery[T] {
	clone := *query
	clone.skipEvents = true
	clone.spec.SkipEvents = true
	return &clone
}

func (query *ModelQuery[T]) UsePrimary() *ModelQuery[T] {
	clone := *query
	clone.spec.UsePrimary = true
	return &clone
}

func (query *ModelQuery[T]) Cache(ttl time.Duration) *ModelQuery[T] {
	clone := *query
	clone.spec.Cache.Enabled = true
	clone.spec.Cache.TTL = int64(ttl)
	if ttl <= 0 {
		clone.spec.SelectErr = &Error{Op: "cache", Kind: ErrInvalidArgument}
	}
	return &clone
}

func (query *ModelQuery[T]) CacheKey(key string) *ModelQuery[T] {
	clone := *query
	clone.spec.Cache.Key = key
	return &clone
}

func (query *ModelQuery[T]) CacheTags(tags ...string) *ModelQuery[T] {
	clone := *query
	clone.spec.Cache.Tags = append(clone.spec.Cache.Tags, tags...)
	return &clone
}

func (query *ModelQuery[T]) Timeout(timeout time.Duration) *ModelQuery[T] {
	clone := *query
	clone.spec.Timeout = int64(timeout)
	return &clone
}

func (query *ModelQuery[T]) Join(source any, fn func(j *Join)) *ModelQuery[T] {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinInner, source, fn))
	return &clone
}

func (query *ModelQuery[T]) LeftJoin(source any, fn func(j *Join)) *ModelQuery[T] {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinLeft, source, fn))
	return &clone
}

func (query *ModelQuery[T]) RightJoin(source any, fn func(j *Join)) *ModelQuery[T] {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinRight, source, fn))
	return &clone
}

func (query *ModelQuery[T]) FullJoin(source any, fn func(j *Join)) *ModelQuery[T] {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinFull, source, fn))
	return &clone
}

func (query *ModelQuery[T]) CrossJoin(table string) *ModelQuery[T] {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, buildJoin(JoinCross, table, nil))
	return &clone
}

func (query *ModelQuery[T]) JoinRaw(sql string, args ...any) *ModelQuery[T] {
	clone := *query
	clone.spec.Joins = append(clone.spec.Joins, JoinAST{
		Raw: &RawSpec{SQL: sql, Args: args},
	})
	return &clone
}

func (query *ModelQuery[T]) OrderBy(fields ...string) *ModelQuery[T] {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, orderExprs(false, fields)...)
	return &clone
}

func (query *ModelQuery[T]) OrderByDesc(fields ...string) *ModelQuery[T] {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, orderExprs(true, fields)...)
	return &clone
}

func (query *ModelQuery[T]) OrderByRaw(sql string, args ...any) *ModelQuery[T] {
	clone := *query
	clone.spec.Order = append(clone.spec.Order, OrderExpr{Expr: sql, Raw: true, Args: args})
	return &clone
}

func (query *ModelQuery[T]) GroupBy(fields ...string) *ModelQuery[T] {
	clone := *query
	clone.spec.Group = append(clone.spec.Group, fields...)
	return &clone
}

func (query *ModelQuery[T]) Having(field string, args ...any) *ModelQuery[T] {
	clone := *query
	conditions, err := buildConditions(field, args...)
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Having = append(clone.spec.Having, conditionsWithBool("and", conditions)...)
	return &clone
}

func (query *ModelQuery[T]) HavingColumn(left string, args ...string) *ModelQuery[T] {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildColumnCondition(left, args...)))
	return &clone
}

func (query *ModelQuery[T]) HavingIn(field string, source QuerySource) *ModelQuery[T] {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildInCondition(field, source)))
	return &clone
}

func (query *ModelQuery[T]) HavingExists(source QuerySource) *ModelQuery[T] {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", buildExistsCondition(source)))
	return &clone
}

func (query *ModelQuery[T]) HavingRaw(sql string, args ...any) *ModelQuery[T] {
	clone := *query
	clone.spec.Having = append(clone.spec.Having, withBool("and", RawCondition(sql, args...)))
	return &clone
}

func (query *ModelQuery[T]) Limit(limit int) *ModelQuery[T] {
	clone := *query
	clone.spec.Limit = &limit
	return &clone
}

func (query *ModelQuery[T]) Offset(offset int) *ModelQuery[T] {
	clone := *query
	clone.spec.Offset = &offset
	return &clone
}

func (query *ModelQuery[T]) LockForUpdate(options ...LockOption) *ModelQuery[T] {
	clone := *query
	clone.spec.Lock = applyLockOptions(LockUpdate, options)
	return &clone
}

func (query *ModelQuery[T]) LockForShare(options ...LockOption) *ModelQuery[T] {
	clone := *query
	clone.spec.Lock = applyLockOptions(LockShare, options)
	return &clone
}

func (query *ModelQuery[T]) WithDeleted() *ModelQuery[T] {
	clone := *query
	clone.softDeleteMode = softDeleteWith
	return &clone
}

func (query *ModelQuery[T]) OnlyDeleted() *ModelQuery[T] {
	clone := *query
	clone.softDeleteMode = softDeleteOnly
	return &clone
}

func (query *ModelQuery[T]) Shard(values Map) *ModelQuery[T] {
	clone := *query
	clone.shard = copyMap(values)
	clone.allShards = false
	return &clone
}

func (query *ModelQuery[T]) AllShards() *ModelQuery[T] {
	clone := *query
	clone.shard = nil
	clone.allShards = true
	return &clone
}

func (query *ModelQuery[T]) First(ctx context.Context) (*T, error) {
	if query.allShards && len(query.spec.Order) == 0 {
		return nil, &Error{Op: "first", Kind: ErrOrderRequired}
	}
	spec, schema, err := modelQuerySpec(query)
	if err != nil {
		return nil, err
	}
	if query.allShards {
		models, err := query.Limit(1).Get(ctx)
		if err != nil || len(models) == 0 {
			return nil, err
		}
		return models[0], nil
	}
	if modelRowsDirectAvailable(query.db, spec) {
		model, err := queryModelFirstDirect[T](ctx, query.db, spec, schema)
		if err != nil || model == nil {
			return nil, err
		}
		if err := loadModelRelations(ctx, query.db, schema, []*T{model}, spec.With); err != nil {
			return nil, err
		}
		if err := query.afterFind(ctx, schema, model); err != nil {
			return nil, err
		}
		return model, nil
	}
	row, err := queryFirstRowPrepared(ctx, query.db, spec)
	if err != nil || row == nil {
		return nil, err
	}
	model := new(T)
	if err := query.db.runtime.Mapper.MapModel(schema, row, model); err != nil {
		return nil, err
	}
	if err := loadModelRelations(ctx, query.db, schema, []*T{model}, spec.With); err != nil {
		return nil, err
	}
	if err := query.afterFind(ctx, schema, model); err != nil {
		return nil, err
	}
	return model, nil
}

func (query *ModelQuery[T]) Get(ctx context.Context) ([]*T, error) {
	spec, schema, err := modelQuerySpec(query)
	if err != nil {
		return nil, err
	}
	if query.allShards {
		return query.getAllShards(ctx, spec, schema)
	}
	if modelRowsDirectAvailable(query.db, spec) {
		models, err := queryModelRowsDirect[T](ctx, query.db, spec, schema)
		if err != nil {
			return nil, err
		}
		if err := loadModelRelations(ctx, query.db, schema, models, spec.With); err != nil {
			return nil, err
		}
		for _, model := range models {
			if err := query.afterFind(ctx, schema, model); err != nil {
				return nil, err
			}
		}
		return models, nil
	}
	rows, err := queryRowsPrepared(ctx, query.db, spec)
	if err != nil {
		return nil, err
	}

	models := make([]*T, 0, len(rows))
	for _, row := range rows {
		model := new(T)
		if err := query.db.runtime.Mapper.MapModel(schema, row, model); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	if err := loadModelRelations(ctx, query.db, schema, models, spec.With); err != nil {
		return nil, err
	}
	for _, model := range models {
		if err := query.afterFind(ctx, schema, model); err != nil {
			return nil, err
		}
	}
	return models, nil
}

func (query *ModelQuery[T]) getAllShards(ctx context.Context, spec QuerySpec, schema *ModelSchema) ([]*T, error) {
	rows, err := queryModelAllShardRows(ctx, query.db, spec)
	if err != nil {
		return nil, err
	}
	models := make([]*T, 0, len(rows))
	for _, row := range rows {
		model := new(T)
		if err := query.db.runtime.Mapper.MapModel(schema, row, model); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	if err := loadModelRelations(ctx, query.db, schema, models, spec.With); err != nil {
		return nil, err
	}
	for _, model := range models {
		if err := query.afterFind(ctx, schema, model); err != nil {
			return nil, err
		}
	}
	return models, nil
}

func queryModelAllShardRows(ctx context.Context, db *DB, spec QuerySpec) ([]Map, error) {
	if spec.ShardGroup == "" {
		return queryRowsPrepared(ctx, db, spec)
	}
	config, ok := db.runtime.Config.Shards[spec.ShardGroup]
	if !ok {
		return nil, &Error{Op: "shard", Kind: ErrShardNotFound, Model: spec.ModelName, Field: spec.ShardGroup}
	}
	rows := []Map{}
	for _, connection := range config.Connections {
		nextSpec := cloneQuerySpec(spec)
		nextSpec.Connection = connection
		nextRows, err := queryRowsPrepared(ctx, db, nextSpec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, nextRows...)
	}
	return rows, nil
}

func (query *ModelQuery[T]) Stream(ctx context.Context) (Stream[*T], error) {
	if query.allShards {
		return nil, &Error{Op: "stream", Kind: ErrUnsupported}
	}
	spec, schema, err := modelQuerySpec(query)
	if err != nil {
		return nil, err
	}
	rows, err := streamQueryPrepared(ctx, query.db, spec)
	if err != nil {
		return nil, err
	}
	return &mappedStream[*T]{
		rows: rows,
		mapFn: func(row Map) (*T, error) {
			model := new(T)
			if err := query.db.runtime.Mapper.MapModel(schema, row, model); err != nil {
				return nil, err
			}
			if err := query.afterFind(ctx, schema, model); err != nil {
				return nil, err
			}
			return model, nil
		},
	}, nil
}

func (query *ModelQuery[T]) Chunk(ctx context.Context, size int, fn func([]*T) error) error {
	if query.allShards {
		return &Error{Op: "chunk", Kind: ErrUnsupported}
	}
	if fn == nil {
		return &Error{Op: "chunk", Kind: ErrInvalidArgument}
	}
	if err := chunkSpecError(query.spec); err != nil {
		return err
	}
	spec, schema, err := modelQuerySpec(query)
	if err != nil {
		return err
	}
	if len(spec.Order) == 0 {
		for _, fieldName := range schema.Primary {
			field := schema.FieldByGo[fieldName]
			spec.Order = append(spec.Order, OrderExpr{Expr: field.Column})
		}
	}
	spec.Limit = &size
	return chunkMaps(ctx, spec, func(chunkSpec QuerySpec) ([]Map, error) {
		return queryRowsPrepared(ctx, query.db, chunkSpec)
	}, func(rows []Map) error {
		models := make([]*T, 0, len(rows))
		for _, row := range rows {
			model := new(T)
			if err := query.db.runtime.Mapper.MapModel(schema, row, model); err != nil {
				return err
			}
			if err := query.afterFind(ctx, schema, model); err != nil {
				return err
			}
			models = append(models, model)
		}
		return fn(models)
	})
}

func (query *ModelQuery[T]) Each(ctx context.Context, fn func(*T) error) error {
	if fn == nil {
		return &Error{Op: "each", Kind: ErrInvalidArgument}
	}
	return query.Chunk(ctx, eachSize(query.db.runtime.Config), func(models []*T) error {
		for _, model := range models {
			if err := fn(model); err != nil {
				return err
			}
		}
		return nil
	})
}

func (query *ModelQuery[T]) Paginate(size int) *Paginator[*T] {
	specErr := paginateSpecError(query.spec)
	return &Paginator[*T]{
		size: size,
		err:  specErr,
		count: func(ctx context.Context) (int64, error) {
			return query.Count(ctx)
		},
		items: func(ctx context.Context, limit int, offset int) ([]*T, error) {
			return query.Limit(limit).Offset(offset).Get(ctx)
		},
	}
}

func (query *ModelQuery[T]) Create(ctx context.Context, model *T, options ...WriteOption) (*T, error) {
	if query.allShards {
		return nil, &Error{Op: "create", Kind: ErrShardRequired}
	}
	spec, schema, err := modelInsertSpec(query)
	if err != nil {
		return nil, err
	}
	return query.createInTransaction(ctx, spec, schema, model, options...)
}

func (query *ModelQuery[T]) Upsert(ctx context.Context, model *T, options ...WriteOption) (*T, error) {
	if query.allShards {
		return nil, &Error{Op: "upsert", Kind: ErrShardRequired}
	}
	spec, schema, err := modelQuerySpec(query)
	if err != nil {
		return nil, err
	}
	writeOptions := applyWriteOptions(options)
	if writeOptions.conflict == nil {
		return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Model: schema.Name, Table: schema.Table}
	}
	row, err := buildModelInsertMap(schema, model, writeOptions)
	if err != nil {
		return nil, err
	}
	writeDB := withSpecConnection(query.db, spec)
	if err := applyTenantColumns(writeDB, schema, row); err != nil {
		return nil, err
	}
	if err := validateShardWriteValuesForDB(writeDB, schema, query.shard, row); err != nil {
		return nil, err
	}
	conflict, err := convertModelConflict(schema, writeOptions.conflict)
	if err != nil {
		return nil, err
	}
	rows, err := upsertRows(ctx, writeDB, WriteSpec{
		QuerySpec: spec,
		Values:    []Map{row},
		Primary:   primaryColumnsForSchema(schema),
		Conflict:  *conflict,
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &Error{Op: "upsert", Kind: ErrScan, Model: schema.Name, Table: schema.Table}
	}
	if err := query.db.runtime.Mapper.MapModel(schema, rows[0], model); err != nil {
		return nil, err
	}
	return model, nil
}

func (query *ModelQuery[T]) Update(ctx context.Context, values Map, options ...WriteOption) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "update", Kind: ErrShardRequired}
	}
	if len(query.spec.Where) == 0 {
		return 0, &Error{Op: "update", Kind: ErrUnsafeUpdate}
	}
	spec, schema, err := modelWriteSpec(query)
	if err != nil {
		return 0, err
	}
	writeOptions := applyWriteOptions(options)
	hookValues := copyMap(values)
	if err := validateShardUpdateValues(schema, hookValues); err != nil {
		return 0, err
	}
	return query.updateInTransaction(ctx, spec, schema, hookValues, writeOptions)
}

func (query *ModelQuery[T]) Delete(ctx context.Context) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "delete", Kind: ErrShardRequired}
	}
	if len(query.spec.Where) == 0 {
		return 0, &Error{Op: "delete", Kind: ErrUnsafeDelete}
	}
	spec, schema, err := modelWriteSpec(query)
	if err != nil {
		return 0, err
	}
	if field, ok := softDeleteField(schema); ok {
		values := Map{field.Name: time.Now()}
		return query.deleteInTransaction(ctx, spec, schema, values, true)
	}
	return query.deleteInTransaction(ctx, spec, schema, nil, false)
}

func (query *ModelQuery[T]) ForceDelete(ctx context.Context) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "delete", Kind: ErrShardRequired}
	}
	if len(query.spec.Where) == 0 {
		return 0, &Error{Op: "delete", Kind: ErrUnsafeDelete}
	}
	spec, schema, err := modelWriteSpec(query)
	if err != nil {
		return 0, err
	}
	return query.deleteInTransaction(ctx, spec, schema, nil, false)
}

func (query *ModelQuery[T]) Restore(ctx context.Context) (int64, error) {
	if query.allShards {
		return 0, &Error{Op: "restore", Kind: ErrShardRequired}
	}
	if len(query.spec.Where) == 0 {
		return 0, &Error{Op: "restore", Kind: ErrUnsafeUpdate}
	}
	spec, schema, err := modelWriteSpec(query)
	if err != nil {
		return 0, err
	}
	field, ok := softDeleteField(schema)
	if !ok {
		return 0, &Error{Op: "restore", Kind: ErrInvalidArgument, Model: schema.Name}
	}

	spec.Where = withoutSoftDeleteConditions(spec.Where, field.Column)
	spec.Where = append(spec.Where, isNotNullCondition(field.Column))
	values := Map{field.Name: nil}
	return query.restoreInTransaction(ctx, spec, schema, values)
}

func (query *ModelQuery[T]) CreateMany(ctx context.Context, models []*T, options ...WriteOption) ([]*T, error) {
	if query.allShards {
		return nil, &Error{Op: "create", Kind: ErrShardRequired}
	}
	if len(models) == 0 {
		return []*T{}, nil
	}

	spec, _, err := modelInsertSpec(query)
	if err != nil {
		return nil, err
	}
	createdModels := make([]*T, 0, len(models))
	err = withSpecConnection(query.db, spec).Transaction(ctx, func(tx *DB) error {
		txQuery := *query
		txQuery.db = tx
		for _, model := range models {
			if model == nil {
				return &Error{Op: "create", Kind: ErrInvalidArgument}
			}
			created, err := txQuery.create(ctx, model, options...)
			if err != nil {
				return err
			}
			createdModels = append(createdModels, created)
		}
		return nil
	})
	return createdModels, err
}

func (query *ModelQuery[T]) UpsertMany(ctx context.Context, models []*T, options ...WriteOption) ([]*T, error) {
	if len(models) == 0 {
		return []*T{}, nil
	}

	upsertedModels := make([]*T, 0, len(models))
	for _, model := range models {
		if model == nil {
			return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument}
		}
		upserted, err := query.Upsert(ctx, model, options...)
		if err != nil {
			return nil, err
		}
		upsertedModels = append(upsertedModels, upserted)
	}
	return upsertedModels, nil
}

func (query *ModelQuery[T]) Find(ctx context.Context, id any) (*T, error) {
	spec, schema, err := modelQuerySpec(query)
	if err != nil {
		return nil, err
	}
	if len(schema.Primary) != 1 {
		return nil, &Error{Op: "find", Kind: ErrInvalidArgument, Model: schema.Name}
	}
	primaryField := schema.FieldByGo[schema.Primary[0]]
	spec.Where = append(spec.Where, Condition{Field: primaryField.Column, Op: "=", Value: id})

	row, err := queryFirstRowPrepared(ctx, query.db, spec)
	if err != nil || row == nil {
		return nil, err
	}
	model := new(T)
	if err := query.db.runtime.Mapper.MapModel(schema, row, model); err != nil {
		return nil, err
	}
	if err := query.afterFind(ctx, schema, model); err != nil {
		return nil, err
	}
	return model, nil
}

func (query *ModelQuery[T]) Count(ctx context.Context) (int64, error) {
	spec, _, err := modelQuerySpec(query)
	if err != nil {
		return 0, err
	}
	if query.allShards {
		spec.Select = []SelectExpr{{Expr: "count(*)", Alias: "total", Raw: true}}
		spec.Order = nil
		spec.Limit = nil
		spec.Offset = nil
		rows, err := queryModelAllShardRows(ctx, query.db, spec)
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

func (query *ModelQuery[T]) Exists(ctx context.Context) (bool, error) {
	spec, _, err := modelQuerySpec(query)
	if err != nil {
		return false, err
	}
	if query.allShards {
		spec.Select = []SelectExpr{{Expr: "1", Raw: true}}
		spec.Order = nil
		limit := 1
		spec.Limit = &limit
		spec.Offset = nil
		rows, err := queryModelAllShardRows(ctx, query.db, spec)
		if err != nil {
			return false, err
		}
		return len(rows) > 0, nil
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

func (query *ModelQuery[T]) Sum(ctx context.Context, field string) (Decimal, error) {
	spec, schema, err := aggregateModelSpec(query, field)
	if err != nil {
		return Decimal("0"), err
	}
	return aggregateDecimal(ctx, query.db, spec, "sum", schema.FieldByGo[field].Column)
}

func (query *ModelQuery[T]) Avg(ctx context.Context, field string) (Decimal, error) {
	spec, schema, err := aggregateModelSpec(query, field)
	if err != nil {
		return Decimal("0"), err
	}
	return aggregateDecimal(ctx, query.db, spec, "avg", schema.FieldByGo[field].Column)
}

func (query *ModelQuery[T]) Min[V any](ctx context.Context, field string) (Null[V], error) {
	spec, schema, err := aggregateModelSpec(query, field)
	if err != nil {
		return NullZero[V](), err
	}
	return aggregateNull[V](ctx, query.db, spec, "min", schema.FieldByGo[field].Column)
}

func (query *ModelQuery[T]) Max[V any](ctx context.Context, field string) (Null[V], error) {
	spec, schema, err := aggregateModelSpec(query, field)
	if err != nil {
		return NullZero[V](), err
	}
	return aggregateNull[V](ctx, query.db, spec, "max", schema.FieldByGo[field].Column)
}

func (query *ModelQuery[T]) createInTransaction(ctx context.Context, spec QuerySpec, schema *ModelSchema, model *T, options ...WriteOption) (*T, error) {
	var created *T
	err := query.runModelWrite(ctx, spec, func(txQuery *ModelQuery[T]) error {
		var err error
		created, err = txQuery.createWithSpec(ctx, spec, schema, model, options...)
		return err
	})
	return created, err
}

func (query *ModelQuery[T]) create(ctx context.Context, model *T, options ...WriteOption) (*T, error) {
	spec, schema, err := modelInsertSpec(query)
	if err != nil {
		return nil, err
	}
	return query.createWithSpec(ctx, spec, schema, model, options...)
}

func (query *ModelQuery[T]) createWithSpec(ctx context.Context, spec QuerySpec, schema *ModelSchema, model *T, options ...WriteOption) (*T, error) {
	if model == nil {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument}
	}
	emitEvents := shouldEmitEvent(query.db, query.skipEvents, BeforeCreate, AfterCreate)
	useHooks := !query.skipHooks && hasCreateHooks(model)
	var hook *Hook
	var event *Event
	if emitEvents {
		event = modelEvent(query.db, schema, model, "create")
		if err := emitEvent(ctx, query.db, event.withName(BeforeCreate)); err != nil {
			return nil, err
		}
	}
	if useHooks {
		hook = &Hook{DB: query.db, Operation: "create"}
		if err := callBeforeCreate(ctx, model, hook); err != nil {
			return nil, err
		}
	}
	writeOptions := applyWriteOptions(options)
	row, err := buildModelInsertMap(schema, model, writeOptions)
	if err != nil {
		return nil, err
	}
	if err := applyTenantColumns(query.db, schema, row); err != nil {
		return nil, err
	}
	if err := validateShardWriteValuesForDB(query.db, schema, query.shard, row); err != nil {
		return nil, err
	}
	if useHooks {
		hook.Values = row
	}
	if emitEvents {
		event.Values = row
	}
	rows, err := createRows(ctx, query.db, WriteSpec{
		QuerySpec: spec,
		Values:    []Map{row},
		Primary:   primaryColumnsForSchema(schema),
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &Error{Op: "create", Kind: ErrScan, Model: schema.Name, Table: schema.Table}
	}
	if err := query.db.runtime.Mapper.MapModel(schema, rows[0], model); err != nil {
		return nil, err
	}
	if useHooks {
		if err := callAfterCreate(ctx, model, hook); err != nil {
			return nil, err
		}
	}
	if emitEvents {
		event.Model = model
		if err := emitEvent(ctx, query.db, event.withName(AfterCreate)); err != nil {
			return nil, err
		}
	}
	return model, nil
}

func (query *ModelQuery[T]) updateInTransaction(ctx context.Context, spec QuerySpec, schema *ModelSchema, values Map, options writeOptions) (int64, error) {
	var affected int64
	err := query.runModelWrite(ctx, spec, func(txQuery *ModelQuery[T]) error {
		tx := txQuery.db
		hookModel := new(T)
		hook := &Hook{DB: tx, Operation: "update", Values: values}
		emitEvents := shouldEmitEvent(tx, txQuery.skipEvents, BeforeUpdate, AfterUpdate)
		var event *Event
		if emitEvents {
			event = modelEvent(tx, schema, hookModel, "update")
			event.Values = values
			if err := emitEvent(ctx, tx, event.withName(BeforeUpdate)); err != nil {
				return err
			}
		}
		if !txQuery.skipHooks {
			if err := callBeforeUpdate(ctx, hookModel, hook); err != nil {
				return err
			}
		}
		values = hook.Values
		if emitEvents {
			event.Values = values
		}
		converted, err := convertModelMap(schema, values, options, true)
		if err != nil {
			return err
		}
		if err := applyOptimisticLock(schema, &spec, converted, options); err != nil {
			return err
		}
		for column, value := range autoUpdateColumns(schema, options) {
			if _, ok := converted[column]; !ok {
				converted[column] = value
			}
		}
		result, err := updateRows(ctx, tx, WriteSpec{
			QuerySpec: spec,
			Values:    []Map{converted},
		})
		if err != nil {
			return err
		}
		if options.version != nil && result == 0 {
			field, _ := optimisticLockField(schema)
			return &Error{Op: "update", Kind: ErrStaleData, Model: schema.Name, Table: schema.Table, Field: field.Name}
		}
		affected = result
		hook.RowsAffected = result
		if emitEvents {
			event.RowsAffected = result
		}
		if !txQuery.skipHooks {
			if err := callAfterUpdate(ctx, hookModel, hook); err != nil {
				return err
			}
		}
		if emitEvents {
			if err := emitEvent(ctx, tx, event.withName(AfterUpdate)); err != nil {
				return err
			}
		}
		return nil
	})
	return affected, err
}

func (query *ModelQuery[T]) deleteInTransaction(ctx context.Context, spec QuerySpec, schema *ModelSchema, values Map, softDelete bool) (int64, error) {
	var affected int64
	err := query.runModelWrite(ctx, spec, func(txQuery *ModelQuery[T]) error {
		tx := txQuery.db
		hookModel := new(T)
		hook := &Hook{DB: tx, Operation: "delete", Values: values, SoftDelete: softDelete}
		emitEvents := shouldEmitEvent(tx, txQuery.skipEvents, BeforeDelete, AfterDelete)
		var event *Event
		if emitEvents {
			event = modelEvent(tx, schema, hookModel, "delete")
			event.Values = values
			event.SoftDelete = softDelete
			if err := emitEvent(ctx, tx, event.withName(BeforeDelete)); err != nil {
				return err
			}
		}
		if !txQuery.skipHooks {
			if err := callBeforeDelete(ctx, hookModel, hook); err != nil {
				return err
			}
		}
		var result int64
		var err error
		if softDelete {
			values = hook.Values
			if emitEvents {
				event.Values = values
			}
			converted, convertErr := convertModelMap(schema, values, writeOptions{}, true)
			if convertErr != nil {
				return convertErr
			}
			for column, value := range autoUpdateColumns(schema, writeOptions{}) {
				if _, ok := converted[column]; !ok {
					converted[column] = value
				}
			}
			result, err = updateRows(ctx, tx, WriteSpec{QuerySpec: spec, Values: []Map{converted}})
		} else {
			result, err = deleteRows(ctx, tx, WriteSpec{QuerySpec: spec})
		}
		if err != nil {
			return err
		}
		affected = result
		hook.RowsAffected = result
		if emitEvents {
			event.RowsAffected = result
		}
		if !txQuery.skipHooks {
			if err := callAfterDelete(ctx, hookModel, hook); err != nil {
				return err
			}
		}
		if emitEvents {
			if err := emitEvent(ctx, tx, event.withName(AfterDelete)); err != nil {
				return err
			}
		}
		return nil
	})
	return affected, err
}

func (query *ModelQuery[T]) restoreInTransaction(ctx context.Context, spec QuerySpec, schema *ModelSchema, values Map) (int64, error) {
	var affected int64
	err := query.runModelWrite(ctx, spec, func(txQuery *ModelQuery[T]) error {
		tx := txQuery.db
		hookModel := new(T)
		hook := &Hook{DB: tx, Operation: "restore", Values: values}
		emitEvents := shouldEmitEvent(tx, txQuery.skipEvents, BeforeRestore, AfterRestore)
		var event *Event
		if emitEvents {
			event = modelEvent(tx, schema, hookModel, "restore")
			event.Values = values
			if err := emitEvent(ctx, tx, event.withName(BeforeRestore)); err != nil {
				return err
			}
		}
		if !txQuery.skipHooks {
			if err := callBeforeRestore(ctx, hookModel, hook); err != nil {
				return err
			}
		}
		values = hook.Values
		if emitEvents {
			event.Values = values
		}
		converted, err := convertModelMap(schema, values, writeOptions{}, true)
		if err != nil {
			return err
		}
		for column, value := range autoUpdateColumns(schema, writeOptions{}) {
			if _, ok := converted[column]; !ok {
				converted[column] = value
			}
		}
		result, err := updateRows(ctx, tx, WriteSpec{QuerySpec: spec, Values: []Map{converted}})
		if err != nil {
			return err
		}
		affected = result
		hook.RowsAffected = result
		if emitEvents {
			event.RowsAffected = result
		}
		if !txQuery.skipHooks {
			if err := callAfterRestore(ctx, hookModel, hook); err != nil {
				return err
			}
		}
		if emitEvents {
			if err := emitEvent(ctx, tx, event.withName(AfterRestore)); err != nil {
				return err
			}
		}
		return nil
	})
	return affected, err
}

func (query *ModelQuery[T]) afterFind(ctx context.Context, schema *ModelSchema, model *T) error {
	if !query.skipHooks {
		if _, ok := any(model).(afterFindHook); ok {
			hook := &Hook{DB: query.db, Operation: "find"}
			if err := callAfterFind(ctx, model, hook); err != nil {
				return err
			}
		}
	}
	if !query.skipEvents && hasEventHandlers(query.db, AfterFind) {
		event := modelEvent(query.db, schema, model, "find").withName(AfterFind)
		if err := emitEvent(ctx, query.db, event); err != nil {
			return err
		}
	}
	return nil
}

func (query *ModelQuery[T]) runModelWrite(ctx context.Context, spec QuerySpec, fn func(*ModelQuery[T]) error) error {
	writeDB := withSpecConnection(query.db, spec)
	if writeDB == nil || writeDB.runtime == nil {
		return &Error{Op: "write", Kind: ErrInvalidArgument}
	}
	if writeDB.session.tx != nil || !writeDB.runtime.Config.SkipDefaultTransaction {
		return writeDB.Transaction(ctx, func(tx *DB) error {
			txQuery := *query
			txQuery.db = tx
			return fn(&txQuery)
		})
	}
	writeQuery := *query
	writeQuery.db = writeDB
	return fn(&writeQuery)
}

func modelEvent(db *DB, schema *ModelSchema, model any, operation string) *Event {
	return &Event{
		DB:        db,
		ModelName: schema.Name,
		Table:     schema.Table,
		Model:     model,
		Operation: operation,
	}
}

func (event *Event) withName(name EventName) *Event {
	cloned := *event
	cloned.Name = name
	return &cloned
}

func copyMap(values Map) Map {
	if values == nil {
		return nil
	}
	copied := Map{}
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func withSpecConnection(db *DB, spec QuerySpec) *DB {
	if db == nil || spec.Connection == "" || db.session.connection == spec.Connection {
		return db
	}
	clone := *db
	clone.session.connection = spec.Connection
	return &clone
}

func withoutSoftDeleteConditions(conditions []Condition, column string) []Condition {
	filtered := make([]Condition, 0, len(conditions))
	for _, condition := range conditions {
		op := strings.ToLower(strings.TrimSpace(condition.Op))
		if condition.Field == column && (op == "is null" || op == "is not null") {
			continue
		}
		filtered = append(filtered, condition)
	}
	return filtered
}
