package mysql

import (
	"errors"
	"testing"

	"github.com/duxweb/oro"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestTranslateDuplicateError(t *testing.T) {
	err := translateError(&mysqlDriver.MySQLError{Number: 1062, Message: "duplicate"})
	if !errors.Is(err, oro.ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestTranslateConstraintError(t *testing.T) {
	err := translateError(&mysqlDriver.MySQLError{Number: 1452, Message: "foreign key"})
	if !errors.Is(err, oro.ErrConstraint) {
		t.Fatalf("expected constraint error, got %v", err)
	}
}
