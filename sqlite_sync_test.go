package oro_test

import (
	"context"
	"testing"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
)

type syncProduct struct {
	oro.Model
	Code  string
	Price uint
}

func (syncProduct) Define(s *oro.SchemaBuilder) {
	s.Table("sync_products")
	s.Field("Code").String().Unique()
	s.Field("Price").Uint()
}

type syncProductWithStock struct {
	oro.Model
	Code  string
	Price uint
	Stock uint
}

func (syncProductWithStock) Define(s *oro.SchemaBuilder) {
	s.Table("sync_products")
	s.Field("Code").String()
	s.Field("Price").Uint()
	s.Field("Stock").Uint()
}

type syncRenameProduct struct {
	oro.Model
	Code string
}

var syncRenameProductCodeColumn = "old_code"

func (syncRenameProduct) Define(s *oro.SchemaBuilder) {
	s.Table("sync_rename_products")
	s.Field("Code").Column(syncRenameProductCodeColumn).String()
}

func TestSQLiteSyncCreatesTable(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
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

	if err := db.Register(syncProduct{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := db.Use[syncProduct]().Create(ctx, &syncProduct{
		Code:  "S001",
		Price: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Code != "S001" || created.Price != 100 {
		t.Fatalf("unexpected synced create result %#v", created)
	}
}

func TestSQLiteSyncAddsColumn(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
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

	_, err = db.Raw(`
		create table sync_products (
			id integer primary key autoincrement,
			code text,
			price integer,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Register(syncProductWithStock{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	row, err := db.Table("sync_products").Create(ctx, oro.Map{
		"code":  "S002",
		"price": 200,
		"stock": 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if row["stock"] != int64(10) {
		t.Fatalf("expected synced stock column, got %#v", row)
	}
}

func TestSQLiteSyncCreatesIndexes(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
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

	if err := db.Register(syncProduct{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	_, err = db.Table("sync_products").Create(ctx, oro.Map{
		"code":  "S003",
		"price": 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Table("sync_products").Create(ctx, oro.Map{
		"code":  "S003",
		"price": 301,
	})
	if err == nil {
		t.Fatal("expected unique index violation")
	}
}

func TestSQLiteSyncStoresSnapshotAndRenamesColumn(t *testing.T) {
	ctx := context.Background()
	syncRenameProductCodeColumn = "old_code"
	t.Cleanup(func() {
		syncRenameProductCodeColumn = "old_code"
	})

	db, err := oro.Open(oro.Config{
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

	if err := db.Register(syncRenameProduct{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = db.Table("sync_rename_products").Create(ctx, oro.Map{
		"old_code": "R001",
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshotRows, err := db.Table("oro_schema").
		Where("model", "syncRenameProduct").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshotRows) == 0 {
		t.Fatal("expected schema snapshot rows")
	}

	syncRenameProductCodeColumn = "code"
	if err := db.Register(syncRenameProduct{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	row, err := db.Table("sync_rename_products").Where("code", "R001").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row["code"] != "R001" {
		t.Fatalf("expected renamed column data, got %#v", row)
	}
}
