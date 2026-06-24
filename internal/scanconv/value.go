package scanconv

import (
	"database/sql"
	"strings"
	"time"

	internaltypes "github.com/duxweb/oro/internal/types"
)

var timeLayouts = [...]string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999 -0700 MST",
	"2006-01-02 15:04:05 -0700 MST",
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func Normalize(value any, dbType string) any {
	if value == nil {
		return nil
	}

	normalizedType := strings.ToLower(dbType)
	switch typedValue := value.(type) {
	case []byte:
		copiedValue := append([]byte(nil), typedValue...)
		if isJSONType(normalizedType) {
			return internaltypes.JSONRaw(copiedValue)
		}
		if isBlobType(normalizedType) {
			return copiedValue
		}
		if isDecimalType(normalizedType) {
			return internaltypes.Decimal(string(copiedValue))
		}
		return string(copiedValue)
	case sql.RawBytes:
		copiedValue := append([]byte(nil), typedValue...)
		if isJSONType(normalizedType) {
			return internaltypes.JSONRaw(copiedValue)
		}
		if isBlobType(normalizedType) {
			return copiedValue
		}
		if isDecimalType(normalizedType) {
			return internaltypes.Decimal(string(copiedValue))
		}
		return string(copiedValue)
	case string:
		if isJSONType(normalizedType) {
			return internaltypes.JSONRaw([]byte(typedValue))
		}
		if isDecimalType(normalizedType) {
			return internaltypes.Decimal(typedValue)
		}
		if isTimeType(normalizedType) {
			if parsedTime, ok := ParseTimeString(typedValue); ok {
				return parsedTime
			}
		}
		return typedValue
	case time.Time:
		return typedValue
	case bool:
		return typedValue
	case int64:
		return typedValue
	case float64:
		return typedValue
	default:
		return typedValue
	}
}

func ParseTimeString(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range timeLayouts {
		parsedTime, err := time.Parse(layout, value)
		if err == nil {
			return parsedTime, true
		}
	}
	return time.Time{}, false
}

func isBlobType(dbType string) bool {
	return strings.Contains(dbType, "blob") || strings.Contains(dbType, "binary")
}

func isDecimalType(dbType string) bool {
	return strings.Contains(dbType, "decimal") || strings.Contains(dbType, "numeric")
}

func isJSONType(dbType string) bool {
	return strings.Contains(dbType, "json")
}

func isTimeType(dbType string) bool {
	return strings.Contains(dbType, "time") || strings.Contains(dbType, "date")
}
