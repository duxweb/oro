package valueconv

import (
	"fmt"
	"strconv"
	"time"

	"github.com/duxweb/oro/internal/scanconv"
	internaltypes "github.com/duxweb/oro/internal/types"
)

func String(value any) (string, error) {
	switch typedValue := value.(type) {
	case string:
		return typedValue, nil
	case []byte:
		return string(typedValue), nil
	case internaltypes.JSONRaw:
		return string(typedValue), nil
	case internaltypes.Decimal:
		return string(typedValue), nil
	case fmt.Stringer:
		return typedValue.String(), nil
	default:
		return fmt.Sprint(value), nil
	}
}

func Bool(value any) (bool, error) {
	switch typedValue := value.(type) {
	case bool:
		return typedValue, nil
	case int64:
		return typedValue != 0, nil
	case int:
		return typedValue != 0, nil
	case uint64:
		return typedValue != 0, nil
	case string:
		return strconv.ParseBool(typedValue)
	default:
		return false, fmt.Errorf("cannot convert %T to bool", value)
	}
}

func Int64(value any) (int64, error) {
	switch typedValue := value.(type) {
	case int64:
		return typedValue, nil
	case int:
		return int64(typedValue), nil
	case int32:
		return int64(typedValue), nil
	case uint64:
		if typedValue > uint64(^uint64(0)>>1) {
			return 0, fmt.Errorf("integer overflow")
		}
		return int64(typedValue), nil
	case float64:
		return int64(typedValue), nil
	case string:
		return strconv.ParseInt(typedValue, 10, 64)
	case internaltypes.Decimal:
		return strconv.ParseInt(string(typedValue), 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", value)
	}
}

func Uint64(value any) (uint64, error) {
	switch typedValue := value.(type) {
	case uint64:
		return typedValue, nil
	case uint:
		return uint64(typedValue), nil
	case uint32:
		return uint64(typedValue), nil
	case int64:
		if typedValue < 0 {
			return 0, fmt.Errorf("negative integer")
		}
		return uint64(typedValue), nil
	case int:
		if typedValue < 0 {
			return 0, fmt.Errorf("negative integer")
		}
		return uint64(typedValue), nil
	case float64:
		if typedValue < 0 {
			return 0, fmt.Errorf("negative float")
		}
		return uint64(typedValue), nil
	case string:
		return strconv.ParseUint(typedValue, 10, 64)
	case internaltypes.Decimal:
		return strconv.ParseUint(string(typedValue), 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to uint64", value)
	}
}

func Float64(value any) (float64, error) {
	switch typedValue := value.(type) {
	case float64:
		return typedValue, nil
	case float32:
		return float64(typedValue), nil
	case int64:
		return float64(typedValue), nil
	case int:
		return float64(typedValue), nil
	case uint64:
		return float64(typedValue), nil
	case string:
		return strconv.ParseFloat(typedValue, 64)
	case internaltypes.Decimal:
		return strconv.ParseFloat(string(typedValue), 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

func Bytes(value any) ([]byte, error) {
	switch typedValue := value.(type) {
	case []byte:
		return append([]byte(nil), typedValue...), nil
	case internaltypes.JSONRaw:
		return append([]byte(nil), typedValue...), nil
	case string:
		return []byte(typedValue), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to bytes", value)
	}
}

func Time(value any) (time.Time, error) {
	switch typedValue := value.(type) {
	case time.Time:
		return typedValue, nil
	case string:
		if parsedTime, ok := scanconv.ParseTimeString(typedValue); ok {
			return parsedTime, nil
		}
	case []byte:
		if parsedTime, ok := scanconv.ParseTimeString(string(typedValue)); ok {
			return parsedTime, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot convert %T to time.Time", value)
}
