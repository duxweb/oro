package memcache

import (
	"context"
	"sync"
	"time"
)

type Store struct {
	mu    sync.RWMutex
	items map[string]item
	tags  map[string]map[string]bool
	now   func() time.Time
	errs  Errors
}

type Errors struct {
	KeyRequired     error
	InvalidArgument error
}

type item struct {
	value     []byte
	expiresAt time.Time
	tags      []string
}

func New(errs Errors) *Store {
	return &Store{
		items: map[string]item{},
		tags:  map[string]map[string]bool{},
		now:   time.Now,
		errs:  errs,
	}
}

func (store *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if key == "" {
		return nil, false, store.errs.KeyRequired
	}
	store.mu.RLock()
	item, ok := store.items[key]
	store.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !item.expiresAt.IsZero() && !store.now().Before(item.expiresAt) {
		_ = store.Forget(ctx, key)
		return nil, false, nil
	}
	return append([]byte(nil), item.value...), true, nil
}

func (store *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration, tags ...string) error {
	if key == "" {
		return store.errs.KeyRequired
	}
	if ttl <= 0 {
		return store.errs.InvalidArgument
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.forgetLocked(key)
	item := item{
		value:     append([]byte(nil), value...),
		expiresAt: store.now().Add(ttl),
		tags:      append([]string(nil), tags...),
	}
	store.items[key] = item
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if store.tags[tag] == nil {
			store.tags[tag] = map[string]bool{}
		}
		store.tags[tag][key] = true
	}
	return nil
}

func (store *Store) Forget(ctx context.Context, key string) error {
	if key == "" {
		return store.errs.KeyRequired
	}
	store.mu.Lock()
	store.forgetLocked(key)
	store.mu.Unlock()
	return nil
}

func (store *Store) ForgetTag(ctx context.Context, tag string) error {
	if tag == "" {
		return store.errs.KeyRequired
	}
	store.mu.Lock()
	keys := store.tags[tag]
	for key := range keys {
		store.forgetLocked(key)
	}
	delete(store.tags, tag)
	store.mu.Unlock()
	return nil
}

func (store *Store) forgetLocked(key string) {
	item, ok := store.items[key]
	if ok {
		for _, tag := range item.tags {
			if keys := store.tags[tag]; keys != nil {
				delete(keys, key)
				if len(keys) == 0 {
					delete(store.tags, tag)
				}
			}
		}
	}
	delete(store.items, key)
}
