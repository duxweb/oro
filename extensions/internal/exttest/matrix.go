package exttest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/mysql"
	"github.com/duxweb/oro/driver/pgsql"
	"github.com/duxweb/oro/driver/sqlite"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type DriverCase struct {
	Name   string
	Driver oro.Driver
}

func DriverCases() []DriverCase {
	mysqlDSN := os.Getenv("ORO_MYSQL_DSN")
	if mysqlDSN == "" {
		mysqlDSN = "root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=false"
	}
	pgsqlDSN := os.Getenv("ORO_PGSQL_DSN")
	if pgsqlDSN == "" {
		pgsqlDSN = "postgres://root@localhost:5432/duxorm?sslmode=disable"
	}
	return []DriverCase{
		{Name: "sqlite", Driver: sqlite.Open(":memory:")},
		{Name: "mysql", Driver: mysql.Open(mysqlDSN)},
		{Name: "pgsql", Driver: pgsql.Open(pgsqlDSN)},
	}
}

type OpenOptions struct {
	Models     []oro.Definer
	Extensions []oro.Extension
	Tables     []string
	Prefix     string
}

func Open(t *testing.T, testCase DriverCase, options OpenOptions) (*oro.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: testCase.Driver},
		},
		TablePrefix: options.Prefix,
		Pool: oro.PoolConfig{
			MaxOpenConns: 4,
			MaxIdleConns: 2,
			PingOnOpen:   true,
		},
		Timeout: oro.TimeoutConfig{
			Connect: 3 * time.Second,
			Query:   10 * time.Second,
		},
		Extensions: options.Extensions,
	})
	if err != nil {
		if Unavailable(err) {
			t.Skipf("%s database unavailable: %v", testCase.Name, err)
		}
		t.Fatal(err)
	}
	reset(t, ctx, db, options.Prefix, options.Tables...)
	if len(options.Models) > 0 {
		if err := db.Register(options.Models...); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		reset(t, ctx, db, options.Prefix, options.Tables...)
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	return db, ctx
}

func reset(t *testing.T, ctx context.Context, db *oro.DB, prefix string, tables ...string) {
	t.Helper()
	for _, table := range tables {
		_, err := db.Raw("drop table if exists " + prefix + table).Exec(ctx)
		if err != nil && !Unavailable(err) {
			t.Fatal(err)
		}
	}
	_, err := db.Raw("drop table if exists " + prefix + "oro_schema").Exec(ctx)
	if err != nil && !Unavailable(err) {
		t.Fatal(err)
	}
}

func Unavailable(err error) bool {
	if err == nil {
		return false
	}
	var ormErr *oro.Error
	if errors.As(err, &ormErr) && ormErr.Cause != nil {
		err = ormErr.Cause
	}
	message := strings.ToLower(fmt.Sprint(err))
	unavailableParts := []string{
		"connection refused",
		"connect: connection refused",
		"no such host",
		"connection reset",
		"server closed",
		"timeout",
		"deadline exceeded",
		"access denied",
		"authentication failed",
		"password authentication failed",
		"unknown database",
		"database \"duxorm\" does not exist",
		"role \"root\" does not exist",
	}
	for _, part := range unavailableParts {
		if strings.Contains(message, part) {
			return true
		}
	}
	return false
}
