package oro

import (
	"context"
	"database/sql"
	"time"
)

type sqlQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type sqlExecutor struct {
	rt *Runtime
}

func (executor sqlExecutor) Query(ctx context.Context, exec ExecContext, compiled CompiledSQL) (*RowsResult, error) {
	querier, ok := exec.(sqlQuerier)
	if !ok {
		return nil, &Error{Op: "query", Kind: ErrInvalidArgument}
	}

	startedAt := time.Now()
	rows, err := querier.QueryContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		executor.log(ctx, "query", compiled, 0, startedAt, err)
		return nil, &Error{Op: "query", Kind: err, SQL: compiled.SQL, Args: compiled.Args, Cause: err}
	}
	defer rows.Close()

	maps, err := scanRows(rows)
	if err != nil {
		executor.log(ctx, "query", compiled, 0, startedAt, err)
		return nil, err
	}
	if err := rows.Err(); err != nil {
		executor.log(ctx, "query", compiled, int64(len(maps)), startedAt, err)
		return nil, &Error{Op: "query", Kind: err, SQL: compiled.SQL, Args: compiled.Args, Cause: err}
	}

	executor.log(ctx, "query", compiled, int64(len(maps)), startedAt, nil)
	return &RowsResult{Rows: maps}, nil
}

func (executor sqlExecutor) Exec(ctx context.Context, exec ExecContext, compiled CompiledSQL) (ExecResult, error) {
	execer, ok := exec.(sqlExecer)
	if !ok {
		return ExecResult{}, &Error{Op: "exec", Kind: ErrInvalidArgument}
	}

	startedAt := time.Now()
	result, err := execer.ExecContext(ctx, compiled.SQL, compiled.Args...)
	if err != nil {
		executor.log(ctx, "exec", compiled, 0, startedAt, err)
		return ExecResult{}, &Error{Op: "exec", Kind: err, SQL: compiled.SQL, Args: compiled.Args, Cause: err}
	}

	rowsAffected, rowsAffectedErr := result.RowsAffected()
	if rowsAffectedErr != nil {
		rowsAffected = 0
	}
	lastInsertID, lastInsertIDErr := result.LastInsertId()

	executor.log(ctx, "exec", compiled, rowsAffected, startedAt, nil)
	return ExecResult{
		RowsAffected:    rowsAffected,
		LastInsertID:    lastInsertID,
		HasLastInsertID: lastInsertIDErr == nil,
	}, nil
}

func (executor sqlExecutor) log(ctx context.Context, operation string, compiled CompiledSQL, rows int64, startedAt time.Time, err error) {
	if executor.rt == nil || executor.rt.Logger == nil {
		return
	}

	duration := time.Since(startedAt)
	level := LogLevelDebug
	if err != nil {
		level = LogLevelError
	} else if executor.rt.Config.SlowQueryThreshold > 0 && duration >= executor.rt.Config.SlowQueryThreshold {
		level = LogLevelWarn
	}
	if executor.rt.Config.LogLevel != LogLevelSilent && level > executor.rt.Config.LogLevel {
		return
	}

	event := LogEvent{
		Level:     level,
		Operation: operation,
		SQL:       compiled.SQL,
		Duration:  duration,
		Rows:      rows,
		Err:       err,
		Slow:      executor.rt.Config.SlowQueryThreshold > 0 && duration >= executor.rt.Config.SlowQueryThreshold,
	}
	if executor.rt.Config.LogArgs {
		event.Args = compiled.Args
	}
	executor.rt.Logger.Log(ctx, event)
}
