package oro

import (
	"testing"
	"time"
)

func TestBuildConditionEqual(t *testing.T) {
	condition := buildCondition("Code", "A001")
	if condition.Field != "Code" || condition.Op != "=" || condition.Value != "A001" {
		t.Fatalf("unexpected condition %#v", condition)
	}
}

func TestBuildConditionOperator(t *testing.T) {
	condition := buildCondition("Price", ">=", 100)
	if condition.Field != "Price" || condition.Op != ">=" || condition.Value != 100 {
		t.Fatalf("unexpected condition %#v", condition)
	}
}

func TestFieldConditions(t *testing.T) {
	condition := Field("Price").Between(100, 200)
	if condition.Field != "Price" || condition.Op != "between" {
		t.Fatalf("unexpected between condition %#v", condition)
	}
	values, ok := condition.Value.([]any)
	if !ok || len(values) != 2 || values[0] != 100 || values[1] != 200 {
		t.Fatalf("unexpected between values %#v", condition.Value)
	}
	notBetween := Field("Price").NotBetween(100, 200)
	if notBetween.Op != "not" || len(notBetween.Conditions) != 1 || notBetween.Conditions[0].Op != "between" {
		t.Fatalf("unexpected not between condition %#v", notBetween)
	}

	condition = Field("Code").NotIn("A001", "A002")
	if condition.Field != "Code" || condition.Op != "not_in_values" {
		t.Fatalf("unexpected not in condition %#v", condition)
	}
}

func TestTimeExprBoundsAndConditions(t *testing.T) {
	loc := time.FixedZone("UTC+08", 8*60*60)
	value := time.Date(2026, 6, 30, 15, 45, 0, 0, time.UTC)

	start, end := DayBounds(value, loc)
	if start.Location() != loc || start.Hour() != 0 || start.Day() != 30 {
		t.Fatalf("unexpected day start %s (%s)", start, start.Location())
	}
	if !end.Equal(start.AddDate(0, 0, 1)) {
		t.Fatalf("unexpected day end %s", end)
	}

	monthStart, monthEnd := MonthBounds(value, loc)
	if monthStart.Month() != time.June || monthStart.Day() != 1 || monthEnd.Month() != time.July || monthEnd.Day() != 1 {
		t.Fatalf("unexpected month bounds %s %s", monthStart, monthEnd)
	}

	yearStart, yearEnd := YearBounds(value, loc)
	if yearStart.Year() != 2026 || yearStart.Month() != time.January || yearEnd.Year() != 2027 {
		t.Fatalf("unexpected year bounds %s %s", yearStart, yearEnd)
	}

	condition := Time("CreatedAt").InRange(start, end)
	if condition.Op != "group" || len(condition.Conditions) != 2 {
		t.Fatalf("unexpected range condition %#v", condition)
	}
	if condition.Conditions[0].Field != "CreatedAt" || condition.Conditions[0].Op != ">=" {
		t.Fatalf("unexpected range start %#v", condition.Conditions[0])
	}
	if condition.Conditions[1].Field != "CreatedAt" || condition.Conditions[1].Op != "<" {
		t.Fatalf("unexpected range end %#v", condition.Conditions[1])
	}

	today := Time("CreatedAt").Today(loc)
	if today.Op != "group" || len(today.Conditions) != 2 {
		t.Fatalf("unexpected today condition %#v", today)
	}
	lastDays := Time("CreatedAt").LastDays(0, loc)
	if lastDays.Op != "group" || len(lastDays.Conditions) != 2 {
		t.Fatalf("unexpected last days condition %#v", lastDays)
	}
}

func TestConditionGroupsKeepParentAndNestedBoolsSeparate(t *testing.T) {
	condition := Or(
		Field("Code").Like("A%"),
		Field("Code").Like("B%"),
	)
	if condition.Bool != "" {
		t.Fatalf("expected parent bool empty, got %#v", condition)
	}
	if len(condition.Conditions) != 2 || condition.Conditions[0].Bool != "or" || condition.Conditions[1].Bool != "or" {
		t.Fatalf("unexpected nested bools %#v", condition.Conditions)
	}
}

func TestWhereBuilderGroups(t *testing.T) {
	condition := buildWhereGroup("and", func(w *WhereBuilder) {
		w.Where("Status", "active").
			OrWhereGroup(func(or *WhereBuilder) {
				or.Where("Code", "A001").
					Where("Price", ">=", 100)
			})
	})
	if condition.Op != "group" || condition.Bool != "and" || len(condition.Conditions) != 2 {
		t.Fatalf("unexpected group %#v", condition)
	}
	if condition.Conditions[1].Bool != "or" || condition.Conditions[1].Op != "group" {
		t.Fatalf("unexpected nested group %#v", condition.Conditions[1])
	}
}
