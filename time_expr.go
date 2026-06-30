package oro

import "time"

type TimeExpr struct {
	field string
}

// Time creates a time expression for field.
//
// Calendar helpers such as OnDate and InMonth compile to half-open ranges so
// they can use normal indexes instead of database date functions.
func Time(field string) TimeExpr {
	return TimeExpr{field: field}
}

func (expr TimeExpr) f() FieldExpr {
	return Field(expr.field)
}

// Between returns a closed BETWEEN condition for two time values.
func (expr TimeExpr) Between(start time.Time, end time.Time) Condition {
	return expr.f().Between(start, end)
}

// NotBetween returns the negation of a closed BETWEEN condition.
func (expr TimeExpr) NotBetween(start time.Time, end time.Time) Condition {
	return expr.f().NotBetween(start, end)
}

// After returns a field > value condition.
func (expr TimeExpr) After(value time.Time) Condition {
	return expr.f().Gt(value)
}

// Before returns a field < value condition.
func (expr TimeExpr) Before(value time.Time) Condition {
	return expr.f().Lt(value)
}

// From returns a field >= value condition.
func (expr TimeExpr) From(value time.Time) Condition {
	return expr.f().Gte(value)
}

// Until returns a field < value condition, suitable as a half-open upper bound.
func (expr TimeExpr) Until(value time.Time) Condition {
	return expr.f().Lt(value)
}

// InRange returns a half-open [start, end) condition.
func (expr TimeExpr) InRange(start time.Time, end time.Time) Condition {
	return And(expr.f().Gte(start), expr.f().Lt(end))
}

// OnDate returns the calendar day containing day in day.Location().
func (expr TimeExpr) OnDate(day time.Time) Condition {
	start, end := DayBounds(day, day.Location())
	return expr.InRange(start, end)
}

// InMonth returns the calendar month containing value in value.Location().
func (expr TimeExpr) InMonth(value time.Time) Condition {
	start, end := MonthBounds(value, value.Location())
	return expr.InRange(start, end)
}

// InYear returns the calendar year containing value in value.Location().
func (expr TimeExpr) InYear(value time.Time) Condition {
	start, end := YearBounds(value, value.Location())
	return expr.InRange(start, end)
}

// Today returns today's calendar day in loc, or UTC when loc is omitted or nil.
func (expr TimeExpr) Today(loc ...*time.Location) Condition {
	start, end := DayBounds(time.Now(), firstLocationOrUTC(loc))
	return expr.InRange(start, end)
}

// LastDays returns a half-open range covering the current day and the previous
// days-1 days in loc, or UTC when loc is omitted or nil.
func (expr TimeExpr) LastDays(days int, loc ...*time.Location) Condition {
	if days < 1 {
		days = 1
	}
	location := firstLocationOrUTC(loc)
	_, end := DayBounds(time.Now(), location)
	start := end.AddDate(0, 0, -days)
	return expr.InRange(start, end)
}

// DayBounds returns the half-open [start, end) bounds for value's day in loc.
func DayBounds(value time.Time, loc *time.Location) (start time.Time, end time.Time) {
	start = startOfDay(value, loc)
	return start, start.AddDate(0, 0, 1)
}

// MonthBounds returns the half-open [start, end) bounds for value's month in loc.
func MonthBounds(value time.Time, loc *time.Location) (start time.Time, end time.Time) {
	location := locationOrUTC(loc)
	year, month, _ := value.In(location).Date()
	start = time.Date(year, month, 1, 0, 0, 0, 0, location)
	return start, start.AddDate(0, 1, 0)
}

// YearBounds returns the half-open [start, end) bounds for value's year in loc.
func YearBounds(value time.Time, loc *time.Location) (start time.Time, end time.Time) {
	location := locationOrUTC(loc)
	year := value.In(location).Year()
	start = time.Date(year, 1, 1, 0, 0, 0, 0, location)
	return start, start.AddDate(1, 0, 0)
}

func firstLocationOrUTC(locations []*time.Location) *time.Location {
	if len(locations) > 0 {
		return locationOrUTC(locations[0])
	}
	return time.UTC
}

func locationOrUTC(loc *time.Location) *time.Location {
	if loc != nil {
		return loc
	}
	return time.UTC
}

func startOfDay(value time.Time, loc *time.Location) time.Time {
	location := locationOrUTC(loc)
	year, month, day := value.In(location).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}
