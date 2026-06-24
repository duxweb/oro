package oro

func relationForConditions(relation Relation, targetSchema *ModelSchema, db *DB) ([]Condition, error) {
	schema := relation.relationSchema()
	switch schema.Kind {
	case RelationBelongsTo:
		return belongsToForConditions(relation, schema)
	case RelationHasOne, RelationHasMany:
		return hasForConditions(relation, schema)
	case RelationDynamicHasMany:
		return dynamicHasForConditions(relation, schema)
	case RelationDynamicBelongsTo:
		return dynamicBelongsToForConditions(relation, schema, targetSchema, db)
	case RelationManyToMany:
		return manyToManyForConditions(relation, schema, db)
	case RelationDynamicManyToMany:
		return dynamicManyToManyForConditions(relation, schema, db)
	default:
		return nil, &Error{Op: "for", Kind: ErrUnsupported, Relation: schema.Name}
	}
}

func belongsToForConditions(relation Relation, schema RelationSchema) ([]Condition, error) {
	value, err := modelFieldValue(relation.source, schema.ForeignKey)
	if err != nil {
		return nil, err
	}
	return []Condition{{Field: schema.ReferenceKey, Op: "=", Value: value}}, nil
}

func hasForConditions(relation Relation, schema RelationSchema) ([]Condition, error) {
	value, err := modelFieldValue(relation.source, schema.ReferenceKey)
	if err != nil {
		return nil, err
	}
	return []Condition{{Field: schema.ForeignKey, Op: "=", Value: value}}, nil
}

func dynamicHasForConditions(relation Relation, schema RelationSchema) ([]Condition, error) {
	value, err := modelFieldValue(relation.source, "ID")
	if err != nil {
		return nil, err
	}
	return []Condition{
		{Field: schema.IDField, Op: "=", Value: value},
		{Field: schema.TypeField, Op: "=", Value: schema.TypeValue},
	}, nil
}

func dynamicBelongsToForConditions(relation Relation, schema RelationSchema, targetSchema *ModelSchema, db *DB) ([]Condition, error) {
	if targetSchema == nil {
		return nil, &Error{Op: "for", Kind: ErrInvalidArgument, Relation: schema.Name}
	}
	idValue, err := modelFieldValue(relation.source, schema.IDField)
	if err != nil {
		return nil, err
	}
	typeValue, err := modelFieldValue(relation.source, schema.TypeField)
	if err != nil {
		return nil, err
	}
	if !matchesDynamicTypeValue(typeValue, targetSchema, db) {
		return []Condition{
			RawCondition("1 = 0"),
		}, nil
	}
	primary, err := singlePrimaryField(targetSchema, "for", schema.Name)
	if err != nil {
		return nil, err
	}
	return []Condition{{Field: primary.Name, Op: "=", Value: idValue}}, nil
}

func matchesDynamicTypeValue(value any, schema *ModelSchema, db *DB) bool {
	typeValue, ok := value.(string)
	if !ok || schema == nil {
		return false
	}
	if typeValue == schema.Name {
		return true
	}
	if db == nil || db.runtime == nil || db.runtime.Registry == nil {
		return false
	}
	typ, ok := db.runtime.Registry.TypeForSchema(schema)
	if !ok {
		return false
	}
	return typeValue == modelIdentifier(typ)
}

func manyToManyForConditions(relation Relation, schema RelationSchema, db *DB) ([]Condition, error) {
	sourceID, err := modelFieldValue(relation.source, "ID")
	if err != nil {
		return nil, err
	}
	source := db.Table(schema.Through).
		Select(Raw(Snake(schema.TargetForeignKey))).
		Where(Snake(schema.SourceForeignKey), sourceID)
	return []Condition{{
		Field: "ID",
		Op:    "in",
		Value: sourceASTPointer(Query(source).sourceAST()),
	}}, nil
}

func dynamicManyToManyForConditions(relation Relation, schema RelationSchema, db *DB) ([]Condition, error) {
	sourceID, err := modelFieldValue(relation.source, "ID")
	if err != nil {
		return nil, err
	}
	source := db.Table(schema.Through).
		Select(Raw(Snake(schema.TargetForeignKey))).
		Where(Snake(schema.SourceForeignKey), sourceID).
		Where(Snake(schema.SourceTypeField), schema.SourceTypeValue)
	return []Condition{{
		Field: "ID",
		Op:    "in",
		Value: sourceASTPointer(Query(source).sourceAST()),
	}}, nil
}

func sourceASTPointer(source SourceAST) *SourceAST {
	return &source
}
