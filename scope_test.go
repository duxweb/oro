package oro_test

import (
	"testing"

	oro "github.com/duxweb/oro"
)

func productMinPrice(price uint) oro.Scope[integrationProduct] {
	return func(q *oro.ScopeQuery[integrationProduct]) {
		q.Where("Price", ">=", price)
	}
}

func productLatest(limit int) oro.Scope[integrationProduct] {
	return func(q *oro.ScopeQuery[integrationProduct]) {
		q.OrderByDesc("Price").Limit(limit)
	}
}

func paidProductRows() oro.TableScope {
	return func(q *oro.TableScopeQuery) {
		q.Where("price", ">=", 100)
	}
}

func tableLatestRows(limit int) oro.TableScope {
	return func(q *oro.TableScopeQuery) {
		q.OrderByDesc("price").Limit(limit)
	}
}

func TestSQLiteModelScope(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, product := range []*integrationProduct{
		{Code: "SC001", Price: 50},
		{Code: "SC002", Price: 100},
		{Code: "SC003", Price: 200},
	} {
		if _, err := db.Use[integrationProduct]().Create(ctx, product); err != nil {
			t.Fatal(err)
		}
	}

	products, err := db.Use[integrationProduct]().
		Scope(productMinPrice(100), productLatest(1)).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 || products[0].Code != "SC003" {
		t.Fatalf("unexpected scoped products %#v", products)
	}
}

func TestSQLiteModelScopeWhen(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, product := range []*integrationProduct{
		{Code: "SW001", Price: 10},
		{Code: "SW002", Price: 20},
	} {
		if _, err := db.Use[integrationProduct]().Create(ctx, product); err != nil {
			t.Fatal(err)
		}
	}

	total, err := db.Use[integrationProduct]().
		ScopeWhen(false, productMinPrice(20)).
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected skipped scope count 2, got %d", total)
	}

	total, err = db.Use[integrationProduct]().
		ScopeWhen(true, productMinPrice(20)).
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("expected applied scope count 1, got %d", total)
	}
}

func TestSQLiteTableScope(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "TS001", "price": 50},
		{"code": "TS002", "price": 100},
		{"code": "TS003", "price": 200},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Table("products").
		Scope(paidProductRows(), tableLatestRows(1)).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["code"] != "TS003" {
		t.Fatalf("unexpected scoped rows %#v", rows)
	}
}

func TestSQLiteTableScopeWhen(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "TW001", "price": 10},
		{"code": "TW002", "price": 20},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	total, err := db.Table("products").
		ScopeWhen(false, paidProductRows()).
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected skipped scope count 2, got %d", total)
	}

	total, err = db.Table("products").
		ScopeWhen(true, paidProductRows()).
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("expected applied scope count 0, got %d", total)
	}
}
