module github.com/duxweb/oro/benchmarks/ormbench

go 1.27rc1

require (
	github.com/duxweb/oro v0.0.0
	github.com/uptrace/bun v1.2.16
	github.com/uptrace/bun/dialect/sqlitedialect v1.2.16
	github.com/uptrace/bun/driver/sqliteshim v1.2.16
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
	xorm.io/xorm v1.3.11
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tmthrgd/go-hex v0.0.0-20190904060850-447a3041c3bc // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.53.0 // indirect
	xorm.io/builder v0.3.13 // indirect
)

replace github.com/duxweb/oro => ../..
