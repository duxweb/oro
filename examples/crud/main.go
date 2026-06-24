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
	Stock oro.Null[uint]
}

func (Product) Define(s *oro.SchemaBuilder) {
	s.Table("products")
	s.Field("Code").String().Unique()
	s.Field("Price").Uint().Default(0)
	s.Field("Stock").Uint().Nullable()
}

type ProductView struct {
	ID    uint64
	Code  string
	Price uint
}

func main() {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{Connections: map[string]oro.ConnectionConfig{
		"default": {Driver: sqlite.Open(":memory:")},
	}})
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

	_, _ = db.Use[Product]().Create(ctx, &Product{Code: "P001", Price: 100, Stock: oro.NullOf[uint](5)})
	_, _ = db.Table("products").Create(ctx, oro.Map{"code": "P002", "price": 200})

	rows, err := db.Use[Product]().
		Where("Price", ">=", 100).
		WhereGroup(func(w *oro.WhereBuilder) {
			w.Where("Code", "P001").OrWhere("Code", "P002")
		}).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}

	view, err := db.Table("products").
		Select("id", "code", "price").
		MapTo[ProductView]().
		Where("code", "P001").
		First(ctx)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Use[Product]().Where("Code", "P001").Update(ctx, oro.Map{"Price": oro.Increment(20)})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("rows=%d view=%+v", len(rows), view)
}
