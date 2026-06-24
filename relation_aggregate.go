package oro

import (
	"context"
	"strings"
)

type RelationAggregateExpr struct {
	Func     string
	Relation any
	Field    string
	Alias    string
	Callback func(*RelationQuery)
}

func CountOf(relation any) RelationAggregateExpr {
	return RelationAggregateExpr{Func: "count", Relation: relation}
}

func ExistsOf(relation any) RelationAggregateExpr {
	return RelationAggregateExpr{Func: "exists", Relation: relation}
}

func SumOf(relation any, field string) RelationAggregateExpr {
	return RelationAggregateExpr{Func: "sum", Relation: relation, Field: field}
}

func AvgOf(relation any, field string) RelationAggregateExpr {
	return RelationAggregateExpr{Func: "avg", Relation: relation, Field: field}
}

func MinOf(relation any, field string) RelationAggregateExpr {
	return RelationAggregateExpr{Func: "min", Relation: relation, Field: field}
}

func MaxOf(relation any, field string) RelationAggregateExpr {
	return RelationAggregateExpr{Func: "max", Relation: relation, Field: field}
}

func (expr RelationAggregateExpr) As(alias string) RelationAggregateExpr {
	expr.Alias = alias
	return expr
}

func (expr RelationAggregateExpr) Filter(fn func(*RelationQuery)) RelationAggregateExpr {
	expr.Callback = fn
	return expr
}

func (query *ModelQuery[T]) WithCount(relation any, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.withRelationAggregate(CountOf(relation), callbacks...)
}

func (query *ModelQuery[T]) WithExists(relation any, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.withRelationAggregate(ExistsOf(relation), callbacks...)
}

func (query *ModelQuery[T]) WithSum(relation any, field string, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.withRelationAggregate(SumOf(relation, field), callbacks...)
}

func (query *ModelQuery[T]) WithAvg(relation any, field string, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.withRelationAggregate(AvgOf(relation, field), callbacks...)
}

func (query *ModelQuery[T]) WithMin(relation any, field string, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.withRelationAggregate(MinOf(relation, field), callbacks...)
}

func (query *ModelQuery[T]) WithMax(relation any, field string, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.withRelationAggregate(MaxOf(relation, field), callbacks...)
}

func (query *ModelQuery[T]) withRelationAggregate(expr RelationAggregateExpr, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	clone := *query
	if len(clone.spec.Select) == 0 {
		schema, err := schemaForModel[T](query.db)
		if err != nil {
			clone.spec.SelectErr = err
			return &clone
		}
		clone.spec.Select = defaultModelSelectExprs(schema)
	}
	if len(callbacks) > 0 {
		expr.Callback = callbacks[0]
	}
	if expr.Alias == "" {
		schema, err := schemaForModel[T](query.db)
		if err != nil {
			clone.spec.SelectErr = err
			return &clone
		}
		relation, err := resolveRelationFilterSchema(schema, expr.Relation)
		if err != nil {
			clone.spec.SelectErr = err
			return &clone
		}
		expr.Alias = defaultRelationAggregateAlias(relation, expr)
	}
	exprs, err := selectExprs([]any{expr})
	if err != nil {
		clone.spec.SelectErr = err
		return &clone
	}
	clone.spec.Select = append(clone.spec.Select, exprs...)
	return &clone
}

func defaultModelSelectExprs(schema *ModelSchema) []SelectExpr {
	selects := make([]SelectExpr, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Hidden || field.Virtual {
			continue
		}
		selects = append(selects, SelectExpr{Expr: field.Name})
	}
	return selects
}

func defaultRelationAggregateAlias(relation RelationSchema, expr RelationAggregateExpr) string {
	switch expr.Func {
	case "count":
		return Snake(relation.Name + "Count")
	case "exists":
		return Snake(relation.Name + "Exists")
	default:
		return Snake(relation.Name + strings.Title(expr.Field) + strings.Title(expr.Func))
	}
}

func resolveModelRelationAggregates(db *DB, sourceSchema *ModelSchema, spec *QuerySpec) error {
	for index := range spec.Select {
		if spec.Select[index].Expr != "__oro_relation_aggregate__" || len(spec.Select[index].Args) == 0 {
			continue
		}
		expr, ok := spec.Select[index].Args[0].(RelationAggregateExpr)
		if !ok {
			return &Error{Op: "relation_aggregate", Kind: ErrInvalidArgument, Model: sourceSchema.Name}
		}
		resolved, err := buildRelationAggregateSelect(db, sourceSchema, *spec, expr)
		if err != nil {
			return err
		}
		spec.Select[index] = resolved
	}
	return nil
}

func buildRelationAggregateSelect(db *DB, sourceSchema *ModelSchema, sourceSpec QuerySpec, expr RelationAggregateExpr) (SelectExpr, error) {
	relation, err := resolveRelationFilterSchema(sourceSchema, expr.Relation)
	if err != nil {
		return SelectExpr{}, err
	}
	if expr.Alias == "" {
		expr.Alias = defaultRelationAggregateAlias(relation, expr)
	}
	targetSchema, err := relationFilterTargetSchema(db, sourceSpec, relation)
	if err != nil {
		return SelectExpr{}, err
	}
	subquery, _, err := relationFilterSubquery(context.Background(), db, sourceSchema, targetSchema, sourceSpec, relation, expr.Callback)
	if err != nil {
		return SelectExpr{}, err
	}
	if expr.Func == "exists" {
		subquery.spec.Select = []SelectExpr{{Expr: "1", Raw: true}}
		subquery.spec.Order = nil
		subquery.spec.Limit = nil
		subquery.spec.Offset = nil
		source := Query(subquery).sourceAST()
		return SelectExpr{Expr: "__oro_relation_exists__", Alias: expr.Alias, Raw: true, Args: []any{source}}, nil
	}
	selectExpr, err := relationAggregateSubquerySelect(targetSchema, expr)
	if err != nil {
		return SelectExpr{}, err
	}
	subquery.spec.Select = []SelectExpr{selectExpr}
	subquery.spec.Order = nil
	subquery.spec.Limit = nil
	subquery.spec.Offset = nil
	source := Query(subquery).sourceAST()
	return SelectExpr{Alias: expr.Alias, Source: &source}, nil
}

func relationAggregateSubquerySelect(schema *ModelSchema, expr RelationAggregateExpr) (SelectExpr, error) {
	switch expr.Func {
	case "count":
		return SelectExpr{Expr: "count(*)", Raw: true}, nil
	case "sum", "avg", "min", "max":
		field, ok := schema.FieldByGo[expr.Field]
		if !ok {
			return SelectExpr{}, &Error{Op: "relation_aggregate", Kind: ErrUnknownField, Model: schema.Name, Field: expr.Field}
		}
		return SelectExpr{Expr: aggregateSQL(expr.Func, field.Column), Raw: true}, nil
	default:
		return SelectExpr{}, &Error{Op: "relation_aggregate", Kind: ErrInvalidArgument, Model: schema.Name}
	}
}
