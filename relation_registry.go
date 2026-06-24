package oro

import "reflect"

var relationType = reflect.TypeOf(Relation{})

func registerModelRelations(registry *Registry, model any, schema *ModelSchema) error {
	relations, err := scanModelRelations(model)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	for _, relation := range relations {
		if relation.Name == "" {
			return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name}
		}
		if _, ok := seen[relation.Name]; ok {
			return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name, Relation: relation.Name}
		}
		seen[relation.Name] = struct{}{}
		if relation.SourceModel == "" {
			relation.SourceModel = schema.Name
		}
		relation.SourceTable = schema.Table
		schema.Relations = append(schema.Relations, relation)
	}
	return validateSchemaRelations(registry, schema)
}

func scanModelRelations(model any) ([]RelationSchema, error) {
	typ := modelType(model)
	value := reflect.New(typ)
	relations := []RelationSchema{}
	for index := 0; index < value.NumMethod(); index++ {
		method := value.Type().Method(index)
		if method.Type.NumOut() != 1 || method.Type.Out(0) != relationType {
			continue
		}
		if method.Type.NumIn() != 1 {
			return nil, &Error{Op: "relation", Kind: ErrInvalidArgument, Model: typ.Name(), Relation: method.Name}
		}
		results := method.Func.Call([]reflect.Value{value})
		if len(results) != 1 {
			return nil, &Error{Op: "relation", Kind: ErrInvalidArgument, Model: typ.Name(), Relation: method.Name}
		}
		relation := results[0].Interface().(Relation).relationSchema()
		if relation.Name == "" {
			relation.Name = method.Name
		}
		relations = append(relations, relation)
	}
	return relations, nil
}

func validateSchemaRelations(registry *Registry, schema *ModelSchema) error {
	for _, relation := range schema.Relations {
		if err := validateRelation(registry, schema, relation); err != nil {
			return err
		}
	}
	return nil
}

func validateRelation(registry *Registry, schema *ModelSchema, relation RelationSchema) error {
	switch relation.Kind {
	case RelationBelongsTo, RelationHasOne, RelationHasMany:
		target, err := relationTargetSchema(registry, relation)
		if err != nil {
			return err
		}
		if relation.ForeignKey == "" || relation.ReferenceKey == "" {
			return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name, Relation: relation.Name}
		}
		if relation.Kind == RelationBelongsTo {
			if _, ok := schema.FieldByGo[relation.ForeignKey]; !ok {
				return &Error{Op: "relation", Kind: ErrUnknownField, Model: schema.Name, Relation: relation.Name, Field: relation.ForeignKey}
			}
			if _, ok := target.FieldByGo[relation.ReferenceKey]; !ok {
				return &Error{Op: "relation", Kind: ErrUnknownField, Model: target.Name, Relation: relation.Name, Field: relation.ReferenceKey}
			}
			return nil
		}
		if _, ok := target.FieldByGo[relation.ForeignKey]; !ok {
			return &Error{Op: "relation", Kind: ErrUnknownField, Model: target.Name, Relation: relation.Name, Field: relation.ForeignKey}
		}
		if _, ok := schema.FieldByGo[relation.ReferenceKey]; !ok {
			return &Error{Op: "relation", Kind: ErrUnknownField, Model: schema.Name, Relation: relation.Name, Field: relation.ReferenceKey}
		}
	case RelationManyToMany:
		if _, err := relationTargetSchema(registry, relation); err != nil {
			return err
		}
		if relation.Through == "" || relation.SourceForeignKey == "" || relation.TargetForeignKey == "" {
			return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name, Relation: relation.Name}
		}
	case RelationDynamicHasMany:
		target, err := relationTargetSchema(registry, relation)
		if err != nil {
			return err
		}
		if relation.IDField == "" || relation.TypeField == "" || relation.TypeValue == "" {
			return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name, Relation: relation.Name}
		}
		if _, ok := target.FieldByGo[relation.IDField]; !ok {
			return &Error{Op: "relation", Kind: ErrUnknownField, Model: target.Name, Relation: relation.Name, Field: relation.IDField}
		}
		if _, ok := target.FieldByGo[relation.TypeField]; !ok {
			return &Error{Op: "relation", Kind: ErrUnknownField, Model: target.Name, Relation: relation.Name, Field: relation.TypeField}
		}
	case RelationDynamicBelongsTo:
		if relation.IDField == "" || relation.TypeField == "" {
			return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name, Relation: relation.Name}
		}
		if _, ok := schema.FieldByGo[relation.IDField]; !ok {
			return &Error{Op: "relation", Kind: ErrUnknownField, Model: schema.Name, Relation: relation.Name, Field: relation.IDField}
		}
		if _, ok := schema.FieldByGo[relation.TypeField]; !ok {
			return &Error{Op: "relation", Kind: ErrUnknownField, Model: schema.Name, Relation: relation.Name, Field: relation.TypeField}
		}
	case RelationDynamicManyToMany:
		if _, err := relationTargetSchema(registry, relation); err != nil {
			return err
		}
		if relation.Through == "" || relation.SourceForeignKey == "" || relation.SourceTypeField == "" || relation.SourceTypeValue == "" || relation.TargetForeignKey == "" {
			return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name, Relation: relation.Name}
		}
	default:
		return &Error{Op: "relation", Kind: ErrInvalidArgument, Model: schema.Name, Relation: relation.Name}
	}
	return nil
}

func relationTargetSchema(registry *Registry, relation RelationSchema) (*ModelSchema, error) {
	if relation.TargetModel == "" {
		return nil, &Error{Op: "relation", Kind: ErrInvalidArgument, Relation: relation.Name}
	}
	target, ok := registry.GetIdentifier(relation.TargetModel)
	if !ok {
		return nil, &Error{Op: "relation", Kind: ErrUnknownRelation, Relation: relation.Name, Model: relation.TargetModel}
	}
	return target, nil
}
