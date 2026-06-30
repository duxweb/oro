package oro

import (
	"context"
	"database/sql"
)

type sqlRunner struct {
	conn *Connection
	db   *sql.DB
	tx   *sql.Tx
}

func (runner sqlRunner) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	args = normalizeTimeArgsUTC(args)
	if runner.tx != nil {
		return runner.tx.QueryContext(ctx, query, args...)
	}
	if runner.db == nil && runner.conn != nil {
		return nil, &Error{Op: "query", Kind: ErrClosed}
	}
	if runner.db == nil {
		return nil, &Error{Op: "query", Kind: ErrInvalidArgument}
	}
	if len(args) == 0 {
		return runner.db.QueryContext(ctx, query, args...)
	}
	stmt, release, err := runner.conn.statement(ctx, runner.db, query)
	if err != nil {
		return nil, err
	}
	if stmt != nil {
		if release != nil {
			defer release()
		}
		return stmt.QueryContext(ctx, args...)
	}
	return runner.db.QueryContext(ctx, query, args...)
}

func (runner sqlRunner) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	args = normalizeTimeArgsUTC(args)
	if runner.tx != nil {
		return runner.tx.ExecContext(ctx, query, args...)
	}
	if runner.db == nil && runner.conn != nil {
		return nil, &Error{Op: "exec", Kind: ErrClosed}
	}
	if runner.db == nil {
		return nil, &Error{Op: "exec", Kind: ErrInvalidArgument}
	}
	if len(args) == 0 {
		return runner.db.ExecContext(ctx, query, args...)
	}
	stmt, release, err := runner.conn.statement(ctx, runner.db, query)
	if err != nil {
		return nil, err
	}
	if stmt != nil {
		if release != nil {
			defer release()
		}
		return stmt.ExecContext(ctx, args...)
	}
	return runner.db.ExecContext(ctx, query, args...)
}
