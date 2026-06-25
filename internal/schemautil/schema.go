package schemautil

import (
	"reflect"
	"strings"
)

func IsBaseModelField(name string) bool {
	switch name {
	case "ID", "CreatedAt", "UpdatedAt":
		return true
	default:
		return false
	}
}

func FieldByName(typ reflect.Type, name string) (reflect.StructField, bool) {
	if IsBaseModelField(name) {
		return reflect.StructField{Name: name}, true
	}
	for index := 0; index < typ.NumField(); index++ {
		structField := typ.Field(index)
		if structField.IsExported() && structField.Name == name {
			return structField, true
		}
	}
	return reflect.StructField{}, false
}

func IsDecimalGoType(goType reflect.Type) bool {
	typeName := goType.String()
	if strings.Contains(typeName, "Decimal") {
		return true
	}
	for goType.Kind() == reflect.Pointer {
		goType = goType.Elem()
	}
	switch goType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func IsSizableFieldType(fieldType string) bool {
	switch fieldType {
	case "string", "enum", "email", "url", "ip", "mac", "phone", "slug", "color":
		return true
	default:
		return false
	}
}

func ContainsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func IsIntegerFieldType(fieldType string) bool {
	switch fieldType {
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func IndexName(table string, column string, name string, defaultMarker string, unique bool) string {
	if name != "" && name != defaultMarker {
		return name
	}
	if unique {
		return "uk_" + table + "_" + column
	}
	return "idx_" + table + "_" + column
}

func FullTextIndexName(table string, column string, name string, defaultMarker string) string {
	if name != "" && name != defaultMarker {
		return name
	}
	return "ft_" + table + "_" + column
}

func CompositeIndexName(table string, columns []string, unique bool) string {
	prefix := "idx_"
	if unique {
		prefix = "uk_"
	}
	name := prefix + table
	for _, column := range columns {
		name += "_" + column
	}
	return name
}

func CompositeFullTextIndexName(table string, columns []string) string {
	name := "ft_" + table
	for _, column := range columns {
		name += "_" + column
	}
	return name
}
