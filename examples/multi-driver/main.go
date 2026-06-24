package main

import (
	"log"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/mysql"
	"github.com/duxweb/oro/driver/pgsql"
	"github.com/duxweb/oro/driver/sqlite"
)

func main() {
	db, err := oro.Open(oro.Config{
		Default: "sqlite",
		Connections: map[string]oro.ConnectionConfig{
			"sqlite": {Driver: sqlite.Open("app.db")},
			"mysql":  {Driver: mysql.Open("root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=false")},
			"pgsql":  {Driver: pgsql.Open("postgres://root@localhost:5432/duxorm?sslmode=disable")},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = db
}
