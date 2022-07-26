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
			log.Printf("%s %s %s %d %s @@PANIC@@ %s", time.Since(now).Round(time.Millisecond).String(), c.IP(), c.Method(), c.Response().StatusCode(), c.Path(), err)
			debug.PrintStack()
			return c.Status(500).SendString("An internal server error has occured.")
		}
		return nil
	}()

	if err := c.Next(); err != nil {
		log.Printf("%s %s %s %d %s @@ERROR@@ %s", time.Since(now).Round(time.Millisecond).String(), c.IP(), c.Method(), c.Response().StatusCode(), c.Path(), err)
	} else {
		log.Printf("%s %s %s %d %s", time.Since(now).Round(time.Millisecond).String(), c.IP(), c.Method(), c.Response().StatusCode(), c.Path())
	}
	return nil
}

// Doesn't leave IPs.
func loggerPrivate(c *fiber.Ctx) error {
	now := time.Now()

	defer func() error { // Panic catcher
		if err := recover(); err != nil {
			// Something *extremely* bad happened.
			log.Printf("%s %s %d %s @@PANIC@@ %s", time.Since(now).Round(time.Millisecond).String(), c.Method(), c.Response().StatusCode(), c.Path(), err)
			debug.PrintStack()
			return c.Status(500).SendString("An internal server error has occured.")
		}
		return nil
	}()

	if err := c.Next(); err != nil {
		log.Printf("%s %s %d %s @@ERROR@@ %s", time.Since(now).Round(time.Millisecond).String(), c.Method(), c.Response().StatusCode(), c.Path(), err)
	} else {
		log.Printf("%s %s %d %s", time.Since(now).Round(time.Millisecond).String(), c.Method(), c.Response().StatusCode(), c.Path())
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

	// Ignore Tor2Web if configured to (by default, yes)
	// If anyone cares (nobody does), I don't like Tor2Web at all.
	// I would go into detail but eh.
	if config.NoT2W {
		app.Use(func(c *fiber.Ctx) error {
			if c.Get("X-tor2web") != "" {
				return c.Status(403).SendString("Tor2Web proxies are not allowed to access this server. Please use the Tor Browser, available for free at https://www.torproject.org/download/.")
			}
			return c.Next()
		})
	}

	// XXX: If for whatever reason you decide to implement Google Analytics
	// (your changes won't be upstreamed), remove this handler.
	// This exists solely to disallow bad Tor2Web proxies that hide the fact
	// that they are tor2web yet send along their GA cookie.
	// This is another one of the reasons why I hate them.
	// Nobody should use them.
	app.Use(func(c *fiber.Ctx) error {
		if c.Cookies("_ga") != "" {
			return c.Status(403).SendString("Feditext doesn't have Google Analytics enabled yet you sent a cookie belonging to them along with your request.\nIf you're accessing this from a tor2web proxy, there's a reason for you to stop now.\nPlease use the Tor Browser, available for free at https://www.torproject.org/download/.")
		}
		return c.Next()
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

	app.Get("/robots.txt", func(c *fiber.Ctx) error {
		// HACK: Hopefully stops robots crawling /board/report links.
		// My instance gets hit with these every day, hopefully not intentionally.
		// (It didn't)
		return c.SendString(`User-agent: *
Disallow: /
`)
	})

	app.Get("/captcha/:id", routes.GetCaptchaID)
	app.Get("/captcha", routes.GetCaptcha)
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
	app.Get("/admin/delete", routes.GetDelete)
	app.Post("/admin/regexps", routes.PostRegexp)
	app.Get("/admin/regexps/delete/:id", routes.GetRegexpDelete)
	app.Get("/admin/:board", routes.GetAdminBoard)

	app.Post("/post", routes.Post)

	app.Get("/:board", routes.GetBoardIndex)

	// ActivityPub stuff
	app.Get("/:board/outbox", routes.GetBoardOutbox)
	app.Post("/:board/inbox", routes.PostBoardInbox)
	app.Get("/:board/followers", routes.GetBoardFollowers)
	app.Get("/:board/following", routes.GetBoardFollowing)

	app.Get("/:board/catalog", routes.GetBoardCatalog)
	app.Get("/:board/report", routes.GetBoardReport)
	app.Post("/:board/report", routes.PostBoardReport)

	app.Get("/:board/:thread", routes.GetBoardThread)

	app.Listen(config.ListenAddress)
}
