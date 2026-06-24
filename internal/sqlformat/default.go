package sqlformat

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func DefaultValue(value any) string {
	switch typedValue := value.(type) {
	case nil:
		return "null"
	case bool:
		if typedValue {
			return "true"
		}
		return "false"
	case string:
		return QuoteString(typedValue)
	case []byte:
		return QuoteString(string(typedValue))
	case time.Time:
		return QuoteString(typedValue.UTC().Format("2006-01-02 15:04:05"))
	case fmt.Stringer:
		return QuoteString(typedValue.String())
	default:
		return fmt.Sprint(typedValue)
	}
}

func QuoteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func SizedType(name string, size int) string {
	if size <= 0 {
		return name
	}
	return name + "(" + strconv.Itoa(size) + ")"
}
