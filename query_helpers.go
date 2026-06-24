package oro

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/duxweb/oro/internal/queryutil"
)

const defaultChunkSize = 1000

type AggregateExpr struct {
	Func  string
	Field string
	Alias string
}

type WriteOption interface {
	applyWriteOption(*writeOptions)
}

type writeOptions struct {
	only      []string
	omit      []string
	batchSize int
	conflict  *ConflictSpec
	version   *versionCheck
}

type writeOptionFunc func(*writeOptions)

func (fn writeOptionFunc) applyWriteOption(options *writeOptions) {
	fn(options)
}

func Only(fields ...string) WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		options.only = append(options.only, fields...)
	})
}

func Omit(fields ...string) WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		options.omit = append(options.omit, fields...)
	})
}

func BatchSize(size int) WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		options.batchSize = size
	})
}

func CheckVersion(value any) WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		options.version = &versionCheck{Value: value}
	})
}

type versionCheck struct {
	Value any
}

type ConflictBuilder struct {
	fields []string
}

func ConflictBy(fields ...string) ConflictBuilder {
	return ConflictBuilder{fields: append([]string(nil), fields...)}
}

func (builder ConflictBuilder) DoNothing() WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		options.conflict = &ConflictSpec{
			Columns:   append([]string(nil), builder.fields...),
			DoNothing: true,
		}
	})
}

func (builder ConflictBuilder) Update(fields ...string) WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		options.conflict = &ConflictSpec{
			Columns: append([]string(nil), builder.fields...),
			Update:  append([]string(nil), fields...),
		}
	})
}

func (builder ConflictBuilder) UpdateAll() WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		options.conflict = &ConflictSpec{
			Columns:   append([]string(nil), builder.fields...),
			UpdateAll: true,
		}
	})
}

func (builder ConflictBuilder) UpdateMap(values Map) WriteOption {
	return writeOptionFunc(func(options *writeOptions) {
		copied := Map{}
		for key, value := range values {
			copied[key] = value
		}
		options.conflict = &ConflictSpec{
			Columns:   append([]string(nil), builder.fields...),
			UpdateMap: copied,
		}
	})
}

func Count(field string) AggregateExpr {
	return AggregateExpr{Func: "count", Field: field}
}

func Sum(field string) AggregateExpr {
	return AggregateExpr{Func: "sum", Field: field}
}

func Avg(field string) AggregateExpr {
	return AggregateExpr{Func: "avg", Field: field}
}

func Min(field string) AggregateExpr {
	return AggregateExpr{Func: "min", Field: field}
}

func Max(field string) AggregateExpr {
	return AggregateExpr{Func: "max", Field: field}
}

func (expr AggregateExpr) As(alias string) AggregateExpr {
	expr.Alias = alias
	return expr
}

func aggregateSQL(fn string, field string) string {
	if field == "*" {
		return fmt.Sprintf("%s(*)", fn)
	}
	return fmt.Sprintf("%s(%s)", fn, field)
}

func aggregateDecimal(ctx context.Context, db *DB, spec QuerySpec, fn string, field string) (Decimal, error) {
	spec.Select = []SelectExpr{{Expr: aggregateSQL(fn, field), Alias: "value", Raw: true}}
	spec.Order = nil
	spec.Limit = nil
	spec.Offset = nil
	row, err := queryFirstRow(ctx, db, spec)
	if err != nil || row == nil || row["value"] == nil {
		return Decimal("0"), err
	}
	value, err := scalarValue[Decimal](row["value"])
	if err != nil {
		return Decimal("0"), err
	}
	if value == "" {
		return Decimal("0"), nil
	}
	return value, nil
}

func aggregateNull[T any](ctx context.Context, db *DB, spec QuerySpec, fn string, field string) (Null[T], error) {
	spec.Select = []SelectExpr{{Expr: aggregateSQL(fn, field), Alias: "value", Raw: true}}
	spec.Order = nil
	spec.Limit = nil
	spec.Offset = nil
	row, err := queryFirstRow(ctx, db, spec)
	if err != nil || row == nil || row["value"] == nil {
		return NullZero[T](), err
	}
	value, err := scalarValue[T](row["value"])
	if err != nil {
		return NullZero[T](), err
	}
	return NullOf(value), nil
}

func scalarValue[T any](value any) (T, error) {
	var result T
	dest := reflect.ValueOf(&result).Elem()
	if err := assignValue(dest, value); err != nil {
		return result, &Error{Op: "aggregate", Kind: ErrScan, Cause: err}
	}
	return result, nil
}

func ensureAggregateSpec(spec QuerySpec) error {
	if spec.Limit != nil || spec.Offset != nil {
		return &Error{Op: "aggregate", Kind: ErrInvalidQuery}
	}
	return nil
}

type Page[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Pages int   `json:"pages"`
}

type Paginator[T any] struct {
	size  int
	count func(context.Context) (int64, error)
	items func(context.Context, int, int) ([]T, error)
	err   error
}

func (p *Paginator[T]) Page(ctx context.Context, page int) (*Page[T], error) {
	if err := p.validate(page); err != nil {
		return nil, err
	}
	total, err := p.Total(ctx)
	if err != nil {
		return nil, err
	}
	items, err := p.Items(ctx, page)
	if err != nil {
		return nil, err
	}
	return &Page[T]{
		Items: items,
		Total: total,
		Page:  page,
		Size:  p.size,
		Pages: pagesForTotal(total, p.size),
	}, nil
}

func (p *Paginator[T]) Items(ctx context.Context, page int) ([]T, error) {
	if err := p.validate(page); err != nil {
		return nil, err
	}
	offset := (page - 1) * p.size
	return p.items(ctx, p.size, offset)
}

func (p *Paginator[T]) Total(ctx context.Context) (int64, error) {
	if err := p.validate(1); err != nil {
		return 0, err
	}
	return p.count(ctx)
}

func (p *Paginator[T]) Pages(ctx context.Context) (int, error) {
	total, err := p.Total(ctx)
	if err != nil {
		return 0, err
	}
	return pagesForTotal(total, p.size), nil
}

func (p *Paginator[T]) Size() int {
	return p.size
}

func (p *Paginator[T]) validate(page int) error {
	if p.err != nil {
		return p.err
	}
	if p.size < 1 {
		return &Error{Op: "paginate", Kind: ErrInvalidArgument, Field: "size"}
	}
	if page < 1 {
		return &Error{Op: "paginate", Kind: ErrInvalidArgument, Field: "page"}
	}
	if p.count == nil || p.items == nil {
		return &Error{Op: "paginate", Kind: ErrInvalidArgument}
	}
	return nil
}

func pagesForTotal(total int64, size int) int {
	if total <= 0 || size <= 0 {
		return 0
	}
	return int((total + int64(size) - 1) / int64(size))
}

func paginateSpecError(spec QuerySpec) error {
	if spec.Limit != nil || spec.Offset != nil {
		return &Error{Op: "paginate", Kind: ErrInvalidQuery}
	}
	return nil
}

func chunkMaps(ctx context.Context, spec QuerySpec, get func(QuerySpec) ([]Map, error), fn func([]Map) error) error {
	if err := validateChunkSpec("chunk", spec, fn); err != nil {
		return err
	}
	for page := 0; ; page++ {
		chunkSpec := cloneQuerySpec(spec)
		limit := *spec.Limit
		offset := page * limit
		chunkSpec.Limit = &limit
		chunkSpec.Offset = &offset
		rows, err := get(chunkSpec)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := fn(rows); err != nil {
			return err
		}
		if len(rows) < limit {
			return nil
		}
	}
}

func validateChunkSpec(op string, spec QuerySpec, fn any) error {
	if fn == nil {
		return &Error{Op: op, Kind: ErrInvalidArgument}
	}
	if spec.Limit == nil || *spec.Limit < 1 {
		return &Error{Op: op, Kind: ErrInvalidArgument, Field: "size"}
	}
	if len(spec.Order) == 0 {
		return &Error{Op: op, Kind: ErrOrderRequired}
	}
	return nil
}

func applyTableChunkOrder(ctx context.Context, db *DB, spec QuerySpec) (QuerySpec, error) {
	if len(spec.Order) > 0 {
		return spec, nil
	}
	if spec.Table == "" {
		return QuerySpec{}, &Error{Op: "chunk", Kind: ErrOrderRequired}
	}
	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return QuerySpec{}, err
	}
	primaryColumns, err := primaryColumns(ctx, conn, WriteSpec{QuerySpec: spec})
	if err != nil {
		return QuerySpec{}, err
	}
	if len(primaryColumns) == 0 {
		return QuerySpec{}, &Error{Op: "chunk", Kind: ErrOrderRequired, Table: spec.Table}
	}
	spec.Order = append(spec.Order, OrderExpr{Expr: primaryColumns[0]})
	return spec, nil
}

func chunkSpecError(spec QuerySpec) error {
	if spec.Limit != nil || spec.Offset != nil {
		return &Error{Op: "chunk", Kind: ErrInvalidQuery}
	}
	return nil
}

func eachSize(config Config) int {
	if config.Batch.ReadSize > 0 {
		return config.Batch.ReadSize
	}
	if config.Batch.CreateSize > 0 {
		return config.Batch.CreateSize
	}
	if config.Batch.UpsertSize > 0 {
		return config.Batch.UpsertSize
	}
	return defaultChunkSize
}

func connectionForQuery(db *DB, connectionName string) (*Connection, error) {
	if db == nil || db.runtime == nil || db.runtime.Conns == nil {
		return nil, &Error{Op: "connection", Kind: ErrInvalidArgument}
	}
	if connectionName == "" {
		connectionName = db.session.connection
	}
	if db.session.tx != nil && !db.session.tx.closed && connectionName != db.session.tx.connection {
		return nil, &Error{Op: "connection", Kind: ErrTransactionConnection, Field: connectionName}
	}
	return db.runtime.Conns.Get(connectionName)
}

func execForQuery(db *DB, conn *Connection) ExecContext {
	if db != nil && db.session.tx != nil && !db.session.tx.closed && db.session.tx.connection == conn.Name {
		return db.session.tx.tx
	}
	return conn.Primary
}

func execForRead(db *DB, conn *Connection, spec QuerySpec) ExecContext {
	if db == nil || conn == nil || db.session.tx != nil || spec.UsePrimary || spec.Lock.Mode != LockNone {
		return execForQuery(db, conn)
	}
	if read := db.runtime.Conns.PickRead(conn); read != nil {
		return read
	}
	return conn.Primary
}

func execForQueryRuntime(db *DB, conn *Connection) ExecContext {
	if usesDefaultExecutor(db) {
		return runnerForQuery(db, conn)
	}
	return execForQuery(db, conn)
}

func execForReadRuntime(db *DB, conn *Connection, spec QuerySpec) ExecContext {
	if usesDefaultExecutor(db) {
		return runnerForRead(db, conn, spec)
	}
	return execForRead(db, conn, spec)
}

func usesDefaultExecutor(db *DB) bool {
	if db == nil || db.runtime == nil {
		return false
	}
	switch db.runtime.Executor.(type) {
	case sqlExecutor:
		return true
	default:
		return false
	}
}

func runnerForQuery(db *DB, conn *Connection) sqlRunner {
	if db != nil && conn != nil && db.session.tx != nil && !db.session.tx.closed && db.session.tx.connection == conn.Name {
		return sqlRunner{conn: conn, tx: db.session.tx.tx}
	}
	if conn == nil {
		return sqlRunner{}
	}
	return sqlRunner{conn: conn, db: conn.Primary}
}

func runnerForRead(db *DB, conn *Connection, spec QuerySpec) sqlRunner {
	if db == nil || conn == nil || db.session.tx != nil || spec.UsePrimary || spec.Lock.Mode != LockNone {
		return runnerForQuery(db, conn)
	}
	if read := db.runtime.Conns.PickRead(conn); read != nil {
		return sqlRunner{conn: conn, db: read}
	}
	return sqlRunner{conn: conn, db: conn.Primary}
}

func rowInt64(row Map, key string) (int64, error) {
	value, ok := row[key]
	if !ok || value == nil {
		return 0, nil
	}
	switch typedValue := value.(type) {
	case int64:
		return typedValue, nil
	case int:
		return int64(typedValue), nil
	case uint64:
		if typedValue > uint64(^uint64(0)>>1) {
			return 0, &Error{Op: "row", Kind: ErrScan, Field: key, Cause: fmt.Errorf("integer overflow")}
		}
		return int64(typedValue), nil
	default:
		intValue, err := toInt64(typedValue)
		if err != nil {
			return 0, &Error{Op: "row", Kind: ErrScan, Field: key, Cause: err}
		}
		return intValue, nil
	}
}

func queryRows(ctx context.Context, db *DB, spec QuerySpec) ([]Map, error) {
	spec = cloneQuerySpec(spec)
	return queryRowsPrepared(ctx, db, spec)
}

func queryRowsPrepared(ctx context.Context, db *DB, spec QuerySpec) ([]Map, error) {
	if spec.SelectErr != nil {
		return nil, spec.SelectErr
	}
	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return nil, err
	}
	if err := validateQueryLock(db, conn, spec.Lock); err != nil {
		return nil, err
	}
	if err := validateQueryJoins(conn, spec.Joins); err != nil {
		return nil, err
	}
	if err := resolveQuerySources(db, &spec); err != nil {
		return nil, err
	}
	tableNames(db).ApplyQuery(&spec)

	compiled, err := compileSelectSQL(db, conn, spec)
	if err != nil {
		return nil, err
	}
	return cachedRows(ctx, db, spec, compiled, func() ([]Map, error) {
		result, err := queryCompiled(ctx, db, execForReadRuntime(db, conn, spec), spec, compiled, "query")
		if err != nil {
			return nil, translateQueryError(conn, err)
		}
		if result == nil || result.Rows == nil {
			return []Map{}, nil
		}
		return result.Rows, nil
	})
}

func resolveQuerySources(db *DB, spec *QuerySpec) error {
	if err := resolveSource(db, &spec.From); err != nil {
		return err
	}
	for index := range spec.Select {
		if err := resolveSelectSource(db, &spec.Select[index]); err != nil {
			return err
		}
	}
	if err := resolveConditionSources(db, spec.Where); err != nil {
		return err
	}
	if err := resolveConditionSources(db, spec.Having); err != nil {
		return err
	}
	for index := range spec.Joins {
		if spec.Joins[index].Err != nil {
			return spec.Joins[index].Err
		}
		if err := resolveSource(db, &spec.Joins[index].Source); err != nil {
			return err
		}
	}
	return nil
}

func resolveSelectSource(db *DB, item *SelectExpr) error {
	if item == nil || item.Source == nil {
		if item != nil && item.Expr == "__oro_relation_exists__" && len(item.Args) == 1 {
			switch source := item.Args[0].(type) {
			case SourceAST:
				if err := resolveSource(db, &source); err != nil {
					return err
				}
				item.Args[0] = source
			case *SourceAST:
				if err := resolveSource(db, source); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return resolveSource(db, item.Source)
}

func resolveConditionSources(db *DB, conditions []Condition) error {
	for index := range conditions {
		if err := resolveConditionSources(db, conditions[index].Conditions); err != nil {
			return err
		}
		switch value := conditions[index].Value.(type) {
		case CountCondition:
			if value.Source != nil {
				if err := resolveSource(db, value.Source); err != nil {
					return err
				}
				conditions[index].Value = value
			}
		case *SourceAST:
			if err := resolveSource(db, value); err != nil {
				return err
			}
		case SourceAST:
			if err := resolveSource(db, &value); err != nil {
				return err
			}
			conditions[index].Value = value
		case QuerySource:
			source := value.sourceAST()
			if err := resolveSource(db, &source); err != nil {
				return err
			}
			conditions[index].Value = &source
		}
	}
	return nil
}

func resolveSource(db *DB, source *SourceAST) error {
	if source == nil || source.PendingQuery() == nil {
		return nil
	}
	resolved, err := compileQuerySource(db, source.PendingQuery())
	if err != nil {
		return err
	}
	source.Resolve(resolved)
	return nil
}

func compileQuerySource(db *DB, query any) (SourceAST, error) {
	switch typedQuery := query.(type) {
	case *TableQuery:
		spec := cloneQuerySpec(typedQuery.spec)
		if err := resolveQuerySources(db, &spec); err != nil {
			return SourceAST{}, err
		}
		tableNames(db).ApplyQuery(&spec)
		statement, err := db.runtime.Planner.BuildSelect(spec)
		if err != nil {
			return SourceAST{}, err
		}
		selectAST, ok := statement.(SelectAST)
		if !ok {
			return SourceAST{}, &Error{Op: "source", Kind: ErrInvalidArgument}
		}
		return SourceAST{Query: &selectAST}, nil
	case *RawQuery:
		return SourceAST{Raw: &typedQuery.raw}, nil
	default:
		return compileModelQuerySource(db, query)
	}
}

func compileModelQuerySource(db *DB, query any) (SourceAST, error) {
	modelQuery, ok := query.(interface {
		querySourceSpec() (QuerySpec, error)
	})
	if !ok {
		return SourceAST{}, &Error{Op: "source", Kind: ErrInvalidArgument}
	}
	spec, err := modelQuery.querySourceSpec()
	if err != nil {
		return SourceAST{}, err
	}
	if err := resolveQuerySources(db, &spec); err != nil {
		return SourceAST{}, err
	}
	tableNames(db).ApplyQuery(&spec)
	statement, err := db.runtime.Planner.BuildSelect(spec)
	if err != nil {
		return SourceAST{}, err
	}
	selectAST, ok := statement.(SelectAST)
	if !ok {
		return SourceAST{}, &Error{Op: "source", Kind: ErrInvalidArgument}
	}
	return SourceAST{Query: &selectAST}, nil
}

func (query *ModelQuery[T]) querySourceSpec() (QuerySpec, error) {
	spec, _, err := modelQuerySpec(query)
	if err != nil {
		return QuerySpec{}, err
	}
	return spec, nil
}

func validateQuerySource(source SourceAST) error {
	if source.Table != "" || source.Query != nil || source.Raw != nil {
		return nil
	}
	return &Error{Op: "source", Kind: ErrInvalidArgument}
}

func validateQueryJoins(conn *Connection, joins []JoinAST) error {
	if len(joins) == 0 {
		return nil
	}
	for _, join := range joins {
		if join.Type == JoinFull && !conn.Dialect.Capabilities().FullJoin {
			return &Error{Op: "join", Kind: ErrUnsupported}
		}
	}
	return nil
}

func validateQueryLock(db *DB, conn *Connection, lock LockSpec) error {
	if lock.Mode == LockNone {
		return nil
	}
	if db == nil || db.session.tx == nil || db.session.tx.closed {
		return &Error{Op: "lock", Kind: ErrTransactionRequired}
	}
	capabilities := conn.Dialect.Capabilities()
	switch lock.Mode {
	case LockUpdate:
		if !capabilities.LockForUpdate {
			if conn.Dialect.Name() == "sqlite" && !lock.NoWait && !lock.SkipLocked {
				return nil
			}
			return &Error{Op: "lock", Kind: ErrUnsupported}
		}
	case LockShare:
		if !capabilities.LockForShare {
			if conn.Dialect.Name() == "sqlite" && !lock.NoWait && !lock.SkipLocked {
				return nil
			}
			return &Error{Op: "lock", Kind: ErrUnsupported}
		}
	default:
		return &Error{Op: "lock", Kind: ErrInvalidArgument}
	}
	if lock.NoWait && !capabilities.LockNoWait {
		return &Error{Op: "lock", Kind: ErrUnsupported}
	}
	if lock.SkipLocked && !capabilities.LockSkipLocked {
		return &Error{Op: "lock", Kind: ErrUnsupported}
	}
	return nil
}

func queryFirstRow(ctx context.Context, db *DB, spec QuerySpec) (Map, error) {
	limit := 1
	spec.Limit = &limit
	rows, err := queryRows(ctx, db, spec)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func queryFirstRowPrepared(ctx context.Context, db *DB, spec QuerySpec) (Map, error) {
	limit := 1
	spec.Limit = &limit
	rows, err := queryRowsPrepared(ctx, db, spec)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func execRawRows(ctx context.Context, db *DB, raw RawSpec, cache CacheSpec, timeout time.Duration) ([]Map, error) {
	conn, err := connectionForQuery(db, db.session.connection)
	if err != nil {
		return nil, err
	}
	spec := QuerySpec{
		Connection: db.session.connection,
		Cache:      cache,
		Timeout:    int64(timeout),
	}
	compiled := CompiledSQL{
		SQL:  raw.SQL,
		Args: raw.Args,
	}
	return cachedRows(ctx, db, spec, compiled, func() ([]Map, error) {
		result, err := queryCompiled(ctx, db, execForReadRuntime(db, conn, spec), spec, compiled, "raw")
		if err != nil {
			return nil, translateQueryError(conn, err)
		}
		if result == nil || result.Rows == nil {
			return []Map{}, nil
		}
		return result.Rows, nil
	})
}

func execRaw(ctx context.Context, db *DB, raw RawSpec, timeout time.Duration) (int64, error) {
	conn, err := connectionForQuery(db, db.session.connection)
	if err != nil {
		return 0, err
	}
	spec := QuerySpec{Connection: db.session.connection, Timeout: int64(timeout)}
	result, err := execCompiled(ctx, db, execForQueryRuntime(db, conn), spec, CompiledSQL{
		SQL:  raw.SQL,
		Args: raw.Args,
	}, "raw")
	if err != nil {
		return 0, translateQueryError(conn, err)
	}
	return result.RowsAffected, nil
}

func updateRows(ctx context.Context, db *DB, spec WriteSpec) (int64, error) {
	if len(spec.Values) == 0 || len(spec.Values[0]) == 0 {
		return 0, &Error{Op: "update", Kind: ErrInvalidArgument, Table: spec.Table}
	}
	if len(spec.Where) == 0 {
		return 0, &Error{Op: "update", Kind: ErrUnsafeUpdate, Table: spec.Table}
	}

	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return 0, err
	}
	tableNames(db).ApplyWrite(&spec)
	compiled, err := compileUpdateSQL(db, conn, spec)
	if err != nil {
		return 0, err
	}
	result, err := execCompiled(ctx, db, execForQueryRuntime(db, conn), spec.QuerySpec, compiled, "update")
	if err != nil {
		return 0, translateQueryError(conn, err)
	}
	return result.RowsAffected, nil
}

func deleteRows(ctx context.Context, db *DB, spec WriteSpec) (int64, error) {
	if len(spec.Where) == 0 {
		return 0, &Error{Op: "delete", Kind: ErrUnsafeDelete, Table: spec.Table}
	}

	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return 0, err
	}
	tableNames(db).ApplyWrite(&spec)
	compiled, err := compileDeleteSQL(db, conn, spec)
	if err != nil {
		return 0, err
	}
	result, err := execCompiled(ctx, db, execForQueryRuntime(db, conn), spec.QuerySpec, compiled, "delete")
	if err != nil {
		return 0, translateQueryError(conn, err)
	}
	return result.RowsAffected, nil
}

func createRows(ctx context.Context, db *DB, spec WriteSpec) ([]Map, error) {
	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return nil, err
	}
	tableNames(db).ApplyWrite(&spec)
	spec.Returning = conn.Dialect.Capabilities().Returning

	compiled, err := compileInsertSQL(db, conn, spec)
	if err != nil {
		return nil, err
	}
	if !spec.Returning {
		return createRowsWithoutReturning(ctx, db, conn, spec, compiled)
	}

	result, err := queryCompiled(ctx, db, execForQueryRuntime(db, conn), spec.QuerySpec, compiled, "create")
	if err != nil {
		return nil, translateQueryError(conn, err)
	}
	if result == nil || result.Rows == nil {
		return []Map{}, nil
	}
	return result.Rows, nil
}

func upsertRows(ctx context.Context, db *DB, spec WriteSpec) ([]Map, error) {
	conn, err := connectionForQuery(db, spec.Connection)
	if err != nil {
		return nil, err
	}
	tableNames(db).ApplyWrite(&spec)
	if !conn.Dialect.Capabilities().Upsert {
		return nil, &Error{Op: "upsert", Kind: ErrUnsupported, Table: spec.Table}
	}
	spec.Returning = conn.Dialect.Capabilities().Returning

	compiled, err := compileUpsertSQL(db, conn, spec)
	if err != nil {
		return nil, err
	}
	if !spec.Returning {
		return upsertRowsWithoutReturning(ctx, db, conn, spec, compiled)
	}

	result, err := queryCompiled(ctx, db, execForQueryRuntime(db, conn), spec.QuerySpec, compiled, "upsert")
	if err != nil {
		return nil, translateQueryError(conn, err)
	}
	if result == nil || result.Rows == nil {
		return []Map{}, nil
	}
	return result.Rows, nil
}

func upsertRowsWithoutReturning(ctx context.Context, db *DB, conn *Connection, spec WriteSpec, compiled CompiledSQL) ([]Map, error) {
	if len(spec.Values) != 1 {
		rows := make([]Map, 0, len(spec.Values))
		for _, value := range spec.Values {
			nextSpec := spec
			nextSpec.Values = []Map{value}
			createdRows, err := upsertRows(ctx, db, nextSpec)
			if err != nil {
				return nil, err
			}
			rows = append(rows, createdRows...)
		}
		return rows, nil
	}

	result, err := execCompiled(ctx, db, execForQueryRuntime(db, conn), spec.QuerySpec, compiled, "upsert")
	if err != nil {
		return nil, translateQueryError(conn, err)
	}

	primaryColumns, err := primaryColumns(ctx, conn, spec)
	if err != nil {
		return nil, err
	}
	where, err := upsertLookupConditions(spec, primaryColumns, result)
	if err != nil {
		return nil, err
	}

	row, err := queryFirstRow(ctx, db, QuerySpec{
		Connection: spec.Connection,
		Table:      spec.Table,
		Where:      where,
	})
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, &Error{Op: "upsert", Kind: ErrScan, Table: spec.Table}
	}
	return []Map{row}, nil
}

func upsertLookupConditions(spec WriteSpec, primaryColumns []string, result ExecResult) ([]Condition, error) {
	if len(spec.Values) == 0 {
		return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Table: spec.Table}
	}
	row := spec.Values[0]
	if result.HasLastInsertID && len(primaryColumns) == 1 {
		return []Condition{{Field: primaryColumns[0], Op: "=", Value: result.LastInsertID}}, nil
	}
	if len(primaryColumns) > 0 {
		conditions := make([]Condition, 0, len(primaryColumns))
		for _, column := range primaryColumns {
			value, ok := row[column]
			if !ok || value == nil {
				conditions = nil
				break
			}
			conditions = append(conditions, Condition{Field: column, Op: "=", Value: value})
		}
		if len(conditions) == len(primaryColumns) {
			return conditions, nil
		}
	}
	if len(spec.Conflict.Columns) == 0 {
		return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Table: spec.Table}
	}
	conditions := make([]Condition, 0, len(spec.Conflict.Columns))
	for _, column := range spec.Conflict.Columns {
		value, ok := row[column]
		if !ok || value == nil {
			return nil, &Error{Op: "upsert", Kind: ErrInvalidArgument, Table: spec.Table, Field: column}
		}
		conditions = append(conditions, Condition{Field: column, Op: "=", Value: value})
	}
	return conditions, nil
}

func createRowsWithoutReturning(ctx context.Context, db *DB, conn *Connection, spec WriteSpec, compiled CompiledSQL) ([]Map, error) {
	if len(spec.Values) != 1 {
		rows := make([]Map, 0, len(spec.Values))
		for _, value := range spec.Values {
			nextSpec := spec
			nextSpec.Values = []Map{value}
			createdRows, err := createRows(ctx, db, nextSpec)
			if err != nil {
				return nil, err
			}
			rows = append(rows, createdRows...)
		}
		return rows, nil
	}

	result, err := execCompiled(ctx, db, execForQueryRuntime(db, conn), spec.QuerySpec, compiled, "create")
	if err != nil {
		return nil, translateQueryError(conn, err)
	}

	primaryColumns, err := primaryColumns(ctx, conn, spec)
	if err != nil {
		return nil, err
	}
	if len(primaryColumns) != 1 {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: spec.Table}
	}

	primaryColumn := primaryColumns[0]
	primaryValue, ok := spec.Values[0][primaryColumn]
	if result.HasLastInsertID {
		primaryValue = result.LastInsertID
		ok = true
	}
	if !ok || primaryValue == nil {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: spec.Table, Field: primaryColumn}
	}

	row, err := queryFirstRow(ctx, db, QuerySpec{
		Connection: spec.Connection,
		Table:      spec.Table,
		Where: []Condition{{
			Field: primaryColumn,
			Op:    "=",
			Value: primaryValue,
		}},
	})
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, &Error{Op: "create", Kind: ErrScan, Table: spec.Table, Field: primaryColumn}
	}
	return []Map{row}, nil
}

func queryCompiled(ctx context.Context, db *DB, exec ExecContext, spec QuerySpec, compiled CompiledSQL, operation string) (*RowsResult, error) {
	ctx, cancel := withOperationTimeout(ctx, queryTimeout(db, spec))
	defer cancel()
	if err := emitSQLEvent(ctx, db, spec, BeforeSQL, compiled, operation, 0, 0, nil); err != nil {
		return nil, err
	}
	startedAt := time.Now()
	result, err := db.runtime.Executor.Query(ctx, exec, compiled)
	err = wrapContextError("query", err)
	rows := int64(0)
	if result != nil {
		rows = int64(len(result.Rows))
	}
	duration := time.Since(startedAt)
	if afterErr := emitSQLEvent(ctx, db, spec, AfterSQL, compiled, operation, rows, duration, err); afterErr != nil && err == nil {
		err = afterErr
	}
	return result, err
}

func execCompiled(ctx context.Context, db *DB, exec ExecContext, spec QuerySpec, compiled CompiledSQL, operation string) (ExecResult, error) {
	ctx, cancel := withOperationTimeout(ctx, queryTimeout(db, spec))
	defer cancel()
	if err := emitSQLEvent(ctx, db, spec, BeforeSQL, compiled, operation, 0, 0, nil); err != nil {
		return ExecResult{}, err
	}
	startedAt := time.Now()
	result, err := db.runtime.Executor.Exec(ctx, exec, compiled)
	err = wrapContextError("exec", err)
	duration := time.Since(startedAt)
	if afterErr := emitSQLEvent(ctx, db, spec, AfterSQL, compiled, operation, result.RowsAffected, duration, err); afterErr != nil && err == nil {
		err = afterErr
	}
	return result, err
}

func emitSQLEvent(ctx context.Context, db *DB, spec QuerySpec, name EventName, compiled CompiledSQL, operation string, rows int64, duration time.Duration, err error) error {
	if spec.SkipEvents || !hasEventHandlers(db, name) {
		return nil
	}
	return emitEvent(ctx, db, &Event{
		Name:         name,
		Operation:    operation,
		ModelName:    spec.ModelName,
		Table:        spec.Table,
		SQL:          compiled.SQL,
		Args:         append([]any(nil), compiled.Args...),
		RowsAffected: rows,
		Duration:     duration,
		Err:          err,
	})
}

func translateQueryError(conn *Connection, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrHook) || errors.Is(err, ErrEvent) {
		return err
	}
	return conn.Driver.TranslateError(err)
}

func primaryColumns(ctx context.Context, conn *Connection, spec WriteSpec) ([]string, error) {
	if len(spec.Primary) > 0 {
		return spec.Primary, nil
	}
	inspector := conn.Driver.Inspector(conn.Primary)
	if inspector == nil {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument, Table: spec.Table}
	}
	table, err := inspector.Table(ctx, spec.Table)
	if err != nil {
		return nil, conn.Driver.TranslateError(err)
	}
	columns := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		if column.Primary {
			columns = append(columns, column.ColumnName)
		}
	}
	return columns, nil
}

func schemaForModel[T any](db *DB) (*ModelSchema, error) {
	var model T
	typ := reflect.TypeOf(model)
	if typ == nil {
		typ = reflect.TypeOf((*T)(nil)).Elem()
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if db != nil && db.runtime != nil && db.runtime.Registry != nil {
		if schema, ok := db.runtime.Registry.GetType(typ); ok {
			return schema, nil
		}
	}

	modelValue := reflect.New(typ).Interface()
	schema, err := db.runtime.SchemaParser.Parse(modelValue)
	if err != nil {
		return nil, err
	}
	if db.runtime.Registry != nil {
		db.runtime.Registry.Register(schema, modelValue)
	}
	return schema, nil
}

func modelQuerySpec[T any](query *ModelQuery[T]) (QuerySpec, *ModelSchema, error) {
	schema, err := schemaForModel[T](query.db)
	if err != nil {
		return QuerySpec{}, nil, err
	}
	spec := cloneQuerySpec(query.spec)
	spec.Table = schema.Table
	spec.ModelName = schema.Name
	if err := applyTenantModelConnection(context.Background(), query.db, schema, &spec); err != nil {
		return QuerySpec{}, nil, err
	}
	if err := applyShardConnection(context.Background(), query.db, schema, &spec, query.shard, query.allShards); err != nil {
		return QuerySpec{}, nil, err
	}
	conditions, err := convertModelConditions(schema, spec.Where)
	if err != nil {
		return QuerySpec{}, nil, err
	}
	spec.Where = conditions
	if err := applyTenantScope(query.db, schema, &spec); err != nil {
		return QuerySpec{}, nil, err
	}
	if err := resolveModelRelationAggregates(query.db, schema, &spec); err != nil {
		return QuerySpec{}, nil, err
	}
	if err := convertModelSelects(schema, &spec); err != nil {
		return QuerySpec{}, nil, err
	}
	if err := applyModelSelectVisibility(schema, &spec, query.selectHidden); err != nil {
		return QuerySpec{}, nil, err
	}
	applySoftDeleteScope(schema, &spec, query.softDeleteMode)
	return spec, schema, nil
}

func modelWriteSpec[T any](query *ModelQuery[T]) (QuerySpec, *ModelSchema, error) {
	schema, err := schemaForModel[T](query.db)
	if err != nil {
		return QuerySpec{}, nil, err
	}
	spec := cloneQuerySpec(query.spec)
	spec.Table = schema.Table
	spec.ModelName = schema.Name
	if err := applyTenantModelConnection(context.Background(), query.db, schema, &spec); err != nil {
		return QuerySpec{}, nil, err
	}
	if err := applyShardConnection(context.Background(), query.db, schema, &spec, query.shard, query.allShards); err != nil {
		return QuerySpec{}, nil, err
	}
	conditions, err := convertModelConditions(schema, spec.Where)
	if err != nil {
		return QuerySpec{}, nil, err
	}
	spec.Where = conditions
	if err := applyTenantScope(query.db, schema, &spec); err != nil {
		return QuerySpec{}, nil, err
	}
	if err := convertModelSelects(schema, &spec); err != nil {
		return QuerySpec{}, nil, err
	}
	if query.softDeleteMode == softDeleteDefault {
		if softDeleteField, ok := softDeleteField(schema); ok {
			spec.Where = append(spec.Where, isNullCondition(softDeleteField.Column))
		}
	}
	return spec, schema, nil
}

func applyModelConnection(db *DB, schema *ModelSchema, spec *QuerySpec) {
	if db != nil && db.session.manualConnection {
		return
	}
	if schema.Connection != "" {
		spec.Connection = schema.Connection
		return
	}
	if spec.Connection == "" && db != nil && db.runtime != nil {
		spec.Connection = db.runtime.Config.defaultConnectionName()
	}
}

func aggregateModelSpec[T any](query *ModelQuery[T], field string) (QuerySpec, *ModelSchema, error) {
	if err := ensureAggregateSpec(query.spec); err != nil {
		return QuerySpec{}, nil, err
	}
	spec, schema, err := modelQuerySpec(query)
	if err != nil {
		return QuerySpec{}, nil, err
	}
	if _, ok := schema.FieldByGo[field]; !ok {
		return QuerySpec{}, nil, &Error{Op: "aggregate", Kind: ErrUnknownField, Model: schema.Name, Field: field}
	}
	return spec, schema, nil
}

func cloneQuerySpec(spec QuerySpec) QuerySpec {
	clone := spec
	clone.From = cloneSourceAST(spec.From)
	clone.Where = cloneConditions(spec.Where)
	clone.Select = cloneSelectExprs(spec.Select)
	clone.Joins = cloneJoinASTs(spec.Joins)
	clone.Group = append([]string(nil), spec.Group...)
	clone.Having = cloneConditions(spec.Having)
	clone.Order = append([]OrderExpr(nil), spec.Order...)
	clone.With = append([]WithSpec(nil), spec.With...)
	clone.Cache.Tags = append([]string(nil), spec.Cache.Tags...)
	if spec.Limit != nil {
		limit := *spec.Limit
		clone.Limit = &limit
	}
	if spec.Offset != nil {
		offset := *spec.Offset
		clone.Offset = &offset
	}
	return clone
}

func cloneSelectExprs(items []SelectExpr) []SelectExpr {
	cloned := append([]SelectExpr(nil), items...)
	for index := range cloned {
		cloned[index].Args = append([]any(nil), cloned[index].Args...)
		if cloned[index].Source != nil {
			source := cloneSourceAST(*cloned[index].Source)
			cloned[index].Source = &source
		}
	}
	return cloned
}

func cloneConditions(conditions []Condition) []Condition {
	cloned := append([]Condition(nil), conditions...)
	for index := range cloned {
		switch value := cloned[index].Value.(type) {
		case *SourceAST:
			if value == nil {
				break
			}
			copied := cloneSourceAST(*value)
			cloned[index].Value = &copied
		case SourceAST:
			cloned[index].Value = cloneSourceAST(value)
		case CountCondition:
			if value.Source != nil {
				source := cloneSourceAST(*value.Source)
				value.Source = &source
				cloned[index].Value = value
			}
		}
		cloned[index].Conditions = cloneConditions(cloned[index].Conditions)
	}
	return cloned
}

func cloneJoinASTs(joins []JoinAST) []JoinAST {
	cloned := append([]JoinAST(nil), joins...)
	for index := range cloned {
		cloned[index].Source = cloneSourceAST(cloned[index].Source)
		cloned[index].Conditions = cloneJoinConditions(cloned[index].Conditions)
		if cloned[index].Raw != nil {
			raw := *cloned[index].Raw
			raw.Args = append([]any(nil), raw.Args...)
			cloned[index].Raw = &raw
		}
	}
	return cloned
}

func cloneJoinConditions(conditions []JoinCondition) []JoinCondition {
	cloned := append([]JoinCondition(nil), conditions...)
	for index := range cloned {
		cloned[index].Group = cloneJoinConditions(cloned[index].Group)
	}
	return cloned
}

func cloneSourceAST(source SourceAST) SourceAST {
	cloned := source
	if source.Query != nil {
		query := cloneSelectAST(*source.Query)
		cloned.Query = &query
	}
	if source.Raw != nil {
		raw := *source.Raw
		raw.Args = append([]any(nil), raw.Args...)
		cloned.Raw = &raw
	}
	return cloned
}

func cloneSelectAST(ast SelectAST) SelectAST {
	clone := ast
	clone.From = cloneSourceAST(ast.From)
	clone.Joins = cloneJoinASTs(ast.Joins)
	clone.Where = cloneConditions(ast.Where)
	clone.Select = cloneSelectExprs(ast.Select)
	clone.Group = append([]string(nil), ast.Group...)
	clone.Having = cloneConditions(ast.Having)
	clone.Order = append([]OrderExpr(nil), ast.Order...)
	if ast.Limit != nil {
		limit := *ast.Limit
		clone.Limit = &limit
	}
	if ast.Offset != nil {
		offset := *ast.Offset
		clone.Offset = &offset
	}
	return clone
}

func applySoftDeleteScope(schema *ModelSchema, spec *QuerySpec, mode softDeleteMode) {
	softDeleteField, ok := softDeleteField(schema)
	if !ok {
		return
	}
	field := softDeleteField.Column
	switch mode {
	case softDeleteDefault:
		spec.Where = append(spec.Where, isNullCondition(field))
	case softDeleteOnly:
		spec.Where = append(spec.Where, isNotNullCondition(field))
	}
}

func applyQualifiedSoftDeleteScope(schema *ModelSchema, spec *QuerySpec, qualifier string, mode softDeleteMode) {
	softDeleteField, ok := softDeleteField(schema)
	if !ok {
		return
	}
	field := softDeleteField.Column
	if qualifier != "" {
		field = qualifier + "." + field
	}
	switch mode {
	case softDeleteDefault:
		spec.Where = append(spec.Where, isNullCondition(field))
	case softDeleteOnly:
		spec.Where = append(spec.Where, isNotNullCondition(field))
	}
}

func softDeleteField(schema *ModelSchema) (FieldSchema, bool) {
	for _, field := range schema.Fields {
		if field.SoftDelete {
			return field, true
		}
	}
	return FieldSchema{}, false
}

func primaryColumnsForSchema(schema *ModelSchema) []string {
	columns := make([]string, 0, len(schema.Primary))
	for _, fieldName := range schema.Primary {
		if field, ok := schema.FieldByGo[fieldName]; ok {
			columns = append(columns, field.Column)
		}
	}
	return columns
}

func convertModelConditions(schema *ModelSchema, conditions []Condition) ([]Condition, error) {
	converted := make([]Condition, 0, len(conditions))
	for _, condition := range conditions {
		op := strings.ToLower(strings.TrimSpace(condition.Op))
		if op == "invalid" {
			if err, ok := condition.Value.(error); ok {
				return nil, err
			}
			return nil, &Error{Op: "where", Kind: ErrInvalidArgument, Model: schema.Name, Field: condition.Field}
		}
		if op == "group" || op == "not" {
			nested, err := convertModelConditions(schema, condition.Conditions)
			if err != nil {
				return nil, err
			}
			condition.Conditions = nested
			converted = append(converted, condition)
			continue
		}
		if op == "json" {
			convertedCondition, err := convertModelJSONCondition(schema, condition)
			if err != nil {
				return nil, err
			}
			converted = append(converted, convertedCondition)
			continue
		}
		if op == "fulltext" {
			convertedCondition, err := convertModelFullTextCondition(schema, condition)
			if err != nil {
				return nil, err
			}
			converted = append(converted, convertedCondition)
			continue
		}
		convertedCondition, err := convertModelBasicCondition(schema, condition, "where")
		if err != nil {
			return nil, err
		}
		converted = append(converted, convertedCondition)
	}
	return converted, nil
}

func convertModelBasicCondition(schema *ModelSchema, condition Condition, opName string) (Condition, error) {
	op := strings.ToLower(strings.TrimSpace(condition.Op))
	if op == "invalid" {
		if err, ok := condition.Value.(error); ok {
			return Condition{}, err
		}
		return Condition{}, &Error{Op: opName, Kind: ErrInvalidArgument, Model: schema.Name, Field: condition.Field}
	}
	if op == "raw" {
		return condition, nil
	}
	if op == "group" || op == "not" {
		nested, err := convertModelConditions(schema, condition.Conditions)
		if err != nil {
			return Condition{}, err
		}
		condition.Conditions = nested
		return condition, nil
	}
	if op == "exists" {
		return condition, nil
	}
	if op == "count" {
		return condition, nil
	}
	if op == "column" {
		columnCondition, ok := condition.Value.(ColumnCondition)
		if !ok {
			return Condition{}, &Error{Op: opName, Kind: ErrInvalidArgument, Model: schema.Name, Field: condition.Field}
		}
		if isQualifiedIdentifier(condition.Field) {
			return condition, nil
		}
		field, ok := schema.FieldByGo[condition.Field]
		if !ok {
			return Condition{}, &Error{Op: opName, Kind: ErrUnknownField, Model: schema.Name, Field: condition.Field}
		}
		condition.Field = field.Column
		if columnCondition.Right != "" && !isQualifiedIdentifier(columnCondition.Right) {
			rightField, ok := schema.FieldByGo[columnCondition.Right]
			if !ok {
				return Condition{}, &Error{Op: opName, Kind: ErrUnknownField, Model: schema.Name, Field: columnCondition.Right}
			}
			columnCondition.Right = rightField.Column
		}
		condition.Value = columnCondition
		return condition, nil
	}
	if isQualifiedIdentifier(condition.Field) {
		return condition, nil
	}
	field, ok := schema.FieldByGo[condition.Field]
	if !ok {
		return Condition{}, &Error{Op: opName, Kind: ErrUnknownField, Model: schema.Name, Field: condition.Field}
	}
	condition.Field = field.Column
	return condition, nil
}

func convertModelJSONCondition(schema *ModelSchema, condition Condition) (Condition, error) {
	jsonCondition, ok := condition.Value.(JSONCondition)
	if !ok {
		return Condition{}, &Error{Op: "where", Kind: ErrInvalidArgument, Model: schema.Name, Field: condition.Field}
	}
	field, ok := schema.FieldByGo[jsonCondition.Field]
	if !ok {
		return Condition{}, &Error{Op: "where", Kind: ErrUnknownField, Model: schema.Name, Field: jsonCondition.Field}
	}
	if !isJSONFieldType(field.Type) {
		return Condition{}, &Error{Op: "where", Kind: ErrInvalidArgument, Model: schema.Name, Field: jsonCondition.Field}
	}
	jsonCondition.Field = field.Column
	condition.Field = field.Column
	condition.Value = jsonCondition
	return condition, nil
}

func isJSONFieldType(fieldType string) bool {
	return fieldType == "json" || strings.Contains(fieldType, "JSON")
}

func convertModelFullTextCondition(schema *ModelSchema, condition Condition) (Condition, error) {
	expr, ok := condition.Value.(FullTextExpr)
	if !ok {
		return Condition{}, &Error{Op: "where", Kind: ErrInvalidArgument, Model: schema.Name}
	}
	fields, err := convertFullTextFields(schema, expr.Fields)
	if err != nil {
		return Condition{}, err
	}
	expr.Fields = fields
	condition.Value = expr
	return condition, nil
}

func convertFullTextFields(schema *ModelSchema, fields []string) ([]string, error) {
	converted := make([]string, 0, len(fields))
	for _, fieldName := range fields {
		if isQualifiedIdentifier(fieldName) {
			converted = append(converted, fieldName)
			continue
		}
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return nil, &Error{Op: "where", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		converted = append(converted, field.Column)
	}
	return converted, nil
}

func convertModelSelects(schema *ModelSchema, spec *QuerySpec) error {
	for index, item := range spec.Select {
		if item.Source != nil {
			continue
		}
		if item.Expr == "__oro_relation_exists__" {
			continue
		}
		if item.Raw {
			if err := convertModelSelectExpression(schema, &spec.Select[index]); err != nil {
				return err
			}
			continue
		}
		if isQualifiedIdentifier(item.Expr) {
			continue
		}
		field, ok := schema.FieldByGo[item.Expr]
		if !ok {
			return &Error{Op: "select", Kind: ErrUnknownField, Model: schema.Name, Field: item.Expr}
		}
		if field.Hidden {
			return &Error{Op: "select", Kind: ErrInvalidArgument, Model: schema.Name, Field: item.Expr}
		}
		spec.Select[index].Expr = field.Column
	}
	for index, item := range spec.Order {
		if item.Raw {
			continue
		}
		if isQualifiedIdentifier(item.Expr) {
			continue
		}
		field, ok := schema.FieldByGo[item.Expr]
		if !ok {
			return &Error{Op: "order", Kind: ErrUnknownField, Model: schema.Name, Field: item.Expr}
		}
		spec.Order[index].Expr = field.Column
	}
	for index, fieldName := range spec.Group {
		if isQualifiedIdentifier(fieldName) {
			continue
		}
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return &Error{Op: "group", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		spec.Group[index] = field.Column
	}
	for index, condition := range spec.Having {
		if condition.Op == "raw" {
			continue
		}
		converted, err := convertModelBasicCondition(schema, condition, "having")
		if err != nil {
			return err
		}
		spec.Having[index] = converted
	}
	return nil
}

func convertModelSelectExpression(schema *ModelSchema, item *SelectExpr) error {
	if item == nil || item.Expr != "__oro_fulltext_score__" || len(item.Args) == 0 {
		return nil
	}
	expr, ok := item.Args[0].(FullTextExpr)
	if !ok {
		return &Error{Op: "select", Kind: ErrInvalidArgument, Model: schema.Name}
	}
	fields, err := convertFullTextFields(schema, expr.Fields)
	if err != nil {
		return err
	}
	expr.Fields = fields
	item.Args[0] = expr
	return nil
}

func isQualifiedIdentifier(name string) bool {
	return queryutil.IsQualifiedIdentifier(name)
}

func applyModelSelectVisibility(schema *ModelSchema, spec *QuerySpec, hiddenFields []string) error {
	for _, fieldName := range hiddenFields {
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return &Error{Op: "select", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		if !field.Hidden {
			return &Error{Op: "select", Kind: ErrInvalidArgument, Model: schema.Name, Field: fieldName}
		}
		spec.Select = append(spec.Select, SelectExpr{Expr: field.Column})
	}
	if len(spec.Select) > 0 {
		return nil
	}
	if len(schema.DefaultExprs) > 0 {
		spec.Select = append([]SelectExpr(nil), schema.DefaultExprs...)
	}
	return nil
}

func convertModelMap(schema *ModelSchema, values Map, options writeOptions, update bool) (Map, error) {
	allowed := optionSet(options.only)
	omitted := optionSet(options.omit)
	converted := Map{}
	for fieldName, value := range values {
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return nil, &Error{Op: "map", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		if field.Ignore || field.Virtual {
			continue
		}
		if field.Primary || (update && field.AutoCreate) {
			if !allowed[fieldName] {
				continue
			}
		}
		if len(allowed) > 0 && !allowed[fieldName] {
			continue
		}
		if omitted[fieldName] {
			continue
		}
		if update && field.Optimistic {
			return nil, &Error{Op: "map", Kind: ErrInvalidArgument, Model: schema.Name, Field: fieldName}
		}
		converted[field.Column] = value
	}
	return converted, nil
}

func applyOptimisticLock(schema *ModelSchema, spec *QuerySpec, values Map, options writeOptions) error {
	if options.version == nil {
		return nil
	}
	field, ok := optimisticLockField(schema)
	if !ok {
		return &Error{Op: "update", Kind: ErrInvalidArgument, Model: schema.Name, Table: schema.Table}
	}
	if _, ok := values[field.Column]; ok {
		return &Error{Op: "update", Kind: ErrInvalidArgument, Model: schema.Name, Table: schema.Table, Field: field.Name}
	}
	spec.Where = append(spec.Where, Condition{Field: field.Column, Op: "=", Value: options.version.Value})
	values[field.Column] = Increment(1)
	return nil
}

func optimisticLockField(schema *ModelSchema) (FieldSchema, bool) {
	for _, field := range schema.Fields {
		if field.Optimistic {
			return field, true
		}
	}
	return FieldSchema{}, false
}

func convertModelConflict(schema *ModelSchema, conflict *ConflictSpec) (*ConflictSpec, error) {
	if conflict == nil {
		return nil, nil
	}
	converted := &ConflictSpec{
		DoNothing: conflict.DoNothing,
		UpdateAll: conflict.UpdateAll,
		UpdateMap: Map{},
	}
	for _, fieldName := range conflict.Columns {
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return nil, &Error{Op: "conflict", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		if field.Virtual {
			return nil, &Error{Op: "conflict", Kind: ErrInvalidArgument, Model: schema.Name, Field: fieldName}
		}
		converted.Columns = append(converted.Columns, field.Column)
	}
	for _, fieldName := range conflict.Update {
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return nil, &Error{Op: "conflict", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		if field.Virtual {
			return nil, &Error{Op: "conflict", Kind: ErrInvalidArgument, Model: schema.Name, Field: fieldName}
		}
		converted.Update = append(converted.Update, field.Column)
	}
	for fieldName, value := range conflict.UpdateMap {
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return nil, &Error{Op: "conflict", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		if field.Virtual {
			return nil, &Error{Op: "conflict", Kind: ErrInvalidArgument, Model: schema.Name, Field: fieldName}
		}
		converted.UpdateMap[field.Column] = value
	}
	if len(converted.UpdateMap) == 0 {
		converted.UpdateMap = nil
	}
	return converted, nil
}

func autoUpdateColumns(schema *ModelSchema, options writeOptions) Map {
	omitted := optionSet(options.omit)
	values := Map{}
	now := time.Now()
	for _, field := range schema.Fields {
		if field.Virtual {
			continue
		}
		if field.AutoUpdate && !omitted[field.Name] {
			values[field.Column] = now
		}
	}
	return values
}

func buildModelInsertMap(schema *ModelSchema, model any, options writeOptions) (Map, error) {
	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() || modelValue.Kind() != reflect.Pointer || modelValue.IsNil() {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument}
	}
	structValue := modelValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument}
	}

	allowed := optionSet(options.only)
	omitted := optionSet(options.omit)
	row := Map{}
	now := time.Now()

	for _, field := range schema.Fields {
		if field.Ignore || field.Virtual {
			continue
		}
		if len(allowed) > 0 && !allowed[field.Name] {
			continue
		}
		if omitted[field.Name] {
			continue
		}

		if len(field.Index) == 0 {
			continue
		}
		fieldValue := structValue.FieldByIndex(field.Index)
		if !fieldValue.IsValid() || !fieldValue.CanInterface() {
			continue
		}

		if field.AutoCreate || field.AutoUpdate {
			if isZeroValue(fieldValue) {
				if err := assignValue(fieldValue, now); err != nil {
					return nil, err
				}
			}
		}

		if field.Primary && isZeroValue(fieldValue) {
			continue
		}

		value, err := valueForWrite(fieldValue)
		if err != nil {
			return nil, &Error{Op: "create", Kind: ErrScan, Field: field.Name, Cause: err}
		}
		row[field.Column] = value
	}

	if len(row) == 0 {
		return nil, &Error{Op: "create", Kind: ErrInvalidArgument}
	}
	return row, nil
}

func applyWriteOptions(options []WriteOption) writeOptions {
	resolved := writeOptions{}
	for _, option := range options {
		if option != nil {
			option.applyWriteOption(&resolved)
		}
	}
	return resolved
}

func optionSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	return queryutil.StringSet(values)
}

func isZeroValue(value reflect.Value) bool {
	return queryutil.IsZeroValue(value)
}

func valueForWrite(value reflect.Value) (any, error) {
	if !value.IsValid() {
		return nil, nil
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, nil
		}
		return valueForWrite(value.Elem())
	}
	if isNullStruct(value.Type()) {
		valid := value.FieldByName("Valid")
		if !valid.Bool() {
			return nil, nil
		}
		return valueForWrite(value.FieldByName("Value"))
	}
	if value.Type() == jsonRawType {
		return []byte(value.Interface().(JSONRaw)), nil
	}
	return value.Interface(), nil
}
