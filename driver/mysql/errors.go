package mysql

import (
	"errors"

	"github.com/duxweb/oro"
	mysqlDriver "github.com/go-sql-driver/mysql"
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
	return &oro.Error{Op: "mysql", Kind: kind, Cause: err}
}

func classifyError(err error) error {
	var mysqlErr *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlErr) {
		return nil
	}

	switch mysqlErr.Number {
	case 1062:
		return oro.ErrConflict
	case 1048, 1216, 1217, 1364, 1406, 1451, 1452, 3819:
		return oro.ErrConstraint
	case 1205, 1213:
		return oro.ErrDeadlock
	default:
		return nil
	}
}
