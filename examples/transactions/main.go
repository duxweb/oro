package main

import (
	"context"
	"log"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
)

type Stock struct {
	oro.Model
	Code  string
	Stock uint
}

func (Stock) Define(s *oro.SchemaBuilder) {
	s.Table("stocks")
	s.Field("Code").String().Unique()
	s.Field("Stock").Uint()
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

	if err := db.Register(Stock{}); err != nil {
		log.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		log.Fatal(err)
	}
	_, _ = db.Use[Stock]().Create(ctx, &Stock{Code: "SKU001", Stock: 10})

	err = db.Transaction(ctx, func(tx *oro.DB) error {
		item, err := tx.Use[Stock]().Where("Code", "SKU001").LockForUpdate().First(ctx)
		if err != nil {
			return err
		}
		if item.Stock == 0 {
			return nil
		}
		_, err = tx.Use[Stock]().Where("ID", item.ID).Update(ctx, oro.Map{"Stock": oro.Decrement(1)})
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}
