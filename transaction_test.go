package oro_test

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"io"
	"testing"

	oro "github.com/duxweb/oro"
)

func TestTransactionCommitAndRollback(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	err := db.Transaction(ctx, func(tx *oro.DB) error {
		_, err := tx.Table("products").Create(ctx, oro.Map{
			"code":  "T001",
			"price": 10,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	exists, err := db.Table("products").Where("code", "T001").Exists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected committed row")
	}

	rollbackErr := errors.New("rollback")
	err = db.Transaction(ctx, func(tx *oro.DB) error {
		_, err := tx.Table("products").Create(ctx, oro.Map{
			"code":  "T002",
			"price": 20,
		})
		if err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollback error, got %v", err)
	}

	exists, err = db.Table("products").Where("code", "T002").Exists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected rolled back row to be missing")
	}
}

func TestNestedTransactionSavepoint(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	err := db.Transaction(ctx, func(tx *oro.DB) error {
		_, err := tx.Table("products").Create(ctx, oro.Map{
			"code":  "N001",
			"price": 10,
		})
		if err != nil {
			return err
		}

		_ = tx.Transaction(ctx, func(tx2 *oro.DB) error {
			_, err := tx2.Table("products").Create(ctx, oro.Map{
				"code":  "N002",
				"price": 20,
			})
			if err != nil {
				return err
			}
			return errors.New("inner rollback")
		})

		_, err = tx.Table("products").Create(ctx, oro.Map{
			"code":  "N003",
			"price": 30,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := db.Table("products").Select("code").OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["code"] != "N001" || rows[1]["code"] != "N003" {
		t.Fatalf("unexpected rows after nested transaction %#v", rows)
	}
}

func TestManualSavepointRollback(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Table("products").Create(ctx, oro.Map{"code": "S001", "price": 10}); err != nil {
		t.Fatal(err)
	}

	savepoint, err := tx.Savepoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Table("products").Create(ctx, oro.Map{"code": "S002", "price": 20}); err != nil {
		t.Fatal(err)
	}
	if err := savepoint.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Table("products").Select("code").OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["code"] != "S001" {
		t.Fatalf("unexpected rows after savepoint rollback %#v", rows)
	}
}

func TestTransactionConnectionMismatch(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	err := db.Transaction(ctx, func(tx *oro.DB) error {
		_, err := tx.Connection("other").Table("products").Get(ctx)
		return err
	})
	if !errors.Is(err, oro.ErrTransactionConnection) {
		t.Fatalf("expected transaction connection error, got %v", err)
	}
}

func TestSQLiteQueryLockRequiresTransaction(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Table("products").LockForUpdate().First(ctx)
	if !errors.Is(err, oro.ErrTransactionRequired) {
		t.Fatalf("expected transaction required error, got %v", err)
	}
}

func TestSQLiteQueryLockInTransaction(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "L001", "price": 10}); err != nil {
		t.Fatal(err)
	}

	err := db.Transaction(ctx, func(tx *oro.DB) error {
		row, err := tx.Table("products").Where("code", "L001").LockForUpdate().First(ctx)
		if err != nil {
			return err
		}
		if row == nil || row["code"] != "L001" {
			t.Fatalf("unexpected locked row %#v", row)
		}
		_, err = tx.Table("products").LockForUpdate(oro.SkipLocked()).First(ctx)
		if !errors.Is(err, oro.ErrUnsupported) {
			t.Fatalf("expected unsupported skip locked, got %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBeginCommitRollbackIdempotency(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestClosedTransactionDoesNotFallBackToPool(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Table("products").Create(ctx, oro.Map{"code": "CLOSED", "price": 10}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Table("products").Create(ctx, oro.Map{"code": "LEAK", "price": 20})
	if !errors.Is(err, oro.ErrClosed) {
		t.Fatalf("expected closed transaction error, got %v", err)
	}
}

func TestTransactionRetriesRetryableCommitError(t *testing.T) {
	ctx := context.Background()
	state := &retrySQLState{commitErrors: []error{oro.ErrSerializationFailure}}
	sql.Register("oro_retry_commit", retrySQLDriver{state: state})
	sqlDB, err := sql.Open("oro_retry_commit", "")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: retryORMDriver{db: sqlDB}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

	calls := 0
	err = db.Transaction(ctx, func(tx *oro.DB) error {
		calls++
		_, execErr := tx.Raw("select 1").Exec(ctx)
		return execErr
	}, oro.TxAttempts(2))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected two attempts, got %d", calls)
	}
}

func TestTransactionUsesConfiguredRetryAttempts(t *testing.T) {
	ctx := context.Background()
	state := &retrySQLState{commitErrors: []error{oro.ErrDeadlock}}
	sql.Register("oro_retry_config", retrySQLDriver{state: state})
	sqlDB, err := sql.Open("oro_retry_config", "")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: retryORMDriver{db: sqlDB}},
		},
		Retry: oro.RetryConfig{TxDeadlockAttempts: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

	calls := 0
	err = db.Transaction(ctx, func(tx *oro.DB) error {
		calls++
		_, execErr := tx.Raw("select 1").Exec(ctx)
		return execErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("expected two configured attempts, got %d", calls)
	}
}

func TestTransactionDoesNotRetryNonRetryableError(t *testing.T) {
	ctx := context.Background()
	state := &retrySQLState{commitErrors: []error{errors.New("commit failed")}}
	sql.Register("oro_retry_nonretry", retrySQLDriver{state: state})
	sqlDB, err := sql.Open("oro_retry_nonretry", "")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: retryORMDriver{db: sqlDB}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)

	calls := 0
	err = db.Transaction(ctx, func(tx *oro.DB) error {
		calls++
		_, execErr := tx.Raw("select 1").Exec(ctx)
		return execErr
	}, oro.TxAttempts(3))
	if err == nil {
		t.Fatal("expected commit error")
	}
	if calls != 1 {
		t.Fatalf("expected one attempt, got %d", calls)
	}
}

var _ = context.Background

type retryORMDriver struct {
	db *sql.DB
}

func (driver retryORMDriver) Name() string {
	return "retry"
}

func (driver retryORMDriver) Open(ctx context.Context) (*sql.DB, error) {
	return driver.db, nil
}

func (driver retryORMDriver) Dialect() oro.Dialect {
	return retryDialect{}
}

func (driver retryORMDriver) Inspector(db *sql.DB) oro.Inspector {
	return nil
}

func (driver retryORMDriver) TranslateError(err error) error {
	return err
}

func (driver retryORMDriver) Owned() bool {
	return false
}

type retryDialect struct{}

func (retryDialect) Name() string {
	return "retry"
}

func (retryDialect) Capabilities() oro.Capabilities {
	return oro.Capabilities{}
}

func (retryDialect) QuoteIdent(name string) string {
	return name
}

func (retryDialect) Placeholder(index int) string {
	return "?"
}

func (retryDialect) DataType(column oro.ColumnSpec) (string, error) {
	return column.Type, nil
}

func (retryDialect) NormalizeType(dbType string) (oro.ColumnType, error) {
	return oro.ColumnType{DBType: dbType}, nil
}

func (retryDialect) Compile(stmt oro.Statement) (oro.CompiledSQL, error) {
	return oro.CompiledSQL{SQL: "select 1"}, nil
}

func (retryDialect) CompileSchema(change oro.SchemaChange) ([]oro.CompiledSQL, error) {
	return nil, nil
}

type retrySQLState struct {
	commitErrors []error
	commits      int
}

type retrySQLDriver struct {
	state *retrySQLState
}

func (driver retrySQLDriver) Open(name string) (sqldriver.Conn, error) {
	return &retrySQLConn{state: driver.state}, nil
}

type retrySQLConn struct {
	state *retrySQLState
}

func (conn *retrySQLConn) Prepare(query string) (sqldriver.Stmt, error) {
	return retrySQLStmt{}, nil
}

func (conn *retrySQLConn) Close() error {
	return nil
}

func (conn *retrySQLConn) Begin() (sqldriver.Tx, error) {
	return &retrySQLTx{state: conn.state}, nil
}

func (conn *retrySQLConn) BeginTx(ctx context.Context, opts sqldriver.TxOptions) (sqldriver.Tx, error) {
	return &retrySQLTx{state: conn.state}, nil
}

func (conn *retrySQLConn) ExecContext(ctx context.Context, query string, args []sqldriver.NamedValue) (sqldriver.Result, error) {
	return sqldriver.RowsAffected(1), nil
}

func (conn *retrySQLConn) QueryContext(ctx context.Context, query string, args []sqldriver.NamedValue) (sqldriver.Rows, error) {
	return retrySQLRows{}, nil
}

type retrySQLTx struct {
	state *retrySQLState
}

func (tx *retrySQLTx) Commit() error {
	tx.state.commits++
	if tx.state.commits <= len(tx.state.commitErrors) {
		return tx.state.commitErrors[tx.state.commits-1]
	}
	return nil
}

func (tx *retrySQLTx) Rollback() error {
	return nil
}

type retrySQLStmt struct{}

func (retrySQLStmt) Close() error {
	return nil
}

func (retrySQLStmt) NumInput() int {
	return -1
}

func (retrySQLStmt) Exec(args []sqldriver.Value) (sqldriver.Result, error) {
	return sqldriver.RowsAffected(1), nil
}

func (retrySQLStmt) Query(args []sqldriver.Value) (sqldriver.Rows, error) {
	return retrySQLRows{}, nil
}

type retrySQLRows struct{}

func (retrySQLRows) Columns() []string {
	return []string{"one"}
}

func (retrySQLRows) Close() error {
	return nil
}

func (retrySQLRows) Next(dest []sqldriver.Value) error {
	return io.EOF
}
