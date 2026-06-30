# 更新记录

[English](CHANGELOG.md) | 简体中文

本文件记录 Oro 的所有重要变更。

格式参考 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
并遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [未发布]

本次为 pre-1.0 预览版的一轮加固：修复 review 中发现的正确性与安全问题，统一
schema/缓存/分片各子系统的行为，移除死代码与重复实现，并补齐缺失的测试。

### 安全

- **操作符白名单。** `Where` / `OrWhere` / `Having`、列条件以及 `JOIN ... ON`
  的操作符现在会按固定白名单校验（`IsSafeConditionOperator` /
  `IsSafeColumnOperator`）并在渲染前归一化。非白名单操作符会被拒绝，而不再被
  拼接进 SQL，堵住了基于操作符的 SQL 注入。
- **聚合标识符加引号。** 表查询的 `Sum/Avg/Min/Max` 及关系聚合现在通过方言对列名
  加引号，不再用字符串拼接。
- **软删除作用域不再泄露。** 预加载关系（`.With(...)`）与底层 `db.Table(...)`
  读取现在会施加软删除条件，已软删的行不再从这些路径返回。
- **表查询的租户作用域。** 对已注册租户模型的 `db.Table(...)` 读取现在会按租户
  过滤（通过 `SchemaForTable` 解析）。

### 新增

- AI/LLM 开发支持：包级 godoc、导出 API 注释、pkg.go.dev 示例、仓库
  `AGENTS.md` / `CLAUDE.md`，以及自动生成的 `/llms.txt` 与
  `/llms-full.txt` 文档索引。
- `oro.Time(field)` 时间范围条件，以及 `DayBounds`、`MonthBounds`、
  `YearBounds` 和 `FieldExpr.NotBetween`。日期桶会编译为可走索引的半开
  区间，而不是数据库日期函数。
- `Config.Location`，用于控制读取 `time.Time` 时转换到的时区。Oro 仍统一以
  UTC 存储时间；未配置时读取默认也是 UTC。
- `oro.EscapeLike` 以及 `FieldExpr.Contains` / `StartsWith` / `EndsWith`，在
  SQLite、MySQL、PostgreSQL 上一致输出 `LIKE ? ESCAPE '\'`，使包含 `%` / `_`
  的用户输入按字面匹配。普通的 `Like` / `NotLike` 仍保留通配符语义。
- `JSONPath.Like`，用于对 JSON 路径取值做 `LIKE` 匹配。
- 跨分片 `Get` / `First` / 分页现在按 `ORDER BY` 做全局归并，并全局应用
  `LIMIT` / `OFFSET`，而不再按单分片处理。
- 实现了 `translation.WhereTransLike`（此前为永久报错的占位实现）。

### 修复

- 时间处理现在在不同驱动和机器时区下保持一致：自动时间戳、用户传入的时间字段、
  查询条件中的时间参数都会在执行前归一为 UTC，读出后再转换到 `Config.Location`。
- 预加载 `.With(...)` 回调内的 `WhereHas` 不再报 "unknown field"；该路径现在会
  解析延迟的关系过滤条件。
- `GroupBy` 下的 `Count()` / `Paginate` 返回分组数量而非首个分组的计数，模型查询
  与表查询均已修复（含跨分片计数路径）。
- `Select(Raw(sql, args...))` 不再丢失其绑定参数。
- 无 `LIMIT` 的 `OFFSET` 现在在 SQLite 与 MySQL 上生成合法 SQL。
- 空的 `IN` / `NOT IN` 列表不再使整条查询失败。
- Schema 同步对 `bool`、`json`、数组、`point` 列具备幂等性；MySQL 与 PostgreSQL
  的全文索引不再每次同步都重建。
- 模型注册表并发安全；多协程首次使用模型不再触发 "concurrent map writes" 崩溃。
- 预编译语句缓存改为引用计数：执行中的语句被淘汰时不再被提前关闭（消除
  use-after-close），被淘汰的语句也不再泄露。
- 请求 `context` 现在贯穿 spec 构建、分片键解析与缓存键派生，因此基于 context 的
  租户/分片可正常工作，超时与取消也会被遵守。
- 跨分片模型聚合（`Sum/Avg/Min/Max`）现在返回错误，而不再静默返回单分片结果。
- 预加载的键匹配现在能处理命名整型与 `Null[T]` 键。
- 审计日志不再记录 `Hidden` 字段的值。
- nestedset 的 `Tree.Update` 不再用零值覆盖未提供的列；新增 `UpdateValues`
  用于显式的部分更新。
- 软删除的 `OnlyDeleted` 作用域现在在写操作上一致施加。

### 变更

- 移除了死代码 `RelationLoader` / `RelationWriter` 工厂接口（其 no-op 实现会静默
  忽略外部重写）。
- 将结构体字段索引反射合并进 `internal/reflectx`，替换原先四份几乎相同的副本。
- 去除了操作符白名单、跨分片扇出循环、批量参数上限计算、整型判断的重复实现。

### 测试

- 新增预编译语句缓存测试，覆盖淘汰、执行中淘汰的延迟关闭、泄漏防护、整体关闭，
  以及 `-race` 下的并发淘汰。
- 新增 `internal/fifocache` 测试，覆盖有界增长、FIFO 淘汰与淘汰回调。
- 新增预加载 `WhereHas` 与表查询软删除/分组计数作用域的回归测试。
