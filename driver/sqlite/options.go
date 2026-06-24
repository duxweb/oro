package sqlite

import "database/sql"

type Option func(*driverConfig)

type driverConfig struct {
	dsn              string
	db               *sql.DB
	owned            bool
	disableReturning bool
}

func OwnsDB(owned bool) Option {
	return func(config *driverConfig) {
		config.owned = owned
	}
}

func DisableReturning() Option {
	return func(config *driverConfig) {
		config.disableReturning = true
	}
}
