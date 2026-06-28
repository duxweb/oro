package logroll

import (
	"context"
	"time"

	"github.com/duxweb/oro"
)

type Result struct {
	Deleted int64
	Batches int
	Cutoff  time.Time
}

type CleanupQuery[T any] struct {
	db     *oro.DB
	config Config
	where  []oro.Condition
}

func Cleanup[T any](db *oro.DB, options ...Option) *CleanupQuery[T] {
	return &CleanupQuery[T]{db: db, config: resolveConfig(options)}
}

func (query *CleanupQuery[T]) Where(field any, args ...any) *CleanupQuery[T] {
	clone := *query
	condition, err := conditionFromArgs(field, args...)
	if err != nil {
		condition = oro.Condition{Op: "invalid", Value: err}
	}
	clone.where = append(clone.where, condition)
	return &clone
}

func (query *CleanupQuery[T]) Run(ctx context.Context) (*Result, error) {
	if query == nil || query.db == nil {
		return nil, &oro.Error{Op: "logroll.cleanup", Kind: oro.ErrInvalidArgument}
	}
	schema, err := oro.SchemaOf[T](query.db)
	if err != nil {
		return nil, err
	}
	return cleanupWithSchemaAndWhere(ctx, query.db, schema, query.config, query.where)
}

func cleanupWithSchema(ctx context.Context, db *oro.DB, schema *oro.ModelSchema, config Config) (*Result, error) {
	return cleanupWithSchemaAndWhere(ctx, db, schema, config, nil)
}

func cleanupWithSchemaAndWhere(ctx context.Context, db *oro.DB, schema *oro.ModelSchema, config Config, extraWhere []oro.Condition) (*Result, error) {
	if db == nil || schema == nil {
		return nil, &oro.Error{Op: "logroll.cleanup", Kind: oro.ErrInvalidArgument}
	}
	if len(config.Policies) == 0 {
		return &Result{}, nil
	}
	state, err := cleanupStateFor(schema, config)
	if err != nil {
		return nil, err
	}
	result := &Result{}
	for {
		ids, cutoff, err := cleanupIDs(ctx, db, schema, state, config, extraWhere)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return result, nil
		}
		deleted, err := deleteIDs(ctx, db, schema, state, config, ids, extraWhere)
		if err != nil {
			return nil, err
		}
		result.Deleted += deleted
		result.Batches++
		if !cutoff.IsZero() {
			result.Cutoff = cutoff
		}
		if len(ids) < config.BatchSize {
			return result, nil
		}
	}
}

func cleanupIDs(ctx context.Context, db *oro.DB, schema *oro.ModelSchema, state cleanupState, config Config, extraWhere []oro.Condition) ([]any, time.Time, error) {
	query := cleanupBaseQuery(db, schema, state, config, extraWhere).Select(state.PrimaryColumn).OrderBy(state.PrimaryColumn).Limit(config.BatchSize)
	cutoff := time.Time{}
	candidates := make([]oro.Condition, 0, len(config.Policies))
	for _, policy := range config.Policies {
		switch typed := policy.(type) {
		case keepForPolicy:
			if typed.duration <= 0 {
				return nil, cutoff, &oro.Error{Op: "logroll.cleanup", Kind: oro.ErrInvalidArgument, Field: "KeepFor"}
			}
			cutoff = config.Now().Add(-typed.duration)
			candidates = append(candidates, oro.Condition{Field: state.TimeColumn, Op: "<", Value: cutoff})
		case keepLastPolicy:
			if typed.count < 0 {
				return nil, cutoff, &oro.Error{Op: "logroll.cleanup", Kind: oro.ErrInvalidArgument, Field: "KeepLast"}
			}
			boundary, all, err := boundaryID(ctx, db, schema, state, config, typed.count, extraWhere)
			if err != nil {
				return nil, cutoff, err
			}
			if boundary == nil {
				continue
			}
			if all {
				candidates = append(candidates, oro.Field(state.PrimaryColumn).IsNotNull())
			} else {
				candidates = append(candidates, oro.Condition{Field: state.PrimaryColumn, Op: "<", Value: boundary})
			}
		}
	}
	if len(candidates) == 0 {
		return nil, cutoff, nil
	}
	if len(candidates) == 1 {
		query = query.Where(candidates[0])
	} else {
		query = query.Where(oro.Or(candidates...))
	}
	rows, err := query.Get(ctx)
	if err != nil {
		return nil, cutoff, err
	}
	ids := make([]any, 0, len(rows))
	for _, row := range rows {
		if value, ok := row[state.PrimaryColumn]; ok {
			ids = append(ids, value)
		}
	}
	return ids, cutoff, nil
}

func boundaryID(ctx context.Context, db *oro.DB, schema *oro.ModelSchema, state cleanupState, config Config, keep int64, extraWhere []oro.Condition) (any, bool, error) {
	if keep == 0 {
		return true, true, nil
	}
	row, err := cleanupBaseQuery(db, schema, state, config, extraWhere).
		Select(state.PrimaryColumn).
		OrderByDesc(state.PrimaryColumn).
		Offset(int(keep - 1)).
		Limit(1).
		First(ctx)
	if err != nil || row == nil {
		return nil, false, err
	}
	return row[state.PrimaryColumn], false, nil
}

func deleteIDs(ctx context.Context, db *oro.DB, schema *oro.ModelSchema, state cleanupState, config Config, ids []any, extraWhere []oro.Condition) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	query := cleanupBaseQuery(db, schema, state, config, extraWhere).Where(oro.Field(state.PrimaryColumn).In(ids...))
	return query.Delete(ctx)
}

func cleanupBaseQuery(db *oro.DB, schema *oro.ModelSchema, state cleanupState, config Config, extraWhere []oro.Condition) *oro.TableQuery {
	if config.Connection != "" {
		db = db.Connection(config.Connection)
	} else if schema.Connection != "" {
		db = db.Connection(schema.Connection)
	}
	query := db.Table(schema.Table)
	for field, value := range config.Scope {
		if schemaField, ok := schema.FieldByGo[field]; ok {
			query = query.Where(schemaField.Column, value)
			continue
		}
		query = query.Where(field, value)
	}
	for _, condition := range extraWhere {
		query = query.Where(mapCondition(schema, condition))
	}
	return query
}

func conditionFromArgs(field any, args ...any) (oro.Condition, error) {
	switch typed := field.(type) {
	case oro.Condition:
		if len(args) == 0 {
			return typed, nil
		}
	case string:
		if typed == "" {
			break
		}
		if len(args) == 1 {
			return oro.Field(typed).Eq(args[0]), nil
		}
		if len(args) == 2 {
			operator, ok := args[0].(string)
			if !ok || !oro.IsSafeConditionOperator(operator) {
				return oro.Condition{}, &oro.Error{Op: "logroll.where", Kind: oro.ErrInvalidArgument, Field: typed}
			}
			return oro.Condition{Field: typed, Op: operator, Value: args[1]}, nil
		}
	}
	return oro.Condition{}, &oro.Error{Op: "logroll.where", Kind: oro.ErrInvalidArgument}
}

func mapCondition(schema *oro.ModelSchema, condition oro.Condition) oro.Condition {
	if schema == nil || condition.Field == "" {
		return condition
	}
	if field, ok := schema.FieldByGo[condition.Field]; ok {
		condition.Field = field.Column
	}
	if len(condition.Conditions) > 0 {
		for index := range condition.Conditions {
			condition.Conditions[index] = mapCondition(schema, condition.Conditions[index])
		}
	}
	return condition
}
