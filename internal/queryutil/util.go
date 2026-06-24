package queryutil

import (
	"fmt"
	"reflect"
	"strings"
)

func StringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func IsQualifiedIdentifier(name string) bool {
	return strings.Contains(name, ".")
}

func AggregateSelectSQL(op string, field string, quote func(string) string) (string, error) {
	if op == "" || field == "" {
		return "", fmt.Errorf("invalid aggregate expression")
	}
	if field == "*" {
		return op + "(*)", nil
	}
	if quote == nil {
		return op + "(" + field + ")", nil
	}
	return op + "(" + quote(field) + ")", nil
}

func IsZeroValue(value reflect.Value) bool {
	return value.IsZero()
}
