package oro

import (
	"context"
	"time"
)

// DB is an immutable query session over a shared runtime.
//
// Methods that change session state return a shallow clone, so it is safe to
// reuse a base DB across goroutines.
type DB struct {
	runtime *Runtime
	session sessionState
}

// RawQuery is a raw SQL query with optional cache and timeout settings.
type RawQuery struct {
	db      *DB
	raw     RawSpec
	cache   CacheSpec
	timeout time.Duration
}

// Open creates a DB from Config, initializes runtime components, and installs
// configured extensions.
func Open(config Config) (*DB, error) {
	factory := resolveFactory(config)

	conns, err := factory.NewConnectionManager(config)
	if err != nil {
		return nil, err
	}

	rt := &Runtime{
		Config:     config,
		Conns:      conns,
		Registry:   factory.NewRegistry(config),
		Events:     factory.NewEventBus(config),
		Cache:      config.Cache,
		Logger:     config.Logger,
		tableNames: newTableNameResolver(config),
		SQLCache:   newSQLCache(config.sqlCacheSize()),
		ScanCache:  newModelScanCache(config.scanCacheSize()),
	}

	rt.SchemaParser = factory.NewSchemaParser(rt)
	rt.Planner = factory.NewQueryPlanner(rt)
	rt.Executor = factory.NewExecutor(rt)
	rt.Mapper = factory.NewMapper(rt)
	rt.Syncer = factory.NewSyncer(rt)
	rt.Serializer = factory.NewSerializer(rt)

	db := &DB{
		runtime: rt,
		session: sessionState{connection: config.defaultConnectionName()},
	}
	if err := installExtensions(db, config.Extensions); err != nil {
		_ = db.Close(context.Background())
		return nil, err
	}
	return db, nil
}

// Close closes all owned database connections.
func (db *DB) Close(ctx context.Context) error {
	if db == nil || db.runtime == nil || db.runtime.Conns == nil {
		return nil
	}
	return db.runtime.Conns.Close()
}

// Connection returns a DB clone pinned to the named connection.
func (db *DB) Connection(name string) *DB {
	clone := *db
	clone.session.connection = name
	clone.session.manualConnection = true
	return &clone
}

// WithExtension returns a DB clone carrying extension-specific session state.
func (db *DB) WithExtension(name string, state any) *DB {
	clone := *db
	clone.session.extensions = cloneExtensionState(db.session.extensions)
	clone.session.extensions[name] = state
	return &clone
}

// ExtensionState returns extension-specific session state by name.
func (db *DB) ExtensionState(name string) (any, bool) {
	if db == nil || db.session.extensions == nil {
		return nil, false
	}
	value, ok := db.session.extensions[name]
	return value, ok
}

// Use starts a model query for T. Model queries use Go field names in
// conditions, ordering, selection, and relation definitions.
func (db *DB) Use[T any]() *ModelQuery[T] {
	return &ModelQuery[T]{
		db: db,
		spec: QuerySpec{
			Connection: db.session.connection,
		},
	}
}

// Table starts a table query by database table name. Table queries use database
// column names.
func (db *DB) Table(name string) *TableQuery {
	return &TableQuery{
		db: db,
		spec: QuerySpec{
			Connection: db.session.connection,
			Table:      name,
		},
	}
}

// TableName returns the physical table name after applying the configured
// prefix resolver.
func (db *DB) TableName(name string) string {
	return tableNames(db).Physical(name)
}

// SchemaOf returns the parsed schema for model type T, registering it lazily
// when necessary.
func SchemaOf[T any](db *DB) (*ModelSchema, error) {
	return schemaForModel[T](db)
}

// SchemaOf returns the parsed schema for a model value, registering it lazily
// when necessary.
func (db *DB) SchemaOf(model Definer) (*ModelSchema, error) {
	if db == nil || db.runtime == nil || db.runtime.SchemaParser == nil {
		return nil, &Error{Op: "schema", Kind: ErrInvalidArgument}
	}
	if model == nil {
		return nil, &Error{Op: "schema", Kind: ErrInvalidArgument}
	}
	if db.runtime.Registry != nil {
		if schema, ok := db.runtime.Registry.Get(model); ok {
			return schema, nil
		}
	}
	schema, err := db.runtime.SchemaParser.Parse(model)
	if err != nil {
		return nil, err
	}
	if db.runtime.Registry != nil {
		db.runtime.Registry.Register(schema, model)
	}
	return schema, nil
}

// From starts a table query from a structured source such as a subquery.
func (db *DB) From(source Source) *TableQuery {
	return &TableQuery{
		db: db,
		spec: QuerySpec{
			Connection: db.session.connection,
			From:       source.sourceAST(),
		},
	}
}

// Raw starts a raw SQL query. Raw SQL uses the target driver's native
// placeholders.
func (db *DB) Raw(sql string, args ...any) *RawQuery {
	return &RawQuery{
		db: db,
		raw: RawSpec{
			SQL:  sql,
			Args: args,
		},
	}
}

// Register parses and registers model schemas and their relation methods.
func (db *DB) Register(models ...Definer) error {
	parsed := make([]*ModelSchema, 0, len(models))
	for _, model := range models {
		schema, err := db.runtime.SchemaParser.Parse(model)
		if err != nil {
			return err
		}
		if schema.Connection != "" {
			if _, err := db.runtime.Conns.Get(schema.Connection); err != nil {
				return err
			}
		}
		if schema.ShardGroup != "" {
			if _, err := shardConfigForSchema(db, schema); err != nil {
				return err
			}
		}
		db.runtime.Registry.Register(schema, model)
		parsed = append(parsed, schema)
	}
	for index, model := range models {
		if err := registerModelRelations(db.runtime.Registry, model, parsed[index]); err != nil {
			return err
		}
	}
	return nil
}

// Sync synchronizes registered model schemas to database tables.
func (db *DB) Sync(ctx context.Context) error {
	if db == nil || db.runtime == nil || db.runtime.Syncer == nil {
		return &Error{Op: "sync", Kind: ErrInvalidArgument}
	}
	return db.runtime.Syncer.Sync(ctx, db)
}

// Cache enables result caching for the raw query for ttl.
func (query *RawQuery) Cache(ttl time.Duration) *RawQuery {
	clone := *query
	clone.cache.Enabled = true
	clone.cache.TTL = int64(ttl)
	return &clone
}

// CacheKey sets an explicit cache key for the raw query.
func (query *RawQuery) CacheKey(key string) *RawQuery {
	clone := *query
	clone.cache.Key = key
	return &clone
}

// CacheTags adds cache invalidation tags to the raw query.
func (query *RawQuery) CacheTags(tags ...string) *RawQuery {
	clone := *query
	clone.cache.Tags = append(clone.cache.Tags, tags...)
	return &clone
}

// Timeout sets a per-query timeout for the raw query.
func (query *RawQuery) Timeout(timeout time.Duration) *RawQuery {
	clone := *query
	clone.timeout = timeout
	return &clone
}

// First returns the first raw result row or nil when no row is found.
func (query *RawQuery) First(ctx context.Context) (Map, error) {
	rows, err := query.Get(ctx)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// Get returns all raw result rows as Map values.
func (query *RawQuery) Get(ctx context.Context) ([]Map, error) {
	return execRawRows(ctx, query.db, query.raw, query.cache, query.timeout)
}

// Stream opens a streaming raw row iterator.
func (query *RawQuery) Stream(ctx context.Context) (Stream[Map], error) {
	rows, err := streamRaw(ctx, query.db, query.raw, query.timeout)
	if err != nil {
		return nil, err
	}
	return &mapStream{rows: rows}, nil
}

// Exec executes raw SQL and returns rows affected.
func (query *RawQuery) Exec(ctx context.Context) (int64, error) {
	return execRaw(ctx, query.db, query.raw, query.timeout)
}
