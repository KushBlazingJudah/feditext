package feditext

import (
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
)

var DB database.Database

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

	DB, err = database.Engines[config.DatabaseEngine](config.DatabaseArg)
	if err != nil {
		panic(err)
	}

	routes.DB = DB
}

func Close() {
	if err := DB.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
}

func Serve() {
	tmpl := html.New("./views", ".html")

	tmpl.AddFunc("unescape", func(s string) template.HTML {
		return template.HTML(s)
	})

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		AppName:               "feditext",
		ServerHeader:          "feditext/" + config.Version,

		Views:       tmpl,
		ViewsLayout: "layouts/main",
		// TODO: Timeouts
	})

	app.Static("/", "./static")

	app.Get("/", routes.GetIndex)

	// Admin
	app.Get("/admin", routes.GetAdmin)
	app.Post("/admin/board", routes.PostBoard)

	// Boards
	app.Get("/:board", routes.GetBoardIndex)
	app.Post("/:board", routes.PostBoardIndex)

	app.Get("/:board/:thread", routes.GetBoardThread)
	app.Post("/:board/:thread", routes.PostBoardThread)

	app.Listen(config.ListenAddress)
}
