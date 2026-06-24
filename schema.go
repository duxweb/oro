package oro

import "github.com/duxweb/oro/internal/meta"

const (
	SchemaCreateTable  = meta.SchemaCreateTable
	SchemaAddColumn    = meta.SchemaAddColumn
	SchemaCreateIndex  = meta.SchemaCreateIndex
	SchemaUnsafeChange = meta.SchemaUnsafeChange
	SchemaRenameColumn = meta.SchemaRenameColumn
)

type ModelSchema = meta.ModelSchema
type FieldSchema = meta.FieldSchema
type TableSpec = meta.TableSpec
type ColumnSpec = meta.ColumnSpec
type DefaultSpec = meta.DefaultSpec
type IndexSpec = meta.IndexSpec
type SchemaChangeKind = meta.SchemaChangeKind
type SchemaChange = meta.SchemaChange
