package metrics_test

import (
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/extensions/internal/exttest"
	"github.com/duxweb/oro/extensions/metrics"
)

type product struct {
	oro.Model
	Code string
}

func (product) Define(s *oro.SchemaBuilder) {
	s.Table("oro_metrics_products")
	s.Field("Code").String()
}

func TestMetricsExtensionRecordsSQLEvents(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			recorder := metrics.NewMemoryRecorder()
			db, ctx := exttest.Open(t, testCase, exttest.OpenOptions{
				Models: []oro.Definer{product{}},
				Tables: []string{"oro_metrics_products"},
				Prefix: "metrics_matrix_",
				Extensions: []oro.Extension{
					metrics.Extension(metrics.WithRecorder(recorder)),
				},
			})

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
		})
	}
}
