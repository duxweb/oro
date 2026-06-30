package oro

import (
	"context"
	"time"

	"github.com/duxweb/oro/internal/eventbus"
)

// EventName identifies an ORM lifecycle event.
type EventName string

const (
	// BeforeCreate is emitted before a model create.
	BeforeCreate EventName = "before_create"
	// AfterCreate is emitted after a model create.
	AfterCreate EventName = "after_create"
	// BeforeUpdate is emitted before a model update.
	BeforeUpdate EventName = "before_update"
	// AfterUpdate is emitted after a model update.
	AfterUpdate EventName = "after_update"
	// BeforeDelete is emitted before a model delete.
	BeforeDelete EventName = "before_delete"
	// AfterDelete is emitted after a model delete.
	AfterDelete EventName = "after_delete"
	// BeforeRestore is emitted before a soft-deleted model restore.
	BeforeRestore EventName = "before_restore"
	// AfterRestore is emitted after a soft-deleted model restore.
	AfterRestore EventName = "after_restore"
	// AfterFind is emitted after a model is loaded.
	AfterFind EventName = "after_find"
	// BeforeSQL is emitted before SQL execution.
	BeforeSQL EventName = "before_sql"
	// AfterSQL is emitted after SQL execution.
	AfterSQL EventName = "after_sql"
	// AfterCacheHit is emitted after a query cache hit.
	AfterCacheHit EventName = "after_cache_hit"
	// AfterCacheMiss is emitted after a query cache miss.
	AfterCacheMiss EventName = "after_cache_miss"
	// AfterCommit is emitted after a transaction commit.
	AfterCommit EventName = "after_commit"
	// AfterRollback is emitted after a transaction rollback.
	AfterRollback EventName = "after_rollback"
)

// EventHandler handles one emitted event.
type EventHandler func(ctx context.Context, event *Event) error

// Unsubscribe removes a previously registered event handler.
type Unsubscribe func()

// Event describes ORM lifecycle, SQL, cache, and transaction activity.
type Event struct {
	Name EventName
	DB   *DB

	ModelName string
	Table     string
	Model     any
	Schema    *ModelSchema

	Operation    string
	Values       Map
	RowsAffected int64
	SoftDelete   bool

	SQL      string
	Args     []any
	Duration time.Duration
	Err      error
}

// EventBus dispatches events to registered handlers.
type EventBus struct {
	inner *eventbus.Bus[EventName, Event]
}

// NewEventBus creates an empty event bus.
func NewEventBus() *EventBus {
	return &EventBus{inner: eventbus.New[EventName, Event]()}
}

// On registers a handler for name and returns an unsubscribe function.
func (bus *EventBus) On(name EventName, handler EventHandler) Unsubscribe {
	if bus == nil || handler == nil {
		return func() {}
	}
	if bus.inner == nil {
		bus.inner = eventbus.New[EventName, Event]()
	}
	off := bus.inner.On(name, eventbus.Handler[Event](handler))
	return func() {
		off()
	}
}

// Emit dispatches event to handlers registered for event.Name.
func (bus *EventBus) Emit(ctx context.Context, event *Event) error {
	if bus == nil || event == nil {
		return nil
	}
	if bus.inner == nil {
		return nil
	}
	if err := bus.inner.Emit(ctx, event.Name, event); err != nil {
		return &Error{Op: "event", Kind: ErrEvent, Cause: err}
	}
	return nil
}

// Has reports whether handlers are registered for name.
func (bus *EventBus) Has(name EventName) bool {
	if bus == nil || bus.inner == nil {
		return false
	}
	return bus.inner.Has(name)
}

// On registers an event handler on db's runtime event bus.
func (db *DB) On(name EventName, handler EventHandler) Unsubscribe {
	if db == nil || db.runtime == nil || db.runtime.Events == nil {
		return func() {}
	}
	return db.runtime.Events.On(name, handler)
}

func emitEvent(ctx context.Context, db *DB, event *Event) error {
	if db == nil || db.runtime == nil || db.runtime.Events == nil || event == nil {
		return nil
	}
	if event.DB == nil {
		event.DB = db
	}
	if event.Name == "" {
		return nil
	}
	return db.runtime.Events.Emit(ctx, event)
}

func hasEventHandlers(db *DB, name EventName) bool {
	return db != nil && db.runtime != nil && db.runtime.Events != nil && db.runtime.Events.Has(name)
}

func shouldEmitEvent(db *DB, skip bool, names ...EventName) bool {
	if skip || db == nil || db.runtime == nil || db.runtime.Events == nil {
		return false
	}
	for _, name := range names {
		if db.runtime.Events.Has(name) {
			return true
		}
	}
	return false
}
