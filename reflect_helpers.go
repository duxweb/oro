package oro

import (
	"reflect"

	"github.com/duxweb/oro/internal/reflectx"
)

// fieldByIndexSafe walks a struct field index allocating nil embedded pointers
// (write access); fieldByIndexReadSafe is the read-only variant. Both delegate
// to internal/reflectx so the walk lives in exactly one place.
func fieldByIndexSafe(value reflect.Value, index []int) (reflect.Value, bool) {
	return reflectx.FieldByIndexAlloc(value, index)
}

func fieldByIndexReadSafe(value reflect.Value, index []int) (reflect.Value, bool) {
	return reflectx.FieldByIndex(value, index)
}
