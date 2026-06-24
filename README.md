# Oro

**A generic-first ORM for Go, designed for direct syntax, explicit schemas, and clean multi-driver architecture.**

Oro keeps the public API small:

```go
db.Use[Product]()    // model query
db.Table("products") // table query
db.Raw("select ...") // raw SQL
```

It avoids code generation, avoids tag-heavy schema definitions, and uses Go generics where they make the API easier to read.

> Current status: **pre-1.0 / release candidate quality**. The core ORM, sync engine, drivers, relations, transactions, table prefix handling, and integration test matrix are implemented. The project currently targets `go1.27rc1` because it relies on method-level generics.

## Why Oro

Most ORMs optimize for one side of the tradeoff:

- GORM is powerful, but its struct tags, zero-value behavior, and association model can become difficult to reason about.
- Laravel Eloquent is extremely readable, but PHP's dynamic model does not translate directly into Go.
- SeaORM is explicit and type-oriented, but Rust entity workflows commonly involve generated entities.
- Ent is strongly typed, but schema/code generation is central to the workflow.
- Prisma and TypeORM provide excellent DX in Node.js, but rely on schema files, decorators, or runtime-heavy metadata.

Oro takes the parts that work well for application code:

- model methods for relationships, inspired by Eloquent
- explicit schema builders, similar in spirit to Laravel migrations, but colocated with the Go model
- generic query entrypoints, shaped for Go
- automatic sync like GORM, with safer diff rules
- relation loading style close to SeaORM's typed relation access
- multi-driver and multi-connection architecture from the start

## Install

```bash
go get github.com/duxweb/oro
```

Choose one or more drivers:

```go
import (
    oro "github.com/duxweb/oro"
    "github.com/duxweb/oro/driver/mysql"
    "github.com/duxweb/oro/driver/pgsql"
    "github.com/duxweb/oro/driver/sqlite"
)
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    oro "github.com/duxweb/oro"
    "github.com/duxweb/oro/driver/sqlite"
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
        Where("Code", "P001").
        First(ctx)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("created=%d found=%s", created.ID, found.Code)
}
```

## Core API

| Entry | Purpose | Return shape |
| --- | --- | --- |
| `db.Use[T]()` | Model query | `*T`, `[]*T`, typed aggregates |
| `db.Table("name")` | Direct table query | `oro.Map`, `[]oro.Map` |
| `db.Raw(sql, args...)` | Raw SQL query | `oro.Map`, typed by `MapTo[T]()` |
| `db.From(oro.Query(...))` | Subquery source | table query |
| `db.Connection("name")` | Select connection | scoped DB clone |
| `db.Tenant(oro.Map{...})` | Apply tenant values | scoped DB clone |

Table queries can be mapped into DTOs without tags:

```go
type ProductView struct {
    ID    uint64
    Code  string
    Price uint
}

view, err := db.Table("products").
    Select("id", "code", "price").
    MapTo[ProductView]().
    Where("code", "P001").
    First(ctx)
```

## Model Definition

Oro uses a model-local `Define` method instead of ORM tags:

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

Field names use Go struct fields. Columns default to snake case unless `Column(...)` is set.

```go
s.Field("Name").String().Size(120)
s.Field("Price").Decimal(12, 2).Default(0)
s.Field("Status").Enum("draft", "published")
s.Field("Meta").JSON().Nullable()
s.Field("DeletedAt").Timestamp().Nullable()
s.Field("Version").UnsignedBigInt().OptimisticLock()

s.Index("idx_posts_status", "Status")
s.Unique("uk_posts_slug", "Slug")
s.FullText("ft_posts_title_body", "Title", "Body")
```

## Schema Sync

Register models, then sync:

```go
err := db.Register(User{}, Product{}, Order{})
err = db.Sync(ctx)
```

Sync is designed for application-driven schemas:

- creates missing tables, columns, indexes, and supported constraints
- updates safe compatible field metadata where supported by the driver
- keeps a schema snapshot for diff decisions
- detects unambiguous field renames internally
- does not perform destructive drops by default
- keeps foreign key enforcement opt-in through explicit field and relation design

## CRUD

```go
created, err := db.Use[Product]().Create(ctx, &Product{
    Code:  "P001",
    Price: 100,
})

rows, err := db.Use[Product]().
    Where("Price", ">=", 100).
    OrderBy("ID").
    Get(ctx)

count, err := db.Use[Product]().
    Where("Price", ">=", 100).
    Count(ctx)

affected, err := db.Use[Product]().
    Where("Code", "P001").
    Update(ctx, oro.Map{"Price": oro.Increment(20)})

deleted, err := db.Use[Product]().
    Where("Code", "P001").
    Delete(ctx)
```

Table writes use the same verbs and return the inserted row when supported:

```go
row, err := db.Table("products").Create(ctx, oro.Map{
    "code":  "P002",
    "price": 200,
})

updated, err := db.Table("products").
    Where("code", "P002").
    Update(ctx, oro.Map{"price": 240})
```

Write options are explicit:

```go
db.Use[Product]().Create(ctx, product, oro.Only("Code", "Price"))
db.Use[Product]().Create(ctx, product, oro.Omit("CreatedAt"))
db.Use[Product]().CreateMany(ctx, products, oro.BatchSize(500))
```

## Conditions

Use simple field conditions for common cases:

```go
db.Use[Product]().
    Where("Price", ">=", 100).
    Where("Status", "active").
    Where("Code", "in", []string{"P001", "P002"})
```

Use condition objects when that reads better:

```go
db.Use[Product]().
    Where(oro.Field("Price").Gte(100)).
    Where(oro.Field("DeletedAt").IsNull()).
    Where(oro.JSON("Meta").Path("color").Eq("red")).
    Where(oro.FullText("Title", "Body").Match("orm"))
```

Use groups and conditional groups for nested logic:

```go
db.Use[Product]().
    WhereGroup(func(w *oro.WhereBuilder) {
        w.Where("Status", "active").
            OrWhere("Status", "draft")
    }).
    WhereWhen(onlyAvailable, func(w *oro.WhereBuilder) {
        w.Where("Stock", ">", 0)
    })
```

Subqueries are first-class:

```go
activeUsers := oro.Query(
    db.Table("users").Select("id").Where("status", "active"),
)

orders, err := db.Table("orders").
    WhereIn("user_id", activeUsers).
    Get(ctx)
```

## Relations

Relations are methods. Models do not need embedded relation fields, which keeps package boundaries cleaner and helps avoid Go import cycles.

```go
type Article struct {
    oro.Model
    Title string
}

func (Article) Define(s *oro.SchemaBuilder) {
    s.Table("articles")
    s.Field("Title").String()
}

func (article Article) Cover() oro.Relation {
    return oro.HasOne(article, "Cover", "Image").
        ForeignKey("ArticleID").
        ReferenceKey("ID")
}

func (article Article) Comments() oro.Relation {
    return oro.HasMany(article, "Comments", "Comment").
        ForeignKey("ArticleID").
        ReferenceKey("ID")
}
```

Preload and read relations:

```go
article, err := db.Use[Article]().
    With(Article{}.Cover()).
    With(Article{}.Comments(), func(q *oro.RelationQuery) {
        q.Where("Status", "approved").OrderByDesc("ID")
    }).
    First(ctx)

cover, err := article.Cover().One[Image]()
comments, err := article.Comments().Many[Comment]()
```

Nested and dynamic relation loading can use strings where a static method is not practical:

```go
article, err := db.Use[Article]().
    With("Comments.User.Profile").
    First(ctx)
```

Relation filters and aggregates are available on model queries:

```go
articles, err := db.Use[Article]().
    WhereHas(Article{}.Comments(), func(q *oro.RelationQuery) {
        q.Where("Status", "approved").Count(">=", 3)
    }).
    WithCount(Article{}.Comments()).
    WithExists(Article{}.Cover()).
    Get(ctx)
```

Many-to-many pivot operations are handled through `db.Relation(...)`:

```go
err := db.Relation(article.Tags()).Attach(ctx, tag)
err = db.Relation(article.Tags()).Sync(ctx, []*Tag{tagA, tagB})
err = db.Relation(article.Tags()).UpdateThrough(ctx, tag, oro.Map{
    "position": 10,
})
```

## Query Features

```go
rows, err := db.Use[Order]().
    Select("Status", oro.Count("*").As("total")).
    Join("users", func(j *oro.Join) {
        j.OnColumn("orders.user_id", "users.id")
    }).
    Where("users.status", "active").
    GroupBy("Status").
    Having("total", ">", 10).
    OrderByDesc("total").
    Limit(20).
    Get(ctx)
```

Supported query features include:

- `Select`, aggregate expressions, raw expressions, aliases
- `Where`, `OrWhere`, `WhereGroup`, `WhereWhen`, `WhereRaw`
- `WhereColumn`, `WhereIn`, `WhereExists`
- `Join`, `LeftJoin`, `RightJoin`, `FullJoin`, `CrossJoin`, `JoinRaw`
- `GroupBy`, `Having`, `HavingRaw`
- `OrderBy`, `Limit`, `Offset`
- `Count`, `Exists`, `Sum`, `Avg`, `Min[T]`, `Max[T]`
- `Paginate`, `Chunk`, `Each`, `Stream`
- JSON path conditions and full-text search conditions
- pessimistic locks: `LockForUpdate`, `LockForShare`, `NoWait`, `SkipLocked`

## Pagination

Pagination follows the SeaORM-style explicit paginator:

```go
paginator := db.Use[Product]().
    Where("Price", ">=", 100).
    OrderBy("ID").
    Paginate(20)

page, err := paginator.Page(ctx, 1)
total, err := paginator.Total(ctx)
items, err := paginator.Items(ctx, 1)
pages, err := paginator.Pages(ctx)
```

`First` returns `nil, nil` when no row is found. `Get` returns an empty slice when no rows are found.

## Transactions

```go
err := db.Transaction(ctx, func(tx *oro.DB) error {
    product, err := tx.Use[Product]().
        Where("Code", "P001").
        LockForUpdate().
        First(ctx)
    if err != nil {
        return err
    }
    if product == nil {
        return nil
    }

    _, err = tx.Use[Product]().
        Where("ID", product.ID).
        Update(ctx, oro.Map{"Stock": oro.Decrement(1)})
    return err
}, oro.TxAttempts(3), oro.TxIsolation(oro.LevelReadCommitted))
```

Nested transactions use savepoints:

```go
tx, err := db.Begin(ctx)
nested, err := tx.Begin(ctx)
err = nested.Rollback(ctx)
err = tx.Commit(ctx)
```

Manual savepoints are available:

```go
sp, err := tx.Savepoint(ctx)
err = sp.Rollback(ctx)
err = sp.Release(ctx)
```

## Multi Driver and Connections

Oro is built on `database/sql`. Drivers are injected through connection config, so standard SQL drivers can be wrapped behind the Oro driver interface.

```go
db, err := oro.Open(oro.Config{
    Default:     "main",
    TablePrefix: "app_",
    Connections: map[string]oro.ConnectionConfig{
        "main": {
            Driver: sqlite.Open("app.db"),
        },
        "mysql": {
            Driver: mysql.Open("root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=false"),
        },
        "pgsql": {
            Driver: pgsql.Open("postgres://root@localhost:5432/duxorm?sslmode=disable"),
        },
    },
})
```

Read replicas are connection-local:

```go
Connections: map[string]oro.ConnectionConfig{
    "main": {
        Driver: sqlite.Open("primary.db"),
        Reads: []oro.Driver{
            sqlite.Open("replica-1.db"),
            sqlite.Open("replica-2.db"),
        },
    },
}
```

Use `db.TableName(...)` when composing raw SQL with configured table prefixes:

```go
rows, err := db.Raw(
    "select count(*) as total from "+db.TableName("products")+" where price >= ?",
    100,
).First(ctx)
```

Models can declare their default connection:

```go
func (Order) Define(s *oro.SchemaBuilder) {
    s.Connection("orders")
    s.Table("orders")
}
```

## Tenancy and Sharding

Tenant values are explicit and use one semantic input type: `oro.Map`.

```go
db, err := oro.Open(oro.Config{
    Tenant: &oro.TenantConfig{
        Fields: []string{"TenantID", "AppID"},
    },
    Connections: connections,
})

rows, err := db.Tenant(oro.Map{
    "TenantID": 10,
    "AppID":    20,
}).Use[Product]().Get(ctx)
```

Models can opt in, override tenant fields, or opt out:

```go
func (Product) Define(s *oro.SchemaBuilder) {
    s.Tenant("TenantID", "AppID")
}

func (SystemLog) Define(s *oro.SchemaBuilder) {
    s.NoTenant()
}
```

Sharding is also explicit:

```go
func (Order) Define(s *oro.SchemaBuilder) {
    s.Shard("orders", "TenantID")
}

orders, err := db.Use[Order]().
    Shard(oro.Map{"TenantID": 10}).
    Get(ctx)
```

## Hooks, Events, Cache, Serialization

Model hooks:

```go
func (product *Product) BeforeCreate(ctx context.Context, h *oro.Hook) error {
    product.Code = strings.TrimSpace(product.Code)
    return nil
}
```

Global events:

```go
unsubscribe := db.On(oro.AfterQuery, func(ctx context.Context, event *oro.Event) error {
    return nil
})
defer unsubscribe()
```

Query cache:

```go
db, err := oro.Open(oro.Config{
    Cache: oro.NewMemoryCacheStore(),
    Connections: connections,
})

rows, err := db.Use[Product]().
    Cache(time.Minute).
    CacheTags("products").
    Get(ctx)

err = db.Cache().ForgetTag(ctx, "products")
```

API-safe serialization hides fields declared with `Hidden()` and includes loaded relations:

```go
payload := oro.Serialize(product)
payloadWithHidden := oro.Serialize(product, oro.ShowHidden())
```

## ORM Comparison

| ORM | Language | Main workflow | Strength | Tradeoff | Oro position |
| --- | --- | --- | --- | --- | --- |
| [GORM](https://gorm.io/docs/) | Go | Struct models + tags + AutoMigrate | Mature Go ecosystem, broad feature set | Tag-heavy metadata, zero-value writes can be surprising, association APIs are broad | Keeps Auto Sync idea, but uses `Define` and explicit `Map` writes |
| [Laravel Eloquent](https://laravel.com/docs/12.x/eloquent) | PHP | Active Record models + migrations | Extremely readable relationship and query syntax | Migration files are separate; dynamic typing does not map to Go directly | Borrows method-based relations and fluent query style |
| [SeaORM](https://www.sea-ql.org/SeaORM/docs/index/) | Rust | Entity/query APIs, often generated from schema | Strong typing, explicit relation loaders, async Rust design | Entity generation and Rust ceremony are part of the workflow | Borrows typed relation access without requiring codegen |
| [Ent](https://entgo.io/docs/getting-started/) | Go | Schema definitions + code generation | Strong compile-time API and graph model | Codegen is central; generated package layout affects architecture | Chooses no-codegen and smaller public surface |
| [Prisma](https://www.prisma.io/docs/orm) | Node.js/TS | Prisma schema + generated client | Excellent developer experience and type-safe client | Requires schema file and generated client | Keeps the simple client feel, but stores schema beside Go model |
| [TypeORM](https://typeorm.io/) | Node.js/TS | Decorators/entities or schema objects | Familiar ORM model for TypeScript apps | Runtime metadata/decorator behavior can be opaque | Keeps direct query ergonomics without decorators |

## Release Readiness

Oro is close to a public preview release:

- implemented: SQLite, MySQL, PostgreSQL drivers
- implemented: schema sync, CRUD, table queries, raw queries, relation loading, many-to-many, dynamic relations
- implemented: transactions, savepoints, locks, upsert, pagination, aggregates, scopes, hooks/events, cache, tenancy, sharding
- tested: shared SQLite/MySQL/PostgreSQL integration matrix
- documented: README, examples, and Astro documentation site

Before calling it `v1.0`, the remaining release work should be treated as product hardening:

- freeze public API names and option semantics
- run real project dogfooding against at least one non-trivial service
- add CI for Go 1.27, SQLite, MySQL, and PostgreSQL
- confirm license, module tags, release notes, and compatibility policy
- switch from `go1.27rc1` to stable Go 1.27 when available

Recommended first tag: **`v0.1.0` or `v0.9.0-preview`**, not `v1.0.0`.

## Tests

Use Go 1.27:

```bash
source ~/.gvm/scripts/gvm
gvm use go1.27rc1
go test ./...
go test -race ./...
```

MySQL and PostgreSQL integration tests use these defaults when environment variables are not set:

```bash
ORO_MYSQL_DSN='root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=false'
ORO_PGSQL_DSN='postgres://root@localhost:5432/duxorm?sslmode=disable'
```

## Examples

```bash
go run ./examples/quickstart
go run ./examples/crud
go run ./examples/relations
go run ./examples/transactions
go run ./examples/multi-driver
```

## Documentation

The full documentation website lives in `docs/` and is built with Astro Starlight.

```bash
cd docs
pnpm install
pnpm run dev
```
