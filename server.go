package feditext

import (
	"log"
	"os"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
)

var db database.Database

func Startup() {
	var err error

	log.Printf("Starting version %s", config.Version)

	if config.DatabaseEngine == "" {
		// Preallocate array
		dbs := make([]string, 0, len(database.Engines))

		for k := range database.Engines {
			dbs = append(dbs, k)
		}

		log.Printf("No database engine configured.")
		log.Printf("Available engines: %s", strings.Join(dbs, ","))

		os.Exit(1)
	}

	db, err = database.Engines[config.DatabaseEngine](config.DatabaseArg)
	if err != nil {
		panic(err)
	}
}

func Close() {
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
}

func Serve() {
	panic("not implemented")
}
