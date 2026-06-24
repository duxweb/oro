package oro

import "time"

type Model struct {
	ID        uint64
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt Null[time.Time]

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
	Define(s *SchemaBuilder)
}
