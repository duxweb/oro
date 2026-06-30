package oro

import "github.com/duxweb/oro/internal/meta"

const (
	// RelationBelongsTo identifies an inverse one-to-one or many-to-one relation.
	RelationBelongsTo = meta.RelationBelongsTo
	// RelationHasOne identifies a one-to-one relation.
	RelationHasOne = meta.RelationHasOne
	// RelationHasMany identifies a one-to-many relation.
	RelationHasMany = meta.RelationHasMany
	// RelationManyToMany identifies a many-to-many relation through a pivot table.
	RelationManyToMany        = meta.RelationManyToMany
	RelationDynamicBelongsTo  = meta.RelationDynamicBelongsTo
	RelationDynamicHasMany    = meta.RelationDynamicHasMany
	RelationDynamicManyToMany = meta.RelationDynamicManyToMany
)

// RelationKind is the kind identifier for a model relation.
type RelationKind = meta.RelationKind

// RelationSchema is the parsed metadata for a model relation.
type RelationSchema = meta.RelationSchema

// Relation describes a model relation and carries loaded relation state.
type Relation struct {
	schema RelationSchema
	state  *modelState
	source any
}

// BelongsTo creates a belongs-to relation.
func BelongsTo(source any, name string, target string) Relation {
	return newRelation(RelationBelongsTo, source, name, target)
}

// HasOne creates a has-one relation.
func HasOne(source any, name string, target string) Relation {
	return newRelation(RelationHasOne, source, name, target)
}

// HasMany creates a has-many relation.
func HasMany(source any, name string, target string) Relation {
	return newRelation(RelationHasMany, source, name, target)
}

// ManyToMany creates a many-to-many relation.
func ManyToMany(source any, name string, target string) Relation {
	return newRelation(RelationManyToMany, source, name, target)
}

// DynamicBelongsTo creates a polymorphic belongs-to relation.
func DynamicBelongsTo(source any, name string) Relation {
	return newRelation(RelationDynamicBelongsTo, source, name, "")
}

// DynamicHasMany creates a polymorphic has-many relation.
func DynamicHasMany(source any, name string, target string) Relation {
	return newRelation(RelationDynamicHasMany, source, name, target)
}

// DynamicManyToMany creates a polymorphic many-to-many relation.
func DynamicManyToMany(source any, name string, target string) Relation {
	return newRelation(RelationDynamicManyToMany, source, name, target)
}

func newRelation(kind RelationKind, source any, name string, target string) Relation {
	return Relation{schema: RelationSchema{
		Kind:        kind,
		Name:        name,
		SourceModel: modelType(source).Name(),
		TargetModel: target,
	}, state: relationStateFromSource(source), source: source}
}

// ForeignKey sets the foreign key field for the relation.
func (relation Relation) ForeignKey(field string) Relation {
	relation.schema.ForeignKey = field
	return relation
}

// ReferenceKey sets the referenced key field for the relation.
func (relation Relation) ReferenceKey(field string) Relation {
	relation.schema.ReferenceKey = field
	return relation
}

// Through sets the pivot table name for a many-to-many relation.
func (relation Relation) Through(table string) Relation {
	relation.schema.Through = table
	return relation
}

// SourceForeignKey sets the source key column on a pivot relation.
func (relation Relation) SourceForeignKey(field string) Relation {
	relation.schema.SourceForeignKey = field
	return relation
}

// TargetForeignKey sets the target key column on a pivot relation.
func (relation Relation) TargetForeignKey(field string) Relation {
	relation.schema.TargetForeignKey = field
	return relation
}

// IDField sets the polymorphic id field.
func (relation Relation) IDField(field string) Relation {
	relation.schema.IDField = field
	return relation
}

// TypeField sets the polymorphic type discriminator field.
func (relation Relation) TypeField(field string) Relation {
	relation.schema.TypeField = field
	return relation
}

// TypeValue sets the polymorphic type discriminator value.
func (relation Relation) TypeValue(value string) Relation {
	relation.schema.TypeValue = value
	return relation
}

// SourceType sets the source-side polymorphic discriminator field and value.
func (relation Relation) SourceType(field string, value string) Relation {
	relation.schema.SourceTypeField = field
	relation.schema.SourceTypeValue = value
	return relation
}

// JSONName sets the serialized name for the relation.
func (relation Relation) JSONName(name string) Relation {
	relation.schema.JSONName = name
	return relation
}

func (relation Relation) relationSchema() RelationSchema {
	return relation.schema
}

// One returns a preloaded single related model.
func (relation Relation) One[T any]() (*T, error) {
	loaded, ok := relation.loaded()
	if !ok {
		return nil, &Error{Op: "relation.one", Kind: ErrRelationNotLoaded, Relation: relation.schema.Name}
	}
	if loaded.one == nil {
		return nil, nil
	}
	value, ok := loaded.one.(*T)
	if !ok {
		return nil, &Error{Op: "relation.one", Kind: ErrInvalidArgument, Relation: relation.schema.Name}
	}
	return value, nil
}

// Many returns preloaded related models.
func (relation Relation) Many[T any]() ([]*T, error) {
	loaded, ok := relation.loaded()
	if !ok {
		return nil, &Error{Op: "relation.many", Kind: ErrRelationNotLoaded, Relation: relation.schema.Name}
	}
	values := make([]*T, 0, len(loaded.many))
	for _, item := range loaded.many {
		value, ok := item.(*T)
		if !ok {
			return nil, &Error{Op: "relation.many", Kind: ErrInvalidArgument, Relation: relation.schema.Name}
		}
		values = append(values, value)
	}
	return values, nil
}

func (relation Relation) loaded() (loadedRelation, bool) {
	if relation.state == nil || relation.state.relations == nil {
		return loadedRelation{}, false
	}
	loaded, ok := relation.state.relations[relation.schema.Name]
	if !ok || !loaded.loaded {
		return loadedRelation{}, false
	}
	return loaded, true
}

func relationStateFromSource(source any) *modelState {
	if provider, ok := source.(interface{ relationState() *modelState }); ok {
		return provider.relationState()
	}
	return nil
}
