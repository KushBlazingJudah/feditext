package feditext

import (
	"context"
	"encoding/hex"
	"log"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/KushBlazingJudah/feditext/captcha"
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/crypto"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/fedi"
	"github.com/KushBlazingJudah/feditext/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

var DB database.Database

func Startup() {
	rand.Seed(time.Now().UnixMicro())

	log.Printf("Starting version %s", config.Version)

	// Assumes database is already loaded at DB

	routes.DB = DB
	captcha.DB = DB
	fedi.DB = DB

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

	// Setup Fedi proxy
	var err error
	fedi.Proxy, err = fedi.NewProxy(config.ProxyUrl)
	if err != nil {
		panic(err)
	}
}

func Close() {
	if err := DB.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
}

func logger(c *fiber.Ctx) error {
	now := time.Now()

	defer func() error { // Panic catcher
		if err := recover(); err != nil {
			// Something *extremely* bad happened.
			log.Printf("%s %s %s %s @@PANIC@@ %s", time.Since(now).Round(time.Millisecond).String(), c.IP(), c.Method(), c.Path(), err)
			debug.PrintStack()
			return c.Status(500).SendString("An internal server error has occured.")
		}
		return nil
	}()

	if err := c.Next(); err != nil {
		log.Printf("%s %s %s %s @@ERROR@@ %s", time.Since(now).Round(time.Millisecond).String(), c.IP(), c.Method(), c.Path(), err)
	} else {
		log.Printf("%s %s %s %s", time.Since(now).Round(time.Millisecond).String(), c.IP(), c.Method(), c.Path())
	}
	return nil
}

// Doesn't leave IPs.
func loggerPrivate(c *fiber.Ctx) error {
	now := time.Now()

	defer func() error { // Panic catcher
		if err := recover(); err != nil {
			// Something *extremely* bad happened.
			log.Printf("%s %s %s @@PANIC@@ %s", time.Since(now).Round(time.Millisecond).String(), c.Method(), c.Path(), err)
			debug.PrintStack()
			return c.Status(500).SendString("An internal server error has occured.")
		}
		return nil
	}()

	if err := c.Next(); err != nil {
		log.Printf("%s %s %s @@ERROR@@ %s", time.Since(now).Round(time.Millisecond).String(), c.Method(), c.Path(), err)
	} else {
		log.Printf("%s %s %s", time.Since(now).Round(time.Millisecond).String(), c.Method(), c.Path())
	}

	return nil
}

func Serve() {
	app := fiber.New(fiber.Config{
		AppName:               "feditext",
		DisableDefaultDate:    true,
		DisableStartupMessage: true,

		Views:             routes.Tmpl,
		ViewsLayout:       "layouts/main",
		PassLocalsToViews: true,

		// (Hopefully) sane defaults
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,

		ServerHeader: "feditext/" + config.Version,
	})

	// Logger middleware
	if config.Private {
		app.Use(loggerPrivate)
	} else {
		app.Use(logger)
	}

	app.Static("/", "./static")

	if config.Pprof {
		app.Use(pprofNew())
	}

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

	// Set the theme local to the theme cookie
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("theme", c.Cookies("theme", "default"))
		return c.Next()
	})

	// TODO: A lot of this needs to be sorted out and changed around.
	// I had no idea how to design the HTTP Form API for this, so it just kinda
	// happened I guess.

	app.Get("/", routes.GetIndex)
	app.Get("/captcha/:id", routes.GetCaptcha)
	if config.PublicAudit {
		app.Get("/audit", routes.GetAudit)
	}
	if !config.Private {
		app.Get("/banned", routes.GetBanned)
	}
	app.Get("/rules", routes.GetRules)
	app.Get("/faq", routes.GetFAQ)

	app.Get("/.well-known/webfinger", routes.Webfinger)

	// Admin
	app.Get("/admin", routes.GetAdmin)
	if !config.Private {
		app.Get("/admin/ban/:ip", routes.GetAdminBan)
		app.Post("/admin/ban/:ip", routes.PostAdminBan)
	}
	app.Get("/admin/login", routes.GetAdminLogin)
	app.Get("/admin/resolve/:report", routes.GetAdminResolve)
	app.Post("/admin/news", routes.PostAdminNews)
	app.Get("/admin/news/delete/:news", routes.GetAdminNewsDelete)
	app.Post("/admin/moderator", routes.PostModerator)
	app.Get("/admin/moderator/delete/:name", routes.GetModeratorDel)
	app.Post("/admin/login", routes.PostAdminLogin)
	app.Post("/admin/board", routes.PostBoard)
	app.Get("/admin/follow", routes.GetAdminFollow)
	app.Get("/admin/unfollow", routes.GetAdminUnfollow)
	app.Get("/admin/fetch", routes.GetAdminFetch)
	app.Get("/admin/resend", routes.GetAdminResend)
	app.Post("/admin/regexps", routes.PostRegexp)
	app.Get("/admin/regexps/delete/:id", routes.GetRegexpDelete)

	// Boards
	app.Get("/:board", routes.GetBoardIndex)
	app.Post("/:board", routes.PostBoardIndex)

	// ActivityPub stuff
	app.Get("/:board/outbox", routes.GetBoardOutbox)
	app.Post("/:board/inbox", routes.PostBoardInbox)
	app.Get("/:board/followers", routes.GetBoardFollowers)
	app.Get("/:board/following", routes.GetBoardFollowing)

	app.Get("/:board/catalog", routes.GetBoardCatalog)
	app.Get("/:board/delete/:post", routes.GetPostDelete)
	app.Get("/:board/report/:post", routes.GetBoardReport)
	app.Post("/:board/report/:post", routes.PostBoardReport)

	app.Get("/:board/:thread/delete", routes.GetThreadDelete)

	app.Get("/:board/:thread", routes.GetBoardThread)
	app.Post("/:board/:thread", routes.PostBoardThread)

	app.Listen(config.ListenAddress)
}
