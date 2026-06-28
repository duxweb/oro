package logroll

import "github.com/duxweb/oro"

type cleanupState struct {
	PrimaryField  string
	PrimaryColumn string
	TimeField     string
	TimeColumn    string
}

func cleanupStateFor(schema *oro.ModelSchema, config Config) (cleanupState, error) {
	if len(schema.Primary) != 1 || len(schema.PrimaryColumns) != 1 {
		return cleanupState{}, &oro.Error{Op: "logroll.cleanup", Kind: oro.ErrInvalidArgument, Model: schema.Name, Field: "primary"}
	}
	state := cleanupState{PrimaryField: schema.Primary[0], PrimaryColumn: schema.PrimaryColumns[0], TimeField: config.TimeField}
	needsTime := false
	for _, policy := range config.Policies {
		if _, ok := policy.(keepForPolicy); ok {
			needsTime = true
			break
		}
	}
	if needsTime {
		field, ok := schema.FieldByGo[state.TimeField]
		if !ok {
			return cleanupState{}, &oro.Error{Op: "logroll.cleanup", Kind: oro.ErrUnknownField, Model: schema.Name, Field: state.TimeField}
		}
		state.TimeColumn = field.Column
	}
	return state, nil
}
