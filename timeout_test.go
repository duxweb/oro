package oro

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type timeoutFactory struct {
	DefaultFactory
	executor Executor
}

func (factory timeoutFactory) NewExecutor(rt *Runtime) Executor {
	return factory.executor
}

type deadlineExecutor struct {
	deadline chan time.Time
	block    bool
}

func (executor deadlineExecutor) Query(ctx context.Context, exec ExecContext, sql CompiledSQL) (*RowsResult, error) {
	if deadline, ok := ctx.Deadline(); ok {
		executor.deadline <- deadline
	} else {
		executor.deadline <- time.Time{}
	}
	if executor.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &RowsResult{Rows: []Map{}}, nil
}

func (executor deadlineExecutor) Exec(ctx context.Context, exec ExecContext, sql CompiledSQL) (ExecResult, error) {
	if deadline, ok := ctx.Deadline(); ok {
		executor.deadline <- deadline
	} else {
		executor.deadline <- time.Time{}
	}
	if executor.block {
		<-ctx.Done()
		return ExecResult{}, ctx.Err()
	}
	return ExecResult{}, nil
}

func newTimeoutTestDB(t *testing.T, executor Executor, config Config) *DB {
	t.Helper()
	if config.Connections == nil {
		config.Connections = map[string]ConnectionConfig{
			"default": {Driver: fakeDriver{db: (*sql.DB)(nil)}},
		}
	}
	config.Factory = timeoutFactory{executor: executor}
	db, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
	return db
}

func TestQueryUsesConfigTimeout(t *testing.T) {
	deadlines := make(chan time.Time, 1)
	executor := deadlineExecutor{deadline: deadlines}
	db := newTimeoutTestDB(t, executor, Config{
		Timeout: TimeoutConfig{Query: 50 * time.Millisecond},
	})

	startedAt := time.Now()
	if _, err := db.Table("products").Get(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := <-deadlines
	if deadline.IsZero() {
		t.Fatal("expected query deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || deadline.Sub(startedAt) > 200*time.Millisecond {
		t.Fatalf("unexpected deadline %s remaining %s", deadline, remaining)
	}
}

func TestQueryTimeoutOverridesConfigTimeout(t *testing.T) {
	deadlines := make(chan time.Time, 1)
	executor := deadlineExecutor{deadline: deadlines}
	db := newTimeoutTestDB(t, executor, Config{
		Timeout: TimeoutConfig{Query: time.Second},
	})

	startedAt := time.Now()
	if _, err := db.Table("products").Timeout(30 * time.Millisecond).Get(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := <-deadlines
	if deadline.IsZero() {
		t.Fatal("expected query deadline")
	}
	if deadline.Sub(startedAt) > 150*time.Millisecond {
		t.Fatalf("expected query timeout override, deadline delta %s", deadline.Sub(startedAt))
	}
}

func TestCallerDeadlineIsNotExtended(t *testing.T) {
	deadlines := make(chan time.Time, 1)
	executor := deadlineExecutor{deadline: deadlines}
	db := newTimeoutTestDB(t, executor, Config{
		Timeout: TimeoutConfig{Query: time.Second},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	callerDeadline, _ := ctx.Deadline()

	if _, err := db.Table("products").Timeout(time.Second).Get(ctx); err != nil {
		t.Fatal(err)
	}
	deadline := <-deadlines
	if !deadline.Equal(callerDeadline) {
		t.Fatalf("expected caller deadline %s, got %s", callerDeadline, deadline)
	}
}

func TestQueryTimeoutReturnsDeadlineExceeded(t *testing.T) {
	deadlines := make(chan time.Time, 1)
	executor := deadlineExecutor{deadline: deadlines, block: true}
	db := newTimeoutTestDB(t, executor, Config{})

	_, err := db.Table("products").Timeout(time.Millisecond).Get(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
