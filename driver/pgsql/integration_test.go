package pgsql_test

import (
	"os"
	"testing"

	"github.com/duxweb/oro/driver/internal/integrationtest"
	"github.com/duxweb/oro/driver/pgsql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgreSQLIntegrationEntry(t *testing.T) {
	dsn := os.Getenv("ORO_PGSQL_DSN")
	if dsn == "" {
		dsn = "postgres://root@localhost:5432/duxorm?sslmode=disable"
	}

	integrationtest.RunSmoke(t, integrationtest.DriverCase{
		Name:   "pgsql",
		Driver: pgsql.Open(dsn),
	})
}
