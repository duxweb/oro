// Package reflectx holds small reflection helpers shared across the ORM core
// and its extensions.
package reflectx

import "reflect"

// FieldByIndex walks index into value the way reflect.Value.FieldByIndex does,
// but never panics on a nil embedded pointer: such a path simply yields ok=false.
// Use it for read-only access.
func FieldByIndex(value reflect.Value, index []int) (reflect.Value, bool) {
	return walkFieldByIndex(value, index, false)
}

// FieldByIndexAlloc behaves like FieldByIndex but allocates any nil embedded
// pointer encountered along the path so the returned field is addressable and
// settable. Use it for write access.
func FieldByIndexAlloc(value reflect.Value, index []int) (reflect.Value, bool) {
	return walkFieldByIndex(value, index, true)
}

func walkFieldByIndex(value reflect.Value, index []int, alloc bool) (reflect.Value, bool) {
	if !value.IsValid() || len(index) == 0 {
		return reflect.Value{}, false
	}
	for position, fieldIndex := range index {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				if !alloc || !value.CanSet() {
					return reflect.Value{}, false
				}
				value.Set(reflect.New(value.Type().Elem()))
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct || fieldIndex < 0 || fieldIndex >= value.NumField() {
			return reflect.Value{}, false
		}
		value = value.Field(fieldIndex)
		if position == len(index)-1 {
			return value, true
		}
	}
	return value, true
}
