package oro

import (
	"context"
	"time"

	"github.com/duxweb/oro/internal/schemameta"
	"github.com/duxweb/oro/internal/schemasnapshot"
	"github.com/duxweb/oro/internal/syncdiff"
)

type schemaSyncer struct {
	rt *Runtime
}

const schemaSnapshotTable = schemasnapshot.Table

type schemaSnapshot = schemasnapshot.Snapshot

func (syncer schemaSyncer) Sync(ctx context.Context, db *DB) error {
	if db == nil || db.runtime == nil || db.runtime.Registry == nil {
		return &Error{Op: "sync", Kind: ErrInvalidArgument}
	}

	schemas := db.runtime.Registry.Schemas()
	schemas = uniqueSyncSchemas(db, schemas)
	for _, schema := range schemas {
		if err := syncer.syncSchema(ctx, db, schema); err != nil {
			return err
		}
	}
	return nil
}

func uniqueSyncSchemas(db *DB, schemas []*ModelSchema) []*ModelSchema {
	if len(schemas) < 2 {
		return schemas
	}
	merged := make(map[string]*ModelSchema, len(schemas))
	order := make([]string, 0, len(schemas))
	for _, schema := range schemas {
		if schema == nil {
			continue
		}
		key := schema.Connection + "|" + schema.ShardGroup + "|" + tableNames(db).Physical(schema.Table)
		if existing := merged[key]; existing != nil {
			mergeSyncSchema(existing, schema)
			continue
		}
		copied := copySyncSchema(schema)
		merged[key] = copied
		order = append(order, key)
	}
	out := make([]*ModelSchema, 0, len(order))
	for _, key := range order {
		out = append(out, merged[key])
	}
	return out
}

func copySyncSchema(schema *ModelSchema) *ModelSchema {
	copied := *schema
	copied.Fields = append([]FieldSchema(nil), schema.Fields...)
	copied.Primary = append([]string(nil), schema.Primary...)
	copied.Indexes = append([]IndexSpec(nil), schema.Indexes...)
	return &copied
}

func mergeSyncSchema(target *ModelSchema, source *ModelSchema) {
	fields := map[string]bool{}
	for _, field := range target.Fields {
		fields[field.Column] = true
	}
	for _, field := range source.Fields {
		if fields[field.Column] {
			continue
		}
		target.Fields = append(target.Fields, field)
		fields[field.Column] = true
	}
	primary := map[string]bool{}
	for _, field := range target.Primary {
		primary[field] = true
	}
	for _, field := range source.Primary {
		if !primary[field] {
			target.Primary = append(target.Primary, field)
			primary[field] = true
		}
	}
	indexes := map[string]bool{}
	for _, index := range target.Indexes {
		indexes[index.Name] = true
	}
	for _, index := range source.Indexes {
		if !indexes[index.Name] {
			target.Indexes = append(target.Indexes, index)
			indexes[index.Name] = true
		}
	}
}

func (syncer schemaSyncer) syncSchema(ctx context.Context, db *DB, schema *ModelSchema) error {
	if schema.ShardGroup != "" {
		connections, err := shardConnections(db, schema)
		if err != nil {
			return err
		}
		for _, connection := range connections {
			if err := syncer.syncSchemaOnConnection(ctx, db.Connection(connection), schema); err != nil {
				return err
			}
		}
		return nil
	}
	return syncer.syncSchemaOnConnection(ctx, syncDBForSchema(db, schema), schema)
}

func syncDBForSchema(db *DB, schema *ModelSchema) *DB {
	if db == nil || schema == nil || db.session.manualConnection || schema.Connection == "" {
		return db
	}
	return db.Connection(schema.Connection)
}

func (syncer schemaSyncer) syncSchemaOnConnection(ctx context.Context, db *DB, schema *ModelSchema) error {
	conn, err := connectionForQuery(db, db.session.connection)
	if err != nil {
		return err
	}
	inspector := conn.Driver.Inspector(conn.Primary)
	if inspector == nil {
		return &Error{Op: "sync", Kind: ErrInvalidArgument, Table: schema.Table}
	}

	target := tableSpecFromSchema(schema)
	tableNames(db).ApplyTableSpec(&target)
	snapshotTable := tableNames(db).Snapshot()
	if err := ensureSchemaSnapshotTable(ctx, conn.Primary, db.runtime.Executor, conn.Dialect, snapshotTable); err != nil {
		return conn.Driver.TranslateError(err)
	}
	snapshot, err := loadSchemaSnapshot(ctx, conn.Primary, db.runtime.Executor, conn.Dialect, snapshotTable, schema.Name)
	if err != nil {
		return conn.Driver.TranslateError(err)
	}
	current, err := inspector.Table(ctx, target.Name)
	if err != nil {
		return conn.Driver.TranslateError(err)
	}

	changes := diffTableSpecWithSnapshot(current, target, snapshotTableSpec(snapshot))
	for _, change := range changes {
		sqlStatements, err := conn.Dialect.CompileSchema(change)
		if err != nil {
			return err
		}
		for _, compiled := range sqlStatements {
			if _, err := db.runtime.Executor.Exec(ctx, conn.Primary, compiled); err != nil {
				return conn.Driver.TranslateError(err)
			}
		}
	}
	if err := saveSchemaSnapshot(ctx, conn.Primary, db.runtime.Executor, conn.Dialect, snapshotTable, schema.Name, target); err != nil {
		return conn.Driver.TranslateError(err)
	}
	return nil
}

func tableSpecFromSchema(schema *ModelSchema) TableSpec {
	return schemameta.TableSpecFromSchema(schema)
}

func diffTableSpec(current *TableSpec, target TableSpec) []SchemaChange {
	return syncdiff.Diff(current, target)
}

func diffTableSpecWithSnapshot(current *TableSpec, target TableSpec, snapshot *TableSpec) []SchemaChange {
	return syncdiff.DiffWithSnapshot(current, target, snapshot)
}

func ensureSchemaSnapshotTable(ctx context.Context, exec ExecContext, executor Executor, dialect Dialect, table string) error {
	compiled, err := schemasnapshot.CreateTableSQL(dialect, table)
	if err != nil {
		return err
	}
	_, err = executor.Exec(ctx, exec, compiled)
	return err
}

func loadSchemaSnapshot(ctx context.Context, exec ExecContext, executor Executor, dialect Dialect, table string, model string) (*schemaSnapshot, error) {
	result, err := executor.Query(ctx, exec, schemasnapshot.SelectSQL(dialect, table, model))
	if err != nil {
		return nil, err
	}
	if result == nil || len(result.Rows) == 0 {
		return nil, nil
	}
	return schemasnapshot.FromRows(model, result.Rows), nil
}

func saveSchemaSnapshot(ctx context.Context, exec ExecContext, executor Executor, dialect Dialect, snapshotTable string, model string, table TableSpec) error {
	if _, err := executor.Exec(ctx, exec, schemasnapshot.DeleteSQL(dialect, snapshotTable, model)); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, column := range table.Columns {
		if _, err := executor.Exec(ctx, exec, schemasnapshot.InsertSQL(dialect, snapshotTable, model, table, column, now)); err != nil {
			return err
		}
	}
	return nil
}

func snapshotTableSpec(snapshot *schemaSnapshot) *TableSpec {
	return schemasnapshot.TableSpec(snapshot)
}

func boolInt(value bool) int {
	return schemasnapshot.BoolInt(value)
}

func truthy(value any) bool {
	return schemasnapshot.Truthy(value)
}
