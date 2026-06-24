# Oro ORM Benchmarks

This directory is a standalone Go module so benchmark-only dependencies do not pollute the root `github.com/duxweb/oro` module.

Compared libraries:

- Oro
- GORM
- XORM
- Bun

Current benchmark database: SQLite in-memory.

## Run

```bash
source ~/.gvm/scripts/gvm
gvm use go1.27rc1
cd benchmarks/ormbench
go mod tidy
go test -bench=. -benchmem -run '^$' -count=5
```

For a quick local check:

```bash
go test -bench=. -benchmem -run '^$' -benchtime=1s -count=1
```

Run against another driver:

```bash
ORO_BENCH_DRIVER=mysql COUNT=1 BENTIME=100ms ./run.sh
ORO_BENCH_DRIVER=pgsql COUNT=1 BENTIME=100ms ./run.sh
```

Driver options:

- `ORO_BENCH_DRIVER=sqlite|mysql|pgsql`
- `ORO_BENCH_DSN=...`
- MySQL default DSN: `root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=true&clientFoundRows=true`
- PostgreSQL default DSN: `postgres://root@localhost/duxorm?sslmode=disable`

## Scenarios

- `BenchmarkCreate`: insert one product per operation.
- `BenchmarkFirstByCode`: query one row by indexed unique code.
- `BenchmarkWhereList`: query 20 rows with `where price >= ? order by id limit 20`.
- `BenchmarkUpdateByCode`: update one row by indexed unique code.
- `BenchmarkDeleteByCode`: delete one row by indexed unique code.

## Notes

- Benchmarks are useful for relative direction, not absolute product claims.
- SQLite in-memory measures ORM overhead plus SQLite driver behavior.
- MySQL and PostgreSQL runs use the local `duxorm` database by default and reset `products` / `oro_schema` for each sub-benchmark.
- Each sub-benchmark gets a fresh in-memory database.
- Delete benchmarks seed `b.N` rows before timing so each operation deletes a unique row.
- Read/update benchmarks seed 1000 rows before timing.

## Latest Local Result

Environment:

- Date: 2026-06-24
- CPU: Apple M4 Pro
- OS/arch: darwin/arm64
- Go: go1.27rc1
- Command: `go test -bench=. -benchmem -run '^$' -benchtime=1s -count=3`

Median-style summary from the 3 local runs:

| Scenario | Oro | GORM | XORM | Bun |
| --- | ---: | ---: | ---: | ---: |
| Create | 9.72 µs/op, 96 allocs | 7.44 µs/op, 56 allocs | 5.76 µs/op, 43 allocs | 9.49 µs/op, 23 allocs |
| First by code | 4.52 µs/op, 72 allocs | 5.25 µs/op, 69 allocs | 7.34 µs/op, 114 allocs | 5.90 µs/op, 31 allocs |
| Where list 20 rows | 22.18 µs/op, 261 allocs | 16.71 µs/op, 203 allocs | 22.32 µs/op, 500 allocs | 15.16 µs/op, 93 allocs |
| Update by code | 4.26 µs/op, 50 allocs | 4.27 µs/op, 48 allocs | 5.25 µs/op, 74 allocs | 4.23 µs/op, 15 allocs |
| Delete by code | 3.44 µs/op, 28 allocs | 3.69 µs/op, 40 allocs | 5.17 µs/op, 72 allocs | 5.37 µs/op, 13 allocs |

Current conclusion:

- Oro and GORM both run with skipped default write transactions for fair high-throughput comparison. Production Oro default remains safe unless `SkipDefaultTransaction` is explicitly enabled.
- Compared with the earlier optimized Oro result, `Create` improved from ~22.34 µs / 139 allocs to ~9.72 µs / 96 allocs; `WhereList` improved from ~28.24 µs / 348 allocs to ~22.18 µs / 261 allocs; `Delete` improved from ~7.66 µs / 44 allocs to ~3.44 µs / 28 allocs.
- Oro leads this matrix on single-row read, update, and delete; create remains behind XORM/GORM/Bun; list reads remain behind Bun/GORM and close to XORM.

## Driver Matrix Smoke Results

These are short local smoke runs, useful for compatibility checks rather than final claims.

Command:

```bash
COUNT=1 BENTIME=100ms ./run.sh
```

### MySQL

Environment: local `duxorm` database, default DSN from `ORO_BENCH_DRIVER=mysql`.

| Scenario | Oro | GORM | XORM | Bun |
| --- | ---: | ---: | ---: | ---: |
| Create | 135.33 µs/op, 100 allocs | 123.45 µs/op, 41 allocs | 121.75 µs/op, 47 allocs | 103.27 µs/op, 15 allocs |
| First by code | 38.01 µs/op, 62 allocs | 65.93 µs/op, 68 allocs | 66.66 µs/op, 118 allocs | 37.84 µs/op, 31 allocs |
| Where list 20 rows | 59.31 µs/op, 290 allocs | 84.34 µs/op, 203 allocs | 89.86 µs/op, 543 allocs | 52.57 µs/op, 93 allocs |
| Update by code | 82.33 µs/op, 50 allocs | 92.66 µs/op, 49 allocs | 92.97 µs/op, 67 allocs | 89.46 µs/op, 18 allocs |
| Delete by code | 114.26 µs/op, 30 allocs | 134.96 µs/op, 40 allocs | 133.56 µs/op, 75 allocs | 115.92 µs/op, 15 allocs |

### PostgreSQL

Environment: local `duxorm` database, default DSN from `ORO_BENCH_DRIVER=pgsql`.

| Scenario | Oro | GORM | XORM | Bun |
| --- | ---: | ---: | ---: | ---: |
| Create | 62.30 µs/op, 92 allocs | 57.56 µs/op, 52 allocs | 56.79 µs/op, 65 allocs | 101.29 µs/op, 33 allocs |
| First by code | 37.35 µs/op, 70 allocs | 35.95 µs/op, 64 allocs | 39.14 µs/op, 125 allocs | 95.28 µs/op, 46 allocs |
| Where list 20 rows | 109.01 µs/op, 222 allocs | 42.54 µs/op, 199 allocs | 97.52 µs/op, 512 allocs | 101.53 µs/op, 93 allocs |
| Update by code | 59.03 µs/op, 48 allocs | 57.37 µs/op, 43 allocs | 58.39 µs/op, 83 allocs | 64.20 µs/op, 13 allocs |
| Delete by code | 53.22 µs/op, 28 allocs | 53.76 µs/op, 35 allocs | 52.91 µs/op, 78 allocs | 59.03 µs/op, 10 allocs |
