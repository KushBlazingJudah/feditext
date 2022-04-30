package routes

// TODO: EVERY ROUTE IN HERE NEEDS AUTHENTICATION AND BADLY

import (
	"time"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

func hasPriv(c *fiber.Ctx, p database.ModType) bool {
	priv, ok := c.Locals("privs").(database.ModType)
	if !ok {
		return false
	}

	return priv >= p
}

func GetAdmin(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return c.SendStatus(401)
	}

	boards, err := DB.Boards(c.Context())
	if err != nil {
		return err
	}

	return c.Render("admin", fiber.Map{
		"title":  "Admin View | " + config.Title,
		"boards": boards,
	})
}

func PostBoard(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return c.SendStatus(401)
	}

	board := database.Board{}
	if err := c.BodyParser(&board); err != nil {
		return err
	}

	if board.ID == "" {
		return c.SendString("no id")
	}

	if err := DB.SaveBoard(c.Context(), board); err != nil {
		return err
	}

	return c.Redirect("/admin")
}

func GetAdminLogin(c *fiber.Ctx) error {
	// Skip login
	ok := hasPriv(c, database.ModTypeMod)
	if ok {
		return c.Redirect("/admin")
	}

	return c.Render("admin_login", fiber.Map{
		"title": "Login | " + config.Title,
	})
}

func PostAdminLogin(c *fiber.Ctx) error {
	// Skip login
	ok := hasPriv(c, database.ModTypeMod)
	if ok {
		return c.Redirect("/admin")
	}

	user := c.FormValue("username")
	pass := c.FormValue("password")

	if ok, err := DB.PasswordCheck(c.Context(), user, pass); err != nil {
		return err
	} else if !ok {
		return c.SendString("invalid credentials")
	} else if ok {
		priv, err := DB.Privilege(c.Context(), user)
		if err != nil {
			return err
		}

		// Generate a token
		exp := time.Now().Add((time.Hour * 24) * 7)
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": user,
			"priv":     int(priv),
			"exp":      exp.Unix(), // one week
		}).SignedString(config.JWTSecret)
		if err != nil {
			return err
		}

		// Set it as a cookie
		c.Cookie(&fiber.Cookie{
			Name:     "token",
			Value:    token,
			Expires:  exp,
			Secure:   true,
			SameSite: "Strict",
			HTTPOnly: true,
		})

	}

	// Redirect to admin page; this will kick them back to login or will work fine
	return c.Redirect("/admin")
}
