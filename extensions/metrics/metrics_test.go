package metrics_test

import (
	"context"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/metrics"
	_ "modernc.org/sqlite"
)

type product struct {
	oro.Model
	Code string
}

func (product) Define(s *oro.SchemaBuilder) {
	s.Table("products")
	s.Field("Code").String()
}

func TestMetricsExtensionRecordsSQLEvents(t *testing.T) {
	ctx := context.Background()
	recorder := metrics.NewMemoryRecorder()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		Extensions: []oro.Extension{
			metrics.Extension(metrics.WithRecorder(recorder)),
		},
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close(ctx)

	if err := db.Register(product{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if _, err := db.Use[product]().Create(ctx, &product{Code: "P001"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Use[product]().Where("Code", "P001").First(ctx); err != nil {
		t.Fatalf("first: %v", err)
	}

	if recorder.Count(oro.AfterSQL) == 0 {
		t.Fatal("expected after_sql metrics")
	}
	samples := recorder.Samples()
	if len(samples) == 0 {
		t.Fatal("expected samples")
	}
	foundCreate := false
	foundSelect := false
	for _, sample := range samples {
		if sample.Operation == "create" {
			foundCreate = true
		}
		if sample.Operation == "select" {
			foundSelect = true
		}
	}
	if !foundCreate || !foundSelect {
		t.Fatalf("expected create and select samples, got %#v", samples)
	}
}
