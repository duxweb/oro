# Repository guide for AI agents

Oro is a generic-first ORM for Go (module `github.com/duxweb/oro`). No code
generation. Requires **Go 1.27+** (the API uses generic methods, e.g.
`db.Use[T]()`), so go.mod pins `go 1.27rc1` until the stable toolchain is broadly
available.

## Build / test / format (run before proposing changes)

- `go build ./...`
- `go vet ./...`
- `gofmt -l .` (must be empty)
- `go test ./...` — SQLite tests are in-memory and always run.
- `go test -race ./...` for concurrency-touching changes.
- MySQL/PostgreSQL matrix needs live DBs (DSNs in `extensions/internal/exttest`
  and `driver/internal/integrationtest`). Skip if unavailable; do not weaken
  assertions to make them pass.
- Docs use pnpm: `pnpm --dir docs build`.

## Architecture

`DB` (session, clone-on-write) → `Runtime` (Factory-built components) →
`QuerySpec`/`WriteSpec` → Planner → AST → dialect compile → `sqlExecutor` →
`sqlRunner` → driver. Internals live under `internal/**`; drivers live under
`driver/{sqlite,mysql,pgsql}`; bundled extensions live under `extensions/**`.

## Conventions (do not break)

- Entry points stay small: `db.Use[T]()` (Go field names), `db.Table(...)` (DB
  column names), `db.Raw(...)`.
- Updates use `oro.Map` (explicit fields; zero values are never guessed).
- Relations are methods returning `oro.Relation`; never embedded struct fields.
- Conditions are composable values: `oro.Field(...)`, `oro.Time(...)`, and
  `oro.JSON(...)` return `Condition`. Operators are validated against an allowlist
  (`IsSafeConditionOperator` / `IsSafeColumnOperator`); never interpolate a raw
  operator.
- Time is stored in UTC and read back in `Config.Location` (default UTC). Time
  args are normalized to UTC in `sqlRunner`. Calendar ranges compile to
  half-open `>= a AND < b`; never use DB date functions for these helpers.
- Struct field-index reflection goes through `internal/reflectx`.
- Read scoping (soft-delete / tenant / extensions) is applied once at
  `finalizeReadSpec` / `modelQuerySpec`; do not duplicate it per call site.

## Changes

- Match surrounding code style and avoid unnecessary dependencies.
- Add or adjust tests for behavior changes; verify regression tests fail without
  the fix by toggling with an editor, not by discarding uncommitted work.
- Docs and changelog are bilingual: update both `docs/.../X.mdx` and
  `docs/.../zh-cn/X.mdx`, and both `CHANGELOG.md` and `CHANGELOG.zh-CN.md` under
  `[Unreleased]` / `[未发布]`.
