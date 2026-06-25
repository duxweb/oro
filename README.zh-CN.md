<p align="center">
  <h1 align="center">Oro</h1>
</p>

<p align="center">
  <b>面向 Go 的人性化泛型 ORM。</b><br/>
  不需要代码生成，不把数据库结构塞进 tag 字符串，不隐藏关联黑盒；只保留类型化模型查询、显式 schema、方法式关联、自动同步和清晰的多驱动架构。
</p>

<p align="center">
  <a href="https://duxweb.github.io/oro/zh-cn/">官网</a> &middot;
  <a href="https://duxweb.github.io/oro/">文档</a> &middot;
  <a href="#快速开始">快速开始</a> &middot;
  <a href="#性能基准">性能基准</a> &middot;
  <a href="https://github.com/duxweb/oro/issues">反馈</a>
</p>

<p align="center">
  <a href="README.md">English</a> | 简体中文
</p>

<p align="center">
  <a href="https://github.com/duxweb/oro/actions/workflows/pages.yml"><img alt="Docs" src="https://github.com/duxweb/oro/actions/workflows/pages.yml/badge.svg"></a>
  <a href="https://pkg.go.dev/github.com/duxweb/oro"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/duxweb/oro.svg"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-MIT-black.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.27+-00ADD8.svg">
</p>

---

## 为什么做 Oro

很多 Go ORM 都会带来某种历史包袱：结构体 tag 变成小型配置语言、代码生成成为核心工作流、零值写入语义容易误判、关联字段导致模块拆包后出现 import cycle。Oro 的目标是把规则减少，并且让规则直接出现在代码表面。

| 常见 ORM 摩擦 | Oro 的处理方式 |
| :--- | :--- |
| schema 信息藏在 tag 里 | 使用模型本地 `Define` 方法和类型化 schema builder。 |
| 依赖生成 client / entity | 模型就是普通 Go struct，查询直接走泛型入口。 |
| 零值更新语义不清楚 | 更新统一使用 `oro.Map`，写什么字段就传什么字段。 |
| 关联字段制造循环依赖 | 关联是返回 `oro.Relation` 的方法，不嵌入结构体字段。 |
| 动态表查询难以映射 | `db.Table(...).MapTo[T]()` 可映射 DTO，不依赖 `db` tag。 |
| Raw SQL 遇到表前缀易出错 | `db.TableName(...)` 暴露当前连接下的带前缀表名。 |
| 多数据库是后补能力 | 命名连接、模型连接、读副本和驱动从一开始就是一等能力。 |

Oro 的公开入口保持很少：

```go
db.Use[Product]()      // 模型查询，使用 Go 字段名
db.Table("products")   // 裸表查询，使用数据库列名
db.Raw("select ...")   // 原生 SQL
```

## 当前状态

Oro 目前是 **pre-1.0 / public preview candidate**。核心 ORM、结构同步、SQLite/MySQL/PostgreSQL 适配器、关联加载、事务、分页、表前缀、Hooks/Events、缓存、租户、分片、示例、性能基准和文档站点都已实现。

项目目标是 **Go 1.27+**，因为公开 API 设计依赖方法级泛型。当前模块可能暂时使用 Go 1.27 RC 版本，直到所有环境都可用稳定版工具链。

## 安装

```bash
go get github.com/duxweb/oro
```

选择 Oro 适配器和具体 `database/sql` 驱动：

```go
import (
    oro "github.com/duxweb/oro"
    "github.com/duxweb/oro/driver/mysql"
    "github.com/duxweb/oro/driver/pgsql"
    "github.com/duxweb/oro/driver/sqlite"

    _ "github.com/go-sql-driver/mysql"
    _ "github.com/jackc/pgx/v5/stdlib"
    _ "modernc.org/sqlite" // 或 _ "github.com/mattn/go-sqlite3"
)
```

Oro 适配器只包装 `database/sql`，不会强制具体数据库驱动。应用自己通过 blank import 选择真实驱动。

## 快速开始

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

## 模型定义

Oro 使用 `Define`，不使用复杂 ORM tag：

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

字段名使用 Go struct 字段。未设置 `Column(...)` 时默认转为 snake_case。

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

需要软删除时显式开启：

```go
type User struct {
    oro.Model
    softdelete.SoftDeleteFields // DeletedAt -> deleted_at
}
```

## 自动同步

注册模型后同步结构：

```go
err := db.Register(User{}, Product{}, Order{})
err = db.Sync(ctx)
```

同步策略面向应用自管 schema：

- 创建缺失的表、字段、索引和受支持约束；
- 在驱动支持时更新安全兼容的字段元信息；
- 使用 schema snapshot 辅助 diff 决策；
- 内部识别明确无歧义的重命名；
- 默认不执行破坏性 drop；
- 外键默认保守，按字段和关系设计显式启用。

## 查询

```go
products, err := db.Use[Product]().
    Where("Price", ">=", 100).
    Where(oro.Field("Code").Like("P%")).
    OrderByDesc("ID").
    Get(ctx)
```

嵌套条件使用明确的 group 方法：

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

查不到不是错误：

- `First` 和 `Find` 查不到返回 `nil, nil`；
- `Get` 查不到返回空切片。

## 裸表、Raw 和 DTO 映射

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

Raw SQL 也可以映射：

```go
rows, err := db.Raw(
    "select * from "+db.TableName("products")+" where price >= ?",
    100,
).MapTo[ProductView]().Get(ctx)
```

## 不制造循环依赖的关联

关联是方法，不是结构体字段。

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

预加载并读取关联值：

```go
article, err := db.Use[Article]().
    With(Article{}.Comments(), func(q *oro.RelationQuery) {
        q.Where("Status", "approved").OrderByDesc("ID")
    }).
    First(ctx)

comments, err := article.Comments().Many[Comment]()
```

## 写入

```go
created, err := db.Use[Product]().Create(ctx, product, oro.Only("Code", "Price"))

result, err := db.Use[Product]().CreateMany(ctx, products, oro.BatchSize(500))
ids, err := result.IDs[uint64]()

updated, err := db.Use[Product]().
    Where("Code", "P001").
    Update(ctx, oro.Map{"Price": oro.Increment(20)})

deleted, err := db.Use[Product]().Where("Code", "P001").Delete(ctx)
```

更新使用 `oro.Map`，不猜测零值语义。

## 事务与锁

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

嵌套事务使用 savepoint，也支持手动 savepoint。

## 多驱动、租户与分片

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

租户和分片统一使用 `oro.Map`：

```go
rows, err := tenant.Use(db, oro.Map{"TenantID": 10, "AppID": 20}).
    Use[Product]().
    Get(ctx)

orders, err := db.Use[Order]().
    Shard(oro.Map{"TenantID": 10}).
    Get(ctx)
```

## 性能基准

Benchmark 在 SQLite、MySQL、PostgreSQL 下执行，并且每个 ORM 使用相同数据库驱动。文档中展示的是连续 10 轮的中位数。

| 场景 | SQLite | MySQL | PostgreSQL |
| :--- | :--- | :--- | :--- |
| CreateMany100 | 本轮 Oro 领先 | 本轮 Oro 领先 | Oro 接近领先 |
| FirstByCode | 本轮 Oro 领先 | 本轮 Oro 领先 | Oro 表现稳定 |
| WhereList | 本轮 Oro 领先 | 本轮 Oro 领先 | Oro 表现稳定 |
| DeleteByCode | 本轮 Oro 领先 | 本轮 Oro 领先 | Oro 表现稳定 |

完整环境、版本、命令和结果见 [性能基准](https://duxweb.github.io/oro/zh-cn/advanced/performance-benchmarks/)。

## 对比

| 项目 | 工作流 | 优势 | 取舍 | Oro 的定位 |
| :--- | :--- | :--- | :--- | :--- |
| GORM | Struct + tags + AutoMigrate | 成熟、生态大 | tag 元数据重，关联行为宽泛 | 保留自动同步思路，但 schema 和写入更显式 |
| Ent | Schema + 代码生成 | 强编译期图 API | codegen 是核心 | 不走生成，公开面更小 |
| Bun | SQL-first builder + models | 快，接近 SQL | 模型自动化较少 | 保留 SQL 清晰度，补 schema sync 和方法式关联 |
| XORM | 传统 ORM 映射 | 简单 CRUD | API 历史感较强 | 使用现代泛型和更明确同步规则 |
| Oro | Plain structs + `Define` + 泛型查询 | 边界清楚、无生成、多驱动 | 需要 Go 1.27+ 泛型 | 面向现代 Go 应用设计 |

## 文档

- English docs: <https://duxweb.github.io/oro/>
- 中文文档: <https://duxweb.github.io/oro/zh-cn/>
- 示例：[`examples/`](examples/)
- 性能基准：[`benchmarks/ormbench/`](benchmarks/ormbench/)

## 参与贡献

Oro 仍处于 pre-1.0，API 命名、驱动能力、结构同步边界、性能和真实项目接入反馈都很重要。欢迎通过 issue 反馈。

## License

MIT
