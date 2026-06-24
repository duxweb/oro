package contracts

import (
	"context"
	"database/sql"
	"time"

	"github.com/duxweb/oro/internal/meta"
	"github.com/duxweb/oro/internal/queryast"
)

type Driver interface {
	Name() string
	Open(ctx context.Context) (*sql.DB, error)
	Dialect() Dialect
	Inspector(db *sql.DB) Inspector
	TranslateError(err error) error
	Owned() bool
}

type Dialect interface {
	Name() string
	Capabilities() Capabilities
	QuoteIdent(name string) string
	Placeholder(index int) string
	DataType(column meta.ColumnSpec) (string, error)
	NormalizeType(dbType string) (ColumnType, error)
	Compile(stmt queryast.Statement) (queryast.CompiledSQL, error)
	CompileSchema(change meta.SchemaChange) ([]queryast.CompiledSQL, error)
}

type Inspector interface {
	Tables(ctx context.Context) ([]TableInfo, error)
	Table(ctx context.Context, name string) (*meta.TableSpec, error)
	Indexes(ctx context.Context, table string) ([]meta.IndexSpec, error)
	Constraints(ctx context.Context, table string) ([]ConstraintSpec, error)
}

type Capabilities struct {
	Returning       bool
	Upsert          bool
	Savepoint       bool
	LockForUpdate   bool
	LockForShare    bool
	LockNoWait      bool
	LockSkipLocked  bool
	FullJoin        bool
	JSON            bool
	FullText        bool
	CheckConstraint bool
}

type ColumnType struct {
	Logical string
	DBType  string
}

type TableInfo struct {
	Name string
}

type ConstraintSpec struct {
	Name string
	Type string
}

type LogLevel int

const (
	LogLevelSilent LogLevel = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

type Logger interface {
	Log(ctx context.Context, event LogEvent)
}

type LoggerFunc func(ctx context.Context, event LogEvent)

func (fn LoggerFunc) Log(ctx context.Context, event LogEvent) {
	fn(ctx, event)
}

type LogEvent struct {
	Level LogLevel

	Operation string
	Model     string
	Table     string

	SQL      string
	Args     []any
	Duration time.Duration
	Rows     int64

	Err  error
	Slow bool
}
