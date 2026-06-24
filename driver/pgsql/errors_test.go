package pgsql

import (
	"errors"
	"testing"

	"github.com/duxweb/oro"
)

func TestTranslateDuplicateError(t *testing.T) {
	err := translateError(pgError{Code: "23505"})
	if !errors.Is(err, oro.ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestTranslateSerializationError(t *testing.T) {
	err := translateError(pgError{Code: "40001"})
	if !errors.Is(err, oro.ErrSerializationFailure) {
		t.Fatalf("expected serialization error, got %v", err)
	}
}

type pgError struct {
	Code string
}

func (err pgError) Error() string {
	return "postgres error"
}
