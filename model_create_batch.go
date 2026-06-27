package oro

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type modelBatchInsert struct {
	fields          []FieldSchema
	columns         []string
	args            []any
	primaryColumn   string
	primaryField    FieldSchema
	explicitPrimary bool
	returning       bool
}

type modelBatchInsertPlan struct {
	fields          []FieldSchema
	columns         []string
	primaryColumn   string
	primaryField    FieldSchema
	explicitPrimary bool
	returning       bool
}

func (query *ModelQuery[T]) createManyBatchFast(ctx context.Context, spec QuerySpec, schema *ModelSchema, models []*T, options writeOptions) (*CreateResult, bool, error) {
	if !canCreateModelsBatchFast(query.db, spec, schema, query.shard, options) {
		return nil, false, nil
	}
	conn, err := connectionForQuery(query.db, spec.Connection)
	if err != nil {
		return nil, false, err
	}
	plan, ok, err := prepareModelBatchInsert(schema, models, conn)
	if err != nil || !ok {
		return nil, ok, err
	}
	var ids any
	rowsAffected := int64(0)
	batchSize := createBatchSize(query.db.runtime.Config, options)
	batchSize = paramCappedBatchSize(len(plan.columns), batchSize, defaultMaxBatchParams)
	for start := 0; start < len(models); start += batchSize {
		end := min(start+batchSize, len(models))
		chunkIDs, affected, err := query.createModelsChunkBatchFast(ctx, spec, schema, conn, plan, models[start:end])
		if err != nil {
			return nil, true, err
		}
		ids = appendPrimaryIDs(ids, chunkIDs)
		rowsAffected += affected
	}
	return createResultFromIDValues(primaryResultKey(schema), ids, rowsAffected), true, nil
}

func (query *ModelQuery[T]) createModelsChunkBatchFast(ctx context.Context, spec QuerySpec, schema *ModelSchema, conn *Connection, plan modelBatchInsertPlan, models []*T) (any, int64, error) {
	insert, err := buildModelBatchInsert(plan, models)
	if err != nil {
		return nil, 0, err
	}

	writeSpec := WriteSpec{
		QuerySpec: spec,
		Primary:   primaryColumnsForSchema(schema),
		Returning: insert.returning,
	}
	tableNames(query.db).ApplyWrite(&writeSpec)

	compiled, err := compileModelBatchInsertSQL(query.db, conn, writeSpec.Table, insert.columns, len(models), insert.returning, insert.args)
	if err != nil {
		return nil, 0, err
	}
	if insert.returning {
		if err := createModelsDirect(ctx, query.db, conn, writeSpec, schema, compiled, models); err != nil {
			return nil, 0, err
		}
		ids, err := primaryValuesFromModels(schema, models)
		return ids, int64(len(models)), err
	}

	result, err := execCompiled(ctx, query.db, execForQueryRuntime(query.db, conn), writeSpec.QuerySpec, compiled, "create")
	if err != nil {
		return nil, 0, translateQueryError(conn, err)
	}
	ids, err := assignModelBatchPrimaryValues(conn, schema, insert, models, result)
	if err != nil {
		return nil, 0, err
	}
	return ids, result.RowsAffected, nil
}

func canCreateModelsBatchFast(db *DB, spec QuerySpec, schema *ModelSchema, shard Map, options writeOptions) bool {
	if db == nil || db.runtime == nil || schema == nil || cacheEnabled(db, spec) {
		return false
	}
	if !usesDefaultExecutor(db) || !usesDefaultMapper(db) || !usesDefaultPlanner(db) {
		return false
	}
	if len(options.only) > 0 || len(options.omit) > 0 || len(schema.PrimaryColumns) != 1 {
		return false
	}
	if len(shard) > 0 || schema.ShardGroup != "" {
		return false
	}
	if hasWriteExtensions(db) {
		return false
	}
	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil || !supportsCreateExecPrimary(conn) {
		return false
	}
	if conn.Dialect.Name() == "pgsql" && !conn.Dialect.Capabilities().Returning {
		return false
	}
	return true
}

func prepareModelBatchInsert[T any](schema *ModelSchema, models []*T, conn *Connection) (modelBatchInsertPlan, bool, error) {
	if len(models) == 0 {
		return modelBatchInsertPlan{}, false, nil
	}
	for _, model := range models {
		if model == nil {
			return modelBatchInsertPlan{}, true, &Error{Op: "create", Kind: ErrInvalidArgument}
		}
	}

	primaryField, ok := schema.FieldByDB[schema.PrimaryColumns[0]]
	if !ok || len(primaryField.Index) == 0 {
		return modelBatchInsertPlan{}, false, nil
	}
	explicitPrimary, mixedPrimary, err := modelBatchPrimaryShape(models, primaryField)
	if err != nil || mixedPrimary {
		return modelBatchInsertPlan{}, false, err
	}

	fields := modelBatchInsertFields(schema, primaryField.Column, explicitPrimary)
	if len(fields) == 0 {
		return modelBatchInsertPlan{}, false, nil
	}

	columns := make([]string, 0, len(fields))
	for _, field := range fields {
		columns = append(columns, field.Column)
	}
	returning := conn.Dialect.Name() == "pgsql" && !explicitPrimary
	return modelBatchInsertPlan{
		fields:          fields,
		columns:         columns,
		primaryColumn:   primaryField.Column,
		primaryField:    primaryField,
		explicitPrimary: explicitPrimary,
		returning:       returning,
	}, true, nil
}

func buildModelBatchInsert[T any](plan modelBatchInsertPlan, models []*T) (modelBatchInsert, error) {
	args := make([]any, 0, len(plan.fields)*len(models))
	now := time.Now()
	for _, model := range models {
		structValue, err := modelStructValue(model)
		if err != nil {
			return modelBatchInsert{}, err
		}
		for _, field := range plan.fields {
			fieldValue, ok := fieldByIndexReadSafe(structValue, field.Index)
			if !ok {
				return modelBatchInsert{}, &Error{Op: "create", Kind: ErrInvalidArgument, Field: field.Name}
			}
			if !fieldValue.IsValid() || !fieldValue.CanInterface() {
				return modelBatchInsert{}, &Error{Op: "create", Kind: ErrInvalidArgument, Field: field.Name}
			}
			if field.AutoCreate || field.AutoUpdate {
				if isZeroValue(fieldValue) {
					if err := assignModelBatchAutoValue(fieldValue, now); err != nil {
						return modelBatchInsert{}, err
					}
				}
			}
			value, err := valueForModelBatchWrite(fieldValue)
			if err != nil {
				return modelBatchInsert{}, &Error{Op: "create", Kind: ErrScan, Field: field.Name, Cause: err}
			}
			args = append(args, value)
		}
	}

	return modelBatchInsert{
		fields:          plan.fields,
		columns:         plan.columns,
		args:            args,
		primaryColumn:   plan.primaryColumn,
		primaryField:    plan.primaryField,
		explicitPrimary: plan.explicitPrimary,
		returning:       plan.returning,
	}, nil
}

func modelBatchInsertFields(schema *ModelSchema, primaryColumn string, explicitPrimary bool) []FieldSchema {
	fields := schema.InsertFields
	if len(fields) == 0 {
		fields = schema.Fields
	}
	out := make([]FieldSchema, 0, len(fields))
	for _, field := range fields {
		if field.Ignore || field.Virtual || len(field.Index) == 0 {
			continue
		}
		if field.Primary && field.Column == primaryColumn && !explicitPrimary {
			continue
		}
		out = append(out, field)
	}
	return out
}

func modelBatchPrimaryShape[T any](models []*T, field FieldSchema) (explicit bool, mixed bool, err error) {
	firstSet := false
	for index, model := range models {
		structValue, err := modelStructValue(model)
		if err != nil {
			return false, false, err
		}
		fieldValue, ok := fieldByIndexReadSafe(structValue, field.Index)
		if !ok {
			return false, false, nil
		}
		if !fieldValue.IsValid() || !fieldValue.CanInterface() {
			return false, false, nil
		}
		hasValue := !isZeroValue(fieldValue)
		if index == 0 {
			firstSet = hasValue
			continue
		}
		if hasValue != firstSet {
			return false, true, nil
		}
	}
	return firstSet, false, nil
}

func modelStructValue(model any) (reflect.Value, error) {
	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() || modelValue.Kind() != reflect.Pointer || modelValue.IsNil() {
		return reflect.Value{}, &Error{Op: "create", Kind: ErrInvalidArgument}
	}
	structValue := modelValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return reflect.Value{}, &Error{Op: "create", Kind: ErrInvalidArgument}
	}
	return structValue, nil
}

func assignModelBatchAutoValue(value reflect.Value, now time.Time) error {
	if value.Type() == timeType {
		setTimeValue(value, now)
		return nil
	}
	return assignValue(value, now)
}

func valueForModelBatchWrite(value reflect.Value) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}
	if isNullStruct(value.Type()) {
		if !value.FieldByName("Valid").Bool() {
			return nil, nil
		}
		return valueForModelBatchWrite(value.FieldByName("Value"))
	}
	if value.Type() == jsonRawType {
		return []byte(value.Bytes()), nil
	}
	if value.Type() == timeType {
		return value.Interface(), nil
	}
	switch value.Kind() {
	case reflect.String:
		return value.String(), nil
	case reflect.Bool:
		return value.Bool(), nil
	case reflect.Int:
		return int(value.Int()), nil
	case reflect.Int8:
		return int8(value.Int()), nil
	case reflect.Int16:
		return int16(value.Int()), nil
	case reflect.Int32:
		return int32(value.Int()), nil
	case reflect.Int64:
		return value.Int(), nil
	case reflect.Uint:
		return uint(value.Uint()), nil
	case reflect.Uint8:
		return uint8(value.Uint()), nil
	case reflect.Uint16:
		return uint16(value.Uint()), nil
	case reflect.Uint32:
		return uint32(value.Uint()), nil
	case reflect.Uint64:
		return value.Uint(), nil
	case reflect.Uintptr:
		return uintptr(value.Uint()), nil
	case reflect.Float32:
		return float32(value.Float()), nil
	case reflect.Float64:
		return value.Float(), nil
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return value.Bytes(), nil
		}
	}
	return valueForWrite(value)
}

func compileModelBatchInsertSQL(db *DB, conn *Connection, table string, columns []string, rowCount int, returning bool, args []any) (CompiledSQL, error) {
	if conn == nil || conn.Dialect == nil || table == "" || len(columns) == 0 || rowCount <= 0 {
		return CompiledSQL{}, &Error{Op: "create", Kind: ErrInvalidArgument, Table: table}
	}
	key := modelBatchInsertSQLCacheKey(conn.Dialect.Name(), table, columns, rowCount, returning)
	if sql, ok := db.runtime.SQLCache.get(key); ok {
		return CompiledSQL{SQL: sql, Args: args}, nil
	}

	quotedColumns := make([]string, 0, len(columns))
	for _, column := range columns {
		quotedColumns = append(quotedColumns, conn.Dialect.QuoteIdent(column))
	}
	rows := make([]string, 0, rowCount)
	placeholderIndex := 1
	for range rowCount {
		placeholders := make([]string, 0, len(columns))
		for range columns {
			placeholders = append(placeholders, conn.Dialect.Placeholder(placeholderIndex))
			placeholderIndex++
		}
		rows = append(rows, "("+strings.Join(placeholders, ", ")+")")
	}
	sql := "insert into " + conn.Dialect.QuoteIdent(table) + " (" + strings.Join(quotedColumns, ", ") + ") values " + strings.Join(rows, ", ")
	if returning {
		sql += " returning *"
	}
	db.runtime.SQLCache.set(key, sql)
	return CompiledSQL{SQL: sql, Args: args}, nil
}

func modelBatchInsertSQLCacheKey(dialect string, table string, columns []string, rowCount int, returning bool) string {
	builder := strings.Builder{}
	builder.WriteString("mi|")
	builder.WriteString(dialect)
	builder.WriteByte('|')
	builder.WriteString(table)
	builder.WriteString("|r:")
	if returning {
		builder.WriteByte('1')
	} else {
		builder.WriteByte('0')
	}
	builder.WriteString("|n:")
	builder.WriteString(strconv.Itoa(rowCount))
	for _, column := range columns {
		builder.WriteByte('|')
		builder.WriteString(column)
	}
	return builder.String()
}

func assignModelBatchPrimaryValues[T any](conn *Connection, schema *ModelSchema, insert modelBatchInsert, models []*T, result ExecResult) (any, error) {
	if insert.explicitPrimary {
		return primaryValuesFromModels(schema, models)
	}
	if !result.HasLastInsertID || len(models) == 0 {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Field: insert.primaryColumn}
	}
	ids := make([]int64, 0, len(models))
	startID := result.LastInsertID
	if conn.Dialect.Name() == "sqlite" {
		startID = result.LastInsertID - int64(len(models)) + 1
	}
	for index, model := range models {
		id := startID + int64(index)
		if err := assignModelPrimaryValue(schema, model, insert.primaryField, id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func assignModelPrimaryValue(schema *ModelSchema, model any, field FieldSchema, value any) error {
	structValue, err := modelStructValue(model)
	if err != nil {
		return err
	}
	fieldValue, ok := fieldByIndexSafe(structValue, field.Index)
	if !ok {
		return nil
	}
	if !fieldValue.IsValid() || !fieldValue.CanSet() {
		return nil
	}
	if err := assignValue(fieldValue, value); err != nil {
		return &Error{Op: "create", Kind: ErrScan, Model: schema.Name, Field: field.Name, Cause: err}
	}
	return nil
}

func usesDefaultPlanner(db *DB) bool {
	if db == nil || db.runtime == nil {
		return false
	}
	switch db.runtime.Planner.(type) {
	case noopQueryPlanner:
		return true
	default:
		return false
	}
}
