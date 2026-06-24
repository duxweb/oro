package oro

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"time"

	"github.com/duxweb/oro/internal/fifocache"
)

type modelColumnMapper struct {
	fieldIndex []int
	fieldName  string
	dbType     string
	fieldType  reflect.Type
	fieldKind  reflect.Kind
}

type cachedModelScanPlan struct {
	mappers []modelColumnMapper
	columns int
}

type modelScanCache struct {
	items *fifocache.Cache[string, cachedModelScanPlan]
}

func newModelScanCache(maxSize int) *modelScanCache {
	if maxSize <= 0 {
		return nil
	}
	return &modelScanCache{items: fifocache.New[string, cachedModelScanPlan](maxSize, nil)}
}

func (cache *modelScanCache) get(key string) (cachedModelScanPlan, bool) {
	if cache == nil || cache.items == nil || key == "" {
		return cachedModelScanPlan{}, false
	}
	return cache.items.Get(key)
}

func (cache *modelScanCache) set(key string, plan cachedModelScanPlan) {
	if cache == nil || cache.items == nil || key == "" {
		return
	}
	cache.items.Set(key, plan)
}

func modelRowsDirectAvailable(db *DB, spec QuerySpec) bool {
	if cacheEnabled(db, spec) {
		return false
	}
	_, ok := db.runtime.Executor.(sqlExecutor)
	return ok
}

func queryModelRowsDirect[T any](ctx context.Context, db *DB, spec QuerySpec, schema *ModelSchema) ([]*T, error) {
	rows, err := openModelRowsPrepared(ctx, db, spec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mappers, values, dests, err := modelScanPlan(rows, schema)
	if err != nil {
		return nil, err
	}

	models := make([]*T, 0)
	for rows.Next() {
		model := new(T)
		if err := scanModelRow(rows, model, mappers, values, dests); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, &Error{Op: "query", Kind: err, Cause: err}
	}
	return models, nil
}

func queryModelFirstDirect[T any](ctx context.Context, db *DB, spec QuerySpec, schema *ModelSchema) (*T, error) {
	limit := 1
	spec.Limit = &limit
	models, err := queryModelRowsDirect[T](ctx, db, spec, schema)
	if err != nil || len(models) == 0 {
		return nil, err
	}
	return models[0], nil
}

func openModelRows(ctx context.Context, db *DB, spec QuerySpec) (*modelRows, error) {
	spec = cloneQuerySpec(spec)
	return openModelRowsPrepared(ctx, db, spec)
}

func openModelRowsPrepared(ctx context.Context, db *DB, spec QuerySpec) (*modelRows, error) {
	if spec.SelectErr != nil {
		return nil, spec.SelectErr
	}
	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return nil, err
	}
	if err := validateQueryLock(db, conn, spec.Lock); err != nil {
		return nil, err
	}
	if err := validateQueryJoins(conn, spec.Joins); err != nil {
		return nil, err
	}
	if err := resolveQuerySources(db, &spec); err != nil {
		return nil, err
	}
	tableNames(db).ApplyQuery(&spec)

	compiled, err := compileSelectSQL(db, conn, spec)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withOperationTimeout(ctx, queryTimeout(db, spec))
	querier, ok := execForReadRuntime(db, conn, spec).(sqlQuerier)
	if !ok {
		cancel()
		return nil, &Error{Op: "query", Kind: ErrInvalidArgument}
	}
	if err := emitSQLEvent(ctx, db, spec, BeforeSQL, compiled, "query", 0, 0, nil); err != nil {
		cancel()
		return nil, err
	}
	startedAt := time.Now()
	rows, err := querier.QueryContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		cancel()
		queryErr := translateQueryError(conn, wrapContextError("query", err))
		_ = emitSQLEvent(ctx, db, spec, AfterSQL, compiled, "query", 0, time.Since(startedAt), queryErr)
		return nil, queryErr
	}
	return &modelRows{Rows: rows, ctx: ctx, cancel: cancel, db: db, spec: spec, compiled: compiled, startedAt: startedAt}, nil
}

type modelRows struct {
	*sql.Rows
	ctx       context.Context
	cancel    context.CancelFunc
	db        *DB
	spec      QuerySpec
	compiled  CompiledSQL
	startedAt time.Time
	count     int64
	closed    bool
}

func (rows *modelRows) Close() error {
	if rows.closed {
		return nil
	}
	rows.closed = true
	err := rows.Rows.Close()
	_ = emitSQLEvent(rows.ctx, rows.db, rows.spec, AfterSQL, rows.compiled, "query", rows.count, time.Since(rows.startedAt), err)
	rows.cancel()
	return err
}

func (rows *modelRows) Next() bool {
	ok := rows.Rows.Next()
	if ok {
		rows.count++
	}
	return ok
}

func modelScanPlan(rows *modelRows, schema *ModelSchema) ([]modelColumnMapper, []any, []any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, nil, &Error{Op: "scan", Kind: ErrScan, Cause: err}
	}
	if key, ok := modelScanPlanKey(schema, rows.compiled.SQL, columns); ok {
		if plan, ok := rows.db.runtime.ScanCache.get(key); ok {
			values, dests := modelScanBuffers(plan.columns)
			return plan.mappers, values, dests, nil
		}
		mappers, values, dests, err := buildModelScanPlan(rows, schema, columns)
		if err != nil {
			return nil, nil, nil, err
		}
		rows.db.runtime.ScanCache.set(key, cachedModelScanPlan{mappers: mappers, columns: len(columns)})
		return mappers, values, dests, nil
	}
	return buildModelScanPlan(rows, schema, columns)
}

func buildModelScanPlan(rows *modelRows, schema *ModelSchema, columns []string) ([]modelColumnMapper, []any, []any, error) {
	columnTypes, _ := rows.ColumnTypes()
	mappers := make([]modelColumnMapper, len(columns))
	for index, column := range columns {
		field, ok := schema.FieldByDB[column]
		if ok && len(field.Index) > 0 {
			mappers[index].fieldIndex = field.Index
			mappers[index].fieldName = field.Name
			if fieldType := fieldTypeByIndex(schema, field.Index); fieldType != nil {
				mappers[index].fieldType = fieldType
				mappers[index].fieldKind = fieldType.Kind()
			}
		}
		if index < len(columnTypes) && columnTypes[index] != nil {
			mappers[index].dbType = strings.ToLower(columnTypes[index].DatabaseTypeName())
		}
	}
	values, dests := modelScanBuffers(len(columns))
	return mappers, values, dests, nil
}

func modelScanBuffers(size int) ([]any, []any) {
	values := make([]any, size)
	dests := make([]any, size)
	for index := range values {
		dests[index] = &values[index]
	}
	return values, dests
}

func modelScanPlanKey(schema *ModelSchema, sql string, columns []string) (string, bool) {
	if schema == nil || sql == "" || len(columns) == 0 {
		return "", false
	}
	builder := strings.Builder{}
	builder.WriteString(schema.Name)
	builder.WriteByte('|')
	builder.WriteString(schema.Table)
	builder.WriteByte('|')
	builder.WriteString(sql)
	for _, column := range columns {
		builder.WriteByte('|')
		builder.WriteString(column)
	}
	return builder.String(), true
}

func scanModelRow[T any](rows *modelRows, model *T, mappers []modelColumnMapper, values []any, dests []any) error {
	if err := rows.Scan(dests...); err != nil {
		return &Error{Op: "scan", Kind: ErrScan, Cause: err}
	}
	structValue := reflect.ValueOf(model).Elem()
	for index, mapper := range mappers {
		if len(mapper.fieldIndex) == 0 {
			continue
		}
		fieldValue := structValue.FieldByIndex(mapper.fieldIndex)
		if !fieldValue.IsValid() || !fieldValue.CanSet() {
			continue
		}
		if err := assignScannedModelValue(fieldValue, values[index], mapper); err != nil {
			return &Error{Op: "map", Kind: ErrScan, Field: mapper.fieldName, Cause: err}
		}
	}
	ensureModelState(model)
	return nil
}

func fieldTypeByIndex(schema *ModelSchema, index []int) reflect.Type {
	if schema == nil || schema.Type == nil || len(index) == 0 {
		return nil
	}
	fieldType := schema.Type
	for _, fieldIndex := range index {
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}
		if fieldType.Kind() != reflect.Struct || fieldIndex < 0 || fieldIndex >= fieldType.NumField() {
			return nil
		}
		fieldType = fieldType.Field(fieldIndex).Type
	}
	return fieldType
}

func assignScannedModelValue(dest reflect.Value, value any, mapper modelColumnMapper) error {
	if value == nil {
		if dest.Kind() == reflect.Pointer || dest.Kind() == reflect.Interface || dest.Kind() == reflect.Slice || dest.Kind() == reflect.Map {
			dest.Set(reflect.Zero(dest.Type()))
		}
		return nil
	}
	if mapper.fieldType == timeType {
		switch typedValue := value.(type) {
		case time.Time:
			dest.Set(reflect.ValueOf(typedValue))
			return nil
		case string:
			if parsed, ok := parseTimeString(typedValue); ok {
				dest.Set(reflect.ValueOf(parsed))
				return nil
			}
		}
	}
	switch mapper.fieldKind {
	case reflect.String:
		if typedValue, ok := value.(string); ok {
			dest.SetString(typedValue)
			return nil
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		switch typedValue := value.(type) {
		case int64:
			if typedValue >= 0 {
				uintValue := uint64(typedValue)
				if !dest.OverflowUint(uintValue) {
					dest.SetUint(uintValue)
					return nil
				}
			}
		case uint64:
			if !dest.OverflowUint(typedValue) {
				dest.SetUint(typedValue)
				return nil
			}
		case int:
			if typedValue >= 0 {
				uintValue := uint64(typedValue)
				if !dest.OverflowUint(uintValue) {
					dest.SetUint(uintValue)
					return nil
				}
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if typedValue, ok := value.(int64); ok && !dest.OverflowInt(typedValue) {
			dest.SetInt(typedValue)
			return nil
		}
	case reflect.Bool:
		if typedValue, ok := value.(bool); ok {
			dest.SetBool(typedValue)
			return nil
		}
	case reflect.Float32, reflect.Float64:
		if typedValue, ok := value.(float64); ok && !dest.OverflowFloat(typedValue) {
			dest.SetFloat(typedValue)
			return nil
		}
	}
	return assignValue(dest, normalizeScannedValue(value, mapper.dbType))
}
