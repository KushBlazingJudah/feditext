package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/fedi"
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

func errpriv(c *fiber.Ctx, exp database.ModType, ret ...string) error {
	retu := ""
	if len(ret) == 1 {
		retu = ret[0]
	}

	var text string
	priv, ok := c.Locals("privs").(database.ModType)
	if !ok {
		text = "You must be authenticated to access this page."
	} else {
		text = fmt.Sprintf("This page requires level %d to access. You are %d.", exp, priv)
	}

	return render(c.Status(403), "Unauthorized", "error", fiber.Map{
		"error":  text,
		"return": retu,
	})
}

func GetAdmin(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	boards, err := DB.Boards(c.Context())
	if err != nil {
		return errhtml(c, err, "/admin")
	}

	reports, err := DB.Reports(c.Context(), false)
	if err != nil {
		return errhtml(c, err, "/admin")
	}

	news, err := DB.News(c.Context())
	if err != nil {
		return errhtml(c, err, "/admin")
	}

	mods, err := DB.Moderators(c.Context())
	if err != nil {
		return errhtml(c, err, "/admin")
	}

	rxps, err := DB.Regexps(c.Context())
	if err != nil {
		return errhtml(c, err, "/admin")
	}

	followers := [][]string{}
	following := [][]string{}

	for _, board := range boards {
		fin, err := DB.Followers(c.Context(), board.ID)
		if err != nil {
			return errhtml(c, err, "/admin")
		}

		fout, err := DB.Following(c.Context(), board.ID)
		if err != nil {
			return errhtml(c, err, "/admin")
		}

		for _, source := range fin {
			followers = append(followers, []string{board.ID, source})
		}

		for _, target := range fout {
			following = append(following, []string{board.ID, target})
		}
	}

	return render(c, "Admin Area", "admin/index", fiber.Map{
		"reports":   reports,
		"news":      news,
		"mods":      mods,
		"regexps":   rxps,
		"followers": followers,
		"following": following,

		"showpicker": true,
	})
}

func GetAdminBoard(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	board, err := DB.Board(c.Context(), c.Params("board"))
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return errhtmlc(c, "That board does not exist.", 404, "/admin")
	} else if err != nil {
		return errhtml(c, err, "/admin")
	}

	reports, err := DB.BoardReports(c.Context(), board.ID, false)
	if err != nil {
		return errhtml(c, fmt.Errorf("error fetching reports: %w", err), "/admin")
	}

	posts, err := DB.RecentPosts(c.Context(), board.ID, 10, false)
	if err != nil {
		return errhtml(c, fmt.Errorf("error fetching recent posts: %w", err), "/admin")
	}

	followers, err := DB.Followers(c.Context(), board.ID)
	if err != nil {
		return errhtml(c, fmt.Errorf("error fetching followers: %w", err), "/admin")
	}

	following, err := DB.Following(c.Context(), board.ID)
	if err != nil {
		return errhtml(c, fmt.Errorf("error fetching following: %w", err), "/admin")
	}

	return render(c, "Admin Area", "admin/board", fiber.Map{
		"reports":   reports,
		"followers": followers,
		"following": following,
		"board":     board,
		"posts":     posts,

		"showpicker": true,
	})
}

func PostBoard(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return errpriv(c, database.ModTypeAdmin, "/")
	}

	board := database.Board{}
	if err := c.BodyParser(&board); err != nil {
		return errhtml(c, err, "/admin")
	}

	if board.ID == "" {
		return errhtmlc(c, "No ID was specified in your request.", 400, "/admin")
	} else if !util.IsAlnum(board.ID) {
		return errhtmlc(c, "The board ID must be alphanumeric.", 400, "/admin")
	}

	if err := DB.SaveBoard(c.Context(), board); err != nil {
		return err
	}

	// Create keys
	if _, err := fedi.CreatePem(board.ID); err != nil {
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

	return render(c, "Login", "admin/login", fiber.Map{})
}

func PostAdminLogin(c *fiber.Ctx) error {
	// Skip login
	ok := hasPriv(c, database.ModTypeMod)
	if ok {
		return c.Redirect("/admin")
	}

	user := util.Trim(c.FormValue("username"), 32)

	// Usernames are alphanumeric
	if !util.IsAlnum(user) {
		return errhtmlc(c, "Invalid credentials.", 403, "/admin/login")
	}

	pass := util.Trim(c.FormValue("password"), 64)

	if ok, err := DB.PasswordCheck(c.Context(), user, pass); err != nil && errors.Is(err, sql.ErrNoRows) {
		return errhtmlc(c, "Invalid credentials.", 403, "/admin/login")
	} else if err != nil {
		return errhtml(c, err, "/admin")
	} else if !ok {
		return errhtmlc(c, "Invalid credentials.", 403, "/admin/login")
	} else if ok {
		priv, err := DB.Privilege(c.Context(), user)
		if err != nil {
			return errhtml(c, err, "/admin")
		}

		log.Printf("Issuing token for %s (priv: %d)", user, priv)

		// Generate a token
		exp := time.Now().UTC().Add((time.Hour * 24) * 7)
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"username": user,
			"priv":     int(priv),
			"exp":      exp.Unix(), // one week
		}).SignedString(config.JWTSecret)
		if err != nil {
			return errhtml(c, err, "/admin")
		}

		// Set it as a cookie
		c.Cookie(&fiber.Cookie{
			Name:     "token",
			Value:    token,
			Expires:  exp,
			Secure:   config.TransportProtocol == "https",
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
		return errpriv(c, database.ModTypeMod, "/")
	}

	rid, err := strconv.Atoi(c.Params("report"))
	if err != nil {
		return errhtmlc(c, "Invalid post number.", 400, "/admin")
	}

	// This fails silently if its a bad report
	if err := DB.Resolve(c.Context(), rid); err != nil {
		return errhtml(c, err, "/admin")
	}

	// Redirect back to the admin panel
	return c.Redirect("/admin")
}

func PostAdminNews(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return errpriv(c, database.ModTypeAdmin, "/")
	}

	subject := c.FormValue("subject", "Untitled")
	content := c.FormValue("content")

	if content == "" {
		return errhtmlc(c, "Invalid post contents.", 400, "/admin")
	}

	if err := DB.SaveNews(c.Context(), &database.News{
		Author:  c.Locals("username").(string),
		Subject: subject,
		Content: content,
	}); err != nil {
		return errhtml(c, err, "/admin")
	}

	return c.Redirect("/admin")
}

func GetAdminNewsDelete(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return errpriv(c, database.ModTypeAdmin, "/")
	}

	nid, err := strconv.Atoi(c.Params("news"))
	if err != nil {
		return errhtml(c, err, "/admin")
	}

	if err := DB.DeleteNews(c.Context(), nid); err != nil {
		return errhtml(c, err, "/admin")
	}

	return c.Redirect("/admin")
}

func PostModerator(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return errpriv(c, database.ModTypeAdmin, "/")
	}

	username := util.Trim(c.FormValue("username"), 32)
	password := util.Trim(c.FormValue("password"), 64)
	priv := c.FormValue("priv")

	if username == "" {
		return errhtmlc(c, "Need a username", 400, "/admin")
	} else if !util.IsAlnum(username) {
		return errhtmlc(c, "Username is not alphanumeric", 400, "/admin")
	} else if password == "" {
		return errhtmlc(c, "Need a password", 400, "/admin")
	} else if priv == "" {
		priv = "0" // assume janitor
	}

	ipriv, err := strconv.Atoi(priv)
	if err != nil {
		return errhtmlc(c, "Privilege number is not a number.", 400, "/admin")
	}

	if err := DB.SaveModerator(c.Context(), username, password, database.ModType(ipriv)); err != nil {
		return errhtml(c, err, "/admin")
	}

	log.Printf("%s created user %s (priv: %d)", c.Locals("username").(string), username, ipriv)

	return c.Redirect("/admin")
}

func GetModeratorDel(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return errpriv(c, database.ModTypeAdmin, "/")
	}

	username := c.Params("name")
	if username == "" {
		return errhtmlc(c, "Need a moderator to delete.", 400, "/admin")
	}

	// TODO: Sanitize

	if err := DB.DeleteModerator(c.Context(), username); err != nil {
		return err
	}

	log.Printf("%s deleted user %s", c.Locals("user").(string), username)

	return c.Redirect("/admin")
}

func GetAdminBan(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	return render(c, "Ban User", "admin/ban", fiber.Map{"ip": c.Params("ip")})
}

func PostAdminBan(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	source := c.Params("ip")
	if source == "" {
		return errhtmlc(c, "Specify an IP to ban", 400, "/admin")
	}

	reason := c.FormValue("reason")
	if reason == "" {
		reason = "Arbitrary."
	}

	exp := c.FormValue("expires")
	exptime, err := time.Parse("2006-01-02T15:04", exp)
	if err != nil {
		return errhtmlc(c, fmt.Sprintf("Invalid time: %s", err), 400, "/admin")
	}

	if err := DB.Ban(c.Context(), database.Ban{
		Target:  source,
		Reason:  reason,
		Expires: exptime,
	}, c.Locals("username").(string)); err != nil {
		return errhtml(c, err, "/admin")
	}

	return c.Redirect("/admin")
}

func GetAdminFollow(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return errpriv(c, database.ModTypeAdmin, "/")
	}

	boardReq := strings.TrimSpace(c.Query("board"))
	targetReq := strings.TrimSpace(c.Query("target"))
	if boardReq == "" || targetReq == "" {
		return errhtmlc(c, "You must specify a board and a target.", 400, "/admin")
	}

	board, err := DB.Board(c.Context(), boardReq)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return errhtmlc(c, "That board does not exist.", 404, "/admin")
	} else if err != nil {
		return errhtml(c, err, "/admin")
	}

	target, err := url.Parse(targetReq)
	if err != nil {
		return errhtmlc(c, "The target link is invalid.", 400, "/admin")
	}

	// Tell them that we want to follow them.
	follow, err := fedi.GenerateFollow(c.Context(), board, target.String())
	if err != nil {
		// This will never happen (for now) but is there to have a consistent
		// schema for Generate functions
		return errhtml(c, err, "/admin")
	}

	log.Printf("Sending follow request to %s for %s", target.String(), board.ID)

	if err := fedi.SendActivity(c.Context(), follow); err != nil {
		return errhtml(c, err, "/admin")
	}

	// TODO: Only do this when we receive an Accept.
	// This should go in PostInbox, however FChannel doesn't send one.
	if err := DB.AddFollowing(c.Context(), board.ID, target.String()); err != nil {
		return errhtml(c, err, "/admin")
	}

	if strings.TrimSpace(c.Query("fetch")) == "1" {
		// If we made it through all of this, import their outbox in the background.
		log.Printf("Fetching outbox of %s for %s", target.String(), board.ID)

		go func() {
			// Give the request a reasonable amount of time to complete.
			ctx, cancel := context.WithTimeout(context.Background(), config.MaxReqTime)
			defer cancel()

			ob, err := fedi.FetchOutbox(ctx, target.String())
			if err != nil {
				log.Printf("error fetching outbox of %s: %s", target.String(), err)
				return
			}

			// Don't worry about times here.
			if err := fedi.MergeOutbox(context.Background(), board.ID, ob); err != nil {
				log.Printf("error merging outbox of %s to %s: %s", target.String(), board.ID, err)
			}
		}()
	}

	return c.Redirect("/admin")
}

func GetAdminFetch(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	boardReq := strings.TrimSpace(c.Query("board"))
	targetReq := strings.TrimSpace(c.Query("target"))
	if boardReq == "" || targetReq == "" {
		return errhtmlc(c, "You must specify a board and a target.", 400, "/admin")
	}

	board, err := DB.Board(c.Context(), boardReq)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return errhtmlc(c, "That board does not exist.", 404, "/admin")
	} else if err != nil {
		return errhtml(c, err, "/admin")
	}

	target, err := url.Parse(targetReq)
	if err != nil {
		return errhtmlc(c, "The target link is invalid.", 400, "/admin")
	}

	// This will take a while, especially on slow machines, so do it in the background.
	// TODO: database is locked almost the entire time.
	go func() {
		// Give the request a reasonable amount of time to complete.
		ctx, cancel := context.WithTimeout(context.Background(), config.MaxReqTime)
		defer cancel()

		log.Printf("Fetching outbox of %s for %s", target.String(), board.ID)

		ob, err := fedi.FetchOutbox(ctx, target.String())
		if err != nil {
			log.Printf("error fetching outbox of %s: %s", target.String(), err)
			return
		}

		// Don't worry about times here.
		if err := fedi.MergeOutbox(context.Background(), board.ID, ob); err != nil {
			log.Printf("error merging outbox of %s to %s: %s", target.String(), board.ID, err)
		}
	}()

	return c.Redirect("/admin")
}

func GetAdminResend(c *fiber.Ctx) error {
	// TODO: Probably doesn't work for threads and we don't check.

	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	boardReq := strings.TrimSpace(c.Query("board"))
	targetReq := strings.TrimSpace(c.Query("post"))
	if boardReq == "" || targetReq == "" {
		return errhtmlc(c, "You must specify a board and a target post.", 400, "/admin")
	}

	board, err := DB.Board(c.Context(), boardReq)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return errhtmlc(c, "That board does not exist.", 404, "/admin")
	} else if err != nil {
		return errhtml(c, err, "/admin")
	}

	pid, err := strconv.Atoi(targetReq)
	if err != nil {
		return errhtmlc(c, "Bad thread number.", 404, "/admin")
	}

	post, err := DB.Post(c.Context(), board.ID, database.PostID(pid))
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	if !post.IsLocal() {
		return errhtmlc(c, "This post is not owned by this instance, and cannot be resent.", 400, "/admin")
	}

	go func() {
		log.Printf("Force resending /%s/%d", board.ID, post.ID)
		if err := fedi.PostOut(context.Background(), board, post); err != nil {
			log.Printf("fedi.PostOut for /%s/%d: error: %s", board.ID, post.ID, err)
		}
	}()

	return c.RedirectBack("/admin")
}

func GetAdminUnfollow(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeAdmin)
	if !ok {
		return errpriv(c, database.ModTypeAdmin, "/")
	}

	boardReq := strings.TrimSpace(c.Query("board"))
	targetReq := strings.TrimSpace(c.Query("target"))
	if boardReq == "" || targetReq == "" {
		return errhtmlc(c, "You must specify a board and a target.", 400, "/admin")
	}

	target, err := url.Parse(targetReq)
	if err != nil {
		return errhtmlc(c, "The target link is invalid.", 400, "/admin")
	}

	board, err := DB.Board(c.Context(), boardReq)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return errhtmlc(c, "That board does not exist.", 404, "/admin")
	} else if err != nil {
		return errhtml(c, err, "/admin")
	}

	if err := DB.DeleteFollowing(c.Context(), board.ID, target.String()); err != nil {
		return errhtml(c, err, "/admin")
	}

	// I don't know why, but FChannel will remove you from the following list if you send another follow request.
	// This really sucks, because there's an Undo type which can be used to undo Follows. Oh well.
	// TODO: Implement Unfollow in FChannel
	follow, err := fedi.GenerateUnfollow(c.Context(), board, target.String())
	if err != nil {
		return errhtml(c, err, "/admin")
	}

	log.Printf("Unfollowing %s for %s", target.String(), board.ID)
	if err := fedi.SendActivity(c.Context(), follow); err != nil {
		return errhtml(c, err, "/admin")
	}

	return c.Redirect("/admin")
}

func PostRegexp(c *fiber.Ctx) error {
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	pattern := c.FormValue("pattern")

	if pattern == "" {
		return errhtmlc(c, "Need a pattern", 400, "/admin")
	}

	if err := DB.AddRegexp(c.Context(), pattern); err != nil {
		return errhtml(c, err, "/admin")
	}

	return c.Redirect("/admin")
}

func GetRegexpDelete(c *fiber.Ctx) error {
	// Need privileges
	ok := hasPriv(c, database.ModTypeMod)
	if !ok {
		return errpriv(c, database.ModTypeMod, "/")
	}

	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return errhtmlc(c, "Invalid regexp number.", 400, "/admin")
	}

	// This fails silently if its a bad report
	if err := DB.DeleteRegexp(c.Context(), id); err != nil {
		return errhtml(c, err, "/admin")
	}

	// Redirect back to the admin panel
	return c.Redirect("/admin")
}
