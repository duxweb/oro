package oro

import (
	"github.com/duxweb/oro/internal/naming"
	"github.com/duxweb/oro/internal/sqlformat"
	internaltypes "github.com/duxweb/oro/internal/types"
)

type Map = internaltypes.Map
type Null[T any] = internaltypes.Null[T]

func NullOf[T any](value T) Null[T] {
	return internaltypes.NullOf(value)
}

func NullZero[T any]() Null[T] {
	return internaltypes.NullZero[T]()
}

type Decimal = internaltypes.Decimal
type JSONRaw = internaltypes.JSONRaw

func Snake(name string) string {
	return naming.Snake(name)
}

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
