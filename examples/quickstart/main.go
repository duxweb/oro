package main

import (
	"context"
	"log"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	_ "modernc.org/sqlite"
)

type Product struct {
	oro.Model
	Code  string
	Price uint
}

func (Product) Define(s *oro.SchemaBuilder) {
	s.Table("products")
	s.Field("Code").String().Unique()
	s.Field("Price").Uint().Default(0)
}

func main() {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(ctx)

	if err := db.Register(Product{}); err != nil {
		log.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		log.Fatal(err)
	}

	created, err := db.Use[Product]().Create(ctx, &Product{Code: "P001", Price: 100})
	if err != nil {
		log.Fatal(err)
	}
	found, err := db.Use[Product]().Where("Code", "P001").First(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("created id=%d found price=%d", created.ID, found.Price)
}
