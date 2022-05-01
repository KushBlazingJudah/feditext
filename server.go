package feditext

import (
	"context"
	"encoding/hex"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/KushBlazingJudah/feditext/captcha"
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/crypto"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

var DB database.Database

func Startup() {
	rand.Seed(time.Now().UnixMicro())

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
	captcha.DB = DB

	// Set a random admin password
	if config.RandAdmin {
		buf := make([]byte, 16)
		rand.Read(buf)

		pass := hex.EncodeToString(buf)
		if err := DB.SaveModerator(context.Background(), "admin", pass, database.ModTypeAdmin); err != nil {
			log.Printf("while setting new admin password: %v", err)
		} else {
			log.Printf("admin password: %s", pass)
		}
	}
}

func Close() {
	if err := DB.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
}

func Serve() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		PassLocalsToViews:     true,
		AppName:               "feditext",
		ServerHeader:          "feditext/" + config.Version,

		Views:       routes.Tmpl,
		ViewsLayout: "layouts/main",
		// TODO: Timeouts
	})

	app.Static("/", "./static")

	// Authentication middleware
	app.Use(func(c *fiber.Ctx) error {
		rawToken := c.Cookies("token")
		if rawToken == "" {
			// No token
			return c.Next()
		}

		t, err := jwt.Parse(rawToken, crypto.JwtKeyfunc)
		if err == nil && t.Valid {
			// Token is valid, so throw it in
			claims := t.Claims.(jwt.MapClaims)
			username := claims["username"].(string)
			priv := claims["priv"].(float64)

			c.Locals("username", username)
			c.Locals("privs", database.ModType(priv))
		} else {
			log.Printf("failed to authenticate token from %s: %v", c.IP(), err)
			c.ClearCookie("token")
		}

		// Fail silently otherwise

		return c.Next()
	})

	app.Get("/", routes.GetIndex)
	app.Get("/audit", routes.GetAudit)
	app.Get("/banned", routes.GetBanned)
	app.Get("/captcha/:id", routes.GetCaptcha)

	// Admin
	app.Get("/admin", routes.GetAdmin)
	app.Get("/admin/ban/:ip", routes.GetAdminBan)
	app.Post("/admin/ban/:ip", routes.PostAdminBan)
	app.Get("/admin/login", routes.GetAdminLogin)
	app.Get("/admin/resolve/:report", routes.GetAdminResolve)
	app.Post("/admin/news", routes.PostAdminNews)
	app.Get("/admin/news/delete/:news", routes.GetAdminNewsDelete)
	app.Post("/admin/moderator", routes.PostModerator)
	app.Get("/admin/moderator/delete/:name", routes.GetModeratorDel)
	app.Post("/admin/login", routes.PostAdminLogin)
	app.Post("/admin/board", routes.PostBoard)

	// Boards
	app.Get("/:board", routes.GetBoardIndex)
	app.Post("/:board", routes.PostBoardIndex)

	app.Get("/:board/delete/:post", routes.GetPostDelete)
	app.Get("/:board/report/:post", routes.GetBoardReport)
	app.Post("/:board/report/:post", routes.PostBoardReport)

	app.Get("/:board/:thread/delete", routes.GetThreadDelete)

	app.Get("/:board/:thread", routes.GetBoardThread)
	app.Post("/:board/:thread", routes.PostBoardThread)

	app.Listen(config.ListenAddress)
}
