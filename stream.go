package oro

import (
	"context"
	"database/sql"
	"time"

	"github.com/duxweb/oro/internal/rowscan"
	"github.com/duxweb/oro/internal/scanconv"
)

type Stream[T any] interface {
	Next() bool
	Value() T
	Err() error
	Close() error
}

type rowStream struct {
	rows      *sql.Rows
	scanner   rowScanner
	value     Map
	err       error
	closed    bool
	onClose   func(int64, error)
	rowCount  int64
	closeErr  error
	closeDone bool
}

type rowScanner struct {
	inner rowscan.Scanner
}

func newRowScanner(rows *sql.Rows) (rowScanner, error) {
	scanner, err := rowscan.New(rows)
	if err != nil {
		return rowScanner{}, &Error{Op: "scan", Kind: ErrScan, Cause: err}
	}
	return rowScanner{inner: scanner}, nil
}

func (scanner rowScanner) scan(rows *sql.Rows) (Map, error) {
	row, err := scanner.inner.Scan(rows)
	if err != nil {
		return nil, &Error{Op: "scan", Kind: ErrScan, Cause: err}
	}
	return row, nil
}

func scanRows(rows *sql.Rows) ([]Map, error) {
	scanner, err := newRowScanner(rows)
	if err != nil {
		return nil, err
	}

	results := make([]Map, 0)
	for rows.Next() {
		row, err := scanner.scan(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, row)
	}

	return results, nil
}

func normalizeScannedValue(value any, dbType string) any {
	return scanconv.Normalize(value, dbType)
}

func parseTimeString(value string) (time.Time, bool) {
	return scanconv.ParseTimeString(value)
}

func (stream *rowStream) Next() bool {
	if stream == nil || stream.err != nil || stream.closed {
		return false
	}
	if !stream.rows.Next() {
		stream.err = stream.rows.Err()
		_ = stream.Close()
		return false
	}
	row, err := stream.scanner.scan(stream.rows)
	if err != nil {
		stream.err = err
		_ = stream.Close()
		return false
	}
	stream.value = row
	stream.rowCount++
	return true
}

func (stream *rowStream) Value() Map {
	if stream == nil {
		return nil
	}
	return stream.value
}

func (stream *rowStream) Err() error {
	if stream == nil {
		return nil
	}
	if stream.err != nil {
		return stream.err
	}
	return stream.closeErr
}

func (stream *rowStream) Close() error {
	if stream == nil || stream.closeDone {
		return stream.closeErr
	}
	stream.closeDone = true
	stream.closed = true
	stream.closeErr = stream.rows.Close()
	if stream.closeErr != nil && stream.err == nil {
		stream.err = stream.closeErr
	}
	if stream.onClose != nil {
		stream.onClose(stream.rowCount, stream.Err())
	}
	return stream.closeErr
}

type mapStream struct {
	rows *rowStream
}

func (stream *mapStream) Next() bool {
	return stream.rows.Next()
}

func (stream *mapStream) Value() Map {
	return stream.rows.Value()
}

func (stream *mapStream) Err() error {
	return stream.rows.Err()
}

func (stream *mapStream) Close() error {
	return stream.rows.Close()
}

type mappedStream[T any] struct {
	rows  *rowStream
	value T
	err   error
	mapFn func(Map) (T, error)
}

func (stream *mappedStream[T]) Next() bool {
	if stream.err != nil {
		return false
	}
	if !stream.rows.Next() {
		return false
	}
	value, err := stream.mapFn(stream.rows.Value())
	if err != nil {
		stream.err = err
		_ = stream.rows.Close()
		return false
	}
	stream.value = value
	return true
}

func (stream *mappedStream[T]) Value() T {
	return stream.value
}

func (stream *mappedStream[T]) Err() error {
	if stream.err != nil {
		return stream.err
	}
	return stream.rows.Err()
}

func (stream *mappedStream[T]) Close() error {
	return stream.rows.Close()
}

func streamQuery(ctx context.Context, db *DB, spec QuerySpec) (*rowStream, error) {
	spec = cloneQuerySpec(spec)
	return streamQueryPrepared(ctx, db, spec)
}

func streamQueryPrepared(ctx context.Context, db *DB, spec QuerySpec) (*rowStream, error) {
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
	return openRowStream(ctx, db, conn, spec, compiled, "stream")
}

func streamRaw(ctx context.Context, db *DB, raw RawSpec, timeout time.Duration) (*rowStream, error) {
	if err := validateRawSQL(db, raw.SQL); err != nil {
		return nil, err
	}
	conn, err := connectionForQuery(db, db.session.connection)
	if err != nil {
		return nil, err
	}
	return openRowStream(ctx, db, conn, QuerySpec{Connection: db.session.connection, Timeout: int64(timeout)}, CompiledSQL{SQL: raw.SQL, Args: raw.Args}, "stream")
}

func openRowStream(ctx context.Context, db *DB, conn *Connection, spec QuerySpec, compiled CompiledSQL, operation string) (*rowStream, error) {
	ctx, cancel := withOperationTimeout(ctx, queryTimeout(db, spec))
	querier, ok := execForReadRuntime(db, conn, spec).(sqlQuerier)
	if !ok {
		cancel()
		return nil, &Error{Op: operation, Kind: ErrInvalidArgument}
	}

	if err := emitSQLEvent(ctx, db, spec, BeforeSQL, compiled, operation, 0, 0, nil); err != nil {
		cancel()
		return nil, err
	}
	startedAt := time.Now()
	rows, err := querier.QueryContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		cancel()
		streamLog(db, ctx, operation, compiled, 0, startedAt, err)
		_ = emitSQLEvent(ctx, db, spec, AfterSQL, compiled, operation, 0, time.Since(startedAt), err)
		return nil, conn.Driver.TranslateError(err)
	}
	scanner, err := newRowScanner(rows)
	if err != nil {
		cancel()
		_ = rows.Close()
		streamLog(db, ctx, operation, compiled, 0, startedAt, err)
		_ = emitSQLEvent(ctx, db, spec, AfterSQL, compiled, operation, 0, time.Since(startedAt), err)
		return nil, err
	}
	return &rowStream{
		rows:    rows,
		scanner: scanner,
		onClose: func(rowCount int64, err error) {
			cancel()
			streamLog(db, ctx, operation, compiled, rowCount, startedAt, err)
			_ = emitSQLEvent(ctx, db, spec, AfterSQL, compiled, operation, rowCount, time.Since(startedAt), err)
		},
	}, nil
}

func streamLog(db *DB, ctx context.Context, operation string, compiled CompiledSQL, rows int64, startedAt time.Time, err error) {
	if db == nil || db.runtime == nil {
		return
	}
	if executor, ok := db.runtime.Executor.(sqlExecutor); ok {
		executor.log(ctx, operation, compiled, rows, startedAt, err)
	}
}
