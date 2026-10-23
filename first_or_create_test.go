package oro_test

import (
	"database/sql"
	"testing"
	"time"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
)

func TestSQLiteFirstOrCreateUsesFreshPrimaryReads(t *testing.T) {
	for _, source := range []string{"cache", "replica"} {
		for _, kind := range []string{"model", "table", "typed"} {
			t.Run(source+"/"+kind, func(t *testing.T) {
				config := oro.Config{
					Cache: oro.NewMemoryCacheStore(),
					Connections: map[string]oro.ConnectionConfig{
						"default": {Driver: sqlite.Open(":memory:")},
					},
				}
				if source == "replica" {
					// An empty replica models a write that has not replicated yet.
					replica, err := sql.Open("sqlite", ":memory:")
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = replica.Close() })
					if _, err := replica.Exec(`create table products (
						id integer primary key, code text unique, price integer,
						created_at datetime, updated_at datetime, deleted_at datetime
					)`); err != nil {
						t.Fatal(err)
					}
					config.Connections["default"] = oro.ConnectionConfig{
						Driver: sqlite.Open(":memory:"), Reads: []oro.Driver{sqlite.Wrap(replica)},
					}
				}
				db, ctx := openSQLiteTestDBWithConfig(t, config)
				modelQuery := db.Use[integrationProduct]().Where("Code", "fresh")
				tableQuery := db.Table("products").Where("code", "fresh")
				if source == "cache" {
					modelQuery = modelQuery.Cache(time.Minute).CacheKey("missing-model")
					tableQuery = tableQuery.Cache(time.Minute).CacheKey("missing-table")
					if row, err := modelQuery.First(ctx); err != nil || row != nil {
						t.Fatalf("prime model cache: row=%#v err=%v", row, err)
					}
					if row, err := tableQuery.First(ctx); err != nil || row != nil {
						t.Fatalf("prime table cache: row=%#v err=%v", row, err)
					}
				}
				for _, wantCreated := range []bool{true, false} {
					var code string
					var created bool
					var err error
					switch kind {
					case "model":
						var row *integrationProduct
						row, created, err = modelQuery.FirstOrCreate(ctx, &integrationProduct{Code: "fresh", Price: 10}, "Code")
						if row != nil {
							code = row.Code
						}
					case "table":
						var row oro.Map
						row, created, err = tableQuery.FirstOrCreate(ctx, oro.Map{"code": "fresh", "price": 10}, "code")
						if row != nil {
							code, _ = row["code"].(string)
						}
					case "typed":
						var row *integrationMappedDTO
						row, created, err = tableQuery.MapTo[integrationMappedDTO]().FirstOrCreate(ctx, oro.Map{"code": "fresh", "price": 10}, "code")
						if row != nil {
							code = row.Code
						}
					}
					if err != nil || code != "fresh" || created != wantCreated {
						t.Fatalf("code=%q created=%v wantCreated=%v err=%v", code, created, wantCreated, err)
					}
				}
				if source == "cache" {
					// FirstOrCreate must not mutate the caller's cache settings.
					if row, err := modelQuery.First(ctx); err != nil || row != nil {
						t.Fatalf("model cache changed: row=%#v err=%v", row, err)
					}
					if row, err := tableQuery.First(ctx); err != nil || row != nil {
						t.Fatalf("table cache changed: row=%#v err=%v", row, err)
					}
				}
			})
		}
	}
}
