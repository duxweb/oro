package oro

import (
	"context"
	"reflect"
	"strings"
)

type RelationOperator struct {
	db       *DB
	relation Relation
}

type RelationItem[T any] struct {
	Model T
	Data  Map
}

func (db *DB) Relation(relation Relation) *RelationOperator {
	return &RelationOperator{db: db, relation: relation}
}

func (writer *RelationOperator) Set(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	switch schema.Kind {
	case RelationBelongsTo:
		return writer.setBelongsTo(ctx, target)
	case RelationHasOne:
		return writer.setHas(ctx, target)
	default:
		return &Error{Op: "relation.set", Kind: ErrUnsupported, Relation: schema.Name}
	}
}

func (writer *RelationOperator) Unset(ctx context.Context) error {
	schema := writer.relation.relationSchema()
	switch schema.Kind {
	case RelationBelongsTo:
		return writer.unsetBelongsTo(ctx)
	case RelationHasOne:
		return writer.unsetHasOne(ctx)
	default:
		return &Error{Op: "relation.unset", Kind: ErrUnsupported, Relation: schema.Name}
	}
}

func (writer *RelationOperator) Replace(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	if schema.Kind != RelationHasOne {
		return &Error{Op: "relation.replace", Kind: ErrUnsupported, Relation: schema.Name}
	}
	return writer.db.Transaction(ctx, func(tx *DB) error {
		operator := tx.Relation(writer.relation)
		if err := operator.Unset(ctx); err != nil {
			return err
		}
		return operator.Set(ctx, target)
	})
}

func (writer *RelationOperator) Add(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	switch schema.Kind {
	case RelationHasMany:
		return writer.setHas(ctx, target)
	case RelationDynamicHasMany:
		return writer.setDynamicHas(ctx, target)
	default:
		return &Error{Op: "relation.add", Kind: ErrUnsupported, Relation: schema.Name}
	}
}

func (writer *RelationOperator) AddMany(ctx context.Context, targets any) error {
	values, err := sliceValues(targets)
	if err != nil {
		return err
	}
	return writer.db.Transaction(ctx, func(tx *DB) error {
		operator := tx.Relation(writer.relation)
		for _, value := range values {
			if err := operator.Add(ctx, value); err != nil {
				return err
			}
		}
		return nil
	})
}

func (writer *RelationOperator) Remove(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	switch schema.Kind {
	case RelationHasMany:
		return writer.removeHas(ctx, target)
	case RelationDynamicHasMany:
		return writer.removeDynamicHas(ctx, target)
	default:
		return &Error{Op: "relation.remove", Kind: ErrUnsupported, Relation: schema.Name}
	}
}

func (writer *RelationOperator) RemoveMany(ctx context.Context, targets any) error {
	values, err := sliceValues(targets)
	if err != nil {
		return err
	}
	return writer.db.Transaction(ctx, func(tx *DB) error {
		operator := tx.Relation(writer.relation)
		for _, value := range values {
			if err := operator.Remove(ctx, value); err != nil {
				return err
			}
		}
		return nil
	})
}

func (writer *RelationOperator) Attach(ctx context.Context, target any, data ...Map) error {
	row, err := writer.throughRow(target, optionalMap(data))
	if err != nil {
		return err
	}
	schema := writer.relation.relationSchema()
	_, err = writer.db.Table(schema.Through).Create(ctx, row)
	return err
}

func (writer *RelationOperator) AttachMany(ctx context.Context, targets any) error {
	rows, err := writer.throughRows(targets)
	if err != nil {
		return err
	}
	return writer.db.Transaction(ctx, func(tx *DB) error {
		for _, row := range rows {
			if _, err := tx.Table(writer.relation.relationSchema().Through).Create(ctx, row); err != nil {
				return err
			}
		}
		return nil
	})
}

func (writer *RelationOperator) Detach(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	if !isThroughRelation(schema.Kind) {
		return &Error{Op: "relation.detach", Kind: ErrUnsupported, Relation: schema.Name}
	}
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return err
	}
	targetID, err := modelFieldValue(target, "ID")
	if err != nil {
		return err
	}
	query := writer.db.Table(schema.Through).
		Where(Snake(schema.SourceForeignKey), sourceID)
	if schema.Kind == RelationDynamicManyToMany {
		query = query.Where(Snake(schema.SourceTypeField), schema.SourceTypeValue)
	}
	_, err = query.Where(Snake(schema.TargetForeignKey), targetID).Delete(ctx)
	return err
}

func (writer *RelationOperator) DetachMany(ctx context.Context, targets any) error {
	targetIDs, err := targetIDsFromAny(targets)
	if err != nil {
		return err
	}
	return writer.db.Transaction(ctx, func(tx *DB) error {
		operator := tx.Relation(writer.relation)
		for _, targetID := range targetIDs {
			if err := operator.detachID(ctx, targetID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (writer *RelationOperator) UpdateThrough(ctx context.Context, target any, data Map) error {
	schema := writer.relation.relationSchema()
	if !isThroughRelation(schema.Kind) {
		return &Error{Op: "relation.update_through", Kind: ErrUnsupported, Relation: schema.Name}
	}
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return err
	}
	targetID, err := modelFieldValue(target, "ID")
	if err != nil {
		return err
	}
	row := Map{}
	for key, value := range data {
		row[Snake(key)] = value
	}
	query := writer.db.Table(schema.Through).
		Where(Snake(schema.SourceForeignKey), sourceID)
	if schema.Kind == RelationDynamicManyToMany {
		query = query.Where(Snake(schema.SourceTypeField), schema.SourceTypeValue)
	}
	_, err = query.Where(Snake(schema.TargetForeignKey), targetID).Update(ctx, row)
	return err
}

func (writer *RelationOperator) Sync(ctx context.Context, targets any) error {
	return writer.db.Transaction(ctx, func(tx *DB) error {
		operator := tx.Relation(writer.relation)
		if err := operator.syncDetachMissing(ctx, targets); err != nil {
			return err
		}
		return operator.SyncWithoutDetach(ctx, targets)
	})
}

func (writer *RelationOperator) SyncWithoutDetach(ctx context.Context, targets any) error {
	rows, err := writer.throughRows(targets)
	if err != nil {
		return err
	}
	existing, err := writer.existingTargetIDs(ctx)
	if err != nil {
		return err
	}
	targetColumn := Snake(writer.relation.relationSchema().TargetForeignKey)
	for _, row := range rows {
		targetKey := comparableKey(row[targetColumn])
		if _, ok := existing[targetKey]; ok {
			continue
		}
		if _, err := writer.db.Table(writer.relation.relationSchema().Through).Create(ctx, row); err != nil {
			return err
		}
		existing[targetKey] = row[targetColumn]
	}
	return nil
}

func (writer *RelationOperator) Clear(ctx context.Context) error {
	schema := writer.relation.relationSchema()
	switch schema.Kind {
	case RelationHasMany:
		return writer.clearHas(ctx)
	case RelationDynamicHasMany:
		return writer.clearDynamicHas(ctx)
	case RelationManyToMany, RelationDynamicManyToMany:
		return writer.clearThrough(ctx)
	default:
		return &Error{Op: "relation.clear", Kind: ErrUnsupported, Relation: schema.Name}
	}
}

func (writer *RelationOperator) setBelongsTo(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	sourceSchema, err := writer.sourceSchema("relation.set")
	if err != nil {
		return err
	}
	targetValue, err := modelFieldValue(target, schema.ReferenceKey)
	if err != nil {
		return err
	}
	sourceID, primary, err := writer.sourcePrimaryValue("relation.set", sourceSchema)
	if err != nil {
		return err
	}
	foreign, err := schemaField(sourceSchema, schema.ForeignKey, "relation.set", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(sourceSchema, Map{foreign.Column: targetValue})
	_, err = writer.db.Table(sourceSchema.Table).
		Where(primary.Column, sourceID).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) unsetBelongsTo(ctx context.Context) error {
	schema := writer.relation.relationSchema()
	sourceSchema, err := writer.sourceSchema("relation.unset")
	if err != nil {
		return err
	}
	sourceID, primary, err := writer.sourcePrimaryValue("relation.unset", sourceSchema)
	if err != nil {
		return err
	}
	foreign, err := schemaField(sourceSchema, schema.ForeignKey, "relation.unset", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(sourceSchema, Map{foreign.Column: nil})
	_, err = writer.db.Table(sourceSchema.Table).
		Where(primary.Column, sourceID).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) setHas(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	sourceSchema, err := writer.sourceSchema("relation.set")
	if err != nil {
		return err
	}
	targetSchema, err := writer.targetSchema("relation.set")
	if err != nil {
		return err
	}
	sourceValue, err := modelFieldValue(writer.relation.source, schema.ReferenceKey)
	if err != nil {
		return err
	}
	targetID, primary, err := targetPrimaryValue(target, "relation.set", targetSchema)
	if err != nil {
		return err
	}
	foreign, err := schemaField(targetSchema, schema.ForeignKey, "relation.set", schema.Name)
	if err != nil {
		return err
	}
	if _, err := schemaField(sourceSchema, schema.ReferenceKey, "relation.set", schema.Name); err != nil {
		return err
	}
	values := updateValuesWithTimestamps(targetSchema, Map{foreign.Column: sourceValue})
	_, err = writer.db.Table(targetSchema.Table).
		Where(primary.Column, targetID).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) unsetHasOne(ctx context.Context) error {
	schema := writer.relation.relationSchema()
	targetSchema, err := writer.targetSchema("relation.unset")
	if err != nil {
		return err
	}
	sourceValue, err := modelFieldValue(writer.relation.source, schema.ReferenceKey)
	if err != nil {
		return err
	}
	foreign, err := schemaField(targetSchema, schema.ForeignKey, "relation.unset", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(targetSchema, Map{foreign.Column: nil})
	_, err = writer.db.Table(targetSchema.Table).
		Where(foreign.Column, sourceValue).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) removeHas(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	targetSchema, err := writer.targetSchema("relation.remove")
	if err != nil {
		return err
	}
	sourceValue, err := modelFieldValue(writer.relation.source, schema.ReferenceKey)
	if err != nil {
		return err
	}
	targetID, primary, err := targetPrimaryValue(target, "relation.remove", targetSchema)
	if err != nil {
		return err
	}
	foreign, err := schemaField(targetSchema, schema.ForeignKey, "relation.remove", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(targetSchema, Map{foreign.Column: nil})
	_, err = writer.db.Table(targetSchema.Table).
		Where(primary.Column, targetID).
		Where(foreign.Column, sourceValue).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) clearHas(ctx context.Context) error {
	schema := writer.relation.relationSchema()
	targetSchema, err := writer.targetSchema("relation.clear")
	if err != nil {
		return err
	}
	sourceValue, err := modelFieldValue(writer.relation.source, schema.ReferenceKey)
	if err != nil {
		return err
	}
	foreign, err := schemaField(targetSchema, schema.ForeignKey, "relation.clear", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(targetSchema, Map{foreign.Column: nil})
	_, err = writer.db.Table(targetSchema.Table).
		Where(foreign.Column, sourceValue).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) setDynamicHas(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	targetSchema, err := writer.targetSchema("relation.add")
	if err != nil {
		return err
	}
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return err
	}
	targetID, primary, err := targetPrimaryValue(target, "relation.add", targetSchema)
	if err != nil {
		return err
	}
	idField, err := schemaField(targetSchema, schema.IDField, "relation.add", schema.Name)
	if err != nil {
		return err
	}
	typeField, err := schemaField(targetSchema, schema.TypeField, "relation.add", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(targetSchema, Map{
		idField.Column:   sourceID,
		typeField.Column: schema.TypeValue,
	})
	_, err = writer.db.Table(targetSchema.Table).
		Where(primary.Column, targetID).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) removeDynamicHas(ctx context.Context, target any) error {
	schema := writer.relation.relationSchema()
	targetSchema, err := writer.targetSchema("relation.remove")
	if err != nil {
		return err
	}
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return err
	}
	targetID, primary, err := targetPrimaryValue(target, "relation.remove", targetSchema)
	if err != nil {
		return err
	}
	idField, err := schemaField(targetSchema, schema.IDField, "relation.remove", schema.Name)
	if err != nil {
		return err
	}
	typeField, err := schemaField(targetSchema, schema.TypeField, "relation.remove", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(targetSchema, Map{
		idField.Column:   nil,
		typeField.Column: nil,
	})
	_, err = writer.db.Table(targetSchema.Table).
		Where(primary.Column, targetID).
		Where(idField.Column, sourceID).
		Where(typeField.Column, schema.TypeValue).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) clearDynamicHas(ctx context.Context) error {
	schema := writer.relation.relationSchema()
	targetSchema, err := writer.targetSchema("relation.clear")
	if err != nil {
		return err
	}
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return err
	}
	idField, err := schemaField(targetSchema, schema.IDField, "relation.clear", schema.Name)
	if err != nil {
		return err
	}
	typeField, err := schemaField(targetSchema, schema.TypeField, "relation.clear", schema.Name)
	if err != nil {
		return err
	}
	values := updateValuesWithTimestamps(targetSchema, Map{
		idField.Column:   nil,
		typeField.Column: nil,
	})
	_, err = writer.db.Table(targetSchema.Table).
		Where(idField.Column, sourceID).
		Where(typeField.Column, schema.TypeValue).
		Update(ctx, values)
	return err
}

func (writer *RelationOperator) clearThrough(ctx context.Context) error {
	schema := writer.relation.relationSchema()
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return err
	}
	query := writer.db.Table(schema.Through).
		Where(Snake(schema.SourceForeignKey), sourceID)
	if schema.Kind == RelationDynamicManyToMany {
		query = query.Where(Snake(schema.SourceTypeField), schema.SourceTypeValue)
	}
	_, err = query.Delete(ctx)
	return err
}

func (writer *RelationOperator) throughRow(target any, data Map) (Map, error) {
	schema := writer.relation.relationSchema()
	if !isThroughRelation(schema.Kind) {
		return nil, &Error{Op: "relation.attach", Kind: ErrUnsupported, Relation: schema.Name}
	}
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return nil, err
	}
	targetID, err := modelFieldValue(target, "ID")
	if err != nil {
		return nil, err
	}
	row := Map{
		Snake(schema.SourceForeignKey): sourceID,
		Snake(schema.TargetForeignKey): targetID,
	}
	if schema.Kind == RelationDynamicManyToMany {
		row[Snake(schema.SourceTypeField)] = schema.SourceTypeValue
	}
	for key, value := range data {
		row[Snake(key)] = value
	}
	return row, nil
}

func (writer *RelationOperator) throughRows(targets any) ([]Map, error) {
	values, err := sliceValues(targets)
	if err != nil {
		return nil, err
	}
	rows := make([]Map, 0, len(values))
	for _, value := range values {
		target, data, err := relationItemValue(value)
		if err != nil {
			return nil, err
		}
		row, err := writer.throughRow(target, data)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (writer *RelationOperator) syncDetachMissing(ctx context.Context, targets any) error {
	targetIDs, err := targetIDsFromAny(targets)
	if err != nil {
		return err
	}
	keep := map[any]struct{}{}
	for _, id := range targetIDs {
		keep[comparableKey(id)] = struct{}{}
	}
	existing, err := writer.existingTargetIDs(ctx)
	if err != nil {
		return err
	}
	for key, id := range existing {
		if _, ok := keep[key]; ok {
			continue
		}
		if err := writer.detachID(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (writer *RelationOperator) existingTargetIDs(ctx context.Context) (map[any]any, error) {
	schema := writer.relation.relationSchema()
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return nil, err
	}
	targetColumn := Snake(schema.TargetForeignKey)
	query := writer.db.Table(schema.Through).
		Where(Snake(schema.SourceForeignKey), sourceID)
	if schema.Kind == RelationDynamicManyToMany {
		query = query.Where(Snake(schema.SourceTypeField), schema.SourceTypeValue)
	}
	rows, err := query.Get(ctx)
	if err != nil {
		return nil, err
	}
	existing := map[any]any{}
	for _, row := range rows {
		id := row[targetColumn]
		existing[comparableKey(id)] = id
	}
	return existing, nil
}

func (writer *RelationOperator) detachID(ctx context.Context, targetID any) error {
	schema := writer.relation.relationSchema()
	if !isThroughRelation(schema.Kind) {
		return &Error{Op: "relation.detach", Kind: ErrUnsupported, Relation: schema.Name}
	}
	sourceID, err := modelFieldValue(writer.relation.source, "ID")
	if err != nil {
		return err
	}
	query := writer.db.Table(schema.Through).
		Where(Snake(schema.SourceForeignKey), sourceID)
	if schema.Kind == RelationDynamicManyToMany {
		query = query.Where(Snake(schema.SourceTypeField), schema.SourceTypeValue)
	}
	_, err = query.Where(Snake(schema.TargetForeignKey), targetID).Delete(ctx)
	return err
}

func optionalMap(values []Map) Map {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func targetIDsFromAny(targets any) ([]any, error) {
	values, err := sliceValues(targets)
	if err != nil {
		return nil, err
	}
	ids := make([]any, 0, len(values))
	for _, value := range values {
		target, _, err := relationItemValue(value)
		if err != nil {
			return nil, err
		}
		id, err := modelFieldValue(target, "ID")
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func sliceValues(values any) ([]any, error) {
	value := reflect.ValueOf(values)
	if !value.IsValid() {
		return nil, &Error{Op: "relation.items", Kind: ErrInvalidArgument}
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, &Error{Op: "relation.items", Kind: ErrInvalidArgument}
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
		return nil, &Error{Op: "relation.items", Kind: ErrInvalidArgument}
	}
	items := make([]any, 0, value.Len())
	for index := 0; index < value.Len(); index++ {
		item := value.Index(index)
		if !item.CanInterface() {
			return nil, &Error{Op: "relation.items", Kind: ErrInvalidArgument}
		}
		items = append(items, item.Interface())
	}
	return items, nil
}

func relationItemValue(value any) (any, Map, error) {
	itemValue := reflect.ValueOf(value)
	if !itemValue.IsValid() {
		return nil, nil, &Error{Op: "relation.item", Kind: ErrInvalidArgument}
	}
	for itemValue.Kind() == reflect.Pointer {
		if itemValue.IsNil() {
			return nil, nil, &Error{Op: "relation.item", Kind: ErrInvalidArgument}
		}
		itemValue = itemValue.Elem()
	}
	if itemValue.Kind() != reflect.Struct {
		return value, nil, nil
	}
	if !isRelationItemType(itemValue.Type()) {
		return value, nil, nil
	}

	modelField := itemValue.FieldByName("Model")
	dataField := itemValue.FieldByName("Data")
	if !modelField.IsValid() || !dataField.IsValid() {
		return value, nil, nil
	}
	if !modelField.CanInterface() || !dataField.CanInterface() {
		return nil, nil, &Error{Op: "relation.item", Kind: ErrInvalidArgument}
	}
	data, ok := dataField.Interface().(Map)
	if !ok {
		return nil, nil, &Error{Op: "relation.item", Kind: ErrInvalidArgument}
	}
	return modelField.Interface(), data, nil
}

func isRelationItemType(typ reflect.Type) bool {
	return typ.PkgPath() == "github.com/duxweb/oro" &&
		(typ.Name() == "RelationItem" || strings.HasPrefix(typ.Name(), "RelationItem["))
}

func isThroughRelation(kind RelationKind) bool {
	return kind == RelationManyToMany || kind == RelationDynamicManyToMany
}

func (writer *RelationOperator) sourceSchema(op string) (*ModelSchema, error) {
	schema := writer.relation.relationSchema()
	modelSchema, ok := writer.db.runtime.Registry.GetIdentifier(schema.SourceModel)
	if !ok {
		return nil, &Error{Op: op, Kind: ErrUnknownRelation, Model: schema.SourceModel, Relation: schema.Name}
	}
	return modelSchema, nil
}

func (writer *RelationOperator) targetSchema(op string) (*ModelSchema, error) {
	schema := writer.relation.relationSchema()
	modelSchema, ok := writer.db.runtime.Registry.GetIdentifier(schema.TargetModel)
	if !ok {
		return nil, &Error{Op: op, Kind: ErrUnknownRelation, Model: schema.TargetModel, Relation: schema.Name}
	}
	return modelSchema, nil
}

func (writer *RelationOperator) sourcePrimaryValue(op string, schema *ModelSchema) (any, FieldSchema, error) {
	primary, err := singlePrimaryField(schema, op, writer.relation.relationSchema().Name)
	if err != nil {
		return nil, FieldSchema{}, err
	}
	value, err := modelFieldValue(writer.relation.source, primary.Name)
	if err != nil {
		return nil, FieldSchema{}, err
	}
	return value, primary, nil
}

func targetPrimaryValue(target any, op string, schema *ModelSchema) (any, FieldSchema, error) {
	primary, err := singlePrimaryField(schema, op, "")
	if err != nil {
		return nil, FieldSchema{}, err
	}
	value, err := modelFieldValue(target, primary.Name)
	if err != nil {
		return nil, FieldSchema{}, err
	}
	return value, primary, nil
}

func singlePrimaryField(schema *ModelSchema, op string, relation string) (FieldSchema, error) {
	if schema == nil || len(schema.Primary) != 1 {
		return FieldSchema{}, &Error{Op: op, Kind: ErrInvalidArgument, Relation: relation}
	}
	return schema.FieldByGo[schema.Primary[0]], nil
}

func schemaField(schema *ModelSchema, field string, op string, relation string) (FieldSchema, error) {
	fieldSchema, ok := schema.FieldByGo[field]
	if !ok {
		return FieldSchema{}, &Error{Op: op, Kind: ErrUnknownField, Model: schema.Name, Field: field, Relation: relation}
	}
	return fieldSchema, nil
}

func updateValuesWithTimestamps(schema *ModelSchema, values Map) Map {
	updated := Map{}
	for key, value := range values {
		updated[key] = value
	}
	for column, value := range autoUpdateColumns(schema, writeOptions{}) {
		if _, ok := updated[column]; !ok {
			updated[column] = value
		}
	}
	return updated
}
