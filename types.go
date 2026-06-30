package oro

import (
	"github.com/duxweb/oro/internal/naming"
	"github.com/duxweb/oro/internal/sqlformat"
	internaltypes "github.com/duxweb/oro/internal/types"
)

// Map is an explicit field/value map used by table queries and updates.
type Map = internaltypes.Map

// Null represents a nullable scalar value returned by aggregate helpers.
type Null[T any] = internaltypes.Null[T]

// NullOf returns a valid nullable value.
func NullOf[T any](value T) Null[T] {
	return internaltypes.NullOf(value)
}

// NullZero returns an invalid nullable zero value.
func NullZero[T any]() Null[T] {
	return internaltypes.NullZero[T]()
}

// Decimal is a string-backed decimal value returned by decimal aggregates.
type Decimal = internaltypes.Decimal

// JSONRaw stores raw JSON bytes for JSON columns.
type JSONRaw = internaltypes.JSONRaw

// Snake converts a Go-style name to snake_case.
func Snake(name string) string {
	return naming.Snake(name)
}

// FormatDefaultValue formats a Go value for schema default SQL.
func FormatDefaultValue(value any) string {
	return sqlformat.DefaultValue(value)
}

func formatDefaultValue(value any) string {
	return sqlformat.DefaultValue(value)
}

func quoteSQLString(value string) string {
	return sqlformat.QuoteString(value)
}

func formatSizedType(name string, size int) string {
	return sqlformat.SizedType(name, size)
}
