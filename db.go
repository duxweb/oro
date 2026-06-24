package oro

import (
	"context"
	"time"
)

type DB struct {
	runtime *Runtime
	session sessionState
}

type RawQuery struct {
	db      *DB
	raw     RawSpec
	cache   CacheSpec
	timeout time.Duration
}

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
	rt.RelationLoader = factory.NewRelationLoader(rt)
	rt.RelationWriter = factory.NewRelationWriter(rt)
	rt.Serializer = factory.NewSerializer(rt)

	return &DB{
		runtime: rt,
		session: sessionState{connection: config.defaultConnectionName()},
	}, nil
}

func (db *DB) Close(ctx context.Context) error {
	if db == nil || db.runtime == nil || db.runtime.Conns == nil {
		return nil
	}
	return db.runtime.Conns.Close()
}

func (db *DB) Connection(name string) *DB {
	clone := *db
	clone.session.connection = name
	clone.session.manualConnection = true
	return &clone
}

func (db *DB) Tenant(values Map) *DB {
	clone := *db
	clone.session.tenant = copyMap(values)
	clone.session.withoutTenant = false
	return &clone
}

func (db *DB) WithoutTenant() *DB {
	clone := *db
	clone.session.tenant = nil
	clone.session.withoutTenant = true
	return &clone
}

func (db *DB) Use[T any]() *ModelQuery[T] {
	return &ModelQuery[T]{
		db: db,
		spec: QuerySpec{
			Connection: db.session.connection,
		},
	}
}

func (db *DB) Table(name string) *TableQuery {
	return &TableQuery{
		db: db,
		spec: QuerySpec{
			Connection: db.session.connection,
			Table:      name,
		},
	}
}

func (db *DB) TableName(name string) string {
	return tableNames(db).Physical(name)
}

func (db *DB) From(source Source) *TableQuery {
	return &TableQuery{
		db: db,
		spec: QuerySpec{
			Connection: db.session.connection,
			From:       source.sourceAST(),
		},
	}
}

func (db *DB) Raw(sql string, args ...any) *RawQuery {
	return &RawQuery{
		db: db,
		raw: RawSpec{
			SQL:  sql,
			Args: args,
		},
	}
}

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

func (db *DB) Sync(ctx context.Context) error {
	if db == nil || db.runtime == nil || db.runtime.Syncer == nil {
		return &Error{Op: "sync", Kind: ErrInvalidArgument}
	}
	return db.runtime.Syncer.Sync(ctx, db)
}

func (query *RawQuery) Cache(ttl time.Duration) *RawQuery {
	clone := *query
	clone.cache.Enabled = true
	clone.cache.TTL = int64(ttl)
	return &clone
}

func (query *RawQuery) CacheKey(key string) *RawQuery {
	clone := *query
	clone.cache.Key = key
	return &clone
}

func (query *RawQuery) CacheTags(tags ...string) *RawQuery {
	clone := *query
	clone.cache.Tags = append(clone.cache.Tags, tags...)
	return &clone
}

func (query *RawQuery) Timeout(timeout time.Duration) *RawQuery {
	clone := *query
	clone.timeout = timeout
	return &clone
}

func (query *RawQuery) First(ctx context.Context) (Map, error) {
	rows, err := query.Get(ctx)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (query *RawQuery) Get(ctx context.Context) ([]Map, error) {
	return execRawRows(ctx, query.db, query.raw, query.cache, query.timeout)
}

func (query *RawQuery) Stream(ctx context.Context) (Stream[Map], error) {
	rows, err := streamRaw(ctx, query.db, query.raw, query.timeout)
	if err != nil {
		return nil, err
	}
	return &mapStream{rows: rows}, nil
}

func (query *RawQuery) Exec(ctx context.Context) (int64, error) {
	return execRaw(ctx, query.db, query.raw, query.timeout)
}
