<p align="center">
  <h1 align="center">Oro</h1>
</p>

<p align="center">
  <b>A humane, generic-first ORM for Go.</b><br/>
  No code generation, no tag-heavy schema strings, no hidden association magic — just typed model queries, explicit schemas, relation methods, automatic sync, and a clean multi-driver architecture.
</p>

<p align="center">
  <a href="https://duxweb.github.io/oro/">Website</a> &middot;
  <a href="https://duxweb.github.io/oro/">Docs</a> &middot;
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#benchmarks">Benchmarks</a> &middot;
  <a href="https://github.com/duxweb/oro/issues">Feedback</a>
</p>

<p align="center">
  English | <a href="README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://github.com/duxweb/oro/actions/workflows/pages.yml"><img alt="Docs" src="https://github.com/duxweb/oro/actions/workflows/pages.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/duxweb/oro"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/duxweb/oro.svg"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-black.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.27+-00ADD8.svg">
</p>

---

## Why Oro

Most Go ORMs force at least one heavy tradeoff: struct tags that become mini-languages, code generation as the center of the workflow, surprising zero-value writes, or relation fields that make package boundaries painful. Oro is built around a smaller set of visible rules.

| Common ORM friction | Oro's answer |
| :--- | :--- |
| Schema metadata hidden in tags | A model-local `Define` method with a typed schema builder. |
| Generated clients and entity packages | Plain Go structs and direct generic query entries. |
| Zero-value updates are ambiguous | Updates use `oro.Map`; writes say exactly which fields change. |
| Associations create import cycles | Relations are methods returning `oro.Relation`, not embedded struct fields. |
| Dynamic table code loses structure | `db.Table(...).MapTo[T]()` maps rows into DTOs without `db` tags. |
| Raw SQL ignores table prefixes | `db.TableName(...)` exposes the current prefixed table name. |
| Multi-DB support is bolted on later | Named connections, model connections, read replicas, and drivers are first-class. |

Oro keeps the public entrypoints intentionally small:

```go
db.Use[Product]()      // model query, Go field names
db.Table("products")   // table query, database column names
db.Raw("select ...")   // raw SQL
```

## Status

Oro is a **pre-1.0 public preview candidate**. The core ORM, schema sync, SQLite/MySQL/PostgreSQL adapters, relation loading, transactions, pagination, table prefix handling, hooks/events, cache, tenancy, sharding, examples, benchmarks, and documentation site are implemented.

The project targets **Go 1.27+** because the API is designed around method-level generics. The current module may use a Go 1.27 release-candidate version until the stable toolchain is available in all environments.

## Install

```bash
go get github.com/duxweb/oro
```

Pick the adapter package and the concrete `database/sql` driver you want:

```go
import (
    oro "github.com/duxweb/oro"
    "github.com/duxweb/oro/driver/mysql"
    "github.com/duxweb/oro/driver/pgsql"
    "github.com/duxweb/oro/driver/sqlite"

    _ "github.com/go-sql-driver/mysql"
    _ "github.com/jackc/pgx/v5/stdlib"
    _ "modernc.org/sqlite" // or _ "github.com/mattn/go-sqlite3"
)
```

Oro adapters wrap `database/sql`; they do not force a concrete SQL driver. Applications choose the real driver with normal blank imports.

## Quick Start

```go
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

    created, err := db.Use[Product]().Create(ctx, &Product{
        Code:  "P001",
        Price: 100,
    })
    if err != nil {
        log.Fatal(err)
    }

    found, err := db.Use[Product]().
        Where(oro.Field("Price").Gte(100)).
        Where("Code", "P001").
        First(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("created=%d found=%s", created.ID, found.Code)
}
```

## Model Definition

Oro uses `Define` instead of ORM tags:

```go
type User struct {
    oro.Model
    Email        string
    PasswordHash string
    Profile      oro.JSONRaw
}

func (User) Define(s *oro.SchemaBuilder) {
    s.Table("users")
    s.Field("Email").String().Size(180).Unique()
    s.Field("PasswordHash").Column("password_hash").String().Hidden()
    s.Field("Profile").JSON().Nullable()
    s.Index("idx_users_email", "Email")
}
```

Field names are Go struct fields. Columns default to snake_case unless `Column(...)` is set.

```go
s.Field("Name").String().Size(120)
s.Field("Price").Decimal(12, 2).Default(0)
s.Field("Status").Enum("draft", "published")
s.Field("Meta").JSON().Nullable()
s.Field("Version").UnsignedBigInt().OptimisticLock()

s.Index("idx_posts_status", "Status")
s.Unique("uk_posts_slug", "Slug")
s.FullText("ft_posts_title_body", "Title", "Body")
```

Enable soft delete explicitly when needed:

```go
type User struct {
    oro.Model
    softdelete.SoftDeleteFields // DeletedAt -> deleted_at
}
```

## Schema Sync

Register models and let Oro sync the structure:

```go
err := db.Register(User{}, Product{}, Order{})
err = db.Sync(ctx)
```

Sync is designed for application-owned schemas:

- creates missing tables, columns, indexes, and supported constraints;
- updates safe compatible field metadata where the driver supports it;
- keeps schema snapshots for diff decisions;
- detects unambiguous renames internally;
- does not perform destructive drops by default;
- keeps foreign-key enforcement opt-in.

## Querying

```go
products, err := db.Use[Product]().
    Where("Price", ">=", 100).
    Where(oro.Field("Code").Like("P%")).
    OrderByDesc("ID").
    Get(ctx)
```

Nested conditions use explicit group methods:

```go
products, err := db.Use[Product]().
    Where("Status", "active").
    WhereGroup(func(w *oro.WhereBuilder) {
        w.Where("Price", ">=", 100).
            OrWhere("Code", "like", "VIP%")
    }).
    WhereWhen(onlyAvailable, func(w *oro.WhereBuilder) {
        w.Where("Stock", ">", 0)
    }).
    Get(ctx)
```

Missing rows are not errors:

- `First` and `Find` return `nil, nil` when no row exists.
- `Get` returns an empty slice.

## Table, Raw, and DTO Mapping

```go
type ProductView struct {
    ID    uint64
    Code  string
    Price uint
}

views, err := db.Table("products").
    Select("id", "code", "price").
    MapTo[ProductView]().
    Where("price", ">=", 100).
    Get(ctx)
```

Raw SQL is explicit and still benefits from mapping:

```go
rows, err := db.Raw(
    "select * from "+db.TableName("products")+" where price >= ?",
    100,
).MapTo[ProductView]().Get(ctx)
```

## Relations Without Import Cycles

Relations are methods. Models do not need embedded relation fields.

```go
type Article struct {
    oro.Model
    Title string
}

func (Article) Define(s *oro.SchemaBuilder) {
    s.Table("articles")
    s.Field("Title").String()
}

func (article Article) Comments() oro.Relation {
    return oro.HasMany(article, "Comments", "Comment").
        ForeignKey("ArticleID").
        ReferenceKey("ID")
}
```

Preload and read relation values:

```go
article, err := db.Use[Article]().
    With(Article{}.Comments(), func(q *oro.RelationQuery) {
        q.Where("Status", "approved").OrderByDesc("ID")
    }).
    First(ctx)

comments, err := article.Comments().Many[Comment]()
```

Relation queries and aggregates:

```go
articles, err := db.Use[Article]().
    WhereHas(Article{}.Comments(), func(q *oro.RelationQuery) {
        q.Where("Status", "approved").Count(">=", 3)
    }).
    WithCount(Article{}.Comments()).
    WithExists(Article{}.Cover()).
    Get(ctx)
```

## Writes

```go
created, err := db.Use[Product]().Create(ctx, product, oro.Only("Code", "Price"))

result, err := db.Use[Product]().CreateMany(ctx, products, oro.BatchSize(500))
ids, err := result.IDs[uint64]()

updated, err := db.Use[Product]().
    Where("Code", "P001").
    Update(ctx, oro.Map{"Price": oro.Increment(20)})

deleted, err := db.Use[Product]().Where("Code", "P001").Delete(ctx)
```

Updates use `oro.Map` so zero values are never guessed.

## Transactions and Locks

```go
err := db.Transaction(ctx, func(tx *oro.DB) error {
    product, err := tx.Use[Product]().
        Where("Code", "P001").
        LockForUpdate().
        First(ctx)
    if err != nil || product == nil {
        return err
    }

    _, err = tx.Use[Product]().
        Where("ID", product.ID).
        Update(ctx, oro.Map{"Stock": oro.Decrement(1)})
    return err
}, oro.TxAttempts(3), oro.TxIsolation(oro.LevelReadCommitted))
```

Nested transactions use savepoints, and manual savepoints are available.

## Multi Driver, Tenancy, and Sharding

```go
db, err := oro.Open(oro.Config{
    Default:     "main",
    TablePrefix: "app_",
    Connections: map[string]oro.ConnectionConfig{
        "main":  {Driver: sqlite.Open("app.db")},
        "mysql": {Driver: mysql.Open(mysqlDSN)},
        "pgsql": {Driver: pgsql.Open(pgDSN)},
    },
    Extensions: []oro.Extension{
        tenant.Extension(tenant.Fields("TenantID", "AppID")),
    },
})
```

Tenant and sharding use one explicit input type: `oro.Map`.

```go
rows, err := tenant.Use(db, oro.Map{"TenantID": 10, "AppID": 20}).
    Use[Product]().
    Get(ctx)

orders, err := db.Use[Order]().
    Shard(oro.Map{"TenantID": 10}).
    Get(ctx)
```

## Benchmarks

Benchmarks run against SQLite, MySQL, and PostgreSQL using the same database drivers for each ORM. The documentation reports the median of 10 runs.

| Scenario | SQLite | MySQL | PostgreSQL |
| :--- | :--- | :--- | :--- |
| CreateMany100 | Oro leads this run | Oro leads this run | Oro is near the lead |
| FirstByCode | Oro leads this run | Oro leads this run | Oro is competitive |
| WhereList | Oro leads this run | Oro leads this run | Oro is competitive |
| DeleteByCode | Oro leads this run | Oro leads this run | Oro is competitive |

See the full benchmark tables, environment, versions, and commands in the [Performance Benchmarks](https://duxweb.github.io/oro/advanced/performance-benchmarks/) page.

## Comparison

| Project | Workflow | Strength | Tradeoff | Oro position |
| :--- | :--- | :--- | :--- | :--- |
| GORM | Struct models + tags + AutoMigrate | Mature, broad ecosystem | Tag-heavy metadata and broad association behavior | Keeps auto-sync style, makes schema and writes explicit |
| Ent | Schema definitions + code generation | Strong compile-time graph API | Codegen is central | No codegen, smaller public surface |
| Bun | SQL-first builder + models | Fast and close to SQL | Less model-level automation | Keeps SQL clarity, adds schema sync and relation methods |
| XORM | Traditional ORM mapping | Simple model CRUD | Older API shape | More explicit generics and sync rules |
| Oro | Plain structs + `Define` + generic queries | Clear boundaries, no codegen, multi-driver | Requires Go 1.27+ generics | Designed for modern Go applications |

## Documentation

- English docs: <https://duxweb.github.io/oro/>
- 中文文档: <https://duxweb.github.io/oro/zh-cn/>
- Examples: [`examples/`](examples/)
- Benchmarks: [`benchmarks/ormbench/`](benchmarks/ormbench/)

## Contributing

Oro is still pre-1.0, so API feedback is valuable. Please open issues for naming problems, missing driver capabilities, schema sync edge cases, benchmark gaps, and real-world adoption feedback.

## License

MIT
