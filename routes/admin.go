package routes

// TODO: EVERY ROUTE IN HERE NEEDS AUTHENTICATION AND BADLY

import (
	"strconv"
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

	reports, err := DB.Reports(c.Context(), false)
	if err != nil {
		return err
	}

	news, err := DB.News(c.Context())
	if err != nil {
		return err
	}

	mods, err := DB.Moderators(c.Context())
	if err != nil {
		return err
	}

	return render(c, "Admin Area", "admin", fiber.Map{
		"reports": reports,
		"news":    news,
		"mods":    mods,
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

	return render(c, "Login", "admin_login", fiber.Map{})
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

func GetAdminResolve(c *fiber.Ctx) error {
	// Need privileges
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return c.Redirect("/admin/login")
	}

	rid, err := strconv.Atoi(c.Params("report"))
	if err != nil {
		return err
	}

	// This fails silently if its a bad report
	if err := DB.Resolve(c.Context(), rid); err != nil {
		return err
	}

	// Redirect back to the admin panel
	return c.Redirect("/admin")
}

func PostAdminNews(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return c.SendStatus(401)
	}

	subject := c.FormValue("subject", "Untitled")
	content := c.FormValue("content")

	if content == "" {
		return database.ErrPostContents
	}

	if err := DB.SaveNews(c.Context(), &database.News{
		Author:  c.Locals("username").(string),
		Subject: subject,
		Content: content,
	}); err != nil {
		return err
	}

	return c.Redirect("/admin")
}

func GetAdminNewsDelete(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return c.SendStatus(401)
	}

	nid, err := strconv.Atoi(c.Params("news"))
	if err != nil {
		return err
	}

	if err := DB.DeleteNews(c.Context(), nid); err != nil {
		return err
	}

	return c.Redirect("/admin")
}

func PostModerator(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return c.SendStatus(401)
	}

	username := c.FormValue("username")
	password := c.FormValue("password")
	priv := c.FormValue("priv")

	if username == "" {
		// TODO: Error page
		return c.SendStatus(400)
	} else if password == "" {
		// TODO: Error page
		return c.SendStatus(400)
	} else if priv == "" {
		priv = "0" // assume janitor
	}

	ipriv, err := strconv.Atoi(priv)
	if err != nil {
		return err
	}

	// TODO: Sanitize

	if err := DB.SaveModerator(c.Context(), username, password, database.ModType(ipriv)); err != nil {
		return err
	}

	return c.Redirect("/admin")
}

func GetModeratorDel(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return c.SendStatus(401)
	}

	username := c.Params("name")
	if username == "" {
		// TODO: Error page
		return c.SendStatus(400)
	}

	// TODO: Sanitize

	if err := DB.DeleteModerator(c.Context(), username); err != nil {
		return err
	}

	return c.Redirect("/admin")
}
