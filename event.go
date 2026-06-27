package oro

import (
	"context"
	"time"

	"github.com/duxweb/oro/internal/eventbus"
)

type EventName string

const (
	BeforeCreate   EventName = "before_create"
	AfterCreate    EventName = "after_create"
	BeforeUpdate   EventName = "before_update"
	AfterUpdate    EventName = "after_update"
	BeforeDelete   EventName = "before_delete"
	AfterDelete    EventName = "after_delete"
	BeforeRestore  EventName = "before_restore"
	AfterRestore   EventName = "after_restore"
	AfterFind      EventName = "after_find"
	BeforeSQL      EventName = "before_sql"
	AfterSQL       EventName = "after_sql"
	AfterCacheHit  EventName = "after_cache_hit"
	AfterCacheMiss EventName = "after_cache_miss"
	AfterCommit    EventName = "after_commit"
	AfterRollback  EventName = "after_rollback"
)

type EventHandler func(ctx context.Context, event *Event) error

type Unsubscribe func()

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

type EventBus struct {
	inner *eventbus.Bus[EventName, Event]
}

func NewEventBus() *EventBus {
	return &EventBus{inner: eventbus.New[EventName, Event]()}
}

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

func (bus *EventBus) Has(name EventName) bool {
	if bus == nil || bus.inner == nil {
		return false
	}
	return bus.inner.Has(name)
}

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
