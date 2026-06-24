package sqlite_test

import (
	"path/filepath"
	"testing"

	"github.com/duxweb/oro/driver/internal/integrationtest"
	"github.com/duxweb/oro/driver/sqlite"
)

func TestSQLiteDriverMatrix(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "oro_matrix.db")
	integrationtest.RunMatrix(t, integrationtest.DriverCase{
		Name:   "sqlite",
		Driver: sqlite.Open(dsn),
	})
}
