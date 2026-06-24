package queryutil

import (
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

func IsZeroValue(value reflect.Value) bool {
	return value.IsZero()
}
