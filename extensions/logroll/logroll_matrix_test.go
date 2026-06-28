package logroll_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/extensions/internal/exttest"
	"github.com/duxweb/oro/extensions/logroll"
)

type matrixLog struct {
	oro.Model
	TenantID   uint64
	Action     string
	OccurredAt time.Time
}

func (matrixLog) Define(s *oro.SchemaBuilder) {
	s.Table("oro_logroll_matrix_logs")
	s.Field("TenantID").UnsignedBigInt().Index()
	s.Field("Action").String().Index()
	s.Field("OccurredAt").Column("occurred_at").Timestamp().Index()
}

func TestLogRollDriverMatrix(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			db, ctx := exttest.Open(t, testCase, exttest.OpenOptions{
				Models: []oro.Definer{matrixLog{}},
				Tables: []string{"oro_logroll_matrix_logs"},
				Prefix: "logroll_matrix_",
			})
			now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
			for i := 0; i < 6; i++ {
				_, err := db.Use[matrixLog]().Create(ctx, &matrixLog{TenantID: 1, Action: fmt.Sprintf("m-%d", i+1), OccurredAt: now.Add(time.Duration(i-6) * time.Hour)})
				if err != nil {
					t.Fatalf("create: %v", err)
				}
			}
			result, err := logroll.Cleanup[matrixLog](db,
				logroll.KeepLast(3),
				logroll.KeepFor(2*time.Hour),
				logroll.TimeField("OccurredAt"),
				logroll.Now(func() time.Time { return now }),
			).Run(ctx)
			if err != nil {
				t.Fatalf("cleanup: %v", err)
			}
			if result.Deleted != 4 {
				t.Fatalf("expected 4 deleted, got %#v", result)
			}
			count, err := db.Use[matrixLog]().Count(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatalf("expected 2 remaining, got %d", count)
			}
		})
	}
}
