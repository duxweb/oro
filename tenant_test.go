package oro_test

import (
	"context"
	"errors"
	"testing"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/tenant"
	_ "modernc.org/sqlite"
)

type tenantOrder struct {
	oro.Model
	TenantID uint64
	AppID    uint64
	Code     string
	Status   string
}

func (tenantOrder) Define(s *oro.SchemaBuilder) {
	s.Table("tenant_orders")
	s.Field("TenantID").UnsignedBigInt()
	s.Field("AppID").UnsignedBigInt()
	s.Field("Code").String().Unique()
	s.Field("Status").String()
}

type tenantProject struct {
	oro.Model
	OrgID uint64
	Name  string
}

func (tenantProject) Define(s *oro.SchemaBuilder) {
	s.Table("tenant_projects")
	s.Tenant("OrgID")
	s.Field("OrgID").UnsignedBigInt()
	s.Field("Name").String()
}

type tenantPlan struct {
	oro.Model
	TenantID uint64
	Name     string
}

func (tenantPlan) Define(s *oro.SchemaBuilder) {
	s.Table("tenant_plans")
	s.NoTenant()
	s.Field("TenantID").UnsignedBigInt()
	s.Field("Name").String()
}

type tenantArticle struct {
	oro.Model
	TenantID uint64
	Title    string
}

func (tenantArticle) Define(s *oro.SchemaBuilder) {
	s.Table("tenant_articles")
	s.Field("TenantID").UnsignedBigInt()
	s.Field("Title").String()
}

func (article tenantArticle) Comments() oro.Relation {
	return oro.HasMany(article, "Comments", "tenantComment").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type tenantComment struct {
	oro.Model
	TenantID  uint64
	ArticleID uint64
	Body      string
}

func (tenantComment) Define(s *oro.SchemaBuilder) {
	s.Table("tenant_comments")
	s.Field("TenantID").UnsignedBigInt()
	s.Field("ArticleID").UnsignedBigInt()
	s.Field("Body").String()
}

func TestTenantSharedTableScopesCRUD(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Extensions: []oro.Extension{tenant.Extension(tenant.Fields("TenantID", "AppID"))},
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if err := db.Register(tenantOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = db.Use[tenantOrder]().Get(ctx)
	if !errors.Is(err, oro.ErrTenantRequired) {
		t.Fatalf("expected ErrTenantRequired, got %v", err)
	}

	tenantOne := tenant.Use(db, oro.Map{"TenantID": uint64(1), "AppID": uint64(10)})
	created, err := tenantOne.Use[tenantOrder]().Create(ctx, &tenantOrder{Code: "A001", Status: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if created.TenantID != 1 || created.AppID != 10 {
		t.Fatalf("expected tenant values on created model, got %#v", created)
	}
	if _, err := tenantOne.Use[tenantOrder]().Create(ctx, &tenantOrder{Code: "A002", Status: "new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tenant.Use(db, oro.Map{"TenantID": uint64(2), "AppID": uint64(10)}).
		Use[tenantOrder]().
		Create(ctx, &tenantOrder{Code: "B001", Status: "new"}); err != nil {
		t.Fatal(err)
	}

	rows, err := tenantOne.Use[tenantOrder]().OrderBy("Code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Code != "A001" || rows[1].Code != "A002" {
		t.Fatalf("unexpected tenant scoped rows %#v", rows)
	}

	updated, err := tenantOne.Use[tenantOrder]().Where("Code", "B001").Update(ctx, oro.Map{"Status": "paid"})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 0 {
		t.Fatalf("expected no cross tenant update, got %d", updated)
	}
	updated, err = tenantOne.Use[tenantOrder]().Where("Code", "A001").Update(ctx, oro.Map{"Status": "paid"})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one tenant update, got %d", updated)
	}

	deleted, err := tenantOne.Use[tenantOrder]().Where("Code", "B001").ForceDelete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("expected no cross tenant delete, got %d", deleted)
	}

	allRows, err := tenant.Without(db).Use[tenantOrder]().OrderBy("Code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(allRows) != 3 {
		t.Fatalf("expected all rows without tenant, got %#v", allRows)
	}
}

func TestTenantModelOverrideAndNoTenant(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Extensions: []oro.Extension{tenant.Extension(tenant.Fields("TenantID"))},
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if err := db.Register(tenantProject{}, tenantPlan{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = tenant.Use(db, oro.Map{"TenantID": uint64(1)}).Use[tenantProject]().Create(ctx, &tenantProject{Name: "missing org"})
	if !errors.Is(err, oro.ErrTenantRequired) {
		t.Fatalf("expected model tenant override error, got %v", err)
	}

	if _, err := tenant.Use(db, oro.Map{"OrgID": uint64(7)}).Use[tenantProject]().Create(ctx, &tenantProject{Name: "p1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tenant.Use(db, oro.Map{"OrgID": uint64(8)}).Use[tenantProject]().Create(ctx, &tenantProject{Name: "p2"}); err != nil {
		t.Fatal(err)
	}
	projects, err := tenant.Use(db, oro.Map{"OrgID": uint64(7)}).Use[tenantProject]().Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].OrgID != 7 || projects[0].Name != "p1" {
		t.Fatalf("unexpected project rows %#v", projects)
	}

	if _, err := db.Use[tenantPlan]().Create(ctx, &tenantPlan{TenantID: 99, Name: "global"}); err != nil {
		t.Fatal(err)
	}
	plans, err := db.Use[tenantPlan]().Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 1 || plans[0].Name != "global" {
		t.Fatalf("unexpected no tenant rows %#v", plans)
	}
}

func TestTenantWithScopesPreloadedRelations(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Extensions: []oro.Extension{tenant.Extension(tenant.Fields("TenantID"))},
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if err := db.Register(tenantArticle{}, tenantComment{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	tenantOne := tenant.Use(db, oro.Map{"TenantID": uint64(1)})
	article, err := tenantOne.Use[tenantArticle]().Create(ctx, &tenantArticle{Title: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tenantOne.Use[tenantComment]().Create(ctx, &tenantComment{ArticleID: article.ID, Body: "visible"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tenant.Use(db, oro.Map{"TenantID": uint64(2)}).Use[tenantComment]().Create(ctx, &tenantComment{ArticleID: article.ID, Body: "hidden"}); err != nil {
		t.Fatal(err)
	}

	loaded, err := tenantOne.Use[tenantArticle]().With(tenantArticle{}.Comments()).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	comments, err := loaded.Comments().Many[tenantComment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].Body != "visible" {
		t.Fatalf("unexpected tenant relation rows %#v", comments)
	}
}

func TestTenantRouterSelectsConnection(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Extensions: []oro.Extension{
			tenant.Extension(tenant.Fields("TenantID"), tenant.WithRouter(tenant.RouterFunc(func(ctx context.Context, values oro.Map) (string, error) {
				tenantID, ok := values["TenantID"]
				if !ok {
					return "", oro.ErrTenantRequired
				}
				switch tenantID {
				case uint64(1):
					return "tenant_1", nil
				case uint64(2):
					return "tenant_2", nil
				default:
					return "", oro.ErrUnknownTenant
				}
			}))),
		},
		Connections: map[string]oro.ConnectionConfig{
			"tenant_1": {Driver: sqlite.Open(":memory:")},
			"tenant_2": {Driver: sqlite.Open(":memory:")},
		},
		Default: "tenant_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if err := db.Register(tenantOrder{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Connection("tenant_1").Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Connection("tenant_2").Sync(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err := tenant.Use(db, oro.Map{"TenantID": uint64(1), "AppID": uint64(10)}).Use[tenantOrder]().Create(ctx, &tenantOrder{Code: "T1", Status: "ok"}); err != nil {
		t.Fatal(err)
	}
	if _, err := tenant.Use(db, oro.Map{"TenantID": uint64(2), "AppID": uint64(10)}).Use[tenantOrder]().Create(ctx, &tenantOrder{Code: "T2", Status: "ok"}); err != nil {
		t.Fatal(err)
	}

	row1, err := tenant.Without(db.Connection("tenant_1")).Use[tenantOrder]().First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row1.Code != "T1" {
		t.Fatalf("expected tenant_1 data, got %#v", row1)
	}
	row2, err := tenant.Without(db.Connection("tenant_2")).Use[tenantOrder]().First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row2.Code != "T2" {
		t.Fatalf("expected tenant_2 data, got %#v", row2)
	}

	_, err = tenant.Use(db, oro.Map{"TenantID": uint64(3), "AppID": uint64(10)}).Use[tenantOrder]().Get(ctx)
	if !errors.Is(err, oro.ErrUnknownTenant) {
		t.Fatalf("expected ErrUnknownTenant, got %v", err)
	}
}
