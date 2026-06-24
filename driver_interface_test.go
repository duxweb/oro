package oro_test

import (
	"context"
	"database/sql"

	oro "github.com/duxweb/oro"
)

var _ oro.Driver = externalDriver{}

type externalDriver struct{}

func (externalDriver) Name() string {
	return "external"
}

func (externalDriver) Open(context.Context) (*sql.DB, error) {
	return nil, nil
}

func (externalDriver) Dialect() oro.Dialect {
	return externalDialect{}
}

func (externalDriver) Inspector(*sql.DB) oro.Inspector {
	return externalInspector{}
}

func (externalDriver) TranslateError(err error) error {
	return err
}

func (externalDriver) Owned() bool {
	return false
}

type externalDialect struct{}

func (externalDialect) Name() string {
	return "external"
}

func (externalDialect) Capabilities() oro.Capabilities {
	return oro.Capabilities{}
}

func (externalDialect) QuoteIdent(name string) string {
	return name
}

func (externalDialect) Placeholder(int) string {
	return "?"
}

func (externalDialect) DataType(column oro.ColumnSpec) (string, error) {
	return column.Type, nil
}

func (externalDialect) NormalizeType(dbType string) (oro.ColumnType, error) {
	return oro.ColumnType{DBType: dbType}, nil
}

func (externalDialect) Compile(oro.Statement) (oro.CompiledSQL, error) {
	return oro.CompiledSQL{}, nil
}

func (externalDialect) CompileSchema(oro.SchemaChange) ([]oro.CompiledSQL, error) {
	return nil, nil
}

type externalInspector struct{}

func (externalInspector) Tables(context.Context) ([]oro.TableInfo, error) {
	return nil, nil
}

func (externalInspector) Table(context.Context, string) (*oro.TableSpec, error) {
	return nil, nil
}

func (externalInspector) Indexes(context.Context, string) ([]oro.IndexSpec, error) {
	return nil, nil
}

func (externalInspector) Constraints(context.Context, string) ([]oro.ConstraintSpec, error) {
	return nil, nil
}
