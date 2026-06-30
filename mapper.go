package oro

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/duxweb/oro/internal/valueconv"
)

var (
	timeType    = reflect.TypeOf(time.Time{})
	decimalType = reflect.TypeOf(Decimal(""))
	jsonRawType = reflect.TypeOf(JSONRaw{})
)

type reflectMapper struct {
	rt *Runtime
}

func (mapper reflectMapper) MapModel(schema *ModelSchema, row Map, dest any) error {
	if err := mapRowToStruct(row, dest, schema, runtimeLocation(mapper.rt)); err != nil {
		return err
	}
	return nil
}

func (mapper reflectMapper) MapDTO(row Map, dest any) error {
	if mapper.rt != nil && mapper.rt.Registry != nil {
		destType, err := structTypeOfDest(dest)
		if err != nil {
			return err
		}
		if schema, ok := mapper.rt.Registry.GetType(destType); ok {
			return mapRowToStruct(row, dest, schema, runtimeLocation(mapper.rt))
		}
	}
	return mapRowToStruct(row, dest, nil, runtimeLocation(mapper.rt))
}

func mapRowToStruct(row Map, dest any, schema *ModelSchema, loc *time.Location) error {
	destValue := reflect.ValueOf(dest)
	if !destValue.IsValid() || destValue.Kind() != reflect.Pointer || destValue.IsNil() {
		return &Error{Op: "map", Kind: ErrInvalidArgument}
	}

	structValue := destValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return &Error{Op: "map", Kind: ErrInvalidArgument}
	}

	if schema != nil {
		for _, field := range schema.Fields {
			value, ok := row[field.Column]
			if !ok {
				continue
			}
			if len(field.Index) == 0 {
				continue
			}
			fieldValue, ok := fieldByIndexSafe(structValue, field.Index)
			if !ok {
				continue
			}
			if !fieldValue.IsValid() || !fieldValue.CanSet() {
				continue
			}
			if err := assignValueInLocation(fieldValue, value, loc); err != nil {
				return &Error{Op: "map", Kind: ErrScan, Field: field.Name, Cause: err}
			}
		}
		return nil
	}

	for _, structField := range reflect.VisibleFields(structValue.Type()) {
		if !structField.IsExported() {
			continue
		}
		value, ok := row[Snake(structField.Name)]
		if !ok {
			continue
		}
		fieldValue, ok := fieldByIndexSafe(structValue, structField.Index)
		if !ok {
			continue
		}
		if !fieldValue.CanSet() {
			continue
		}
		if err := assignValueInLocation(fieldValue, value, loc); err != nil {
			return &Error{Op: "map", Kind: ErrScan, Field: structField.Name, Cause: err}
		}
	}

	return nil
}

func structTypeOfDest(dest any) (reflect.Type, error) {
	destType := reflect.TypeOf(dest)
	if destType == nil {
		return nil, &Error{Op: "map", Kind: ErrInvalidArgument}
	}
	for destType.Kind() == reflect.Pointer {
		destType = destType.Elem()
	}
	if destType.Kind() != reflect.Struct {
		return nil, &Error{Op: "map", Kind: ErrInvalidArgument}
	}
	return destType, nil
}

func assignValue(dest reflect.Value, value any) error {
	return assignValueInLocation(dest, value, nil)
}

func assignValueInLocation(dest reflect.Value, value any, loc *time.Location) error {
	if !dest.CanSet() {
		return nil
	}

	if isNullStruct(dest.Type()) {
		return assignNullValue(dest, value, loc)
	}

	if value == nil {
		if dest.Kind() == reflect.Pointer || dest.Kind() == reflect.Interface || dest.Kind() == reflect.Slice || dest.Kind() == reflect.Map {
			dest.Set(reflect.Zero(dest.Type()))
		}
		return nil
	}

	if scanner, ok := scannerFor(dest); ok {
		return scanner.Scan(value)
	}

	if dest.Kind() == reflect.Pointer {
		elemValue := reflect.New(dest.Type().Elem())
		if err := assignValueInLocation(elemValue.Elem(), value, loc); err != nil {
			return err
		}
		dest.Set(elemValue)
		return nil
	}

	valueReflect := reflect.ValueOf(value)
	if valueReflect.IsValid() && dest.Type() == timeType && valueReflect.Type() == timeType {
		dest.Set(reflect.ValueOf(timeInLocation(valueReflect.Interface().(time.Time), loc)))
		return nil
	}
	if valueReflect.IsValid() && valueReflect.Type().AssignableTo(dest.Type()) {
		dest.Set(valueReflect)
		return nil
	}
	if valueReflect.IsValid() && valueReflect.Type().ConvertibleTo(dest.Type()) && safeConvertible(valueReflect, dest.Type()) {
		dest.Set(valueReflect.Convert(dest.Type()))
		return nil
	}

	switch dest.Kind() {
	case reflect.String:
		text, err := toString(value)
		if err != nil {
			return err
		}
		dest.SetString(text)
	case reflect.Bool:
		boolValue, err := toBool(value)
		if err != nil {
			return err
		}
		dest.SetBool(boolValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if dest.Type() == timeType {
			timeValue, err := toTime(value)
			if err != nil {
				return err
			}
			setTimeValue(dest, timeInLocation(timeValue, loc))
			return nil
		}
		intValue, err := toInt64(value)
		if err != nil {
			return err
		}
		if dest.OverflowInt(intValue) {
			return fmt.Errorf("integer overflow")
		}
		dest.SetInt(intValue)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		uintValue, err := toUint64(value)
		if err != nil {
			return err
		}
		if dest.OverflowUint(uintValue) {
			return fmt.Errorf("unsigned integer overflow")
		}
		dest.SetUint(uintValue)
	case reflect.Float32, reflect.Float64:
		floatValue, err := toFloat64(value)
		if err != nil {
			return err
		}
		if dest.OverflowFloat(floatValue) {
			return fmt.Errorf("float overflow")
		}
		dest.SetFloat(floatValue)
	case reflect.Slice:
		if dest.Type().Elem().Kind() == reflect.Uint8 {
			bytesValue, err := toBytes(value)
			if err != nil {
				return err
			}
			dest.SetBytes(bytesValue)
			return nil
		}
		return assignJSONValue(dest, value)
	case reflect.Map, reflect.Struct:
		if dest.Type() == decimalType {
			text, err := toString(value)
			if err != nil {
				return err
			}
			dest.Set(reflect.ValueOf(Decimal(text)))
			return nil
		}
		if dest.Type() == jsonRawType {
			bytesValue, err := toBytes(value)
			if err != nil {
				return err
			}
			dest.Set(reflect.ValueOf(JSONRaw(bytesValue)))
			return nil
		}
		if dest.Type() == timeType {
			timeValue, err := toTime(value)
			if err != nil {
				return err
			}
			setTimeValue(dest, timeInLocation(timeValue, loc))
			return nil
		}
		return assignJSONValue(dest, value)
	case reflect.Interface:
		dest.Set(valueReflect)
	default:
		return fmt.Errorf("unsupported destination type %s", dest.Type())
	}

	return nil
}

func setTimeValue(dest reflect.Value, value time.Time) {
	if dest.CanAddr() && dest.Addr().CanInterface() {
		if target, ok := dest.Addr().Interface().(*time.Time); ok {
			*target = value
			return
		}
	}
	dest.Set(reflect.ValueOf(value))
}

func scannerFor(dest reflect.Value) (sql.Scanner, bool) {
	if !dest.CanAddr() {
		return nil, false
	}
	scanner, ok := dest.Addr().Interface().(sql.Scanner)
	return scanner, ok
}

func isNullStruct(destType reflect.Type) bool {
	if destType.Kind() != reflect.Struct || !strings.HasPrefix(destType.Name(), "Null[") {
		return false
	}
	switch destType.PkgPath() {
	case "github.com/duxweb/oro", "github.com/duxweb/oro/internal/types":
		return true
	default:
		return false
	}
}

func assignNullValue(dest reflect.Value, value any, loc *time.Location) error {
	validField := dest.FieldByName("Valid")
	valueField := dest.FieldByName("Value")
	if !validField.IsValid() || !valueField.IsValid() || !validField.CanSet() || !valueField.CanSet() {
		return fmt.Errorf("invalid null type")
	}
	if value == nil {
		valueField.Set(reflect.Zero(valueField.Type()))
		validField.SetBool(false)
		return nil
	}
	if err := assignValueInLocation(valueField, value, loc); err != nil {
		return err
	}
	validField.SetBool(true)
	return nil
}

func assignJSONValue(dest reflect.Value, value any) error {
	bytesValue, err := toBytes(value)
	if err != nil {
		return err
	}
	if len(bytesValue) == 0 {
		return nil
	}
	return json.Unmarshal(bytesValue, dest.Addr().Interface())
}

func safeConvertible(value reflect.Value, destType reflect.Type) bool {
	if destType.Kind() == reflect.String && value.Kind() >= reflect.Int && value.Kind() <= reflect.Uintptr {
		return false
	}
	if value.Kind() >= reflect.Int && value.Kind() <= reflect.Int64 && destType.Kind() >= reflect.Uint && destType.Kind() <= reflect.Uintptr {
		return value.Int() >= 0
	}
	return true
}

func toString(value any) (string, error) {
	return valueconv.String(value)
}

func toBool(value any) (bool, error) {
	return valueconv.Bool(value)
}

func toInt64(value any) (int64, error) {
	return valueconv.Int64(value)
}

func toUint64(value any) (uint64, error) {
	return valueconv.Uint64(value)
}

func toFloat64(value any) (float64, error) {
	return valueconv.Float64(value)
}

func toBytes(value any) ([]byte, error) {
	return valueconv.Bytes(value)
}

func toTime(value any) (time.Time, error) {
	return valueconv.Time(value)
}
