package oro

import "github.com/duxweb/oro/internal/meta"

const (
	RelationBelongsTo         = meta.RelationBelongsTo
	RelationHasOne            = meta.RelationHasOne
	RelationHasMany           = meta.RelationHasMany
	RelationManyToMany        = meta.RelationManyToMany
	RelationDynamicBelongsTo  = meta.RelationDynamicBelongsTo
	RelationDynamicHasMany    = meta.RelationDynamicHasMany
	RelationDynamicManyToMany = meta.RelationDynamicManyToMany
)

type RelationKind = meta.RelationKind
type RelationSchema = meta.RelationSchema

type Relation struct {
	schema RelationSchema
	state  *modelState
	source any
}

func BelongsTo(source any, name string, target string) Relation {
	return newRelation(RelationBelongsTo, source, name, target)
}

func HasOne(source any, name string, target string) Relation {
	return newRelation(RelationHasOne, source, name, target)
}

func HasMany(source any, name string, target string) Relation {
	return newRelation(RelationHasMany, source, name, target)
}

func ManyToMany(source any, name string, target string) Relation {
	return newRelation(RelationManyToMany, source, name, target)
}

func DynamicBelongsTo(source any, name string) Relation {
	return newRelation(RelationDynamicBelongsTo, source, name, "")
}

func DynamicHasMany(source any, name string, target string) Relation {
	return newRelation(RelationDynamicHasMany, source, name, target)
}

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

func (relation Relation) ForeignKey(field string) Relation {
	relation.schema.ForeignKey = field
	return relation
}

func (relation Relation) ReferenceKey(field string) Relation {
	relation.schema.ReferenceKey = field
	return relation
}

func (relation Relation) Through(table string) Relation {
	relation.schema.Through = table
	return relation
}

func (relation Relation) SourceForeignKey(field string) Relation {
	relation.schema.SourceForeignKey = field
	return relation
}

func (relation Relation) TargetForeignKey(field string) Relation {
	relation.schema.TargetForeignKey = field
	return relation
}

func (relation Relation) IDField(field string) Relation {
	relation.schema.IDField = field
	return relation
}

func (relation Relation) TypeField(field string) Relation {
	relation.schema.TypeField = field
	return relation
}

func (relation Relation) TypeValue(value string) Relation {
	relation.schema.TypeValue = value
	return relation
}

func (relation Relation) SourceType(field string, value string) Relation {
	relation.schema.SourceTypeField = field
	relation.schema.SourceTypeValue = value
	return relation
}

func (relation Relation) JSONName(name string) Relation {
	relation.schema.JSONName = name
	return relation
}

func (relation Relation) relationSchema() RelationSchema {
	return relation.schema
}

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
