package fifocache

import "testing"

// TestCacheEvictsOldestAndFiresOnEvict verifies FIFO eviction past capacity and
// that onEvict is invoked for the evicted value (the hook statementCache relies
// on to close prepared statements).
func TestCacheEvictsOldestAndFiresOnEvict(t *testing.T) {
	var evicted []int
	cache := New[int, int](2, func(value int) { evicted = append(evicted, value) })

	cache.Set(1, 1)
	cache.Set(2, 2)
	cache.Set(3, 3) // over capacity -> evicts oldest (1)

	if _, ok := cache.Get(1); ok {
		t.Fatal("key 1 should have been evicted")
	}
	if _, ok := cache.Get(2); !ok {
		t.Fatal("key 2 should remain")
	}
	if _, ok := cache.Get(3); !ok {
		t.Fatal("key 3 should remain")
	}
	if len(evicted) != 1 || evicted[0] != 1 {
		t.Fatalf("onEvict should fire once for key 1, got %v", evicted)
	}
}

// TestCacheStaysBounded ensures the backing map and order slice never grow past
// maxSize, so sqlCache / modelScanCache cannot leak memory under churn.
func TestCacheStaysBounded(t *testing.T) {
	const maxSize = 8
	evictions := 0
	cache := New[int, int](maxSize, func(int) { evictions++ })

	const inserts = 1000
	for i := 0; i < inserts; i++ {
		cache.Set(i, i)
		if got := len(cache.items); got > maxSize {
			t.Fatalf("items exceeded maxSize: %d > %d", got, maxSize)
		}
		if got := len(cache.order); got > maxSize {
			t.Fatalf("order exceeded maxSize: %d > %d", got, maxSize)
		}
	}
	if want := inserts - maxSize; evictions != want {
		t.Fatalf("expected %d evictions, got %d", want, evictions)
	}
}

// TestCacheUpdateExistingKeepsFIFOOrder documents that updating an existing key
// replaces its value without re-ordering (FIFO, not LRU) and without evicting.
func TestCacheUpdateExistingKeepsFIFOOrder(t *testing.T) {
	evictions := 0
	cache := New[int, int](2, func(int) { evictions++ })

	cache.Set(1, 1)
	cache.Set(2, 2)
	cache.Set(1, 100) // update existing: no eviction, no reorder
	if evictions != 0 {
		t.Fatalf("updating an existing key must not evict, got %d", evictions)
	}
	if value, _ := cache.Get(1); value != 100 {
		t.Fatalf("expected updated value 100, got %d", value)
	}

	cache.Set(3, 3) // evicts the oldest-inserted key (1), despite the update
	if _, ok := cache.Get(1); ok {
		t.Fatal("FIFO must evict the oldest-inserted key even after an update")
	}
	if evictions != 1 {
		t.Fatalf("expected 1 eviction, got %d", evictions)
	}
}
