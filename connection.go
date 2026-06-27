package oro

import (
	"context"
	"database/sql"
	"sync/atomic"
)

type ConnectionManager struct {
	conns     map[string]*Connection
	readIndex atomic.Uint64
}

type Connection struct {
	Name      string
	Primary   *sql.DB
	Reads     []*sql.DB
	readOwned []bool
	Driver    Driver
	Dialect   Dialect
	owned     bool
	stmts     map[*sql.DB]*statementCache
}

func NewConnectionManager(config Config) (*ConnectionManager, error) {
	if len(config.Connections) == 0 {
		return nil, &Error{Op: "open", Kind: ErrInvalidArgument, Field: "Connections"}
	}

	manager := &ConnectionManager{conns: make(map[string]*Connection, len(config.Connections))}
	ctx, cancel := withOperationTimeout(context.Background(), config.Timeout.Connect)
	defer cancel()

	for name, connConfig := range config.Connections {
		if connConfig.Driver == nil {
			return nil, &Error{Op: "open", Kind: ErrInvalidArgument, Field: "Driver"}
		}

		primary, err := connConfig.Driver.Open(ctx)
		if err != nil {
			return nil, connConfig.Driver.TranslateError(err)
		}
		applyPool(primary, mergePool(config.Pool, connConfig.Pool))
		if mergePool(config.Pool, connConfig.Pool).PingOnOpen {
			if err := primary.PingContext(ctx); err != nil {
				if connConfig.Driver.Owned() {
					_ = primary.Close()
				}
				return nil, connConfig.Driver.TranslateError(err)
			}
		}

		conn := &Connection{
			Name:    name,
			Primary: primary,
			Driver:  connConfig.Driver,
			Dialect: connConfig.Driver.Dialect(),
			owned:   connConfig.Driver.Owned(),
			stmts:   map[*sql.DB]*statementCache{},
		}
		if size := config.statementCacheSize(); size > 0 {
			conn.stmts[primary] = newStatementCache(size)
		}

		for _, readDriver := range connConfig.Reads {
			readDB, err := readDriver.Open(ctx)
			if err != nil {
				return nil, readDriver.TranslateError(err)
			}
			applyPool(readDB, mergePool(config.Pool, connConfig.Pool))
			if mergePool(config.Pool, connConfig.Pool).PingOnOpen {
				if err := readDB.PingContext(ctx); err != nil {
					if readDriver.Owned() {
						_ = readDB.Close()
					}
					return nil, readDriver.TranslateError(err)
				}
			}
			conn.Reads = append(conn.Reads, readDB)
			conn.readOwned = append(conn.readOwned, readDriver.Owned())
			if size := config.statementCacheSize(); size > 0 {
				conn.stmts[readDB] = newStatementCache(size)
			}
		}

		manager.conns[name] = conn
	}

	defaultName := config.defaultConnectionName()
	if _, ok := manager.conns[defaultName]; !ok {
		return nil, &Error{Op: "open", Kind: ErrUnknownConnection, Field: defaultName}
	}
	for group, shard := range config.Shards {
		if len(shard.Connections) == 0 || shard.Strategy == nil {
			return nil, &Error{Op: "open", Kind: ErrShardNotFound, Field: group}
		}
		for _, connection := range shard.Connections {
			if _, ok := manager.conns[connection]; !ok {
				return nil, &Error{Op: "open", Kind: ErrUnknownConnection, Field: connection}
			}
		}
	}

	return manager, nil
}

func (manager *ConnectionManager) Get(name string) (*Connection, error) {
	conn, ok := manager.conns[name]
	if !ok {
		return nil, &Error{Op: "connection", Kind: ErrUnknownConnection, Field: name}
	}
	return conn, nil
}

func (manager *ConnectionManager) Close() error {
	var closeErr error
	for _, conn := range manager.conns {
		if err := conn.closeStatements(); err != nil && closeErr == nil {
			closeErr = err
		}
		if conn.owned && conn.Primary != nil {
			if err := conn.Primary.Close(); err != nil && closeErr == nil {
				closeErr = err
			}
		}
		for index, read := range conn.Reads {
			if index < len(conn.readOwned) && conn.readOwned[index] && read != nil {
				if err := read.Close(); err != nil && closeErr == nil {
					closeErr = err
				}
			}
		}
	}
	return closeErr
}

func (manager *ConnectionManager) PickRead(conn *Connection) *sql.DB {
	if manager == nil || conn == nil || len(conn.Reads) == 0 {
		return nil
	}
	next := manager.readIndex.Add(1)
	return conn.Reads[int((next-1)%uint64(len(conn.Reads)))]
}

func (conn *Connection) statement(ctx context.Context, db *sql.DB, query string) (*sql.Stmt, func(), error) {
	if conn == nil || db == nil || conn.stmts == nil {
		return nil, nil, nil
	}
	cache := conn.stmts[db]
	if cache == nil {
		return nil, nil, nil
	}
	return cache.acquire(ctx, db, query)
}

func (conn *Connection) closeStatements() error {
	if conn == nil || len(conn.stmts) == 0 {
		return nil
	}
	var closeErr error
	for _, cache := range conn.stmts {
		if err := cache.close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func mergePool(global PoolConfig, override *PoolConfig) PoolConfig {
	if override == nil {
		return global
	}
	return *override
}

func applyPool(db *sql.DB, pool PoolConfig) {
	if db == nil {
		return
	}
	if pool.MaxOpenConns > 0 {
		db.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns > 0 {
		db.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}
	if pool.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
	}
}
