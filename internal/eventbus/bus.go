package eventbus

import (
	"context"
	"sync"
)

type Handler[E any] func(context.Context, *E) error

type Unsubscribe func()

type Bus[N comparable, E any] struct {
	mu       sync.RWMutex
	nextID   uint64
	handlers map[N][]entry[E]
}

type entry[E any] struct {
	id      uint64
	handler Handler[E]
}

func New[N comparable, E any]() *Bus[N, E] {
	return &Bus[N, E]{handlers: map[N][]entry[E]{}}
}

func (bus *Bus[N, E]) On(name N, handler Handler[E]) Unsubscribe {
	if bus == nil || handler == nil {
		return func() {}
	}
	bus.mu.Lock()
	bus.nextID++
	id := bus.nextID
	bus.handlers[name] = append(bus.handlers[name], entry[E]{id: id, handler: handler})
	bus.mu.Unlock()

	return func() {
		bus.off(name, id)
	}
}

func (bus *Bus[N, E]) off(name N, id uint64) {
	if bus == nil {
		return
	}
	bus.mu.Lock()
	defer bus.mu.Unlock()
	entries := bus.handlers[name]
	for index, entry := range entries {
		if entry.id == id {
			entries = append(entries[:index], entries[index+1:]...)
			break
		}
	}
	if len(entries) == 0 {
		delete(bus.handlers, name)
		return
	}
	bus.handlers[name] = entries
}

func (bus *Bus[N, E]) Emit(ctx context.Context, name N, event *E) error {
	if bus == nil || event == nil {
		return nil
	}
	bus.mu.RLock()
	entries := append([]entry[E](nil), bus.handlers[name]...)
	bus.mu.RUnlock()
	for _, entry := range entries {
		if err := entry.handler(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func (bus *Bus[N, E]) Has(name N) bool {
	if bus == nil {
		return false
	}
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	return len(bus.handlers[name]) > 0
}
