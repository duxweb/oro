package registry

import (
	"reflect"
	"sync"
)

type Registry[S any] struct {
	mu          sync.RWMutex
	models      map[reflect.Type]*S
	identifiers map[string]*S
}

func New[S any]() *Registry[S] {
	return &Registry[S]{
		models:      map[reflect.Type]*S{},
		identifiers: map[string]*S{},
	}
}

func (registry *Registry[S]) Register(schema *S, model any) {
	typ := ModelType(model)
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.models[typ] = schema
	registry.identifiers[typ.Name()] = schema
	registry.identifiers[ModelIdentifier(typ)] = schema
}

func (registry *Registry[S]) Get(model any) (*S, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	schema, ok := registry.models[ModelType(model)]
	return schema, ok
}

func (registry *Registry[S]) GetType(typ reflect.Type) (*S, bool) {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	schema, ok := registry.models[typ]
	return schema, ok
}

func (registry *Registry[S]) GetIdentifier(identifier string) (*S, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	schema, ok := registry.identifiers[identifier]
	return schema, ok
}

func (registry *Registry[S]) TypeFor(match func(*S) bool) (reflect.Type, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	for typ, schema := range registry.models {
		if match(schema) {
			return typ, true
		}
	}
	return nil, false
}

func (registry *Registry[S]) Schemas() []*S {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	schemas := make([]*S, 0, len(registry.models))
	for _, schema := range registry.models {
		schemas = append(schemas, schema)
	}
	return schemas
}

func ModelType(model any) reflect.Type {
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}

func ModelIdentifier(typ reflect.Type) string {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.String()
}
