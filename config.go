package oro

import "time"

// Config controls database connections, runtime behavior, and optional
// infrastructure such as logging, caching, timeouts, sharding, and extensions.
type Config struct {
	// Default is the connection name used when a query does not select one.
	// It defaults to "default".
	Default string

	// Connections maps connection names to primary and read drivers.
	Connections map[string]ConnectionConfig
	// TablePrefix is prepended to model and table names resolved by Oro.
	TablePrefix string
	// Location is used when scanning time.Time values. Oro stores times in UTC
	// and converts reads to Location; nil means UTC.
	Location *time.Location

	// Pool sets default database/sql pool options for every connection.
	Pool PoolConfig
	// Batch controls default chunk sizes for batched reads and writes.
	Batch BatchConfig
	// Shards configures named shard groups.
	Shards map[string]ShardConfig
	// Cache is the optional application-level query cache store.
	Cache CacheStore

	// Extensions are installed during Open and can modify query/write behavior.
	Extensions []Extension

	// Factory overrides runtime component construction for advanced use.
	Factory Factory

	// LogLevel filters SQL log events emitted to Logger.
	LogLevel LogLevel
	// Logger receives SQL log events when configured.
	Logger Logger

	// SlowQueryThreshold marks log events as slow when duration exceeds it.
	SlowQueryThreshold time.Duration
	// LogArgs includes bound arguments in SQL logs.
	LogArgs bool

	// AllowRawMultiStatement permits multiple statements in Raw SQL.
	AllowRawMultiStatement bool
	// SkipDefaultTransaction disables the default transaction around writes.
	SkipDefaultTransaction bool
	// Timeout controls operation-level deadlines.
	Timeout TimeoutConfig
	// Retry controls built-in retry behavior for selected operations.
	Retry RetryConfig
	// StatementCache configures prepared statement caching.
	StatementCache StatementCacheConfig
	// SQLCache configures internal SQL compilation caching.
	SQLCache SQLCacheConfig
	// ScanCache configures model scan-plan caching.
	ScanCache ScanCacheConfig
}

// TimeoutConfig contains optional deadlines for common operation classes.
type TimeoutConfig struct {
	Connect     time.Duration
	Query       time.Duration
	Transaction time.Duration
}

// RetryConfig configures retry behavior for reads and transaction deadlocks.
type RetryConfig struct {
	ReadAttempts       int
	TxDeadlockAttempts int
	Backoff            func(attempt int) time.Duration
}

// StatementCacheConfig controls prepared statement cache behavior.
type StatementCacheConfig struct {
	Disabled bool
	MaxSize  int
}

// SQLCacheConfig controls internal SQL compilation cache behavior.
type SQLCacheConfig struct {
	Disabled bool
	MaxSize  int
}

// ScanCacheConfig controls internal model scan-plan cache behavior.
type ScanCacheConfig struct {
	Disabled bool
	MaxSize  int
}

// ConnectionConfig describes one named database connection.
type ConnectionConfig struct {
	Driver Driver
	Reads  []Driver
	Pool   *PoolConfig
}

// PoolConfig maps to database/sql connection pool settings.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingOnOpen      bool
}

// BatchConfig controls default batch sizes for create, upsert, relation, and read operations.
type BatchConfig struct {
	CreateSize   int
	UpsertSize   int
	RelationSize int
	ReadSize     int
}

func (config Config) defaultConnectionName() string {
	if config.Default != "" {
		return config.Default
	}
	return "default"
}

func (config Config) location() *time.Location {
	if config.Location != nil {
		return config.Location
	}
	return time.UTC
}

func (config Config) statementCacheSize() int {
	if config.StatementCache.Disabled {
		return 0
	}
	if config.StatementCache.MaxSize > 0 {
		return config.StatementCache.MaxSize
	}
	return 128
}

func (config Config) sqlCacheSize() int {
	if config.SQLCache.Disabled {
		return 0
	}
	if config.SQLCache.MaxSize > 0 {
		return config.SQLCache.MaxSize
	}
	return 256
}

func (config Config) scanCacheSize() int {
	if config.ScanCache.Disabled {
		return 0
	}
	if config.ScanCache.MaxSize > 0 {
		return config.ScanCache.MaxSize
	}
	return 256
}
