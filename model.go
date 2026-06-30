package oro

import "time"

// Model provides the default unsigned primary key and automatic timestamps.
//
// Embed Model in application structs when the conventional ID, CreatedAt, and
// UpdatedAt fields are desired.
type Model struct {
	ID        uint64
	CreatedAt time.Time
	UpdatedAt time.Time

	state *modelState
}

type modelState struct {
	relations map[string]loadedRelation
}

type loadedRelation struct {
	one    any
	many   []any
	loaded bool
}

func (model Model) relationState() *modelState {
	return model.state
}

func (model *Model) ensureRelationState() *modelState {
	if model.state == nil {
		model.state = &modelState{}
	}
	return model.state
}

func (model *Model) ensureRelationMap() *modelState {
	state := model.ensureRelationState()
	if state.relations == nil {
		state.relations = map[string]loadedRelation{}
	}
	return state
}

type Definer interface {
	// Define declares the model schema.
	Define(s *SchemaBuilder)
}

// EmbeddedFields marks a struct as an embedded field bundle for Oro schema parsing.
type EmbeddedFields interface {
	OroEmbeddedFields()
}

// EmbeddedFieldDefiner declares schema fields for an embedded field bundle.
type EmbeddedFieldDefiner interface {
	EmbeddedFields
	DefineOroFields(s *SchemaBuilder)
}
