package oro

import (
	"context"
	"time"

	"github.com/duxweb/oro/internal/cacheutil"
	"github.com/duxweb/oro/internal/memcache"
)

// CacheManager exposes manual invalidation for the configured query cache.
type CacheManager struct {
	store CacheStore
}

// MemoryCacheStore is Oro's built-in in-memory cache implementation.
type MemoryCacheStore = memcache.Store

type cachePayload struct {
	Rows []Map `json:"rows"`
}

func NewMemoryCacheStore() *MemoryCacheStore {
	return memcache.New(memcache.Errors{
		KeyRequired:     &Error{Op: "cache", Kind: ErrCacheKeyRequired},
		InvalidArgument: &Error{Op: "cache", Kind: ErrInvalidArgument},
	})
}

// Cache returns the manual cache manager for db.
func (db *DB) Cache() *CacheManager {
	if db == nil || db.runtime == nil {
		return &CacheManager{}
	}
	return &CacheManager{store: db.runtime.Cache}
}

// Forget removes one cache entry by key.
func (manager *CacheManager) Forget(ctx context.Context, key string) error {
	if key == "" {
		return &Error{Op: "cache", Kind: ErrCacheKeyRequired}
	}
	if manager == nil || manager.store == nil {
		return &Error{Op: "cache", Kind: ErrCacheStoreRequired}
	}
	return manager.store.Forget(ctx, key)
}

// ForgetTag removes all cache entries associated with tag.
func (manager *CacheManager) ForgetTag(ctx context.Context, tag string) error {
	if tag == "" {
		return &Error{Op: "cache", Kind: ErrCacheKeyRequired}
	}
	if manager == nil || manager.store == nil {
		return &Error{Op: "cache", Kind: ErrCacheStoreRequired}
	}
	return manager.store.ForgetTag(ctx, tag)
}

func cacheDuration(spec CacheSpec) time.Duration {
	return time.Duration(spec.TTL)
}

func cacheEnabled(db *DB, spec QuerySpec) bool {
	if !spec.Cache.Enabled {
		return false
	}
	if db == nil || db.session.tx != nil || spec.Lock.Mode != LockNone {
		return false
	}
	return true
}

func validateCacheSpec(db *DB, spec QuerySpec) error {
	if !spec.Cache.Enabled {
		return nil
	}
	if spec.Cache.TTL <= 0 {
		return &Error{Op: "cache", Kind: ErrInvalidArgument, Table: spec.Table, Model: spec.ModelName}
	}
	if db == nil || db.runtime == nil || db.runtime.Cache == nil {
		return &Error{Op: "cache", Kind: ErrCacheStoreRequired, Table: spec.Table, Model: spec.ModelName}
	}
	return nil
}

func cachedRows(ctx context.Context, db *DB, spec QuerySpec, compiled CompiledSQL, load func() ([]Map, error)) ([]Map, error) {
	if err := validateCacheSpec(db, spec); err != nil {
		return nil, err
	}
	if !cacheEnabled(db, spec) {
		return load()
	}
	key, err := cacheKey(ctx, db, spec, compiled)
	if err != nil {
		return nil, err
	}
	value, ok, err := db.runtime.Cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if ok {
		rows, err := decodeCachedRows(value)
		if err != nil {
			return nil, err
		}
		if err := emitCacheEvent(ctx, db, spec, AfterCacheHit, key, nil); err != nil {
			return nil, err
		}
		return rows, nil
	}
	if err := emitCacheEvent(ctx, db, spec, AfterCacheMiss, key, nil); err != nil {
		return nil, err
	}
	rows, err := load()
	if err != nil {
		return nil, err
	}
	payload, err := encodeCachedRows(rows)
	if err != nil {
		return nil, err
	}
	if err := db.runtime.Cache.Set(ctx, key, payload, cacheDuration(spec.Cache), spec.Cache.Tags...); err != nil {
		return nil, err
	}
	return rows, nil
}

func emitCacheEvent(ctx context.Context, db *DB, spec QuerySpec, name EventName, key string, err error) error {
	if !shouldEmitEvent(db, spec.SkipEvents, name) {
		return nil
	}
	return emitEvent(ctx, db, &Event{
		Name:      name,
		Operation: "cache",
		ModelName: spec.ModelName,
		Table:     spec.Table,
		Values:    Map{"key": key},
		Err:       err,
	})
}

func encodeCachedRows(rows []Map) ([]byte, error) {
	value, err := cacheutil.EncodeRows(rows)
	if err != nil {
		return nil, &Error{Op: "cache", Kind: ErrScan, Cause: err}
	}
	return value, nil
}

func decodeCachedRows(value []byte) ([]Map, error) {
	rows, err := cacheutil.DecodeRows(value)
	if err != nil {
		return nil, &Error{Op: "cache", Kind: ErrScan, Cause: err}
	}
	return rows, nil
}

func cacheKey(ctx context.Context, db *DB, spec QuerySpec, compiled CompiledSQL) (string, error) {
	if spec.Cache.Key != "" {
		return spec.Cache.Key, nil
	}
	extensionValues, err := extensionCacheKeyValues(ctx, db)
	if err != nil {
		return "", &Error{Op: "cache", Kind: ErrInvalidArgument, Cause: err}
	}
	key, err := cacheutil.Key(Map{
		"connection": spec.Connection,
		"extensions": extensionValues,
		"shard":      spec.ShardGroup,
		"table":      spec.Table,
		"model":      spec.ModelName,
		"sql":        compiled.SQL,
		"args":       compiled.Args,
		"with":       cacheWithNames(spec.With),
		"lock":       spec.Lock.Mode,
	})
	if err != nil {
		return "", &Error{Op: "cache", Kind: ErrInvalidArgument, Cause: err}
	}
	return key, nil
}

func cacheWithNames(with []WithSpec) []string {
	names := make([]string, 0, len(with))
	for _, item := range with {
		names = append(names, item.Name)
	}
	return names
}
