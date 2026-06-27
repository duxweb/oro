package oro

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"time"

	internalregistry "github.com/duxweb/oro/internal/registry"
)

type Runtime struct {
	Config Config

	Conns      *ConnectionManager
	Registry   *Registry
	Events     *EventBus
	Cache      CacheStore
	Logger     Logger
	tableNames *tableNameResolver
	SQLCache   *sqlCache
	ScanCache  *modelScanCache

	SchemaParser SchemaParser
	Planner      QueryPlanner
	Executor     Executor
	Mapper       Mapper
	Syncer       Syncer
	Serializer   Serializer
}

type Factory interface {
	NewConnectionManager(config Config) (*ConnectionManager, error)
	NewRegistry(config Config) *Registry
	NewEventBus(config Config) *EventBus
	NewSchemaParser(rt *Runtime) SchemaParser
	NewQueryPlanner(rt *Runtime) QueryPlanner
	NewExecutor(rt *Runtime) Executor
	NewMapper(rt *Runtime) Mapper
	NewSyncer(rt *Runtime) Syncer
	NewSerializer(rt *Runtime) Serializer
}

type sessionState struct {
	connection       string
	manualConnection bool
	extensions       map[string]any
	tx               *txState
}

type txState struct {
	connection string
	tx         *sql.Tx
	depth      int
	closed     bool
}

type CacheStore interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags ...string) error
	Forget(ctx context.Context, key string) error
	ForgetTag(ctx context.Context, tag string) error
}

type SchemaParser interface {
	Parse(model any) (*ModelSchema, error)
}

type QueryPlanner interface {
	BuildSelect(spec QuerySpec) (Statement, error)
	BuildInsert(spec WriteSpec) (Statement, error)
	BuildUpsert(spec WriteSpec) (Statement, error)
	BuildUpdate(spec WriteSpec) (Statement, error)
	BuildDelete(spec WriteSpec) (Statement, error)
}

type ExecContext interface{}

type RowsResult struct {
	Rows []Map
}

type ExecResult struct {
	RowsAffected    int64
	LastInsertID    int64
	HasLastInsertID bool
}

type Executor interface {
	Query(ctx context.Context, exec ExecContext, sql CompiledSQL) (*RowsResult, error)
	Exec(ctx context.Context, exec ExecContext, sql CompiledSQL) (ExecResult, error)
}

type Mapper interface {
	MapModel(schema *ModelSchema, row Map, dest any) error
	MapDTO(row Map, dest any) error
}

type Syncer interface {
	Sync(ctx context.Context, db *DB) error
}

type Serializer interface {
	Serialize(value any, opts SerializeOptions) any
}

type DefaultFactory struct{}

func (DefaultFactory) NewConnectionManager(config Config) (*ConnectionManager, error) {
	return NewConnectionManager(config)
}

func (DefaultFactory) NewRegistry(config Config) *Registry {
	return NewRegistry()
}

func (DefaultFactory) NewEventBus(config Config) *EventBus {
	return NewEventBus()
}

func (DefaultFactory) NewSchemaParser(rt *Runtime) SchemaParser {
	return schemaParser{}
}

func (DefaultFactory) NewQueryPlanner(rt *Runtime) QueryPlanner {
	return noopQueryPlanner{}
}

func (DefaultFactory) NewExecutor(rt *Runtime) Executor {
	return sqlExecutor{rt: rt}
}

func (DefaultFactory) NewMapper(rt *Runtime) Mapper {
	return reflectMapper{rt: rt}
}

func (DefaultFactory) NewSyncer(rt *Runtime) Syncer {
	return schemaSyncer{rt: rt}
}

func (DefaultFactory) NewSerializer(rt *Runtime) Serializer {
	return reflectSerializer{rt: rt}
}

func resolveFactory(config Config) Factory {
	if config.Factory != nil {
		return config.Factory
	}
	return DefaultFactory{}
}

type noopQueryPlanner struct{}

func (noopQueryPlanner) BuildSelect(spec QuerySpec) (Statement, error) {
	return SelectAST{Table: spec.Table, Alias: spec.Alias, From: spec.From, Joins: spec.Joins, Where: spec.Where, Select: spec.Select, Group: spec.Group, Having: spec.Having, Order: spec.Order, Limit: spec.Limit, Offset: spec.Offset, Lock: spec.Lock}, nil
}

func (noopQueryPlanner) BuildInsert(spec WriteSpec) (Statement, error) {
	return InsertAST{Table: spec.Table, Values: spec.Values, Returning: spec.Returning}, nil
}

func (noopQueryPlanner) BuildUpsert(spec WriteSpec) (Statement, error) {
	return InsertAST{Table: spec.Table, Values: spec.Values, Conflict: spec.Conflict, Returning: spec.Returning}, nil
}

func (noopQueryPlanner) BuildUpdate(spec WriteSpec) (Statement, error) {
	values := Map{}
	if len(spec.Values) > 0 {
		values = spec.Values[0]
	}
	return UpdateAST{Table: spec.Table, Values: values, Where: spec.Where}, nil
}

func (noopQueryPlanner) BuildDelete(spec WriteSpec) (Statement, error) {
	return DeleteAST{Table: spec.Table, Where: spec.Where}, nil
}

type noopExecutor struct{}

func (noopExecutor) Query(ctx context.Context, exec ExecContext, sql CompiledSQL) (*RowsResult, error) {
	return &RowsResult{}, nil
}

func (noopExecutor) Exec(ctx context.Context, exec ExecContext, sql CompiledSQL) (ExecResult, error) {
	return ExecResult{}, nil
}

type noopMapper struct{}

func (noopMapper) MapModel(schema *ModelSchema, row Map, dest any) error {
	return nil
}

func (noopMapper) MapDTO(row Map, dest any) error {
	return nil
}

type noopSyncer struct{}

func (noopSyncer) Sync(ctx context.Context, db *DB) error {
	return nil
}

type Registry struct {
	inner *internalregistry.Registry[ModelSchema]
}

func NewRegistry() *Registry {
	return &Registry{
		inner: internalregistry.New[ModelSchema](),
	}
}

func (registry *Registry) Register(schema *ModelSchema, model any) {
	registry.inner.Register(schema, model)
}

func (registry *Registry) Get(model any) (*ModelSchema, bool) {
	return registry.inner.Get(model)
}

func (registry *Registry) GetType(typ reflect.Type) (*ModelSchema, bool) {
	return registry.inner.GetType(typ)
}

func (registry *Registry) GetIdentifier(identifier string) (*ModelSchema, bool) {
	return registry.inner.GetIdentifier(identifier)
}

func (registry *Registry) TypeForSchema(target *ModelSchema) (reflect.Type, bool) {
	return registry.inner.TypeFor(func(schema *ModelSchema) bool {
		if schema == target || (schema.Name == target.Name && schema.Table == target.Table) {
			return true
		}
		return false
	})
}

func (registry *Registry) Schemas() []*ModelSchema {
	return registry.inner.Schemas()
}

func SchemaForTable(db *DB, table string) *ModelSchema {
	if db == nil || db.runtime == nil || db.runtime.Registry == nil || table == "" {
		return nil
	}
	resolver := tableNames(db)
	physical := resolver.Physical(table)
	for _, schema := range db.runtime.Registry.Schemas() {
		if schema == nil {
			continue
		}
		// Physical is idempotent, so the physical comparison matches whether the
		// caller passed a logical or an already-prefixed name; the exact logical
		// match is a cheap fast path.
		if schema.Table == table || resolver.Physical(schema.Table) == physical {
			return schema
		}
	}
	return nil
}

func modelType(model any) reflect.Type {
	return internalregistry.ModelType(model)
}

func modelIdentifier(typ reflect.Type) string {
	return internalregistry.ModelIdentifier(typ)
}

func queryTimeout(db *DB, spec QuerySpec) time.Duration {
	if spec.Timeout > 0 {
		return time.Duration(spec.Timeout)
	}
	if db != nil && db.runtime != nil {
		return db.runtime.Config.Timeout.Query
	}
	return 0
}

func transactionTimeout(db *DB) time.Duration {
	if db != nil && db.runtime != nil {
		return db.runtime.Config.Timeout.Transaction
	}
	return 0
}

func withOperationTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= timeout {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func wrapContextError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{Op: op, Kind: err, Cause: err}
	}
	return err
}
