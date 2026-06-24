package pgsql

import (
	"errors"
	"testing"

	"github.com/duxweb/oro"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestTranslateDuplicateError(t *testing.T) {
	err := translateError(&pgconn.PgError{Code: "23505", Message: "duplicate"})
	if !errors.Is(err, oro.ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestTranslateSerializationError(t *testing.T) {
	err := translateError(&pgconn.PgError{Code: "40001", Message: "serialization"})
	if !errors.Is(err, oro.ErrSerializationFailure) {
		t.Fatalf("expected serialization error, got %v", err)
	}
}
