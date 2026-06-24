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

## Scenarios

- `BenchmarkCreate`: insert one product per operation.
- `BenchmarkFirstByCode`: query one row by indexed unique code.
- `BenchmarkWhereList`: query 20 rows with `where price >= ? order by id limit 20`.
- `BenchmarkUpdateByCode`: update one row by indexed unique code.
- `BenchmarkDeleteByCode`: delete one row by indexed unique code.

## Notes

- Benchmarks are useful for relative direction, not absolute product claims.
- SQLite in-memory measures ORM overhead plus SQLite driver behavior.
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
