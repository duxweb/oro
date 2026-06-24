# Oro ORM Benchmarks

This directory is a standalone Go module so benchmark-only dependencies do not pollute the root `github.com/duxweb/oro` module.

Compared libraries:

- Oro
- GORM
- XORM
- Bun

## Drivers

The benchmark can run against SQLite, MySQL, and PostgreSQL.

Driver options:

- `ORO_BENCH_DRIVER=sqlite|mysql|pgsql`
- `ORO_BENCH_SQLITE_DRIVER=modernc|mattn`
- `ORO_BENCH_DSN=...`

Default DSNs:

- SQLite: in-memory database.
- MySQL: `root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=true&clientFoundRows=true`
- PostgreSQL: `postgres://root@localhost/duxorm?sslmode=disable`

The published benchmark reference uses the same underlying database driver for each compared library:

- SQLite: `github.com/mattn/go-sqlite3`
- MySQL: `github.com/go-sql-driver/mysql`
- PostgreSQL: `github.com/jackc/pgx/v5`

Oro root module and adapter packages do not import concrete SQL driver implementations. Applications opt in with blank imports or by opening their own `*sql.DB` and using `Wrap`.

## Run

SQLite fair-driver run:

```bash
source ~/.gvm/scripts/gvm && gvm use go1.27rc1
cd benchmarks/ormbench
ORO_BENCH_DRIVER=sqlite ORO_BENCH_SQLITE_DRIVER=mattn \
  go test -bench=. -benchmem -run '^$' -benchtime=1s -count=10 -timeout=0
```

MySQL:

```bash
source ~/.gvm/scripts/gvm && gvm use go1.27rc1
cd benchmarks/ormbench
ORO_BENCH_DRIVER=mysql \
  go test -bench=. -benchmem -run '^$' -benchtime=1s -count=10 -timeout=0
```

PostgreSQL:

```bash
source ~/.gvm/scripts/gvm && gvm use go1.27rc1
cd benchmarks/ormbench
ORO_BENCH_DRIVER=pgsql \
  go test -bench=. -benchmem -run '^$' -benchtime=1s -count=10 -timeout=0
```

For a quick smoke check, use the helper script with a shorter count:

```bash
COUNT=1 BENTIME=100ms ./run.sh
ORO_BENCH_DRIVER=mysql COUNT=1 BENTIME=100ms ./run.sh
ORO_BENCH_DRIVER=pgsql COUNT=1 BENTIME=100ms ./run.sh
```

## Scenarios

- `BenchmarkCreate`: insert one product per operation.
- `BenchmarkCreateMany100`: insert 100 products per operation.
- `BenchmarkFirstByCode`: query one row by indexed unique code.
- `BenchmarkWhereList`: query 20 rows with `where price >= ? order by id limit 20`.
- `BenchmarkUpdateByCode`: update one row by indexed unique code.
- `BenchmarkDeleteByCode`: delete one row by indexed unique code.

Read/update benchmarks seed 1000 rows before timing. Delete benchmarks seed `b.N` rows before timing so each operation deletes a unique row.

## Published Reference Run

Environment:

- Date: 2026-06-24
- CPU: Apple M4 Pro
- OS/arch: macOS 26.5.1, darwin/arm64
- Go: go1.27rc1
- MySQL: 8.0.45, local `localhost`
- PostgreSQL: 18.4 Homebrew, local `localhost`
- SQLite: in-memory with `mattn/go-sqlite3`

The tables report the median from 10 runs.

### SQLite

| Scenario | Oro | GORM | XORM | Bun |
| --- | ---: | ---: | ---: | ---: |
| Create | 9.17 µs/op | 9.44 µs/op | 6.15 µs/op | 8.15 µs/op |
| CreateMany100 | 119.82 µs/op | 195.59 µs/op | 206.18 µs/op | 173.10 µs/op |
| FirstByCode | 5.02 µs/op | 7.31 µs/op | 11.07 µs/op | 6.13 µs/op |
| WhereList | 24.32 µs/op | 34.13 µs/op | 56.75 µs/op | 28.58 µs/op |
| UpdateByCode | 3.97 µs/op | 5.72 µs/op | 4.16 µs/op | 3.31 µs/op |
| DeleteByCode | 2.84 µs/op | 6.84 µs/op | 8.06 µs/op | 4.62 µs/op |

### MySQL

| Scenario | Oro | GORM | XORM | Bun |
| --- | ---: | ---: | ---: | ---: |
| Create | 134.44 µs/op | 127.72 µs/op | 128.43 µs/op | 102.39 µs/op |
| CreateMany100 | 634.42 µs/op | 868.68 µs/op | 861.11 µs/op | 813.34 µs/op |
| FirstByCode | 38.50 µs/op | 73.49 µs/op | 79.83 µs/op | 41.75 µs/op |
| WhereList | 60.96 µs/op | 103.45 µs/op | 123.31 µs/op | 67.11 µs/op |
| UpdateByCode | 100.19 µs/op | 131.75 µs/op | 118.71 µs/op | 108.04 µs/op |
| DeleteByCode | 113.57 µs/op | 160.96 µs/op | 148.68 µs/op | 121.98 µs/op |

### PostgreSQL

| Scenario | Oro | GORM | XORM | Bun |
| --- | ---: | ---: | ---: | ---: |
| Create | 61.56 µs/op | 62.71 µs/op | 62.65 µs/op | 113.80 µs/op |
| CreateMany100 | 755.59 µs/op | 756.51 µs/op | 1.28 ms/op | 1.68 ms/op |
| FirstByCode | 38.72 µs/op | 83.25 µs/op | 46.55 µs/op | 113.36 µs/op |
| WhereList | 107.95 µs/op | 122.95 µs/op | 138.04 µs/op | 101.86 µs/op |
| UpdateByCode | 57.62 µs/op | 104.80 µs/op | 58.54 µs/op | 62.78 µs/op |
| DeleteByCode | 52.12 µs/op | 65.31 µs/op | 64.87 µs/op | 70.55 µs/op |

## Notes

- Benchmarks are useful for relative direction, not fixed production guarantees.
- SQLite driver choice has a large impact; published SQLite results use `mattn/go-sqlite3` for every library.
- MySQL and PostgreSQL runs use the local `duxorm` database by default and reset `products` / `oro_schema` for each sub-benchmark.
- Oro and GORM both run with skipped default write transactions for fair high-throughput comparison.
