package oro

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openStmtCacheDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// stmtUsable reports whether a prepared statement is still open. A closed
// *sql.Stmt fails QueryContext with "sql: statement is closed" before touching
// the driver, so this distinguishes live from closed statements.
func stmtUsable(stmt *sql.Stmt) bool {
	rows, err := stmt.QueryContext(context.Background())
	if err != nil {
		return false
	}
	_ = rows.Close()
	return true
}

// TestStatementCacheClosesEvictedIdle proves an idle (refs==0) statement is
// actually closed when evicted — i.e. eviction does not leak prepared
// statements — and that the cache stays bounded by maxSize.
func TestStatementCacheClosesEvictedIdle(t *testing.T) {
	db := openStmtCacheDB(t)
	ctx := context.Background()
	cache := newStatementCache(2)

	s0, rel0, err := cache.acquire(ctx, db, "select 0")
	if err != nil {
		t.Fatal(err)
	}
	rel0() // refs back to 0; s0 stays cached and usable
	if !stmtUsable(s0) {
		t.Fatal("s0 should still be open while cached")
	}

	// Two more distinct queries push the cache past capacity, evicting s0.
	_, rel1, err := cache.acquire(ctx, db, "select 1")
	if err != nil {
		t.Fatal(err)
	}
	rel1()
	_, rel2, err := cache.acquire(ctx, db, "select 2")
	if err != nil {
		t.Fatal(err)
	}
	rel2()

	if stmtUsable(s0) {
		t.Fatal("evicted idle statement must be closed (statement leak)")
	}

	cache.mu.Lock()
	items, order := len(cache.items), len(cache.order)
	cache.mu.Unlock()
	if items > 2 || order > 2 {
		t.Fatalf("cache must stay bounded by maxSize: items=%d order=%d", items, order)
	}
}

// TestStatementCacheDefersCloseWhileInFlight proves a statement evicted while
// still referenced is not closed out from under its user (no use-after-close),
// and is closed by the last release.
func TestStatementCacheDefersCloseWhileInFlight(t *testing.T) {
	db := openStmtCacheDB(t)
	ctx := context.Background()
	cache := newStatementCache(1)

	s0, rel0, err := cache.acquire(ctx, db, "select 0") // refs = 1, in flight
	if err != nil {
		t.Fatal(err)
	}

	// A second distinct query evicts s0 while it is still referenced.
	_, rel1, err := cache.acquire(ctx, db, "select 1")
	if err != nil {
		t.Fatal(err)
	}
	rel1()

	if !stmtUsable(s0) {
		t.Fatal("in-flight evicted statement must stay open until released")
	}

	rel0() // last reference released -> now closed
	if stmtUsable(s0) {
		t.Fatal("evicted statement must be closed after its last release")
	}
}

// TestStatementCacheCloseClosesAll proves teardown closes every cached
// statement.
func TestStatementCacheCloseClosesAll(t *testing.T) {
	db := openStmtCacheDB(t)
	ctx := context.Background()
	cache := newStatementCache(4)

	s0, rel0, err := cache.acquire(ctx, db, "select 0")
	if err != nil {
		t.Fatal(err)
	}
	rel0()
	s1, rel1, err := cache.acquire(ctx, db, "select 1")
	if err != nil {
		t.Fatal(err)
	}
	rel1()

	if err := cache.close(); err != nil {
		t.Fatal(err)
	}
	if stmtUsable(s0) || stmtUsable(s1) {
		t.Fatal("close() must close all cached statements")
	}

	cache.mu.Lock()
	items := len(cache.items)
	cache.mu.Unlock()
	if items != 0 {
		t.Fatalf("close() must clear the cache, got %d items", items)
	}
}
