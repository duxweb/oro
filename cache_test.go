package oro_test

import (
	"context"
	"errors"
	"testing"
	"time"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	_ "modernc.org/sqlite"
)

func openSQLiteCacheTestDB(t *testing.T) (*oro.DB, context.Context) {
	t.Helper()

	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		Cache: oro.NewMemoryCacheStore(),
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
		create table products (
			id integer primary key autoincrement,
			code text not null unique,
			price integer not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(integrationProduct{}); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestSQLiteModelQueryCache(t *testing.T) {
	db, ctx := openSQLiteCacheTestDB(t)

	var sqlCount int
	db.On(oro.BeforeSQL, func(ctx context.Context, event *oro.Event) error {
		if event.Operation == "select" {
			sqlCount++
		}
		return nil
	})

	if _, err := db.Use[integrationProduct]().Create(ctx, &integrationProduct{Code: "CQ001", Price: 100}); err != nil {
		t.Fatal(err)
	}

	query := db.Use[integrationProduct]().
		Where("Code", "CQ001").
		Cache(time.Minute).
		CacheTags("products")

	product, err := query.First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if product == nil || product.Code != "CQ001" {
		t.Fatalf("unexpected product %#v", product)
	}
	firstSQLCount := sqlCount

	product, err = query.First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if product == nil || product.Code != "CQ001" {
		t.Fatalf("unexpected cached product %#v", product)
	}
	if sqlCount != firstSQLCount {
		t.Fatalf("expected cache hit without SQL, before=%d after=%d", firstSQLCount, sqlCount)
	}

	if err := db.Cache().ForgetTag(ctx, "products"); err != nil {
		t.Fatal(err)
	}
	if _, err := query.First(ctx); err != nil {
		t.Fatal(err)
	}
	if sqlCount <= firstSQLCount {
		t.Fatalf("expected SQL after tag forget, before=%d after=%d", firstSQLCount, sqlCount)
	}
}

func TestSQLiteTableAndRawQueryCache(t *testing.T) {
	db, ctx := openSQLiteCacheTestDB(t)

	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "CT001", "price": 10}); err != nil {
		t.Fatal(err)
	}

	row, err := db.Table("products").
		Where("code", "CT001").
		Cache(time.Minute).
		CacheKey("products:ct001").
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row["code"] != "CT001" {
		t.Fatalf("unexpected row %#v", row)
	}

	if _, err := db.Table("products").Where("code", "CT001").Update(ctx, oro.Map{"price": 20}); err != nil {
		t.Fatal(err)
	}
	row, err = db.Table("products").
		Where("code", "CT001").
		Cache(time.Minute).
		CacheKey("products:ct001").
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row["price"] == int64(20) {
		t.Fatalf("expected stale cached row before forget, got %#v", row)
	}

	if err := db.Cache().Forget(ctx, "products:ct001"); err != nil {
		t.Fatal(err)
	}
	row, err = db.Raw("select code, price from products where code = ?", "CT001").
		Cache(time.Minute).
		CacheKey("raw:ct001").
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row["price"] != int64(20) {
		t.Fatalf("unexpected raw row %#v", row)
	}
}

func TestSQLiteQueryCacheRequiresStore(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Use[integrationProduct]().
		Where("Code", "missing").
		Cache(time.Minute).
		First(ctx)
	if !errors.Is(err, oro.ErrCacheStoreRequired) {
		t.Fatalf("expected cache store required, got %v", err)
	}
}
