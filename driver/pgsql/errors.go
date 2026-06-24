package pgsql

import (
	"errors"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/internal/drivererr"
)

func translateError(err error) error {
	if err == nil {
		return nil
	}

	kind := classifyError(err)
	if ormErr := new(oro.Error); errors.As(err, &ormErr) {
		if kind == nil {
			return err
		}
		translated := *ormErr
		translated.Kind = kind
		return &translated
	}
	if kind == nil {
		kind = err
	}
	return &oro.Error{Op: "pgsql", Kind: kind, Cause: err}
}

func classifyError(err error) error {
	code, ok := drivererr.StringField(err, "Code")
	if !ok {
		code, ok = drivererr.StringMethod(err, "SQLState")
	}
	if !ok {
		return nil
	}
	switch code {
	case "23505":
		return oro.ErrConflict
	case "23502", "23503", "23514":
		return oro.ErrConstraint
	case "40P01":
		return oro.ErrDeadlock
	case "40001":
		return oro.ErrSerializationFailure
	default:
		if len(code) >= 2 && code[:2] == "23" {
			return oro.ErrConstraint
		}
		return nil
	}
}
