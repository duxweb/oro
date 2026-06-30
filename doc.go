// Package oro is a humane, generic-first ORM for Go: no code generation, no
// tag-heavy schema strings, no hidden association magic — typed model queries,
// explicit schemas, relation methods, automatic sync, and a clean multi-driver
// architecture.
//
// The public entrypoints are intentionally small:
//
//	db.Use[Product]()    // model query, Go field names
//	db.Table("products") // table query, database column names
//	db.Raw("select ...") // raw SQL
//
// Models embed [Model] and declare their schema with a Define method instead of
// struct tags. Writes use [Map] so zero values are never guessed. Relations are
// methods returning [Relation], avoiding import cycles. Times are stored in UTC
// and read back in Config.Location, which defaults to UTC.
//
// Quick start:
//
//	type Product struct {
//		oro.Model
//		Code  string
//		Price uint
//	}
//
//	func (Product) Define(s *oro.SchemaBuilder) {
//		s.Table("products")
//		s.Field("Code").String().Unique()
//		s.Field("Price").Uint().Default(0)
//	}
//
//	db, _ := oro.Open(oro.Config{
//		Connections: map[string]oro.ConnectionConfig{
//			"default": {Driver: sqlite.Open(":memory:")},
//		},
//	})
//	_ = db.Register(Product{})
//	_ = db.Sync(ctx)
//	created, _ := db.Use[Product]().Create(ctx, &Product{Code: "P001", Price: 100})
//
// See https://duxweb.github.io/oro for the full documentation.
package oro
