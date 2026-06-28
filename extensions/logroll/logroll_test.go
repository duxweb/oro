package logroll_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/logroll"
	_ "modernc.org/sqlite"
)

type loginLog struct {
	oro.Model
	TenantID   uint64
	Action     string
	OccurredAt time.Time
}

func (loginLog) Define(s *oro.SchemaBuilder) {
	s.Table("oro_logroll_login_logs")
	s.Field("TenantID").UnsignedBigInt().Index()
	s.Field("Action").String().Index()
	s.Field("OccurredAt").Column("occurred_at").Timestamp().Index()
}

type noTimeLog struct {
	oro.Model
	Action string
}

func (noTimeLog) Define(s *oro.SchemaBuilder) {
	s.Table("oro_logroll_no_time_logs")
	s.Field("Action").String()
}

func openLogRollDB(t *testing.T) (*oro.DB, time.Time) {
	t.Helper()
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
	if err := db.Register(loginLog{}, noTimeLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(t.Context()); err != nil {
		t.Fatal(err)
	}
	return db, now
}

func seedLogs(t *testing.T, db *oro.DB, now time.Time, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		_, err := db.Use[loginLog]().Create(t.Context(), &loginLog{
			TenantID:   1,
			Action:     fmt.Sprintf("login-%02d", i+1),
			OccurredAt: now.Add(time.Duration(i-count) * time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestCleanupKeepLast(t *testing.T) {
	db, now := openLogRollDB(t)
	seedLogs(t, db, now, 10)

	result, err := logroll.Cleanup[loginLog](db, logroll.KeepLast(3), logroll.BatchSize(2)).Run(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 7 || result.Batches != 4 {
		t.Fatalf("unexpected result %#v", result)
	}
	rows, err := db.Use[loginLog]().OrderBy("ID").Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Action != "login-08" || rows[2].Action != "login-10" {
		t.Fatalf("unexpected remaining rows %#v", rows)
	}
}

func TestCleanupKeepFor(t *testing.T) {
	db, now := openLogRollDB(t)
	seedLogs(t, db, now, 8)

	result, err := logroll.Cleanup[loginLog](db,
		logroll.KeepFor(3*time.Hour),
		logroll.TimeField("OccurredAt"),
		logroll.Now(func() time.Time { return now }),
	).Run(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 5 {
		t.Fatalf("unexpected deleted %d", result.Deleted)
	}
	count, err := db.Use[loginLog]().Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected 3 rows, got %d", count)
	}
}

func TestCleanupCombinedPoliciesUseOrSemantics(t *testing.T) {
	db, now := openLogRollDB(t)
	seedLogs(t, db, now, 10)

	result, err := logroll.Cleanup[loginLog](db,
		logroll.KeepLast(8),
		logroll.KeepFor(3*time.Hour),
		logroll.TimeField("OccurredAt"),
		logroll.Now(func() time.Time { return now }),
	).Run(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 7 {
		t.Fatalf("expected OR pruning to delete 7 rows, got %#v", result)
	}
}

func TestCleanupKeepForStillRunsWhenKeepLastNotExceeded(t *testing.T) {
	db, now := openLogRollDB(t)
	seedLogs(t, db, now, 4)

	result, err := logroll.Cleanup[loginLog](db,
		logroll.KeepLast(100),
		logroll.KeepFor(2*time.Hour),
		logroll.TimeField("OccurredAt"),
		logroll.Now(func() time.Time { return now }),
	).Run(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 2 {
		t.Fatalf("expected time policy to delete 2 rows, got %#v", result)
	}
}

func TestCleanupScope(t *testing.T) {
	db, now := openLogRollDB(t)
	seedLogs(t, db, now, 5)
	for i := 0; i < 5; i++ {
		_, err := db.Use[loginLog]().Create(t.Context(), &loginLog{TenantID: 2, Action: fmt.Sprintf("tenant2-%d", i+1), OccurredAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := logroll.Cleanup[loginLog](db, logroll.KeepLast(2), logroll.Scope(oro.Map{"TenantID": uint64(1)})).Run(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 3 {
		t.Fatalf("unexpected deleted %d", result.Deleted)
	}
	tenantTwo, err := db.Use[loginLog]().Where("TenantID", 2).Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if tenantTwo != 5 {
		t.Fatalf("expected tenant two untouched, got %d", tenantTwo)
	}
}

func TestCleanupWhereUsesModelFieldNames(t *testing.T) {
	db, now := openLogRollDB(t)
	seedLogs(t, db, now, 4)
	for i := 0; i < 4; i++ {
		_, err := db.Use[loginLog]().Create(t.Context(), &loginLog{TenantID: 1, Action: "api", OccurredAt: now.Add(-10 * time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
	}

	result, err := logroll.Cleanup[loginLog](db,
		logroll.KeepFor(time.Hour),
		logroll.TimeField("OccurredAt"),
		logroll.Now(func() time.Time { return now }),
	).Where("OccurredAt", "<", now.Add(-time.Hour)).Where("Action", "api").Run(t.Context())
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if result.Deleted != 4 {
		t.Fatalf("expected 4 api rows deleted, got %#v", result)
	}
	remaining, err := db.Use[loginLog]().Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 4 {
		t.Fatalf("expected non-api rows to remain, got %d", remaining)
	}
}

func TestRollApplyAfterCreate(t *testing.T) {
	db, now := openLogRollDB(t)
	apply := logroll.Roll(logroll.KeepLast(3))
	for i := 0; i < 6; i++ {
		_, err := db.Use[loginLog]().Apply(apply).Create(t.Context(), &loginLog{TenantID: 1, Action: fmt.Sprintf("roll-%d", i+1), OccurredAt: now})
		if err != nil {
			t.Fatalf("create %d: %v", i+1, err)
		}
	}
	rows, err := db.Use[loginLog]().OrderBy("ID").Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Action != "roll-4" || rows[2].Action != "roll-6" {
		t.Fatalf("unexpected rows %#v", rows)
	}
}

func TestRollEvery(t *testing.T) {
	db, now := openLogRollDB(t)
	apply := logroll.Roll(logroll.KeepLast(3), logroll.Every(2))
	for i := 0; i < 5; i++ {
		_, err := db.Use[loginLog]().Apply(apply).Create(t.Context(), &loginLog{TenantID: 1, Action: fmt.Sprintf("every-%d", i+1), OccurredAt: now})
		if err != nil {
			t.Fatal(err)
		}
	}
	count, err := db.Use[loginLog]().Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("expected cleanup only every second write, got %d rows", count)
	}
}

func TestKeepForRequiresTimeField(t *testing.T) {
	db, _ := openLogRollDB(t)
	_, err := logroll.Cleanup[noTimeLog](db, logroll.KeepFor(time.Hour), logroll.TimeField("OccurredAt")).Run(t.Context())
	if !errors.Is(err, oro.ErrUnknownField) {
		t.Fatalf("expected ErrUnknownField, got %v", err)
	}
}
