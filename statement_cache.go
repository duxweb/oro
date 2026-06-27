package oro

import (
	"context"
	"database/sql"
	"sync"
)

// statementCache is a bounded FIFO cache of prepared statements with reference
// counting. A statement evicted while still in flight is not closed immediately
// (which would race a concurrent execution into "statement is closed"); instead
// it is marked and closed by the last in-flight user. This avoids both the
// use-after-close race and leaking evicted statements.
type statementCache struct {
	mu      sync.Mutex
	maxSize int
	items   map[string]*cachedStatement
	order   []string
}

type cachedStatement struct {
	stmt    *sql.Stmt
	refs    int
	evicted bool
}

func newStatementCache(maxSize int) *statementCache {
	if maxSize <= 0 {
		return nil
	}
	return &statementCache{
		maxSize: maxSize,
		items:   make(map[string]*cachedStatement, maxSize),
	}
}

// acquire returns a prepared statement and a release func that the caller MUST
// invoke once it has finished issuing the query/exec. Returns (nil, nil, nil)
// when the cache is disabled so callers can fall back to an unprepared call.
func (cache *statementCache) acquire(ctx context.Context, db *sql.DB, query string) (*sql.Stmt, func(), error) {
	if cache == nil || db == nil || query == "" {
		return nil, nil, nil
	}

	cache.mu.Lock()
	if entry, ok := cache.items[query]; ok {
		entry.refs++
		cache.mu.Unlock()
		return entry.stmt, cache.releaser(entry), nil
	}
	cache.mu.Unlock()

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	cache.mu.Lock()
	if entry, ok := cache.items[query]; ok {
		// Lost a race: another goroutine cached the same statement first.
		entry.refs++
		cache.mu.Unlock()
		_ = stmt.Close()
		return entry.stmt, cache.releaser(entry), nil
	}
	entry := &cachedStatement{stmt: stmt, refs: 1}
	cache.evictLocked()
	cache.items[query] = entry
	cache.order = append(cache.order, query)
	cache.mu.Unlock()
	return stmt, cache.releaser(entry), nil
}

func (cache *statementCache) releaser(entry *cachedStatement) func() {
	return func() {
		cache.mu.Lock()
		entry.refs--
		shouldClose := entry.refs <= 0 && entry.evicted
		cache.mu.Unlock()
		if shouldClose {
			_ = entry.stmt.Close()
		}
	}
}

// evictLocked drops oldest entries until there is room for one more. An evicted
// entry still in use is marked and closed later by its last releaser.
func (cache *statementCache) evictLocked() {
	for len(cache.items) >= cache.maxSize && len(cache.order) > 0 {
		oldest := cache.order[0]
		cache.order = cache.order[1:]
		entry, ok := cache.items[oldest]
		if !ok {
			continue
		}
		delete(cache.items, oldest)
		if entry.refs <= 0 {
			_ = entry.stmt.Close()
		} else {
			entry.evicted = true
		}
	}
}

func (cache *statementCache) close() error {
	if cache == nil {
		return nil
	}
	cache.mu.Lock()
	entries := make([]*cachedStatement, 0, len(cache.items))
	for _, entry := range cache.items {
		entries = append(entries, entry)
	}
	cache.items = make(map[string]*cachedStatement, cache.maxSize)
	cache.order = nil
	cache.mu.Unlock()

	var closeErr error
	for _, entry := range entries {
		if entry == nil || entry.stmt == nil {
			continue
		}
		if err := entry.stmt.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}
