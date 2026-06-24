package fifocache

import "sync"

type Cache[K comparable, V any] struct {
	mu      sync.Mutex
	maxSize int
	items   map[K]V
	order   []K
	onEvict func(V)
}

func New[K comparable, V any](maxSize int, onEvict func(V)) *Cache[K, V] {
	if maxSize <= 0 {
		return nil
	}
	return &Cache[K, V]{
		maxSize: maxSize,
		items:   make(map[K]V, maxSize),
		onEvict: onEvict,
	}
}

func (cache *Cache[K, V]) Get(key K) (V, bool) {
	var zero V
	if cache == nil {
		return zero, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	value, ok := cache.items[key]
	return value, ok
}

func (cache *Cache[K, V]) Set(key K, value V) {
	if cache == nil {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.setLocked(key, value)
}

func (cache *Cache[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	var zero V
	if cache == nil {
		return zero, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if existing, ok := cache.items[key]; ok {
		return existing, true
	}
	cache.setLocked(key, value)
	return value, false
}

func (cache *Cache[K, V]) Clear() []V {
	if cache == nil {
		return nil
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	values := make([]V, 0, len(cache.items))
	for _, value := range cache.items {
		values = append(values, value)
	}
	cache.items = make(map[K]V, cache.maxSize)
	cache.order = nil
	return values
}

func (cache *Cache[K, V]) setLocked(key K, value V) {
	if _, ok := cache.items[key]; ok {
		cache.items[key] = value
		return
	}
	if len(cache.items) >= cache.maxSize && len(cache.order) > 0 {
		oldest := cache.order[0]
		cache.order = cache.order[1:]
		if oldValue, ok := cache.items[oldest]; ok && cache.onEvict != nil {
			cache.onEvict(oldValue)
		}
		delete(cache.items, oldest)
	}
	cache.items[key] = value
	cache.order = append(cache.order, key)
}
