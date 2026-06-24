package oro

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
)

func loadModelRelations[T any](ctx context.Context, db *DB, schema *ModelSchema, models []*T, with []WithSpec) error {
	values := make([]any, 0, len(models))
	for _, model := range models {
		values = append(values, model)
	}
	return loadAnyModelRelations(ctx, db, schema, values, with)
}

func resolveWithRelation(schema *ModelSchema, item WithSpec) (RelationSchema, error) {
	if item.Relation != nil {
		return *item.Relation, nil
	}
	for _, relation := range schema.Relations {
		if relation.Name == item.Name {
			return relation, nil
		}
	}
	return RelationSchema{}, &Error{Op: "with", Kind: ErrUnknownRelation, Model: schema.Name, Relation: item.Name}
}

func loadBelongsTo(ctx context.Context, db *DB, sourceSchema *ModelSchema, models []any, relation RelationSchema, callback func(*RelationQuery)) error {
	targetSchema, ok := db.runtime.Registry.GetIdentifier(relation.TargetModel)
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownRelation, Model: relation.TargetModel, Relation: relation.Name}
	}
	sourceField, ok := sourceSchema.FieldByGo[relation.ForeignKey]
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownField, Model: sourceSchema.Name, Relation: relation.Name, Field: relation.ForeignKey}
	}
	targetField, ok := targetSchema.FieldByGo[relation.ReferenceKey]
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownField, Model: targetSchema.Name, Relation: relation.Name, Field: relation.ReferenceKey}
	}

	keys, err := modelFieldValues(models, relation.ForeignKey)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		for _, model := range models {
			if err := setLoadedRelation(model, relation.Name, loadedRelation{loaded: true}); err != nil {
				return err
			}
		}
		return nil
	}
	rows, nestedWith, err := relationRows(ctx, db, targetSchema, targetField.Column, keys, callback)
	if err != nil {
		return err
	}
	byKey := map[any]any{}
	for _, row := range rows {
		key := comparableKey(row[targetField.Column])
		if key == nil {
			continue
		}
		target, err := newMappedModel(db, targetSchema, row)
		if err != nil {
			return err
		}
		byKey[key] = target
	}
	if len(nestedWith) > 0 {
		targets := make([]any, 0, len(byKey))
		for _, target := range byKey {
			targets = append(targets, target)
		}
		if err := loadAnyModelRelations(ctx, db, targetSchema, targets, nestedWith); err != nil {
			return err
		}
	}
	for _, model := range models {
		sourceKey, err := modelFieldValue(model, sourceField.Name)
		if err != nil {
			return err
		}
		loaded := loadedRelation{one: byKey[comparableKey(sourceKey)], loaded: true}
		if err := setLoadedRelation(model, relation.Name, loaded); err != nil {
			return err
		}
	}
	return nil
}

func loadHas(ctx context.Context, db *DB, sourceSchema *ModelSchema, models []any, relation RelationSchema, callback func(*RelationQuery), many bool) error {
	targetSchema, ok := db.runtime.Registry.GetIdentifier(relation.TargetModel)
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownRelation, Model: relation.TargetModel, Relation: relation.Name}
	}
	sourceField, ok := sourceSchema.FieldByGo[relation.ReferenceKey]
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownField, Model: sourceSchema.Name, Relation: relation.Name, Field: relation.ReferenceKey}
	}
	targetField, ok := targetSchema.FieldByGo[relation.ForeignKey]
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownField, Model: targetSchema.Name, Relation: relation.Name, Field: relation.ForeignKey}
	}

	keys, err := modelFieldValues(models, relation.ReferenceKey)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		for _, model := range models {
			if err := setLoadedRelation(model, relation.Name, loadedRelation{many: []any{}, loaded: true}); err != nil {
				return err
			}
		}
		return nil
	}
	rows, nestedWith, err := relationRows(ctx, db, targetSchema, targetField.Column, keys, callback)
	if err != nil {
		return err
	}
	grouped := map[any][]any{}
	targets := []any{}
	for _, row := range rows {
		key := comparableKey(row[targetField.Column])
		if key == nil {
			continue
		}
		target, err := newMappedModel(db, targetSchema, row)
		if err != nil {
			return err
		}
		targets = append(targets, target)
		grouped[key] = append(grouped[key], target)
	}
	if len(nestedWith) > 0 {
		if err := loadAnyModelRelations(ctx, db, targetSchema, targets, nestedWith); err != nil {
			return err
		}
	}
	for _, model := range models {
		sourceKey, err := modelFieldValue(model, sourceField.Name)
		if err != nil {
			return err
		}
		items := grouped[comparableKey(sourceKey)]
		if items == nil {
			items = []any{}
		}
		loaded := loadedRelation{many: items, loaded: true}
		if !many && len(items) > 0 {
			loaded.one = items[0]
			loaded.many = nil
		}
		if err := setLoadedRelation(model, relation.Name, loaded); err != nil {
			return err
		}
	}
	return nil
}

func relationRows(ctx context.Context, db *DB, schema *ModelSchema, column string, keys []any, callback func(*RelationQuery)) ([]Map, []WithSpec, error) {
	return relationRowsWithConditions(ctx, db, schema, []Condition{{Field: column, Op: "in_values", Value: keys}}, callback)
}

func relationRowsWithConditions(ctx context.Context, db *DB, schema *ModelSchema, conditions []Condition, callback func(*RelationQuery)) ([]Map, []WithSpec, error) {
	query := &RelationQuery{db: db, schema: schema, spec: QuerySpec{
		Connection: db.session.connection,
		Table:      schema.Table,
	}}
	if err := applyTenantModelConnection(ctx, db, schema, &query.spec); err != nil {
		return nil, nil, err
	}
	if callback != nil {
		callback(query)
		if query.spec.SelectErr != nil {
			return nil, nil, query.spec.SelectErr
		}
		conditions, err := convertModelConditions(schema, query.spec.Where)
		if err != nil {
			return nil, nil, err
		}
		query.spec.Where = conditions
		if err := convertModelSelects(schema, &query.spec); err != nil {
			return nil, nil, err
		}
	}
	if err := applyShardConnection(ctx, db, schema, &query.spec, query.shard, query.allShards); err != nil {
		return nil, nil, err
	}
	query.spec.Where = append(append([]Condition(nil), conditions...), query.spec.Where...)
	if err := applyTenantScope(db, schema, &query.spec); err != nil {
		return nil, nil, err
	}
	var rows []Map
	var err error
	if query.allShards {
		rows, err = queryModelAllShardRows(ctx, db, query.spec)
	} else {
		rows, err = queryRows(ctx, db, query.spec)
	}
	return rows, query.spec.With, err
}

func loadAnyModelRelations(ctx context.Context, db *DB, schema *ModelSchema, models []any, with []WithSpec) error {
	if len(models) == 0 || len(with) == 0 {
		return nil
	}
	for _, item := range with {
		relation, err := resolveWithRelation(schema, item)
		if err != nil {
			return err
		}
		switch relation.Kind {
		case RelationBelongsTo:
			if err := loadBelongsTo(ctx, db, schema, models, relation, item.Callback); err != nil {
				return err
			}
		case RelationHasOne:
			if err := loadHas(ctx, db, schema, models, relation, item.Callback, false); err != nil {
				return err
			}
		case RelationHasMany:
			if err := loadHas(ctx, db, schema, models, relation, item.Callback, true); err != nil {
				return err
			}
		case RelationManyToMany:
			if err := loadManyToMany(ctx, db, schema, models, relation, item.Callback); err != nil {
				return err
			}
		case RelationDynamicHasMany:
			if err := loadDynamicHasMany(ctx, db, schema, models, relation, item.Callback); err != nil {
				return err
			}
		case RelationDynamicManyToMany:
			if err := loadManyToMany(ctx, db, schema, models, relation, item.Callback); err != nil {
				return err
			}
		case RelationDynamicBelongsTo:
			if err := loadDynamicBelongsTo(ctx, db, schema, models, relation, item.Callback); err != nil {
				return err
			}
		default:
			return &Error{Op: "with", Kind: ErrUnsupported, Model: schema.Name, Relation: relation.Name}
		}
	}
	return nil
}

func loadDynamicBelongsTo(ctx context.Context, db *DB, sourceSchema *ModelSchema, models []any, relation RelationSchema, callback func(*RelationQuery)) error {
	sourceIDField, err := schemaField(sourceSchema, relation.IDField, "with", relation.Name)
	if err != nil {
		return err
	}
	sourceTypeField, err := schemaField(sourceSchema, relation.TypeField, "with", relation.Name)
	if err != nil {
		return err
	}
	groupedIDs := map[string][]any{}
	modelKeys := map[any]string{}
	for _, model := range models {
		idValue, err := modelFieldValue(model, sourceIDField.Name)
		if err != nil {
			return err
		}
		typeValue, err := modelFieldValue(model, sourceTypeField.Name)
		if err != nil {
			return err
		}
		typeName, ok := typeValue.(string)
		if !ok || typeName == "" {
			continue
		}
		idKey := comparableKey(idValue)
		if idKey == nil {
			continue
		}
		modelKeys[model] = dynamicRelationKey(typeName, idKey)
		groupedIDs[typeName] = appendUniqueComparable(groupedIDs[typeName], idValue)
	}
	targetsByKey := map[string]any{}
	for typeName, ids := range groupedIDs {
		targetSchema, ok := dynamicRelationSchemaForType(db, typeName)
		if !ok {
			continue
		}
		targetPrimary, err := singlePrimaryField(targetSchema, "with", relation.Name)
		if err != nil {
			return err
		}
		rows, nestedWith, err := relationRows(ctx, db, targetSchema, targetPrimary.Column, ids, callback)
		if err != nil {
			return err
		}
		targets := []any{}
		for _, row := range rows {
			key := comparableKey(row[targetPrimary.Column])
			if key == nil {
				continue
			}
			target, err := newMappedModel(db, targetSchema, row)
			if err != nil {
				return err
			}
			targets = append(targets, target)
			targetsByKey[dynamicRelationKey(typeName, key)] = target
		}
		if len(nestedWith) > 0 {
			if err := loadAnyModelRelations(ctx, db, targetSchema, targets, nestedWith); err != nil {
				return err
			}
		}
	}
	for _, model := range models {
		if err := setLoadedRelation(model, relation.Name, loadedRelation{
			one:    targetsByKey[modelKeys[model]],
			loaded: true,
		}); err != nil {
			return err
		}
	}
	return nil
}

func dynamicRelationSchemaForType(db *DB, typeName string) (*ModelSchema, bool) {
	if db == nil || db.runtime == nil || db.runtime.Registry == nil {
		return nil, false
	}
	if schema, ok := db.runtime.Registry.GetIdentifier(typeName); ok {
		return schema, true
	}
	for _, schema := range db.runtime.Registry.Schemas() {
		typ, ok := db.runtime.Registry.TypeForSchema(schema)
		if ok && modelIdentifier(typ) == typeName {
			return schema, true
		}
	}
	return nil, false
}

func dynamicRelationKey(typeName string, id any) string {
	return typeName + "\x00" + stringifyComparableKey(id)
}

func stringifyComparableKey(value any) string {
	key := comparableKey(value)
	if key == nil {
		return ""
	}
	return fmt.Sprintf("%T:%v", key, key)
}

func appendUniqueComparable(values []any, value any) []any {
	key := comparableKey(value)
	if key == nil {
		return values
	}
	for _, item := range values {
		if comparableKey(item) == key {
			return values
		}
	}
	return append(values, value)
}

func loadDynamicHasMany(ctx context.Context, db *DB, sourceSchema *ModelSchema, models []any, relation RelationSchema, callback func(*RelationQuery)) error {
	targetSchema, ok := db.runtime.Registry.GetIdentifier(relation.TargetModel)
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownRelation, Model: relation.TargetModel, Relation: relation.Name}
	}
	sourcePrimary, err := singlePrimaryField(sourceSchema, "with", relation.Name)
	if err != nil {
		return err
	}
	targetIDField, err := schemaField(targetSchema, relation.IDField, "with", relation.Name)
	if err != nil {
		return err
	}
	targetTypeField, err := schemaField(targetSchema, relation.TypeField, "with", relation.Name)
	if err != nil {
		return err
	}
	keys, err := modelFieldValues(models, sourcePrimary.Name)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		for _, model := range models {
			if err := setLoadedRelation(model, relation.Name, loadedRelation{many: []any{}, loaded: true}); err != nil {
				return err
			}
		}
		return nil
	}
	rows, nestedWith, err := relationRowsWithConditions(ctx, db, targetSchema, []Condition{
		{Field: targetIDField.Column, Op: "in_values", Value: keys},
		{Field: targetTypeField.Column, Op: "=", Value: relation.TypeValue},
	}, callback)
	if err != nil {
		return err
	}
	grouped := map[any][]any{}
	targets := []any{}
	for _, row := range rows {
		key := comparableKey(row[targetIDField.Column])
		if key == nil {
			continue
		}
		target, err := newMappedModel(db, targetSchema, row)
		if err != nil {
			return err
		}
		targets = append(targets, target)
		grouped[key] = append(grouped[key], target)
	}
	if len(nestedWith) > 0 {
		if err := loadAnyModelRelations(ctx, db, targetSchema, targets, nestedWith); err != nil {
			return err
		}
	}
	for _, model := range models {
		sourceKey, err := modelFieldValue(model, sourcePrimary.Name)
		if err != nil {
			return err
		}
		items := grouped[comparableKey(sourceKey)]
		if items == nil {
			items = []any{}
		}
		if err := setLoadedRelation(model, relation.Name, loadedRelation{many: items, loaded: true}); err != nil {
			return err
		}
	}
	return nil
}

func loadManyToMany(ctx context.Context, db *DB, sourceSchema *ModelSchema, models []any, relation RelationSchema, callback func(*RelationQuery)) error {
	targetSchema, ok := db.runtime.Registry.GetIdentifier(relation.TargetModel)
	if !ok {
		return &Error{Op: "with", Kind: ErrUnknownRelation, Model: relation.TargetModel, Relation: relation.Name}
	}
	sourceFieldName := "ID"
	if len(sourceSchema.Primary) > 0 {
		sourceFieldName = sourceSchema.Primary[0]
	}
	sourceKeys, err := modelFieldValues(models, sourceFieldName)
	if err != nil {
		return err
	}
	if len(sourceKeys) == 0 {
		for _, model := range models {
			if err := setLoadedRelation(model, relation.Name, loadedRelation{many: []any{}, loaded: true}); err != nil {
				return err
			}
		}
		return nil
	}

	throughSpec := QuerySpec{
		Connection: db.session.connection,
		Table:      relation.Through,
		Where:      []Condition{{Field: Snake(relation.SourceForeignKey), Op: "in_values", Value: sourceKeys}},
	}
	if relation.Kind == RelationDynamicManyToMany {
		throughSpec.Where = append(throughSpec.Where, Condition{
			Field: Snake(relation.SourceTypeField),
			Op:    "=",
			Value: relation.SourceTypeValue,
		})
	}
	if err := applyTenantModelConnection(ctx, db, sourceSchema, &throughSpec); err != nil {
		return err
	}
	if err := applyShardConnection(ctx, db, sourceSchema, &throughSpec, nil, false); err != nil {
		return err
	}
	throughRows, err := queryRows(ctx, db, throughSpec)
	if err != nil {
		return err
	}
	sourceToTarget := map[any][]any{}
	targetKeySet := map[any]struct{}{}
	targetKeys := []any{}
	for _, row := range throughRows {
		sourceKey := comparableKey(row[Snake(relation.SourceForeignKey)])
		targetKey := comparableKey(row[Snake(relation.TargetForeignKey)])
		if sourceKey == nil || targetKey == nil {
			continue
		}
		sourceToTarget[sourceKey] = append(sourceToTarget[sourceKey], targetKey)
		if _, ok := targetKeySet[targetKey]; !ok {
			targetKeySet[targetKey] = struct{}{}
			targetKeys = append(targetKeys, row[Snake(relation.TargetForeignKey)])
		}
	}
	if len(targetKeys) == 0 {
		for _, model := range models {
			if err := setLoadedRelation(model, relation.Name, loadedRelation{many: []any{}, loaded: true}); err != nil {
				return err
			}
		}
		return nil
	}

	targetPrimary := "ID"
	if len(targetSchema.Primary) > 0 {
		targetPrimary = targetSchema.Primary[0]
	}
	targetField := targetSchema.FieldByGo[targetPrimary]
	rows, nestedWith, err := relationRows(ctx, db, targetSchema, targetField.Column, targetKeys, callback)
	if err != nil {
		return err
	}
	targetByKey := map[any]any{}
	targets := []any{}
	for _, row := range rows {
		key := comparableKey(row[targetField.Column])
		if key == nil {
			continue
		}
		target, err := newMappedModel(db, targetSchema, row)
		if err != nil {
			return err
		}
		targetByKey[key] = target
		targets = append(targets, target)
	}
	if len(nestedWith) > 0 {
		if err := loadAnyModelRelations(ctx, db, targetSchema, targets, nestedWith); err != nil {
			return err
		}
	}
	for _, model := range models {
		sourceValue, err := modelFieldValue(model, sourceFieldName)
		if err != nil {
			return err
		}
		items := []any{}
		for _, targetKey := range sourceToTarget[comparableKey(sourceValue)] {
			if target, ok := targetByKey[targetKey]; ok {
				items = append(items, target)
			}
		}
		if err := setLoadedRelation(model, relation.Name, loadedRelation{many: items, loaded: true}); err != nil {
			return err
		}
	}
	return nil
}

func newMappedModel(db *DB, schema *ModelSchema, row Map) (any, error) {
	typ, ok := db.runtime.Registry.TypeForSchema(schema)
	if !ok {
		return nil, &Error{Op: "with", Kind: ErrInvalidArgument, Model: schema.Name}
	}
	value := reflect.New(typ).Interface()
	if err := db.runtime.Mapper.MapModel(schema, row, value); err != nil {
		return nil, err
	}
	return value, nil
}

func modelFieldValues(models []any, field string) ([]any, error) {
	values := make([]any, 0, len(models))
	seen := map[any]struct{}{}
	for _, model := range models {
		value, err := modelFieldValue(model, field)
		if err != nil {
			return nil, err
		}
		key := comparableKey(value)
		if key == nil {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, value)
	}
	return values, nil
}

func modelFieldValue(model any, field string) (any, error) {
	value := reflect.ValueOf(model)
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, &Error{Op: "with", Kind: ErrInvalidArgument}
		}
		value = value.Elem()
	}
	fieldValue := value.FieldByName(field)
	if !fieldValue.IsValid() {
		return nil, &Error{Op: "with", Kind: ErrUnknownField, Field: field}
	}
	return fieldValue.Interface(), nil
}

func setLoadedRelation(model any, name string, loaded loadedRelation) error {
	value := reflect.ValueOf(model)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return &Error{Op: "with", Kind: ErrInvalidArgument}
	}
	structValue := value.Elem()
	modelField := structValue.FieldByName("Model")
	if !modelField.IsValid() || !modelField.CanAddr() || modelField.Type() != reflect.TypeOf(Model{}) {
		return &Error{Op: "with", Kind: ErrInvalidArgument}
	}
	state := modelField.Addr().Interface().(*Model).ensureRelationMap()
	state.relations[name] = loaded
	return nil
}

func comparableKey(value any) any {
	if value == nil {
		return nil
	}
	switch typedValue := value.(type) {
	case int:
		return strconv.FormatInt(int64(typedValue), 10)
	case int8:
		return strconv.FormatInt(int64(typedValue), 10)
	case int16:
		return strconv.FormatInt(int64(typedValue), 10)
	case int32:
		return strconv.FormatInt(int64(typedValue), 10)
	case int64:
		return strconv.FormatInt(typedValue, 10)
	case uint:
		return strconv.FormatUint(uint64(typedValue), 10)
	case uint8:
		return strconv.FormatUint(uint64(typedValue), 10)
	case uint16:
		return strconv.FormatUint(uint64(typedValue), 10)
	case uint32:
		return strconv.FormatUint(uint64(typedValue), 10)
	case uint64:
		return strconv.FormatUint(typedValue, 10)
	}
	reflectValue := reflect.ValueOf(value)
	if !reflectValue.IsValid() || !reflectValue.Type().Comparable() {
		return nil
	}
	return value
}
