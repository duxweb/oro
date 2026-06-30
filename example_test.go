package oro_test

import (
	"context"
	"fmt"
	"log"
	"time"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	_ "modernc.org/sqlite"
)

type exampleProduct struct {
	oro.Model
	Code  string
	Name  string
	Price uint
}

func (exampleProduct) Define(s *oro.SchemaBuilder) {
	s.Table("example_products")
	s.Field("Code").String().Unique()
	s.Field("Name").String()
	s.Field("Price").Uint().Default(0)
}

type exampleOrder struct {
	oro.Model
	Code string
}

func (exampleOrder) Define(s *oro.SchemaBuilder) {
	s.Table("example_orders")
	s.Field("Code").String().Unique()
}

type exampleProductRow struct {
	ID    uint64
	Code  string
	Price uint
}

func exampleDB(ctx context.Context, models ...oro.Definer) *oro.DB {
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		Pool: oro.PoolConfig{MaxOpenConns: 1},
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(models) == 0 {
		models = []oro.Definer{exampleProduct{}, exampleOrder{}}
	}
	if err := db.Register(models...); err != nil {
		log.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		log.Fatal(err)
	}
	return db
}

func Example() {
	ctx := context.Background()
	db := exampleDB(ctx, exampleProduct{})
	defer db.Close(ctx)

	created, err := db.Use[exampleProduct]().Create(ctx, &exampleProduct{
		Code:  "P001",
		Name:  "Keyboard",
		Price: 100,
	})
	if err != nil {
		log.Fatal(err)
	}
	found, err := db.Use[exampleProduct]().Where("Code", created.Code).First(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(found.Code, found.Price)
	// Output: P001 100
}

func ExampleDB_Use() {
	ctx := context.Background()
	db := exampleDB(ctx, exampleProduct{})
	defer db.Close(ctx)

	_, _ = db.Use[exampleProduct]().Create(ctx, &exampleProduct{Code: "P001", Price: 100})
	products, err := db.Use[exampleProduct]().
		Where(oro.Field("Price").Gte(100)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(products), products[0].Code)
	// Output: 1 P001
}

func ExampleDB_Table() {
	ctx := context.Background()
	db := exampleDB(ctx, exampleProduct{})
	defer db.Close(ctx)

	_, _ = db.Table("example_products").Create(ctx, oro.Map{
		"code":  "P001",
		"name":  "Keyboard",
		"price": 100,
	})
	rows, err := db.Table("example_products").
		Select("id", "code", "price").
		MapTo[exampleProductRow]().
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(rows), rows[0].Code, rows[0].Price)
	// Output: 1 P001 100
}

func ExampleDB_Raw() {
	ctx := context.Background()
	db := exampleDB(ctx, exampleProduct{})
	defer db.Close(ctx)

	_, _ = db.Table("example_products").Create(ctx, oro.Map{"code": "P001", "price": 100})
	rows, err := db.Raw("select code, price from example_products where price >= ?", 100).
		MapTo[exampleProductRow]().
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(rows), rows[0].Code, rows[0].Price)
	// Output: 1 P001 100
}

func ExampleField() {
	ctx := context.Background()
	db := exampleDB(ctx, exampleProduct{})
	defer db.Close(ctx)

	products, err := db.Use[exampleProduct]().Where(
		oro.And(
			oro.Field("Price").Between(100, 500),
			oro.Field("Code").StartsWith("P"),
		),
	).Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(products))
	// Output: 0
}

func ExampleTime() {
	ctx := context.Background()
	db := exampleDB(ctx, exampleOrder{})
	defer db.Close(ctx)

	loc := time.FixedZone("UTC+8", 8*60*60)
	day := time.Date(2026, 6, 30, 0, 0, 0, 0, loc)
	orders, err := db.Use[exampleOrder]().
		Where(oro.Time("CreatedAt").OnDate(day)).
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(orders))
	// Output: 0
}

func ExampleDB_Transaction() {
	ctx := context.Background()
	db := exampleDB(ctx, exampleProduct{})
	defer db.Close(ctx)

	err := db.Transaction(ctx, func(tx *oro.DB) error {
		created, err := tx.Use[exampleProduct]().Create(ctx, &exampleProduct{Code: "P001", Price: 100})
		if err != nil {
			return err
		}
		_, err = tx.Use[exampleProduct]().
			Where("ID", created.ID).
			LockForUpdate().
			First(ctx)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("committed")
	// Output: committed
}
