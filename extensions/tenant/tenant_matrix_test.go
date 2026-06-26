package tenant_test

import (
	"errors"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/extensions/internal/exttest"
	"github.com/duxweb/oro/extensions/tenant"
)

type order struct {
	oro.Model
	TenantID uint64
	AppID    uint64
	Code     string
	Status   string
}

func (order) Define(s *oro.SchemaBuilder) {
	s.Table("oro_tenant_orders")
	s.Field("TenantID").UnsignedBigInt()
	s.Field("AppID").UnsignedBigInt()
	s.Field("Code").String().Unique()
	s.Field("Status").String()
}

func TestTenantDriverMatrix(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			db, ctx := exttest.Open(t, testCase, exttest.OpenOptions{
				Models: []oro.Definer{order{}},
				Tables: []string{"oro_tenant_orders"},
				Prefix: "tenant_matrix_",
				Extensions: []oro.Extension{
					tenant.Extension(tenant.Fields("TenantID", "AppID")),
				},
			})

			_, err := db.Use[order]().Get(ctx)
			if !errors.Is(err, oro.ErrTenantRequired) {
				t.Fatalf("expected ErrTenantRequired, got %v", err)
			}

			tenantOne := tenant.Use(db, oro.Map{"TenantID": uint64(1), "AppID": uint64(10)})
			created, err := tenantOne.Use[order]().Create(ctx, &order{Code: "A001", Status: "new"})
			if err != nil {
				t.Fatalf("create tenant one: %v", err)
			}
			if created.TenantID != 1 || created.AppID != 10 {
				t.Fatalf("expected tenant values, got %#v", created)
			}
			if _, err := tenant.Use(db, oro.Map{"TenantID": uint64(2), "AppID": uint64(10)}).
				Use[order]().
				Create(ctx, &order{Code: "B001", Status: "new"}); err != nil {
				t.Fatalf("create tenant two: %v", err)
			}

			rows, err := tenantOne.Use[order]().Get(ctx)
			if err != nil {
				t.Fatalf("tenant one get: %v", err)
			}
			if len(rows) != 1 || rows[0].Code != "A001" {
				t.Fatalf("unexpected tenant rows %#v", rows)
			}

			updated, err := tenantOne.Use[order]().Where("Code", "B001").Update(ctx, oro.Map{"Status": "paid"})
			if err != nil {
				t.Fatalf("cross tenant update: %v", err)
			}
			if updated != 0 {
				t.Fatalf("expected no cross tenant update, got %d", updated)
			}

			allRows, err := tenant.Without(db).Use[order]().OrderBy("Code").Get(ctx)
			if err != nil {
				t.Fatalf("without tenant: %v", err)
			}
			if len(allRows) != 2 {
				t.Fatalf("expected two rows without tenant, got %#v", allRows)
			}
		})
	}
}
