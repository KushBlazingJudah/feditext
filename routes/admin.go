package routes

import (
	"fmt"
	"strconv"
	"time"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/util"
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
		return errResp(c, "Unauthorized", 403)
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
		return errResp(c, "Unauthorized", 403)
	}

	board := database.Board{}
	if err := c.BodyParser(&board); err != nil {
		return err
	}

	if board.ID == "" {
		return errResp(c, "No ID was specified in your request.", 400, "/admin")
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

	user := c.FormValue("username")[:32]

	// Usernames are alphanumeric
	if !util.IsAlnum(user) {
		return errResp(c, "Invalid credentials.", 403, "/admin/login")
	}

	pass := c.FormValue("password")[:64]

	if ok, err := DB.PasswordCheck(c.Context(), user, pass); err != nil {
		return err
	} else if !ok {
		return errResp(c, "Invalid credentials.", 403, "/admin/login")
	} else if ok {
		priv, err := DB.Privilege(c.Context(), user)
		if err != nil {
			return err
		}

		// Generate a token
		exp := time.Now().UTC().Add((time.Hour * 24) * 7)
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
		return errResp(c, "Invalid post number.", 400, "/admin")
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
		return errResp(c, "Unauthorized", 403, "/admin")
	}

	subject := c.FormValue("subject", "Untitled")
	content := c.FormValue("content")

	if content == "" {
		return errResp(c, "Invalid post contents.", 400, "/admin")
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
		return errResp(c, "Unauthorized", 403, "/admin")
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

	username := c.FormValue("username")[:32]
	password := c.FormValue("password")[:64]
	priv := c.FormValue("priv")

	if username == "" {
		return errResp(c, "Need a username", 400, "/admin")
	} else if !util.IsAlnum(username) {
		return errResp(c, "Username is not alphanumeric", 400, "/admin")
	} else if password == "" {
		return errResp(c, "Need a password", 400, "/admin")
	} else if priv == "" {
		priv = "0" // assume janitor
	}

	ipriv, err := strconv.Atoi(priv)
	if err != nil {
		return err
	}

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

func GetAdminBan(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errResp(c, "Unauthorized", 403, "/admin")
	}

	return render(c, "Ban User", "ban", fiber.Map{"ip": c.Params("ip")})
}

func PostAdminBan(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errResp(c, "Unauthorized", 403, "/admin")
	}

	source := c.Params("ip")
	if source == "" {
		return errResp(c, "Specify an IP to ban", 400, "/admin")
	}

	reason := c.FormValue("reason")
	if reason == "" {
		reason = "Arbitrary."
	}

	exp := c.FormValue("expires")
	exptime, err := time.Parse("2006-01-02T15:04", exp)
	if err != nil {
		return errResp(c, fmt.Sprintf("Invalid time: %s", err), 400, "/admin")
	}

	if err := DB.Ban(c.Context(), database.Ban{
		Target:  source,
		Reason:  reason,
		Expires: exptime,
	}, c.Locals("username").(string)); err != nil {
		return err
	}

	return c.Redirect("/admin")
}
