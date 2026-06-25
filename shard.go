package oro

import (
	"context"
	"reflect"

	"github.com/duxweb/oro/internal/shardstrategy"
)

type ShardConfig struct {
	Connections []string
	Strategy    ShardStrategy
}

type ShardStrategy interface {
	Pick(ctx context.Context, values Map, shards []string) (string, error)
}

type ShardFunc func(ctx context.Context, values Map, shards []string) (string, error)

func (fn ShardFunc) Pick(ctx context.Context, values Map, shards []string) (string, error) {
	return fn(ctx, values, shards)
}

func CustomShard(fn ShardFunc) ShardStrategy {
	return fn
}

func ModShard(field string) ShardStrategy {
	return shardstrategy.Mod{Field: field, Errors: shardErrors()}
}

func HashShard(field string) ShardStrategy {
	return shardstrategy.Hash{Field: field, Errors: shardErrors()}
}

type ShardRange = shardstrategy.Range

func RangeShard(field string, ranges ...ShardRange) ShardStrategy {
	return shardstrategy.RangeStrategy{Field: field, Ranges: append([]ShardRange(nil), ranges...), Errors: shardErrors()}
}

func shardUint(value any) (uint64, error) {
	return shardstrategy.Uint(value, ErrShardNotFound)
}

func shardInt(value any) (int64, error) {
	return shardstrategy.Int(value, ErrShardNotFound)
}

func shardErrors() shardstrategy.ErrorSet {
	return shardstrategy.ErrorSet{
		Required: ErrShardRequired,
		NotFound: ErrShardNotFound,
	}
}

func shardValuesForSchema(db *DB, schema *ModelSchema, explicit Map) Map {
	values := Map{}
	if extensionValues, err := extensionShardValues(context.Background(), db); err == nil {
		for key, value := range extensionValues {
			values[key] = value
		}
	}
	for key, value := range explicit {
		values[key] = value
	}
	return values
}

func shardConfigForSchema(db *DB, schema *ModelSchema) (*ShardConfig, error) {
	if db == nil || db.runtime == nil || schema == nil || schema.ShardGroup == "" {
		return nil, nil
	}
	config, ok := db.runtime.Config.Shards[schema.ShardGroup]
	if !ok {
		return nil, &Error{Op: "shard", Kind: ErrShardNotFound, Model: schema.Name, Field: schema.ShardGroup}
	}
	if len(config.Connections) == 0 || config.Strategy == nil {
		return nil, &Error{Op: "shard", Kind: ErrShardNotFound, Model: schema.Name, Field: schema.ShardGroup}
	}
	for _, connection := range config.Connections {
		if _, err := db.runtime.Conns.Get(connection); err != nil {
			return nil, &Error{Op: "shard", Kind: ErrShardNotFound, Model: schema.Name, Field: schema.ShardGroup, Cause: err}
		}
	}
	return &config, nil
}

func applyShardConnection(ctx context.Context, db *DB, schema *ModelSchema, spec *QuerySpec, explicit Map, all bool) error {
	config, err := shardConfigForSchema(db, schema)
	if err != nil || config == nil {
		return err
	}
	spec.ShardGroup = schema.ShardGroup
	if all {
		if db.session.tx != nil {
			return &Error{Op: "shard", Kind: ErrCrossShardTransaction, Model: schema.Name, Field: schema.ShardGroup}
		}
		return nil
	}
	connection, err := pickShardConnection(ctx, db, schema, config, explicit)
	if err != nil {
		return err
	}
	if err := ensureTransactionShard(db, connection, schema); err != nil {
		return err
	}
	spec.Connection = connection
	return nil
}

func pickShardConnection(ctx context.Context, db *DB, schema *ModelSchema, config *ShardConfig, explicit Map) (string, error) {
	values := shardValuesForSchema(db, schema, explicit)
	for _, field := range schema.ShardFields {
		if _, ok := values[field]; !ok {
			return "", &Error{Op: "shard", Kind: ErrShardRequired, Model: schema.Name, Field: field}
		}
	}
	connection, err := config.Strategy.Pick(ctx, values, config.Connections)
	if err != nil {
		kind := ErrShardNotFound
		if err == ErrShardRequired {
			kind = ErrShardRequired
		}
		return "", &Error{Op: "shard", Kind: kind, Model: schema.Name, Field: schema.ShardGroup, Cause: err}
	}
	if connection == "" {
		return "", &Error{Op: "shard", Kind: ErrShardRequired, Model: schema.Name, Field: schema.ShardGroup}
	}
	if _, err := db.runtime.Conns.Get(connection); err != nil {
		return "", &Error{Op: "shard", Kind: ErrShardNotFound, Model: schema.Name, Field: schema.ShardGroup, Cause: err}
	}
	return connection, nil
}

func ensureTransactionShard(db *DB, connection string, schema *ModelSchema) error {
	if db == nil || db.session.tx == nil || db.session.tx.closed {
		return nil
	}
	if db.session.tx.connection != connection {
		return &Error{Op: "shard", Kind: ErrCrossShardTransaction, Model: schema.Name, Field: schema.ShardGroup}
	}
	return nil
}

func shardConnections(db *DB, schema *ModelSchema) ([]string, error) {
	config, err := shardConfigForSchema(db, schema)
	if err != nil || config == nil {
		return nil, err
	}
	return append([]string(nil), config.Connections...), nil
}

func validateShardWriteValues(schema *ModelSchema, explicit Map, values Map) error {
	if schema == nil || schema.ShardGroup == "" || len(explicit) == 0 || len(values) == 0 {
		return nil
	}
	for _, fieldName := range schema.ShardFields {
		explicitValue, ok := explicit[fieldName]
		if !ok {
			continue
		}
		field := schema.FieldByGo[fieldName]
		value, ok := values[field.Column]
		if !ok {
			value, ok = values[fieldName]
		}
		if ok && !reflect.DeepEqual(explicitValue, value) {
			return &Error{Op: "shard", Kind: ErrShardConflict, Model: schema.Name, Field: fieldName}
		}
	}
	return nil
}

func validateShardWriteValuesForDB(db *DB, schema *ModelSchema, explicit Map, values Map) error {
	return validateShardWriteValues(schema, shardValuesForSchema(db, schema, explicit), values)
}

func validateShardUpdateValues(schema *ModelSchema, values Map) error {
	if schema == nil || schema.ShardGroup == "" || len(values) == 0 {
		return nil
	}
	for _, fieldName := range schema.ShardFields {
		if _, ok := values[fieldName]; ok {
			return &Error{Op: "shard", Kind: ErrShardConflict, Model: schema.Name, Field: fieldName}
		}
		if field, ok := schema.FieldByGo[fieldName]; ok {
			if _, ok := values[field.Column]; ok {
				return &Error{Op: "shard", Kind: ErrShardConflict, Model: schema.Name, Field: fieldName}
			}
		}
	}
	return nil
}

func tableShardSpec(query *TableQuery) (QuerySpec, error) {
	spec := cloneQuerySpec(query.spec)
	if spec.SelectErr != nil {
		return QuerySpec{}, spec.SelectErr
	}
	if spec.ShardGroup == "" {
		return spec, nil
	}
	config, ok := query.db.runtime.Config.Shards[spec.ShardGroup]
	if !ok || config.Strategy == nil || len(config.Connections) == 0 {
		return QuerySpec{}, &Error{Op: "shard", Kind: ErrShardNotFound, Table: spec.Table, Field: spec.ShardGroup}
	}
	for _, connection := range config.Connections {
		if _, err := query.db.runtime.Conns.Get(connection); err != nil {
			return QuerySpec{}, &Error{Op: "shard", Kind: ErrShardNotFound, Table: spec.Table, Field: spec.ShardGroup, Cause: err}
		}
	}
	if query.allShards {
		return spec, nil
	}
	connection, err := config.Strategy.Pick(context.Background(), query.shard, config.Connections)
	if err != nil {
		kind := ErrShardNotFound
		if err == ErrShardRequired {
			kind = ErrShardRequired
		}
		return QuerySpec{}, &Error{Op: "shard", Kind: kind, Table: spec.Table, Field: spec.ShardGroup, Cause: err}
	}
	if connection == "" {
		return QuerySpec{}, &Error{Op: "shard", Kind: ErrShardRequired, Table: spec.Table, Field: spec.ShardGroup}
	}
	if err := ensureTableTransactionShard(query.db, connection, spec); err != nil {
		return QuerySpec{}, err
	}
	spec.Connection = connection
	return spec, nil
}

func ensureTableTransactionShard(db *DB, connection string, spec QuerySpec) error {
	if db == nil || db.session.tx == nil || db.session.tx.closed {
		return nil
	}
	if db.session.tx.connection != connection {
		return &Error{Op: "shard", Kind: ErrCrossShardTransaction, Table: spec.Table, Field: spec.ShardGroup}
	}
	return nil
}

func queryAllShardRows(ctx context.Context, query *TableQuery) ([]Map, error) {
	spec, err := tableShardSpec(query)
	if err != nil {
		return nil, err
	}
	config := query.db.runtime.Config.Shards[spec.ShardGroup]
	rows := []Map{}
	for _, connection := range config.Connections {
		nextSpec := cloneQuerySpec(spec)
		nextSpec.Connection = connection
		nextRows, err := queryRowsPrepared(ctx, query.db, nextSpec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, nextRows...)
	}
	return rows, nil
}

func queryAllShardCounts(ctx context.Context, query *TableQuery) ([]Map, error) {
	spec := cloneQuerySpec(query.spec)
	spec.Select = []SelectExpr{{Expr: "count(*)", Alias: "total", Raw: true}}
	spec.Order = nil
	spec.Limit = nil
	spec.Offset = nil
	countQuery := *query
	countQuery.spec = spec
	return queryAllShardRows(ctx, &countQuery)
}

func queryAllShardExists(ctx context.Context, query *TableQuery) ([]Map, error) {
	spec := cloneQuerySpec(query.spec)
	spec.Select = []SelectExpr{{Expr: "1", Raw: true}}
	spec.Order = nil
	limit := 1
	spec.Limit = &limit
	spec.Offset = nil
	existsQuery := *query
	existsQuery.spec = spec
	rows := []Map{}
	shardSpec, err := tableShardSpec(&existsQuery)
	if err != nil {
		return nil, err
	}
	config := query.db.runtime.Config.Shards[shardSpec.ShardGroup]
	for _, connection := range config.Connections {
		nextSpec := cloneQuerySpec(shardSpec)
		nextSpec.Connection = connection
		nextRows, err := queryRowsPrepared(ctx, query.db, nextSpec)
		if err != nil {
			return nil, err
		}
		if len(nextRows) > 0 {
			return nextRows, nil
		}
	}
	return rows, nil
}
