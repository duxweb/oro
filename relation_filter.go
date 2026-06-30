package oro

import (
	"context"
	"strings"
)

type relationFilterOptions struct {
	boolOp string
	not    bool
}

type relationFilterPayload struct {
	relation any
	callback func(*RelationQuery)
	options  relationFilterOptions
}

// WhereHas filters models to those having relation rows that match callbacks.
func (query *ModelQuery[T]) WhereHas(relation any, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.whereHas(relation, relationFilterOptions{boolOp: "and"}, callbacks...)
}

// OrWhereHas adds an OR relation-exists filter.
func (query *ModelQuery[T]) OrWhereHas(relation any, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.whereHas(relation, relationFilterOptions{boolOp: "or"}, callbacks...)
}

// WhereDoesntHave filters models to those without matching relation rows.
func (query *ModelQuery[T]) WhereDoesntHave(relation any, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.whereHas(relation, relationFilterOptions{boolOp: "and", not: true}, callbacks...)
}

// OrWhereDoesntHave adds an OR relation-not-exists filter.
func (query *ModelQuery[T]) OrWhereDoesntHave(relation any, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	return query.whereHas(relation, relationFilterOptions{boolOp: "or", not: true}, callbacks...)
}

func (query *ModelQuery[T]) whereHas(relation any, options relationFilterOptions, callbacks ...func(*RelationQuery)) *ModelQuery[T] {
	clone := *query
	clone.spec.Where = append(clone.spec.Where, Condition{
		Bool: options.boolOp,
		Op:   "relation_filter",
		Value: relationFilterPayload{
			relation: relation,
			callback: firstRelationCallback(callbacks),
			options:  options,
		},
	})
	return &clone
}

// WhereHas filters relation rows to those having nested relation rows.
func (query *RelationQuery) WhereHas(relation any, callbacks ...func(*RelationQuery)) *RelationQuery {
	return query.whereHas(relation, relationFilterOptions{boolOp: "and"}, callbacks...)
}

// OrWhereHas adds an OR nested relation-exists filter.
func (query *RelationQuery) OrWhereHas(relation any, callbacks ...func(*RelationQuery)) *RelationQuery {
	return query.whereHas(relation, relationFilterOptions{boolOp: "or"}, callbacks...)
}

// WhereDoesntHave filters relation rows to those without nested relation rows.
func (query *RelationQuery) WhereDoesntHave(relation any, callbacks ...func(*RelationQuery)) *RelationQuery {
	return query.whereHas(relation, relationFilterOptions{boolOp: "and", not: true}, callbacks...)
}

// OrWhereDoesntHave adds an OR nested relation-not-exists filter.
func (query *RelationQuery) OrWhereDoesntHave(relation any, callbacks ...func(*RelationQuery)) *RelationQuery {
	return query.whereHas(relation, relationFilterOptions{boolOp: "or", not: true}, callbacks...)
}

func (query *RelationQuery) whereHas(relation any, options relationFilterOptions, callbacks ...func(*RelationQuery)) *RelationQuery {
	if query.db == nil || query.schema == nil {
		query.spec.SelectErr = &Error{Op: "where_has", Kind: ErrInvalidArgument}
		return query
	}
	query.spec.Where = append(query.spec.Where, Condition{
		Bool: options.boolOp,
		Op:   "relation_filter",
		Value: relationFilterPayload{
			relation: relation,
			callback: firstRelationCallback(callbacks),
			options:  options,
		},
	})
	return query
}

func resolveRelationFilterConditions(ctx context.Context, db *DB, schema *ModelSchema, sourceSpec QuerySpec, conditions []Condition) ([]Condition, error) {
	if len(conditions) == 0 {
		return nil, nil
	}
	resolved := make([]Condition, 0, len(conditions))
	for _, condition := range conditions {
		op := strings.ToLower(strings.TrimSpace(condition.Op))
		if op == "group" || op == "not" {
			nested, err := resolveRelationFilterConditions(ctx, db, schema, sourceSpec, condition.Conditions)
			if err != nil {
				return nil, err
			}
			condition.Conditions = nested
			resolved = append(resolved, condition)
			continue
		}
		if op == "relation_filter" {
			payload, ok := condition.Value.(relationFilterPayload)
			if !ok {
				return nil, &Error{Op: "where_has", Kind: ErrInvalidArgument, Model: schema.Name}
			}
			options := payload.options
			if condition.Bool != "" {
				options.boolOp = condition.Bool
			}
			filter, err := buildRelationFilter(ctx, db, schema, sourceSpec, payload.relation, payload.callback, options)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, filter)
			continue
		}
		resolved = append(resolved, condition)
	}
	return resolved, nil
}

func buildRelationFilter(ctx context.Context, db *DB, sourceSchema *ModelSchema, sourceSpec QuerySpec, relation any, callback func(*RelationQuery), options relationFilterOptions) (Condition, error) {
	relationSchema, err := resolveRelationFilterSchema(sourceSchema, relation)
	if err != nil {
		return Condition{}, err
	}
	targetSchema, err := relationFilterTargetSchema(db, sourceSpec, relationSchema)
	if err != nil {
		return Condition{}, err
	}
	subquery, count, err := relationFilterSubquery(ctx, db, sourceSchema, targetSchema, sourceSpec, relationSchema, callback)
	if err != nil {
		return Condition{}, err
	}
	var condition Condition
	if count != nil {
		source := Query(subquery).sourceAST()
		condition = Condition{Bool: options.boolOp, Op: "count", Value: CountCondition{Source: &source, Op: count.Op, Value: count.Value}}
		if options.not {
			condition = withBool(options.boolOp, Not(conditionWithoutBool(condition)))
		}
	} else {
		condition = withBool(options.boolOp, buildExistsCondition(Query(subquery)))
		if options.not {
			condition = withBool(options.boolOp, Not(conditionWithoutBool(condition)))
		}
	}
	return condition, nil
}

func resolveRelationFilterSchema(schema *ModelSchema, relation any) (RelationSchema, error) {
	switch typedRelation := relation.(type) {
	case Relation:
		return typedRelation.relationSchema(), nil
	case string:
		if typedRelation == "" || strings.Contains(typedRelation, ".") {
			return RelationSchema{}, &Error{Op: "where_has", Kind: ErrInvalidArgument, Model: schema.Name, Relation: typedRelation}
		}
		for _, item := range schema.Relations {
			if item.Name == typedRelation {
				return item, nil
			}
		}
		return RelationSchema{}, &Error{Op: "where_has", Kind: ErrUnknownRelation, Model: schema.Name, Relation: typedRelation}
	default:
		return RelationSchema{}, &Error{Op: "where_has", Kind: ErrInvalidArgument, Model: schema.Name}
	}
}

func relationFilterTargetSchema(db *DB, sourceSpec QuerySpec, relation RelationSchema) (*ModelSchema, error) {
	if db == nil || db.runtime == nil || db.runtime.Registry == nil {
		return nil, &Error{Op: "where_has", Kind: ErrInvalidArgument, Relation: relation.Name}
	}
	switch relation.Kind {
	case RelationDynamicBelongsTo:
		return nil, &Error{Op: "where_has", Kind: ErrUnsupported, Relation: relation.Name}
	default:
		target, ok := db.runtime.Registry.GetIdentifier(relation.TargetModel)
		if !ok {
			return nil, &Error{Op: "where_has", Kind: ErrUnknownRelation, Relation: relation.Name, Model: relation.TargetModel}
		}
		return target, nil
	}
}

func relationFilterSubquery(ctx context.Context, db *DB, sourceSchema *ModelSchema, targetSchema *ModelSchema, sourceSpec QuerySpec, relation RelationSchema, callback func(*RelationQuery)) (*TableQuery, *CountCondition, error) {
	if db == nil {
		return nil, nil, &Error{Op: "where_has", Kind: ErrInvalidArgument, Relation: relation.Name}
	}
	query := db.Table(targetSchema.Table).Select(Raw("1"))
	query.spec.Connection = sourceSpec.Connection
	query.spec.ModelName = targetSchema.Name
	query.spec.Model = targetSchema
	applyModelConnection(db, targetSchema, &query.spec)
	query.spec.Connection = sourceSpec.Connection
	var count *CountCondition
	if callback != nil {
		relationQuery := &RelationQuery{db: db, schema: targetSchema, spec: query.spec}
		callback(relationQuery)
		query.spec = relationQuery.spec
		query.shard = relationQuery.shard
		query.allShards = relationQuery.allShards
		count = relationQuery.count
	}
	if query.spec.SelectErr != nil {
		return nil, nil, query.spec.SelectErr
	}
	if err := applyRelationFilterCorrelation(db, sourceSchema, targetSchema, sourceSpec, relation, query); err != nil {
		return nil, nil, err
	}
	if err := convertRelationFilterQuery(ctx, db, targetSchema, sourceSpec, query); err != nil {
		return nil, nil, err
	}
	if count != nil {
		query.spec.Select = []SelectExpr{{Expr: "count(*)", Raw: true}}
		query.spec.Order = nil
		query.spec.Limit = nil
		query.spec.Offset = nil
	}
	return query, count, nil
}

func applyRelationFilterCorrelation(db *DB, sourceSchema *ModelSchema, targetSchema *ModelSchema, sourceSpec QuerySpec, relation RelationSchema, query *TableQuery) error {
	sourceColumnPrefix := relationFilterSourceQualifier(db, sourceSchema, sourceSpec)
	targetColumnPrefix := relationFilterTargetQualifier(db, targetSchema, query.spec)
	switch relation.Kind {
	case RelationBelongsTo:
		sourceField, err := schemaField(sourceSchema, relation.ForeignKey, "where_has", relation.Name)
		if err != nil {
			return err
		}
		targetField, err := schemaField(targetSchema, relation.ReferenceKey, "where_has", relation.Name)
		if err != nil {
			return err
		}
		query.spec.Where = append([]Condition{{
			Field: targetColumnPrefix + "." + targetField.Column,
			Op:    "column",
			Value: ColumnCondition{Op: "=", Right: sourceColumnPrefix + "." + sourceField.Column},
		}}, query.spec.Where...)
	case RelationHasOne, RelationHasMany:
		sourceField, err := schemaField(sourceSchema, relation.ReferenceKey, "where_has", relation.Name)
		if err != nil {
			return err
		}
		targetField, err := schemaField(targetSchema, relation.ForeignKey, "where_has", relation.Name)
		if err != nil {
			return err
		}
		query.spec.Where = append([]Condition{{
			Field: targetColumnPrefix + "." + targetField.Column,
			Op:    "column",
			Value: ColumnCondition{Op: "=", Right: sourceColumnPrefix + "." + sourceField.Column},
		}}, query.spec.Where...)
	case RelationManyToMany:
		return applyManyToManyRelationFilterCorrelation(db, sourceSchema, targetSchema, sourceSpec, relation, query, false)
	case RelationDynamicHasMany:
		return applyDynamicHasManyRelationFilterCorrelation(db, sourceSchema, targetSchema, sourceSpec, relation, query)
	case RelationDynamicManyToMany:
		return applyManyToManyRelationFilterCorrelation(db, sourceSchema, targetSchema, sourceSpec, relation, query, true)
	default:
		return &Error{Op: "where_has", Kind: ErrUnsupported, Relation: relation.Name}
	}
	return nil
}

func applyDynamicHasManyRelationFilterCorrelation(db *DB, sourceSchema *ModelSchema, targetSchema *ModelSchema, sourceSpec QuerySpec, relation RelationSchema, query *TableQuery) error {
	sourcePrimary, err := singlePrimaryField(sourceSchema, "where_has", relation.Name)
	if err != nil {
		return err
	}
	targetIDField, err := schemaField(targetSchema, relation.IDField, "where_has", relation.Name)
	if err != nil {
		return err
	}
	targetTypeField, err := schemaField(targetSchema, relation.TypeField, "where_has", relation.Name)
	if err != nil {
		return err
	}
	sourceColumnPrefix := relationFilterSourceQualifier(db, sourceSchema, sourceSpec)
	targetColumnPrefix := relationFilterTargetQualifier(db, targetSchema, query.spec)
	query.spec.Where = append([]Condition{
		{
			Field: targetColumnPrefix + "." + targetIDField.Column,
			Op:    "column",
			Value: ColumnCondition{Op: "=", Right: sourceColumnPrefix + "." + sourcePrimary.Column},
		},
		{
			Field: targetColumnPrefix + "." + targetTypeField.Column,
			Op:    "=",
			Value: relation.TypeValue,
		},
	}, query.spec.Where...)
	return nil
}

func applyManyToManyRelationFilterCorrelation(db *DB, sourceSchema *ModelSchema, targetSchema *ModelSchema, sourceSpec QuerySpec, relation RelationSchema, query *TableQuery, dynamic bool) error {
	sourcePrimary, err := singlePrimaryField(sourceSchema, "where_has", relation.Name)
	if err != nil {
		return err
	}
	targetPrimary, err := singlePrimaryField(targetSchema, "where_has", relation.Name)
	if err != nil {
		return err
	}
	throughAlias := relationFilterThroughAlias(relation)
	sourceColumnPrefix := relationFilterSourceQualifier(db, sourceSchema, sourceSpec)
	targetColumnPrefix := relationFilterTargetQualifier(db, targetSchema, query.spec)
	query.spec.Joins = append([]JoinAST{{
		Type:  JoinInner,
		Table: relation.Through,
		Alias: throughAlias,
		Conditions: []JoinCondition{{
			Bool:   "and",
			Left:   targetColumnPrefix + "." + targetPrimary.Column,
			Op:     "=",
			Right:  throughAlias + "." + Snake(relation.TargetForeignKey),
			Column: true,
		}},
	}}, query.spec.Joins...)
	conditions := []Condition{{
		Field: throughAlias + "." + Snake(relation.SourceForeignKey),
		Op:    "column",
		Value: ColumnCondition{Op: "=", Right: sourceColumnPrefix + "." + sourcePrimary.Column},
	}}
	if dynamic {
		conditions = append(conditions, Condition{
			Field: throughAlias + "." + Snake(relation.SourceTypeField),
			Op:    "=",
			Value: relation.SourceTypeValue,
		})
	}
	query.spec.Where = append(conditions, query.spec.Where...)
	return nil
}

func convertRelationFilterQuery(ctx context.Context, db *DB, targetSchema *ModelSchema, sourceSpec QuerySpec, query *TableQuery) error {
	conditions, err := resolveRelationFilterConditions(ctx, db, targetSchema, query.spec, query.spec.Where)
	if err != nil {
		return err
	}
	conditions, err = convertModelConditions(targetSchema, conditions)
	if err != nil {
		return err
	}
	query.spec.Where = conditions
	if err := applyQueryExtensions(ctx, db, &query.spec); err != nil {
		return err
	}
	if err := convertModelSelects(targetSchema, &query.spec); err != nil {
		return err
	}
	if err := convertRelationFilterJoins(targetSchema, query); err != nil {
		return err
	}
	if err := applyShardConnection(ctx, db, targetSchema, &query.spec, query.shard, query.allShards); err != nil {
		return err
	}
	query.spec.Connection = sourceSpec.Connection
	applyQualifiedSoftDeleteScope(targetSchema, &query.spec, relationFilterTargetQualifier(db, targetSchema, query.spec), softDeleteDefault)
	return nil
}

func convertRelationFilterJoins(schema *ModelSchema, query *TableQuery) error {
	for joinIndex := range query.spec.Joins {
		for conditionIndex := range query.spec.Joins[joinIndex].Conditions {
			condition, err := convertRelationFilterJoinCondition(schema, query.spec.Joins[joinIndex].Conditions[conditionIndex])
			if err != nil {
				return err
			}
			query.spec.Joins[joinIndex].Conditions[conditionIndex] = condition
		}
	}
	return nil
}

func convertRelationFilterJoinCondition(schema *ModelSchema, condition JoinCondition) (JoinCondition, error) {
	if len(condition.Group) > 0 {
		for index := range condition.Group {
			converted, err := convertRelationFilterJoinCondition(schema, condition.Group[index])
			if err != nil {
				return JoinCondition{}, err
			}
			condition.Group[index] = converted
		}
		return condition, nil
	}
	if !isQualifiedIdentifier(condition.Left) {
		field, ok := schema.FieldByGo[condition.Left]
		if !ok {
			return JoinCondition{}, &Error{Op: "join", Kind: ErrUnknownField, Model: schema.Name, Field: condition.Left}
		}
		condition.Left = field.Column
	}
	if condition.Column && condition.Right != "" && !isQualifiedIdentifier(condition.Right) {
		field, ok := schema.FieldByGo[condition.Right]
		if !ok {
			return JoinCondition{}, &Error{Op: "join", Kind: ErrUnknownField, Model: schema.Name, Field: condition.Right}
		}
		condition.Right = field.Column
	}
	return condition, nil
}

func relationFilterSourceQualifier(db *DB, schema *ModelSchema, spec QuerySpec) string {
	return tableNames(db).Qualifier(spec, schema.Table)
}

func relationFilterTargetQualifier(db *DB, schema *ModelSchema, spec QuerySpec) string {
	return tableNames(db).Qualifier(spec, schema.Table)
}

func relationFilterThroughAlias(relation RelationSchema) string {
	alias := "__" + relation.Name + "_through"
	return Snake(alias)
}

func firstRelationCallback(callbacks []func(*RelationQuery)) func(*RelationQuery) {
	if len(callbacks) == 0 {
		return nil
	}
	return callbacks[0]
}

func conditionWithoutBool(condition Condition) Condition {
	condition.Bool = ""
	return condition
}
