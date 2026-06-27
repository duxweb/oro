# Changelog

English | [简体中文](CHANGELOG.zh-CN.md)

All notable changes to Oro are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

This is a hardening pass over the pre-1.0 preview: it closes correctness and
security gaps found in review, makes the schema/cache/shard subsystems behave
consistently, removes dead and duplicated code, and adds the missing tests.

### Security

- **Operator allowlist.** `Where` / `OrWhere` / `Having`, column conditions, and
  `JOIN ... ON` operators are now validated against a fixed allowlist
  (`IsSafeConditionOperator` / `IsSafeColumnOperator`) and normalized before
  rendering. A non-allowlisted operator is rejected instead of being
  interpolated into SQL, closing an operator-based SQL-injection vector.
- **Quoted aggregate identifiers.** Table-query `Sum/Avg/Min/Max` and relation
  aggregates now quote the column via the dialect instead of string-formatting
  it.
- **Soft-delete scope no longer leaks.** Eager-loaded relations (`.With(...)`)
  and low-level `db.Table(...)` reads now apply the soft-delete predicate, so
  soft-deleted rows are no longer returned through those paths.
- **Tenant scope on table queries.** `db.Table(...)` reads on a registered
  tenant model are now tenant-scoped (resolved via `SchemaForTable`).

### Added

- `oro.EscapeLike` plus `FieldExpr.Contains` / `StartsWith` / `EndsWith`, which
  emit `LIKE ? ESCAPE '\'` consistently across SQLite, MySQL, and PostgreSQL so
  user input containing `%` / `_` is matched literally. Plain `Like` / `NotLike`
  keep their wildcard semantics.
- `JSONPath.Like` for `LIKE` matching on JSON-path values.
- Cross-shard `Get` / `First` / pagination now perform a global merge by
  `ORDER BY` and apply `LIMIT` / `OFFSET` globally rather than per shard.
- `translation.WhereTransLike` is implemented (was previously a stub).

### Fixed

- `WhereHas` inside an eager-load `.With(...)` callback no longer fails with
  "unknown field"; the deferred relation filter is now resolved on that path.
- `Count()` / `Paginate` with `GroupBy` returns the number of groups instead of
  the first group's count, for both model and table queries (including the
  cross-shard count path).
- `Select(Raw(sql, args...))` no longer drops its bound arguments.
- `OFFSET` without `LIMIT` now produces valid SQL on SQLite and MySQL.
- Empty `IN` / `NOT IN` value lists no longer abort the whole query.
- Schema sync is idempotent for `bool`, `json`, array, and `point` columns; the
  MySQL and PostgreSQL full-text indexes no longer churn on every sync.
- The model registry is concurrency-safe; concurrent first-use of models no
  longer triggers a "concurrent map writes" crash.
- The prepared-statement cache is reference-counted: a statement evicted while
  in flight is no longer closed out from under a running query (no
  use-after-close), and evicted statements are no longer leaked.
- The request `context` is threaded through spec building, shard-key
  resolution, and cache-key derivation, so context-based tenancy/sharding works
  and deadlines/cancellation are honored.
- Cross-shard model aggregates (`Sum/Avg/Min/Max`) return an error instead of a
  silent single-shard result.
- Eager-load key matching handles named integer types and `Null[T]` keys.
- The audit log no longer records `Hidden` field values.
- nestedset `Tree.Update` no longer overwrites unprovided columns with zero
  values; added `UpdateValues` for explicit partial updates.
- Soft-delete `OnlyDeleted` scoping is now applied consistently on writes.

### Changed

- Removed the dead `RelationLoader` / `RelationWriter` factory surface (no-op
  implementations that silently ignored overrides).
- Consolidated struct field-index reflection into `internal/reflectx`,
  replacing four near-identical copies.
- De-duplicated the operator allowlist, cross-shard fan-out loop, batch
  parameter capping, and integer-type detection.

### Tests

- Added prepared-statement cache tests covering eviction, deferred close while
  in flight, leak prevention, full teardown, and concurrent eviction under
  `-race`.
- Added `internal/fifocache` tests for bounded growth, FIFO eviction, and the
  eviction hook.
- Added regression tests for eager-load `WhereHas` and table-query
  soft-delete / grouped-count scoping.
