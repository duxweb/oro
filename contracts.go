package oro

import "github.com/duxweb/oro/internal/contracts"

type Driver = contracts.Driver
type Dialect = contracts.Dialect
type Inspector = contracts.Inspector
type Capabilities = contracts.Capabilities
type ColumnType = contracts.ColumnType
type TableInfo = contracts.TableInfo
type ConstraintSpec = contracts.ConstraintSpec
type LogLevel = contracts.LogLevel
type Logger = contracts.Logger
type LoggerFunc = contracts.LoggerFunc
type LogEvent = contracts.LogEvent

const (
	LogLevelSilent = contracts.LogLevelSilent
	LogLevelError  = contracts.LogLevelError
	LogLevelWarn   = contracts.LogLevelWarn
	LogLevelInfo   = contracts.LogLevelInfo
	LogLevelDebug  = contracts.LogLevelDebug
)
