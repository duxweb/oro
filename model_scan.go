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

func structRowsDirectAvailable(db *DB, spec QuerySpec) bool {
	if db == nil || db.runtime == nil || cacheEnabled(db, spec) {
		return false
	}
	return usesDefaultExecutor(db) && usesDefaultMapper(db)
}

func usesDefaultMapper(db *DB) bool {
	if db == nil || db.runtime == nil {
		return false
	}
	switch db.runtime.Mapper.(type) {
	case reflectMapper:
		return true
	default:
		return false
	}
}

func queryModelRowsDirect[T any](ctx context.Context, db *DB, spec QuerySpec, schema *ModelSchema) ([]*T, error) {
	return queryStructRowsDirect[T](ctx, db, spec, schema)
}

func queryStructRowsDirect[T any](ctx context.Context, db *DB, spec QuerySpec, schema *ModelSchema) ([]*T, error) {
	rows, err := openModelRowsPrepared(ctx, db, spec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	destType, err := structTypeOfGeneric[T]()
	if err != nil {
		return nil, err
	}
	mappers, values, dests, err := modelScanPlan(rows, schema, destType)
	if err != nil {
		return nil, err
	}

	models := make([]*T, 0, resultCapacity(spec.Limit))
	for rows.Next() {
		model := new(T)
		if err := scanStructRow(rows, model, schema, mappers, values, dests); err != nil {
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
	return queryStructFirstDirect[T](ctx, db, spec, schema)
}

func queryStructFirstDirect[T any](ctx context.Context, db *DB, spec QuerySpec, schema *ModelSchema) (*T, error) {
	limit := 1
	spec.Limit = &limit
	models, err := queryStructRowsDirect[T](ctx, db, spec, schema)
	if err != nil || len(models) == 0 {
		return nil, err
	}
	return models[0], nil
}

func createModelsDirect[T any](ctx context.Context, db *DB, conn *Connection, spec WriteSpec, schema *ModelSchema, compiled CompiledSQL, models []*T) error {
	rows, err := openCompiledRowsWithExec(ctx, db, execForQueryRuntime(db, conn), spec.QuerySpec, compiled, "create", conn)
	if err != nil {
		return err
	}
	defer rows.Close()

	destType, err := structTypeOfGeneric[T]()
	if err != nil {
		return err
	}
	mappers, values, dests, err := modelScanPlan(rows, schema, destType)
	if err != nil {
		return err
	}
	index := 0
	for rows.Next() {
		if index >= len(models) {
			return &Error{Op: "create", Kind: ErrScan, Model: spec.ModelName, Table: spec.Table}
		}
		if err := scanStructRow(rows, models[index], schema, mappers, values, dests); err != nil {
			return err
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return &Error{Op: "create", Kind: err, Cause: err}
	}
	if index != len(models) {
		return &Error{Op: "create", Kind: ErrScan, Model: spec.ModelName, Table: spec.Table}
	}
	return nil
}

func queryRawStructRowsDirect[T any](ctx context.Context, db *DB, raw RawSpec, timeout time.Duration, schema *ModelSchema) ([]*T, error) {
	rows, err := openRawRowsDirect(ctx, db, raw, timeout)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	destType, err := structTypeOfGeneric[T]()
	if err != nil {
		return nil, err
	}
	mappers, values, dests, err := modelScanPlan(rows, schema, destType)
	if err != nil {
		return nil, err
	}

	models := make([]*T, 0)
	for rows.Next() {
		model := new(T)
		if err := scanStructRow(rows, model, schema, mappers, values, dests); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	if err := rows.Err(); err != nil {
		return nil, &Error{Op: "query", Kind: err, Cause: err}
	}
	return models, nil
}

func queryRawStructFirstDirect[T any](ctx context.Context, db *DB, raw RawSpec, timeout time.Duration, schema *ModelSchema) (*T, error) {
	models, err := queryRawStructRowsDirect[T](ctx, db, raw, timeout, schema)
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
	return openCompiledRows(ctx, db, conn, spec, compiled, "select")
}

func openRawRowsDirect(ctx context.Context, db *DB, raw RawSpec, timeout time.Duration) (*modelRows, error) {
	conn, err := connectionForQuery(db, db.session.connection)
	if err != nil {
		return nil, err
	}
	spec := QuerySpec{
		Connection: db.session.connection,
		Timeout:    int64(timeout),
	}
	return openCompiledRows(ctx, db, conn, spec, CompiledSQL{SQL: raw.SQL, Args: raw.Args}, "raw")
}

func openCompiledRows(ctx context.Context, db *DB, conn *Connection, spec QuerySpec, compiled CompiledSQL, operation string) (*modelRows, error) {
	return openCompiledRowsWithExec(ctx, db, execForReadRuntime(db, conn, spec), spec, compiled, operation, conn)
}

func openCompiledRowsWithExec(ctx context.Context, db *DB, exec ExecContext, spec QuerySpec, compiled CompiledSQL, operation string, conn *Connection) (*modelRows, error) {
	ctx, cancel := withOperationTimeout(ctx, queryTimeout(db, spec))
	querier, ok := exec.(sqlQuerier)
	if !ok {
		cancel()
		return nil, &Error{Op: operation, Kind: ErrInvalidArgument}
	}
	if err := emitSQLEvent(ctx, db, spec, BeforeSQL, compiled, operation, 0, 0, nil); err != nil {
		cancel()
		return nil, err
	}
	startedAt := time.Now()
	rows, err := querier.QueryContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		cancel()
		queryErr := wrapContextError(operation, err)
		if conn != nil {
			queryErr = translateQueryError(conn, queryErr)
		}
		_ = emitSQLEvent(ctx, db, spec, AfterSQL, compiled, operation, 0, time.Since(startedAt), queryErr)
		return nil, queryErr
	}
	return &modelRows{Rows: rows, ctx: ctx, cancel: cancel, db: db, spec: spec, compiled: compiled, operation: operation, startedAt: startedAt}, nil
}

type modelRows struct {
	*sql.Rows
	ctx       context.Context
	cancel    context.CancelFunc
	db        *DB
	spec      QuerySpec
	compiled  CompiledSQL
	operation string
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
	_ = emitSQLEvent(rows.ctx, rows.db, rows.spec, AfterSQL, rows.compiled, rows.operation, rows.count, time.Since(rows.startedAt), err)
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

func modelScanPlan(rows *modelRows, schema *ModelSchema, destType reflect.Type) ([]modelColumnMapper, []any, []any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, nil, &Error{Op: "scan", Kind: ErrScan, Cause: err}
	}
	if key, ok := modelScanPlanKey(schema, destType, rows.compiled.SQL, columns); ok {
		if plan, ok := rows.db.runtime.ScanCache.get(key); ok {
			values, dests := modelScanBuffers(plan.columns)
			return plan.mappers, values, dests, nil
		}
		mappers, values, dests, err := buildModelScanPlan(rows, schema, destType, columns)
		if err != nil {
			return nil, nil, nil, err
		}
		rows.db.runtime.ScanCache.set(key, cachedModelScanPlan{mappers: mappers, columns: len(columns)})
		return mappers, values, dests, nil
	}
	return buildModelScanPlan(rows, schema, destType, columns)
}

func buildModelScanPlan(rows *modelRows, schema *ModelSchema, destType reflect.Type, columns []string) ([]modelColumnMapper, []any, []any, error) {
	columnTypes, _ := rows.ColumnTypes()
	mappers := make([]modelColumnMapper, len(columns))
	dtoFields := map[string]reflect.StructField(nil)
	if schema == nil {
		dtoFields = structFieldsByColumn(destType)
	}
	for index, column := range columns {
		applyColumnMapper(&mappers[index], schema, dtoFields, column)
		if index < len(columnTypes) && columnTypes[index] != nil {
			mappers[index].dbType = strings.ToLower(columnTypes[index].DatabaseTypeName())
		}
	}
	values, dests := modelScanBuffers(len(columns))
	return mappers, values, dests, nil
}

func applyColumnMapper(mapper *modelColumnMapper, schema *ModelSchema, dtoFields map[string]reflect.StructField, column string) {
	if schema != nil {
		field, ok := schema.FieldByDB[column]
		if ok && len(field.Index) > 0 {
			mapper.fieldIndex = field.Index
			mapper.fieldName = field.Name
			if fieldType := fieldTypeByIndex(schema, field.Index); fieldType != nil {
				mapper.fieldType = fieldType
				mapper.fieldKind = fieldType.Kind()
			}
		}
		return
	}

	field, ok := dtoFields[column]
	if !ok || len(field.Index) == 0 {
		return
	}
	mapper.fieldIndex = field.Index
	mapper.fieldName = field.Name
	mapper.fieldType = field.Type
	mapper.fieldKind = field.Type.Kind()
}

func structFieldsByColumn(destType reflect.Type) map[string]reflect.StructField {
	fields := map[string]reflect.StructField{}
	if destType == nil {
		return fields
	}
	for _, field := range reflect.VisibleFields(destType) {
		if !field.IsExported() {
			continue
		}
		fields[Snake(field.Name)] = field
	}
	return fields
}

func modelScanBuffers(size int) ([]any, []any) {
	values := make([]any, size)
	dests := make([]any, size)
	for index := range values {
		dests[index] = &values[index]
	}
	return values, dests
}

func modelScanPlanKey(schema *ModelSchema, destType reflect.Type, sql string, columns []string) (string, bool) {
	if sql == "" || len(columns) == 0 {
		return "", false
	}
	builder := strings.Builder{}
	if schema != nil {
		builder.WriteString("schema:")
		builder.WriteString(schema.Name)
		builder.WriteByte('|')
		builder.WriteString(schema.Table)
	} else if destType != nil {
		builder.WriteString("dto:")
		builder.WriteString(destType.PkgPath())
		builder.WriteByte('|')
		builder.WriteString(destType.String())
	} else {
		return "", false
	}
	builder.WriteByte('|')
	builder.WriteString(sql)
	for _, column := range columns {
		builder.WriteByte('|')
		builder.WriteString(column)
	}
	return builder.String(), true
}

func scanModelRow[T any](rows *modelRows, model *T, schema *ModelSchema, mappers []modelColumnMapper, values []any, dests []any) error {
	return scanStructRow(rows, model, schema, mappers, values, dests)
}

func scanStructRow(rows *modelRows, dest any, schema *ModelSchema, mappers []modelColumnMapper, values []any, dests []any) error {
	if err := rows.Scan(dests...); err != nil {
		return &Error{Op: "scan", Kind: ErrScan, Cause: err}
	}
	destValue := reflect.ValueOf(dest)
	if !destValue.IsValid() || destValue.Kind() != reflect.Pointer || destValue.IsNil() {
		return &Error{Op: "map", Kind: ErrInvalidArgument}
	}
	structValue := destValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return &Error{Op: "map", Kind: ErrInvalidArgument}
	}
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
	return nil
}

func structTypeOfGeneric[T any]() (reflect.Type, error) {
	destType := reflect.TypeOf((*T)(nil)).Elem()
	if destType.Kind() != reflect.Struct {
		return nil, &Error{Op: "map", Kind: ErrInvalidArgument}
	}
	return destType, nil
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
			setTimeValue(dest, typedValue)
			return nil
		case string:
			if parsed, ok := parseTimeString(typedValue); ok {
				setTimeValue(dest, parsed)
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
