package pgsql

import "database/sql"

type Option func(*driverConfig)

type driverConfig struct {
	dsn   string
	db    *sql.DB
	owned bool
}

func OwnsDB(owned bool) Option {
	return func(config *driverConfig) {
		config.owned = owned
	}
}
