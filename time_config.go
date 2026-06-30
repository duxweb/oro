package oro

import "time"

func normalizeTimeArgsUTC(args []any) []any {
	var normalized []any
	for index, arg := range args {
		var normalizedValue any
		shouldNormalize := false
		switch value := arg.(type) {
		case time.Time:
			normalizedValue = value.UTC()
			shouldNormalize = true
		case *time.Time:
			if value != nil {
				utcValue := value.UTC()
				normalizedValue = utcValue
				shouldNormalize = true
			}
		}
		if !shouldNormalize {
			continue
		}
		if normalized == nil {
			normalized = make([]any, len(args))
			copy(normalized, args)
		}
		normalized[index] = normalizedValue
	}
	if normalized != nil {
		return normalized
	}
	return args
}

func timeInLocation(value time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	return value.In(loc)
}

func runtimeLocation(rt *Runtime) *time.Location {
	if rt == nil {
		return time.UTC
	}
	return rt.Config.location()
}

func optionalLocation(values []*time.Location) *time.Location {
	if len(values) > 0 && values[0] != nil {
		return values[0]
	}
	return time.UTC
}

func normalizeRowTimes(row Map, loc *time.Location) Map {
	if len(row) == 0 {
		return row
	}
	for key, value := range row {
		if timeValue, ok := value.(time.Time); ok {
			row[key] = timeInLocation(timeValue, loc)
		}
	}
	return row
}
