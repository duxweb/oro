package oro

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

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

func shardValuesForSchemaContext(ctx context.Context, db *DB, schema *ModelSchema, explicit Map) Map {
	values := Map{}
	if extensionValues, err := extensionShardValues(ctx, db); err == nil {
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
	values := shardValuesForSchemaContext(ctx, db, schema, explicit)
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

func validateShardWriteValuesForDB(ctx context.Context, db *DB, schema *ModelSchema, explicit Map, values Map) error {
	return validateShardWriteValues(schema, shardValuesForSchemaContext(ctx, db, schema, explicit), values)
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

func tableShardSpec(ctx context.Context, query *TableQuery) (QuerySpec, error) {
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
	connection, err := config.Strategy.Pick(ctx, query.shard, config.Connections)
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
	spec, err := tableShardSpec(ctx, query)
	if err != nil {
		return nil, err
	}
	config := query.db.runtime.Config.Shards[spec.ShardGroup]
	return fanOutShardRows(ctx, query.db, spec, config.Connections)
}

// fanOutShardRows runs spec against every shard connection (with offset/limit
// stripped so paging is global), concatenates the rows, then applies the global
// ORDER BY and offset/limit. Shared by the model and table all-shard paths.
func fanOutShardRows(ctx context.Context, db *DB, spec QuerySpec, connections []string) ([]Map, error) {
	if err := validateAllShardOrder(spec.Order, spec.Table, spec.ModelName, spec.ShardGroup); err != nil {
		return nil, err
	}
	offset, limit := spec.Offset, spec.Limit
	spec.Offset = nil
	spec.Limit = nil
	rows := []Map{}
	for _, connection := range connections {
		nextSpec := cloneQuerySpec(spec)
		nextSpec.Connection = connection
		nextRows, err := queryRowsPrepared(ctx, db, nextSpec)
		if err != nil {
			return nil, err
		}
		rows = append(rows, nextRows...)
	}
	return applyGlobalShardOrderAndPage(rows, spec.Order, offset, limit)
}

func queryAllShardCounts(ctx context.Context, query *TableQuery) ([]Map, error) {
	base := cloneQuerySpec(query.spec)
	// Finalize before wrapping so scoping lands inside the grouped-count
	// subquery; per-shard execution then skips re-applying (finalized).
	if err := finalizeReadSpec(ctx, query.db, &base); err != nil {
		return nil, err
	}
	spec, err := countQuerySpec(base)
	if err != nil {
		return nil, err
	}
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
	shardSpec, err := tableShardSpec(ctx, &existsQuery)
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

func validateAllShardOrder(order []OrderExpr, table string, model string, shardGroup string) error {
	for _, item := range order {
		if item.Raw {
			return &Error{Op: "shard", Kind: ErrUnsupported, Table: table, Model: model, Field: shardGroup}
		}
	}
	return nil
}

func applyGlobalShardOrderAndPage(rows []Map, order []OrderExpr, offset *int, limit *int) ([]Map, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	if len(order) > 0 {
		if err := validateShardOrderColumns(rows, order); err != nil {
			return nil, err
		}
		sort.SliceStable(rows, func(left int, right int) bool {
			return compareOrderedRows(rows[left], rows[right], order) < 0
		})
	}
	start := 0
	if offset != nil && *offset > 0 {
		start = *offset
		if start >= len(rows) {
			return []Map{}, nil
		}
	}
	end := len(rows)
	if limit != nil && *limit >= 0 && start+*limit < end {
		end = start + *limit
	}
	return rows[start:end], nil
}

func validateShardOrderColumns(rows []Map, order []OrderExpr) error {
	for _, row := range rows {
		for _, item := range order {
			if _, ok := row[item.Expr]; !ok {
				return &Error{Op: "shard", Kind: ErrInvalidQuery, Field: item.Expr}
			}
		}
	}
	return nil
}

func compareOrderedRows(left Map, right Map, order []OrderExpr) int {
	for _, item := range order {
		result := compareOrderValues(left[item.Expr], right[item.Expr])
		if result == 0 {
			continue
		}
		if item.Desc {
			return -result
		}
		return result
	}
	return 0
}

func compareOrderValues(left any, right any) int {
	if left == nil && right == nil {
		return 0
	}
	if left == nil {
		return -1
	}
	if right == nil {
		return 1
	}
	if leftTime, ok := asTime(left); ok {
		if rightTime, ok := asTime(right); ok {
			return leftTime.Compare(rightTime)
		}
	}
	if leftFloat, ok := asFloat64(left); ok {
		if rightFloat, ok := asFloat64(right); ok {
			switch {
			case leftFloat < rightFloat:
				return -1
			case leftFloat > rightFloat:
				return 1
			default:
				return 0
			}
		}
	}
	leftText := stringifyOrderValue(left)
	rightText := stringifyOrderValue(right)
	return strings.Compare(leftText, rightText)
}

func asFloat64(value any) (float64, bool) {
	switch typedValue := value.(type) {
	case int:
		return float64(typedValue), true
	case int8:
		return float64(typedValue), true
	case int16:
		return float64(typedValue), true
	case int32:
		return float64(typedValue), true
	case int64:
		return float64(typedValue), true
	case uint:
		return float64(typedValue), true
	case uint8:
		return float64(typedValue), true
	case uint16:
		return float64(typedValue), true
	case uint32:
		return float64(typedValue), true
	case uint64:
		return float64(typedValue), true
	case float32:
		return float64(typedValue), true
	case float64:
		return typedValue, true
	default:
		return 0, false
	}
}

func asTime(value any) (time.Time, bool) {
	switch typedValue := value.(type) {
	case time.Time:
		return typedValue, true
	default:
		return time.Time{}, false
	}
}

func stringifyOrderValue(value any) string {
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case []byte:
		return string(typedValue)
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
	case float32:
		return strconv.FormatFloat(float64(typedValue), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(typedValue, 'g', -1, 64)
	case time.Time:
		return typedValue.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(value)
	}
}
