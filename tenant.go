package oro

import (
	"context"
	"errors"
)

func tenantFieldsForSchema(config Config, schema *ModelSchema) []string {
	if schema == nil || schema.NoTenant {
		return nil
	}
	if len(schema.TenantFields) > 0 {
		return append([]string(nil), schema.TenantFields...)
	}
	if config.Tenant == nil || len(config.Tenant.Fields) == 0 {
		return nil
	}
	for _, field := range config.Tenant.Fields {
		if _, ok := schema.FieldByGo[field]; !ok {
			return nil
		}
	}
	return append([]string(nil), config.Tenant.Fields...)
}

func applyTenantConnection(ctx context.Context, db *DB, spec *QuerySpec) error {
	if db == nil || db.runtime == nil || db.runtime.Config.Tenant == nil || db.runtime.Config.Tenant.Router == nil {
		return nil
	}
	if db.session.manualConnection || db.session.withoutTenant {
		return nil
	}
	connection, err := db.runtime.Config.Tenant.Router.Connection(ctx, db.session.tenant)
	if err != nil {
		kind := ErrUnknownTenant
		if errors.Is(err, ErrTenantRequired) {
			kind = ErrTenantRequired
		}
		return &Error{Op: "tenant", Kind: kind, Cause: err}
	}
	if connection == "" {
		return nil
	}
	if _, err := db.runtime.Conns.Get(connection); err != nil {
		return &Error{Op: "tenant", Kind: ErrUnknownTenant, Cause: err}
	}
	spec.Connection = connection
	return nil
}

func applyTenantModelConnection(ctx context.Context, db *DB, schema *ModelSchema, spec *QuerySpec) error {
	applyModelConnection(db, schema, spec)
	if schema != nil && schema.NoTenant {
		return nil
	}
	return applyTenantConnection(ctx, db, spec)
}

func applyTenantScope(db *DB, schema *ModelSchema, spec *QuerySpec) error {
	fields := tenantFieldsForSchema(db.runtime.Config, schema)
	if len(fields) == 0 {
		return nil
	}
	if db.session.withoutTenant {
		return nil
	}
	if len(db.session.tenant) == 0 {
		return &Error{Op: "tenant", Kind: ErrTenantRequired, Model: schema.Name}
	}
	for _, fieldName := range fields {
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return &Error{Op: "tenant", Kind: ErrUnknownTenant, Model: schema.Name, Field: fieldName}
		}
		value, ok := db.session.tenant[fieldName]
		if !ok {
			return &Error{Op: "tenant", Kind: ErrTenantRequired, Model: schema.Name, Field: fieldName}
		}
		spec.Where = append(spec.Where, Condition{Field: field.Column, Op: "=", Value: value})
	}
	return nil
}

func applyTenantColumns(db *DB, schema *ModelSchema, row Map) error {
	fields := tenantFieldsForSchema(db.runtime.Config, schema)
	if len(fields) == 0 {
		return nil
	}
	if db.session.withoutTenant {
		return nil
	}
	if len(db.session.tenant) == 0 {
		return &Error{Op: "tenant", Kind: ErrTenantRequired, Model: schema.Name}
	}
	for _, fieldName := range fields {
		value, ok := db.session.tenant[fieldName]
		if !ok {
			return &Error{Op: "tenant", Kind: ErrTenantRequired, Model: schema.Name, Field: fieldName}
		}
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return &Error{Op: "tenant", Kind: ErrUnknownTenant, Model: schema.Name, Field: fieldName}
		}
		row[field.Column] = value
	}
	return nil
}
