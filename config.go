package oro

import "time"

type Config struct {
	Default string

	Connections map[string]ConnectionConfig
	TablePrefix string
	Location    *time.Location

	Pool   PoolConfig
	Batch  BatchConfig
	Shards map[string]ShardConfig
	Cache  CacheStore

	Extensions []Extension

	Factory Factory

	LogLevel LogLevel
	Logger   Logger

	SlowQueryThreshold time.Duration
	LogArgs            bool

	AllowRawMultiStatement bool
	SkipDefaultTransaction bool
	Timeout                TimeoutConfig
	Retry                  RetryConfig
	StatementCache         StatementCacheConfig
	SQLCache               SQLCacheConfig
	ScanCache              ScanCacheConfig
}

type TimeoutConfig struct {
	Connect     time.Duration
	Query       time.Duration
	Transaction time.Duration
}

type RetryConfig struct {
	ReadAttempts       int
	TxDeadlockAttempts int
	Backoff            func(attempt int) time.Duration
}

type StatementCacheConfig struct {
	Disabled bool
	MaxSize  int
}

type SQLCacheConfig struct {
	Disabled bool
	MaxSize  int
}

type ScanCacheConfig struct {
	Disabled bool
	MaxSize  int
}

type ConnectionConfig struct {
	Driver Driver
	Reads  []Driver
	Pool   *PoolConfig
}

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PingOnOpen      bool
}

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
