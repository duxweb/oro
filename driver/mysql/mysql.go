package mysql

import (
	"context"
	"database/sql"

	"github.com/duxweb/oro"
	_ "github.com/go-sql-driver/mysql"
)

type driver struct {
	config driverConfig
}

func Open(dsn string, options ...Option) oro.Driver {
	config := driverConfig{dsn: dsn, owned: true}
	for _, option := range options {
		option(&config)
	}
	return driver{config: config}
}

func Wrap(db *sql.DB, options ...Option) oro.Driver {
	config := driverConfig{db: db, owned: false}
	for _, option := range options {
		option(&config)
	}
	return driver{config: config}
}

func (driver driver) Name() string {
	return "mysql"
}

func (driver driver) Open(ctx context.Context) (*sql.DB, error) {
	if driver.config.db != nil {
		return driver.config.db, nil
	}
	return sql.Open("mysql", driver.config.dsn)
}

func (driver driver) Dialect() oro.Dialect {
	return dialect{}
}

func (driver driver) Inspector(db *sql.DB) oro.Inspector {
	return inspector{db: db}
}

func (driver driver) TranslateError(err error) error {
	return translateError(err)
}

func (driver driver) Owned() bool {
	return driver.config.owned
}
