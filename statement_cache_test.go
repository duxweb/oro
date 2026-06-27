package oro_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	_ "modernc.org/sqlite"
)

// TestStatementCacheConcurrentEviction stresses the prepared-statement cache
// with many distinct queries and a tiny capacity, so entries are constantly
// evicted while other goroutines are still executing them. With reference
// counting this must never surface "statement is closed".
func TestStatementCacheConcurrentEviction(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		StatementCache: oro.StatementCacheConfig{MaxSize: 2},
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				// Distinct SQL text forces eviction; the bound args route the
				// call through the prepared-statement path.
				query := fmt.Sprintf("select %d as n where ? = ?", i%50)
				if _, err := db.Raw(query, 1, 1).Get(ctx); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent prepared query failed: %v", err)
	}
}
