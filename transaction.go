package oro

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type TxOption interface {
	applyTxOption(*txOptions)
}

type txOptions struct {
	sqlOptions sql.TxOptions
	attempts   int
	timeout    time.Duration
}

type txOptionFunc func(*txOptions)

func (fn txOptionFunc) applyTxOption(options *txOptions) {
	fn(options)
}

type IsolationLevel = sql.IsolationLevel

const (
	// LevelDefault uses the database driver's default isolation level.
	LevelDefault = sql.LevelDefault
	// LevelReadUncommitted maps to sql.LevelReadUncommitted.
	LevelReadUncommitted = sql.LevelReadUncommitted
	// LevelReadCommitted maps to sql.LevelReadCommitted.
	LevelReadCommitted = sql.LevelReadCommitted
	// LevelWriteCommitted maps to sql.LevelWriteCommitted.
	LevelWriteCommitted = sql.LevelWriteCommitted
	// LevelRepeatableRead maps to sql.LevelRepeatableRead.
	LevelRepeatableRead = sql.LevelRepeatableRead
	// LevelSnapshot maps to sql.LevelSnapshot.
	LevelSnapshot = sql.LevelSnapshot
	// LevelSerializable maps to sql.LevelSerializable.
	LevelSerializable = sql.LevelSerializable
	// LevelLinearizable maps to sql.LevelLinearizable.
	LevelLinearizable = sql.LevelLinearizable
)

// TxIsolation sets the transaction isolation level.
func TxIsolation(level IsolationLevel) TxOption {
	return txOptionFunc(func(options *txOptions) {
		options.sqlOptions.Isolation = level
	})
}

// TxReadOnly marks a transaction as read-only.
func TxReadOnly() TxOption {
	return txOptionFunc(func(options *txOptions) {
		options.sqlOptions.ReadOnly = true
	})
}

// TxAttempts sets the number of retry attempts for retryable transaction errors.
func TxAttempts(attempts int) TxOption {
	return txOptionFunc(func(options *txOptions) {
		options.attempts = attempts
	})
}

// TxTimeout sets a timeout for the whole transaction.
func TxTimeout(timeout time.Duration) TxOption {
	return txOptionFunc(func(options *txOptions) {
		options.timeout = timeout
	})
}

// Transaction runs fn inside a transaction and commits when fn returns nil.
//
// If fn returns an error or panics, the transaction is rolled back.
func (db *DB) Transaction(ctx context.Context, fn func(tx *DB) error, opts ...TxOption) error {
	if fn == nil {
		return &Error{Op: "transaction", Kind: ErrInvalidArgument}
	}

	options := applyTxOptions(opts)
	attempts := options.attempts
	if attempts <= 0 {
		if db != nil && db.runtime != nil {
			attempts = db.runtime.Config.Retry.TxDeadlockAttempts
		}
	}
	if attempts <= 0 {
		attempts = 1
	}
	timeout := options.timeout
	if timeout <= 0 {
		timeout = transactionTimeout(db)
	}
	txCtx, cancel := withOperationTimeout(ctx, timeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		err := db.transactionOnce(txCtx, fn, options.sqlOptions)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableTransactionError(err) {
			return err
		}
		if attempt < attempts-1 {
			if err := waitRetryBackoff(txCtx, db, attempt+1); err != nil {
				return err
			}
		}
	}
	return lastErr
}

func (db *DB) transactionOnce(ctx context.Context, fn func(tx *DB) error, sqlOptions sql.TxOptions) (err error) {
	txDB, err := db.Begin(ctx, txOptionsAsOption(sqlOptions))
	if err != nil {
		return err
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = txDB.Rollback(ctx)
			panic(recovered)
		}
		if err != nil {
			_ = txDB.Rollback(ctx)
			return
		}
		err = txDB.Commit(ctx)
	}()

	err = fn(txDB)
	return err
}

// Begin starts a transaction and returns a DB clone bound to it.
func (db *DB) Begin(ctx context.Context, opts ...TxOption) (*DB, error) {
	options := applyTxOptions(opts)
	if db == nil || db.runtime == nil {
		return nil, &Error{Op: "begin", Kind: ErrInvalidArgument}
	}
	if db.session.tx != nil {
		return db.beginNested(ctx)
	}

	conn, err := connectionForQuery(db, db.session.connection)
	if err != nil {
		return nil, err
	}
	tx, err := conn.Primary.BeginTx(ctx, &options.sqlOptions)
	if err != nil {
		return nil, conn.Driver.TranslateError(err)
	}

	clone := *db
	clone.session.tx = &txState{
		connection: conn.Name,
		tx:         tx,
		depth:      0,
	}
	clone.session.connection = conn.Name
	return &clone, nil
}

func (db *DB) beginNested(ctx context.Context) (*DB, error) {
	state := db.session.tx
	if state == nil || state.tx == nil || state.closed {
		return nil, &Error{Op: "begin", Kind: ErrTransactionRequired}
	}
	nextDepth := state.depth + 1
	name := savepointName(nextDepth)
	if _, err := state.tx.ExecContext(ctx, "savepoint "+name); err != nil {
		return nil, translateTxError(db, err)
	}

	clone := *db
	cloneState := *state
	cloneState.depth = nextDepth
	clone.session.tx = &cloneState
	return &clone, nil
}

// Commit commits the current transaction or releases a nested savepoint.
func (db *DB) Commit(ctx context.Context) error {
	state := db.txState()
	if state == nil {
		return &Error{Op: "commit", Kind: ErrTransactionRequired}
	}
	if state.closed {
		return &Error{Op: "commit", Kind: ErrClosed}
	}

	if state.depth > 0 {
		if _, err := state.tx.ExecContext(ctx, "release savepoint "+savepointName(state.depth)); err != nil {
			return translateTxError(db, err)
		}
		state.closed = true
		if err := emitEvent(ctx, db, &Event{Name: AfterCommit, Operation: "commit"}); err != nil {
			return err
		}
		return nil
	}
	if err := state.tx.Commit(); err != nil {
		return translateTxError(db, err)
	}
	state.closed = true
	return emitEvent(ctx, db, &Event{Name: AfterCommit, Operation: "commit"})
}

// Rollback rolls back the current transaction or nested savepoint.
func (db *DB) Rollback(ctx context.Context) error {
	state := db.txState()
	if state == nil {
		return nil
	}
	if state.closed {
		return nil
	}

	if state.depth > 0 {
		if _, err := state.tx.ExecContext(ctx, "rollback to savepoint "+savepointName(state.depth)); err != nil {
			return translateTxError(db, err)
		}
		state.closed = true
		return emitEvent(ctx, db, &Event{Name: AfterRollback, Operation: "rollback"})
	}
	if err := state.tx.Rollback(); err != nil {
		return translateTxError(db, err)
	}
	state.closed = true
	return emitEvent(ctx, db, &Event{Name: AfterRollback, Operation: "rollback"})
}

// Savepoint represents a manually controlled transaction savepoint.
type Savepoint struct {
	db     *DB
	name   string
	closed bool
}

// Savepoint creates a savepoint on the current transaction.
func (db *DB) Savepoint(ctx context.Context) (*Savepoint, error) {
	state := db.txState()
	if state == nil || state.tx == nil || state.closed {
		return nil, &Error{Op: "savepoint", Kind: ErrTransactionRequired}
	}
	name := savepointName(state.depth + 1)
	if _, err := state.tx.ExecContext(ctx, "savepoint "+name); err != nil {
		return nil, translateTxError(db, err)
	}
	return &Savepoint{db: db, name: name}, nil
}

// Rollback rolls back to the savepoint.
func (savepoint *Savepoint) Rollback(ctx context.Context) error {
	if savepoint == nil || savepoint.db == nil || savepoint.closed {
		return nil
	}
	state := savepoint.db.txState()
	if state == nil || state.tx == nil || state.closed {
		return &Error{Op: "savepoint.rollback", Kind: ErrTransactionRequired}
	}
	_, err := state.tx.ExecContext(ctx, "rollback to savepoint "+savepoint.name)
	if err != nil {
		return translateTxError(savepoint.db, err)
	}
	savepoint.closed = true
	return nil
}

// Release releases the savepoint.
func (savepoint *Savepoint) Release(ctx context.Context) error {
	if savepoint == nil || savepoint.db == nil || savepoint.closed {
		return nil
	}
	state := savepoint.db.txState()
	if state == nil || state.tx == nil || state.closed {
		return &Error{Op: "savepoint.release", Kind: ErrTransactionRequired}
	}
	_, err := state.tx.ExecContext(ctx, "release savepoint "+savepoint.name)
	if err != nil {
		return translateTxError(savepoint.db, err)
	}
	savepoint.closed = true
	return nil
}

func (db *DB) txState() *txState {
	if db == nil {
		return nil
	}
	return db.session.tx
}

func applyTxOptions(options []TxOption) txOptions {
	resolved := txOptions{}
	for _, option := range options {
		if option != nil {
			option.applyTxOption(&resolved)
		}
	}
	return resolved
}

func txOptionsAsOption(sqlOptions sql.TxOptions) TxOption {
	return txOptionFunc(func(options *txOptions) {
		options.sqlOptions = sqlOptions
	})
}

func savepointName(depth int) string {
	return fmt.Sprintf("oro_sp_%d", depth)
}

func isRetryableTransactionError(err error) bool {
	return errors.Is(err, ErrDeadlock) || errors.Is(err, ErrSerializationFailure)
}

func waitRetryBackoff(ctx context.Context, db *DB, attempt int) error {
	if db == nil || db.runtime == nil || db.runtime.Config.Retry.Backoff == nil {
		return nil
	}
	duration := db.runtime.Config.Retry.Backoff(attempt)
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func translateTxError(db *DB, err error) error {
	if err == nil {
		return nil
	}
	state := db.txState()
	if state == nil || state.connection == "" {
		return err
	}
	conn, connErr := connectionForQuery(db, state.connection)
	if connErr != nil {
		return err
	}
	return conn.Driver.TranslateError(err)
}
