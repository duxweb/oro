package mysql_test

import (
	"os"
	"testing"

	"github.com/duxweb/oro/driver/internal/integrationtest"
	"github.com/duxweb/oro/driver/mysql"
)

func TestMySQLIntegrationEntry(t *testing.T) {
	dsn := os.Getenv("ORO_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=false"
	}

	integrationtest.RunSmoke(t, integrationtest.DriverCase{
		Name:   "mysql",
		Driver: mysql.Open(dsn),
	})
}
