package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/KushBlazingJudah/feditext/captcha"
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/fedi"
	"github.com/KushBlazingJudah/feditext/util"
	"github.com/gofiber/fiber/v2"
)

var (
	ErrInvalidID = errors.New("invalid post id")
)

type indexData struct {
	Posts           []database.Post
	NPosts, Posters int
}

type catalogData struct {
	database.Post
	NPosts, Posters int
}

func board(c *fiber.Ctx) ([]database.Board, database.Board, error) {
	board := database.Board{}

	boards, err := DB.Boards(c.Context())
	if err != nil {
		return boards, board, err
	}

	for _, v := range boards {
		if v.ID == c.Params("board") {
			board = v
			break
		}
	}

	if board.ID == "" {
		err = sql.ErrNoRows
	}

	return boards, board, err
}

func resolvePost(c *fiber.Ctx, board database.Board, match string) (database.Post, error) {
	var post database.Post
	var err error

	if strings.HasPrefix(match, "http") {
		// This is an ActivityPub ID we're looking for.
		// Since there's nothing to fall back on, just fail if it does fail.

		post, err = DB.FindAPID(c.Context(), board.ID, match)
		return post, err
	}

	pid, err := strconv.Atoi(match) // TODO: SOME HEX IDS *ARE* VALID.
	if err != nil {
		// Try to look it up in the database.
		// Since externally we use the randomly generated IDs like FChannel to
		// avoid confusion with several posts being fprog-1, FChannel correctly
		// assumes that the post will be available at /prog/deadbeef even
		// though it is actually /prog/420.
		q := fmt.Sprintf("%s://%s/%s/%s", config.TransportProtocol, config.FQDN, board.ID, match)
		post, err = DB.FindAPID(c.Context(), board.ID, q)
		return post, err
	}

	post, err = DB.Post(c.Context(), board.ID, database.PostID(pid))
	return post, err
}

func checkCaptcha(c *fiber.Ctx) bool {
	// If you're logged in, we won't worry about the captcha.
	if ok := hasPriv(c, database.ModTypeJanitor); !ok {
		capID := c.FormValue("captchaCode")
		sol := c.FormValue("captcha")
		if len(capID) != captcha.CaptchaIDLen || len(sol) != captcha.CaptchaLen {
			return false
		}

		if ok, err := DB.Solve(c.Context(), capID, sol); err == nil {
			return ok
		}

		return false
	}

	// Fall through for those who are authenticated
	return true
}

func GetBoardIndex(c *fiber.Ctx) error {
	if isStreams(c) {
		return GetBoardActor(c)
	}

	_, board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	threads, err := DB.Threads(c.Context(), board.ID)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	posts := []indexData{}

	for _, thread := range threads {
		t, err := DB.Thread(c.Context(), board.ID, thread.ID, 5)
		if err != nil {
			return errhtml(c, err) // TODO: update
		}

		nposts, posters, err := DB.ThreadStat(c.Context(), board.ID, thread.ID)
		if err != nil {
			return errhtml(c, err) // TODO: update
		}

		posts = append(posts, indexData{t, nposts, posters})
	}

	return render(c, board.Title, "board", fiber.Map{
		"board":   board,
		"threads": posts,

		"showpicker": true,
	})
}

func GetBoardCatalog(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	threads, err := DB.Threads(c.Context(), board.ID)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	posts := []catalogData{}

	for _, thread := range threads {
		nposts, posters, err := DB.ThreadStat(c.Context(), board.ID, thread.ID)
		if err != nil {
			return errhtml(c, err) // TODO: update
		}

		posts = append(posts, catalogData{thread, nposts, posters})
	}

	return render(c, board.Title, "catalog", fiber.Map{
		"board":   board,
		"threads": posts,

		"showpicker": true,
	})
}

func GetBoardThread(c *fiber.Ctx) error {
	if isStreams(c) {
		return GetBoardNote(c)
	}

	_, board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	op, err := resolvePost(c, board, c.Params("thread"))
	if err != nil {
		return errhtml(c, err, "/"+board.ID)
	}

	if op.Thread != 0 {
		// Redirect to the true location.
		return c.Redirect(fmt.Sprintf("/%s/%d#p%d", board.ID, op.Thread, op.ID))
	}

	posts, err := DB.Thread(c.Context(), board.ID, op.ID, 0)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errhtmlc(c, "This thread does not exist.", 404, fmt.Sprintf("/%s", board.ID))
		}

		return errhtml(c, err) // TODO: update
	}

	nposts, posters, err := DB.ThreadStat(c.Context(), board.ID, op.ID)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	return render(c, fmt.Sprintf("/%s/%d", board.ID, op.ID), "thread", fiber.Map{
		"board": board,
		"posts": posts,

		"nposts":  nposts,
		"posters": posters,

		"showpicker": true,
	})
}

func GetThreadDelete(c *fiber.Ctx) error {
	// Need privileges
	ok := hasPriv(c, database.ModTypeJanitor)
	if !ok {
		return c.Redirect("/admin/login")
	}

	_, board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	tid, err := strconv.Atoi(c.Params("thread"))
	if err != nil {
		return errhtmlc(c, "Bad thread number.", 400, fmt.Sprintf("/%s", board.ID))
	}

	post, err := DB.Post(c.Context(), board.ID, database.PostID(tid))
	if err != nil {
		return errhtmlc(c, "The thread you are looking for doesn't exist.", 404, fmt.Sprintf("/%s", board.ID))
	}

	if post.Thread != 0 {
		return errhtmlc(c, "The thread you are looking for is actually a post.", 404, fmt.Sprintf("/%s/%d", board.ID, post.Thread))
	}

	if err := DB.DeleteThread(c.Context(), board.ID, post.ID, database.ModerationAction{
		Author: c.Locals("username").(string),
		Type:   database.ModActionDelete,
		Board:  board.ID,
		Post:   post.ID,
		Reason: "TODO",
		Date:   time.Now().UTC(),
	}); err != nil {
		return errhtml(c, err)
	}

	// Tell everyone else if it's local
	if !strings.HasPrefix(post.Source, "http") {
		go func() {
			if err := fedi.PostDel(context.Background(), board, post); err != nil {
				log.Printf("fedi.PostDel for /%s/%d: error: %s", board.ID, post.ID, err)
			}
		}()
	}

	// Redirect back to the board
	return c.Redirect("/" + board.ID)
}

func GetPostDelete(c *fiber.Ctx) error {
	// Need privileges
	ok := hasPriv(c, database.ModTypeJanitor)
	if !ok {
		return c.Redirect("/admin/login")
	}

	_, board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	pid, err := strconv.Atoi(c.Params("post"))
	if err != nil {
		return errhtmlc(c, "Bad post number.", 400, fmt.Sprintf("/%s", board.ID))
	}

	// Check if it's a valid post
	post, err := DB.Post(c.Context(), board.ID, database.PostID(pid))
	if err != nil {
		return errhtmlc(c, "The post you are looking for doesn't exist.", 404, fmt.Sprintf("/%s", board.ID))
	}

	if post.Thread == 0 {
		// It's a thread
		return errhtmlc(c, "The post you are looking for is actually a thread.", 400, fmt.Sprintf("/%s", board.ID))
	}

	if err := DB.DeletePost(c.Context(), board.ID, post.ID, database.ModerationAction{
		Author: c.Locals("username").(string),
		Type:   database.ModActionDelete,
		Board:  board.ID,
		Post:   post.ID,
		Reason: "TODO",
		Date:   time.Now().UTC(),
	}); err != nil {
		return errhtml(c, err)
	}

	// Tell everyone else if it's local
	if !strings.HasPrefix(post.Source, "http") {
		go func() {
			if err := fedi.PostDel(context.Background(), board, post); err != nil {
				log.Printf("fedi.PostDel for /%s/%d: error: %s", board.ID, post.ID, err)
			}
		}()
	}

	// Redirect back to the thread
	return c.Redirect(fmt.Sprintf("/%s/%d", board.ID, post.Thread))
}

func GetBoardReport(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	// Check ban status
	if ok, err := redirBanned(c); err != nil || !ok {
		// User was redirected already
		return err
	}

	pid, err := strconv.Atoi(c.Query("post"))
	if err != nil {
		return errhtmlc(c, "Bad post number.", 400, fmt.Sprintf("/%s", board.ID))
	}

	post, err := DB.Post(c.Context(), board.ID, database.PostID(pid))
	if err != nil {
		return errhtmlc(c, "The post you are looking for doesn't exist.", 403, fmt.Sprintf("/%s", board.ID))
	}

	return render(c, fmt.Sprintf("Report Post /%s/%d", board.ID, pid), "report", fiber.Map{
		"board": board,
		"post":  post,
	})
}

func PostBoardReport(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	// Check ban status
	if ok, err := redirBanned(c); err != nil || !ok {
		// User was redirected already
		return err
	}

	// Check captcha
	if ok := checkCaptcha(c); !ok {
		return errhtmlc(c, "Bad captcha response.", 400, fmt.Sprintf("/%s/%s", board.ID, c.Params("thread")))
	}

	pid, err := strconv.Atoi(c.Query("post"))
	if err != nil {
		return errhtmlc(c, "Bad post number.", 400, fmt.Sprintf("/%s", board.ID))
	}

	// Ensure it exists
	_, err = DB.Post(c.Context(), board.ID, database.PostID(pid))
	if err != nil {
		return errhtmlc(c, "Post was not found.", 404, "/"+board.ID)
	}

	reason := util.Trim(c.FormValue("reason"), config.ReportCutoff)

	if err := DB.FileReport(c.Context(), database.Report{
		Source: getIP(c),
		Board:  board.ID,
		Post:   database.PostID(pid),
		Reason: reason,
		Date:   time.Now().UTC(),
	}); err != nil {
		return errhtml(c, err)
	}

	// Redirect back to the index
	return c.Redirect("/" + board.ID)
}
