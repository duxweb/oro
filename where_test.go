package oro

import "testing"

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

	condition = Field("Code").NotIn("A001", "A002")
	if condition.Field != "Code" || condition.Op != "not_in_values" {
		t.Fatalf("unexpected not in condition %#v", condition)
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
