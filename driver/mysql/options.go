package mysql

import "database/sql"

type Option func(*driverConfig)

type driverConfig struct {
	driverName string
	dsn        string
	db         *sql.DB
	owned      bool
}

func DriverName(name string) Option {
	return func(config *driverConfig) {
		if name != "" {
			config.driverName = name
		}
	}
}

func OwnsDB(owned bool) Option {
	return func(config *driverConfig) {
		config.owned = owned
	}
}
