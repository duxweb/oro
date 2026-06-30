package oro

import (
	"context"
	"reflect"
)

// CreateResult contains batch insert metadata and generated primary IDs.
type CreateResult struct {
	RowsAffected int64
	PrimaryKey   string
	primaryIDs   any
}

// IDs returns generated primary IDs converted to T.
func (result CreateResult) IDs[T any]() ([]T, error) {
	count := result.IDCount()
	ids := make([]T, 0, count)
	for index := 0; index < count; index++ {
		value := primaryIDAt(result.primaryIDs, index)
		converted, err := scalarValue[T](value)
		if err != nil {
			return nil, err
		}
		ids = append(ids, converted)
	}
	return ids, nil
}

// FirstID returns the first generated primary ID converted to T.
func (result CreateResult) FirstID[T any]() (T, error) {
	var zero T
	if result.IDCount() == 0 {
		return zero, &Error{Op: "create", Kind: ErrScan, Field: result.PrimaryKey}
	}
	return scalarValue[T](primaryIDAt(result.primaryIDs, 0))
}

// IDCount returns the number of generated primary IDs captured in the result.
func (result CreateResult) IDCount() int {
	return primaryIDCount(result.primaryIDs)
}

func createResultFromIDs(primaryKey string, ids []any, rowsAffected int64) *CreateResult {
	return createResultFromIDValues(primaryKey, ids, rowsAffected)
}

func createResultFromIDValues(primaryKey string, ids any, rowsAffected int64) *CreateResult {
	if rowsAffected == 0 {
		rowsAffected = int64(primaryIDCount(ids))
	}
	return &CreateResult{
		RowsAffected: rowsAffected,
		PrimaryKey:   primaryKey,
		primaryIDs:   clonePrimaryIDs(ids),
	}
}

func primaryResultKey(schema *ModelSchema) string {
	if schema == nil || len(schema.PrimaryColumns) != 1 {
		return ""
	}
	return schema.PrimaryColumns[0]
}

func appendPrimaryID(ids []any, value any) []any {
	if value == nil {
		return ids
	}
	return append(ids, value)
}

func primaryValueFromModel(schema *ModelSchema, model any) (any, bool, error) {
	if schema == nil || len(schema.PrimaryColumns) != 1 {
		return nil, false, nil
	}
	field, ok := schema.FieldByDB[schema.PrimaryColumns[0]]
	if !ok || len(field.Index) == 0 {
		return nil, false, nil
	}
	structValue, err := modelStructValue(model)
	if err != nil {
		return nil, false, err
	}
	fieldValue, ok := fieldByIndexReadSafe(structValue, field.Index)
	if !ok {
		return nil, false, nil
	}
	if !fieldValue.IsValid() || !fieldValue.CanInterface() || isZeroValue(fieldValue) {
		return nil, false, nil
	}
	return valueForCreateResultID(fieldValue), true, nil
}

func primaryValuesFromModels[T any](schema *ModelSchema, models []*T) ([]any, error) {
	ids := make([]any, 0, len(models))
	for _, model := range models {
		value, ok, err := primaryValueFromModel(schema, model)
		if err != nil {
			return nil, err
		}
		if ok {
			ids = append(ids, value)
		}
	}
	return ids, nil
}

func primaryIDCount(ids any) int {
	switch values := ids.(type) {
	case nil:
		return 0
	case []any:
		return len(values)
	case []int64:
		return len(values)
	case []uint64:
		return len(values)
	case []string:
		return len(values)
	default:
		value := reflect.ValueOf(ids)
		if value.IsValid() && value.Kind() == reflect.Slice {
			return value.Len()
		}
		return 0
	}
}

func primaryIDAt(ids any, index int) any {
	switch values := ids.(type) {
	case []any:
		return values[index]
	case []int64:
		return values[index]
	case []uint64:
		return values[index]
	case []string:
		return values[index]
	default:
		value := reflect.ValueOf(ids)
		if value.IsValid() && value.Kind() == reflect.Slice && index >= 0 && index < value.Len() {
			return value.Index(index).Interface()
		}
		return nil
	}
}

func clonePrimaryIDs(ids any) any {
	switch values := ids.(type) {
	case nil:
		return nil
	case []any:
		return append([]any(nil), values...)
	case []int64:
		return append([]int64(nil), values...)
	case []uint64:
		return append([]uint64(nil), values...)
	case []string:
		return append([]string(nil), values...)
	default:
		return ids
	}
}

func appendPrimaryIDs(left any, right any) any {
	if primaryIDCount(right) == 0 {
		return left
	}
	if primaryIDCount(left) == 0 {
		return clonePrimaryIDs(right)
	}
	switch leftValues := left.(type) {
	case []int64:
		if rightValues, ok := right.([]int64); ok {
			return append(leftValues, rightValues...)
		}
	case []uint64:
		if rightValues, ok := right.([]uint64); ok {
			return append(leftValues, rightValues...)
		}
	case []string:
		if rightValues, ok := right.([]string); ok {
			return append(leftValues, rightValues...)
		}
	case []any:
		for index := 0; index < primaryIDCount(right); index++ {
			leftValues = append(leftValues, primaryIDAt(right, index))
		}
		return leftValues
	}
	values := make([]any, 0, primaryIDCount(left)+primaryIDCount(right))
	for index := 0; index < primaryIDCount(left); index++ {
		values = append(values, primaryIDAt(left, index))
	}
	for index := 0; index < primaryIDCount(right); index++ {
		values = append(values, primaryIDAt(right, index))
	}
	return values
}

func tablePrimaryKey(ctx context.Context, db *DB, spec QuerySpec) (string, error) {
	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return "", err
	}
	writeSpec := WriteSpec{QuerySpec: spec}
	tableNames(db).ApplyWrite(&writeSpec)
	primaryColumns, err := primaryColumns(ctx, conn, writeSpec)
	if err != nil || len(primaryColumns) != 1 {
		return "", err
	}
	return primaryColumns[0], nil
}

func valueForCreateResultID(value reflect.Value) any {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.String:
		return value.String()
	default:
		if value.CanInterface() {
			return value.Interface()
		}
	}
	return nil
}
