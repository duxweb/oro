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
| Create | 10.25 µs/op, 101 allocs | 7.55 µs/op, 56 allocs | 5.80 µs/op, 43 allocs | 9.39 µs/op, 23 allocs |
| First by code | 4.64 µs/op, 74 allocs | 5.18 µs/op, 69 allocs | 7.35 µs/op, 114 allocs | 5.87 µs/op, 31 allocs |
| Where list 20 rows | 22.97 µs/op, 268 allocs | 17.06 µs/op, 203 allocs | 22.12 µs/op, 500 allocs | 15.08 µs/op, 93 allocs |
| Update by code | 4.21 µs/op, 51 allocs | 4.44 µs/op, 48 allocs | 5.24 µs/op, 74 allocs | 4.43 µs/op, 15 allocs |
| Delete by code | 3.57 µs/op, 29 allocs | 3.77 µs/op, 40 allocs | 5.10 µs/op, 72 allocs | 5.37 µs/op, 13 allocs |

Current conclusion:

- Oro and GORM both run with skipped default write transactions for fair high-throughput comparison. Production Oro default remains safe unless `SkipDefaultTransaction` is explicitly enabled.
- Compared with the earlier optimized Oro result, `Create` improved from ~22.34 µs / 139 allocs to ~10.25 µs / 101 allocs; `WhereList` improved from ~28.24 µs / 348 allocs to ~22.97 µs / 268 allocs; `Delete` improved from ~7.66 µs / 44 allocs to ~3.57 µs / 29 allocs.
- Oro leads this matrix on single-row read, update, and delete; create remains behind XORM/GORM/Bun; list reads remain behind Bun/GORM and close to XORM.
