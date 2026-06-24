package oro

import (
	"context"
	"database/sql"

	"github.com/duxweb/oro/internal/fifocache"
)

type statementCache struct {
	items *fifocache.Cache[string, *sql.Stmt]
}

func newStatementCache(maxSize int) *statementCache {
	if maxSize <= 0 {
		return nil
	}
	return &statementCache{
		items: fifocache.New[string, *sql.Stmt](maxSize, func(stmt *sql.Stmt) {
			if stmt != nil {
				_ = stmt.Close()
			}
		}),
	}
}

func (cache *statementCache) prepare(ctx context.Context, db *sql.DB, query string) (*sql.Stmt, error) {
	if cache == nil || db == nil || query == "" {
		return nil, nil
	}
	if stmt, ok := cache.items.Get(query); ok {
		return stmt, nil
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	if existing, ok := cache.items.SetIfAbsent(query, stmt); ok {
		_ = stmt.Close()
		return existing, nil
	}
	return stmt, nil
}

func (cache *statementCache) close() error {
	if cache == nil {
		return nil
	}
	var closeErr error
	for _, stmt := range cache.items.Clear() {
		if stmt == nil {
			continue
		}
		if err := stmt.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}
