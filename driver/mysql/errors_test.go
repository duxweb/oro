package mysql

import (
	"errors"
	"testing"

	"github.com/duxweb/oro"
)

func TestTranslateDuplicateError(t *testing.T) {
	err := translateError(mysqlError{Number: 1062})
	if !errors.Is(err, oro.ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestTranslateConstraintError(t *testing.T) {
	err := translateError(mysqlError{Number: 1452})
	if !errors.Is(err, oro.ErrConstraint) {
		t.Fatalf("expected constraint error, got %v", err)
	}
}

type mysqlError struct {
	Number uint16
}

func (err mysqlError) Error() string {
	return "mysql error"
}
