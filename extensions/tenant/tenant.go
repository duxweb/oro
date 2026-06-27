package tenant

import (
	"context"
	"errors"

	"github.com/duxweb/oro"
)

const extensionName = "tenant"

type Config struct {
	Fields   []string
	Resolver Resolver
	Router   Router
}

type Resolver interface {
	ResolveTenant(ctx context.Context) (oro.Map, bool, error)
}

type ResolverFunc func(ctx context.Context) (oro.Map, bool, error)

func (fn ResolverFunc) ResolveTenant(ctx context.Context) (oro.Map, bool, error) {
	return fn(ctx)
}

type Router interface {
	Connection(ctx context.Context, values oro.Map) (string, error)
}

type RouterFunc func(ctx context.Context, values oro.Map) (string, error)

func (fn RouterFunc) Connection(ctx context.Context, values oro.Map) (string, error) {
	return fn(ctx, values)
}

type Option interface {
	applyTenantOption(*Config)
}

type optionFunc func(*Config)

func (fn optionFunc) applyTenantOption(config *Config) {
	fn(config)
}

func Fields(fields ...string) Option {
	return optionFunc(func(config *Config) {
		config.Fields = append([]string(nil), fields...)
	})
}

func WithResolver(resolver Resolver) Option {
	return optionFunc(func(config *Config) {
		config.Resolver = resolver
	})
}

func WithRouter(router Router) Option {
	return optionFunc(func(config *Config) {
		config.Router = router
	})
}

type extension struct {
	config Config
}

func Extension(options ...Option) oro.Extension {
	config := Config{}
	for _, option := range options {
		if option != nil {
			option.applyTenantOption(&config)
		}
	}
	return extension{config: config}
}

func (extension extension) Name() string {
	return extensionName
}

func (extension extension) Install(db *oro.DB) error {
	return nil
}

func Use(db *oro.DB, values oro.Map) *oro.DB {
	return db.WithExtension(extensionName, state{values: copyMap(values)})
}

func Without(db *oro.DB) *oro.DB {
	return db.WithExtension(extensionName, state{without: true})
}

func With(ctx context.Context, values oro.Map) context.Context {
	return context.WithValue(ctx, contextKey{}, state{values: copyMap(values)})
}

func WithoutContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, state{without: true})
}

func Values(ctx context.Context) (oro.Map, bool) {
	state, ok := stateFromContext(ctx)
	if !ok || state.without || len(state.values) == 0 {
		return nil, false
	}
	return copyMap(state.values), true
}

func (extension extension) ApplyConnection(ctx context.Context, db *oro.DB, spec *oro.QuerySpec) error {
	if extension.config.Router == nil || db == nil || spec == nil {
		return nil
	}
	values, without, ok, err := extension.values(ctx, db)
	if err != nil || without || !ok {
		return err
	}
	connection, err := extension.config.Router.Connection(ctx, values)
	if err != nil {
		kind := oro.ErrUnknownTenant
		if errors.Is(err, oro.ErrTenantRequired) {
			kind = oro.ErrTenantRequired
		}
		return &oro.Error{Op: "tenant", Kind: kind, Cause: err}
	}
	if connection == "" {
		return nil
	}
	spec.Connection = connection
	return nil
}

func (extension extension) ApplyQuery(ctx context.Context, db *oro.DB, spec *oro.QuerySpec) error {
	if db == nil || spec == nil {
		return nil
	}
	model := spec.Model
	if model == nil && spec.Table != "" {
		model = oro.SchemaForTable(db, spec.Table)
	}
	if model == nil {
		return nil
	}
	fields := oro.TenantFieldsForSchema(extension.config.Fields, model)
	if len(fields) == 0 {
		return nil
	}
	values, without, ok, err := extension.values(ctx, db)
	if err != nil || without {
		return err
	}
	if !ok {
		return &oro.Error{Op: "tenant", Kind: oro.ErrTenantRequired, Model: model.Name}
	}
	for _, fieldName := range fields {
		field, ok := model.FieldByGo[fieldName]
		if !ok {
			return &oro.Error{Op: "tenant", Kind: oro.ErrUnknownTenant, Model: model.Name, Field: fieldName}
		}
		value, ok := values[fieldName]
		if !ok {
			return &oro.Error{Op: "tenant", Kind: oro.ErrTenantRequired, Model: model.Name, Field: fieldName}
		}
		spec.Where = append(spec.Where, oro.Condition{Field: field.Column, Op: "=", Value: value})
	}
	return nil
}

func (extension extension) ApplyWrite(ctx context.Context, db *oro.DB, spec *oro.WriteSpec) error {
	if db == nil || spec == nil || spec.Model == nil {
		return nil
	}
	fields := oro.TenantFieldsForSchema(extension.config.Fields, spec.Model)
	if len(fields) == 0 {
		return nil
	}
	values, without, ok, err := extension.values(ctx, db)
	if err != nil || without {
		return err
	}
	if !ok {
		return &oro.Error{Op: "tenant", Kind: oro.ErrTenantRequired, Model: spec.Model.Name}
	}
	for rowIndex := range spec.Values {
		for _, fieldName := range fields {
			value, ok := values[fieldName]
			if !ok {
				return &oro.Error{Op: "tenant", Kind: oro.ErrTenantRequired, Model: spec.Model.Name, Field: fieldName}
			}
			field, ok := spec.Model.FieldByGo[fieldName]
			if !ok {
				return &oro.Error{Op: "tenant", Kind: oro.ErrUnknownTenant, Model: spec.Model.Name, Field: fieldName}
			}
			spec.Values[rowIndex][field.Column] = value
		}
	}
	return nil
}

func (extension extension) ShardValues(ctx context.Context, db *oro.DB) (oro.Map, bool, error) {
	values, without, ok, err := extension.values(ctx, db)
	if err != nil || without || !ok {
		return nil, false, err
	}
	return values, true, nil
}

func (extension extension) CacheKeyValues(ctx context.Context, db *oro.DB) (oro.Map, bool, error) {
	values, without, ok, err := extension.values(ctx, db)
	if err != nil || without || !ok {
		return nil, false, err
	}
	return values, true, nil
}

func (extension extension) values(ctx context.Context, db *oro.DB) (oro.Map, bool, bool, error) {
	if state, ok := stateFromDB(db); ok {
		if state.without {
			return nil, true, true, nil
		}
		if len(state.values) > 0 {
			return copyMap(state.values), false, true, nil
		}
	}
	if state, ok := stateFromContext(ctx); ok {
		if state.without {
			return nil, true, true, nil
		}
		if len(state.values) > 0 {
			return copyMap(state.values), false, true, nil
		}
	}
	if extension.config.Resolver != nil {
		values, ok, err := extension.config.Resolver.ResolveTenant(ctx)
		if err != nil || !ok {
			return nil, false, ok, err
		}
		return copyMap(values), false, true, nil
	}
	return nil, false, false, nil
}

type state struct {
	values  oro.Map
	without bool
}

type contextKey struct{}

func stateFromDB(db *oro.DB) (state, bool) {
	value, ok := db.ExtensionState(extensionName)
	if !ok {
		return state{}, false
	}
	state, ok := value.(state)
	return state, ok
}

func stateFromContext(ctx context.Context) (state, bool) {
	value, ok := ctx.Value(contextKey{}).(state)
	return value, ok
}

func copyMap(values oro.Map) oro.Map {
	if len(values) == 0 {
		return nil
	}
	copied := make(oro.Map, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
