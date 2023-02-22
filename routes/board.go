package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
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
	Hidden          int
}

type catalogData struct {
	database.Post
	NPosts, Posters int
}

func board(c *fiber.Ctx) (database.Board, error) {
	board, err := DB.Board(c.Context(), c.Params("board"))
	if err != nil {
		return board, err
	}

	return board, err
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

	board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	page := 1

	if q := c.Query("page"); q != "" {
		page, err = strconv.Atoi(c.Query("page"))
		if err != nil {
			return errhtml(c, err)
		}
		if page < 0 {
			page = 1
		}
	}

	pages := int(math.Ceil(float64(board.Threads) / config.ThreadsPerPage))
	if page > pages && pages != 0 {
		return errhtmlc(c, "This page does not exist.", 404, fmt.Sprintf("/%s", board.ID))
	}

	threads, err := DB.Threads(c.Context(), board.ID, page)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	posts := []indexData{}

	for _, thread := range threads {
		t, err := DB.Thread(c.Context(), board.ID, thread.ID, 5, true)
		if err != nil {
			return errhtml(c, err) // TODO: update
		}

		nposts, posters, err := DB.ThreadStat(c.Context(), board.ID, thread.ID)
		if err != nil {
			return errhtml(c, err) // TODO: update
		}

		posts = append(posts, indexData{t, nposts, posters, nposts - len(t)})
	}

	return render(c, board.Title, "board/index", fiber.Map{
		"board":   board,
		"threads": posts,
		"page":    page,

		// This is a really, *really* horrible hack but it works?
		"pages": make([]struct{}, pages+1),

		"showpicker": true,
	})
}

func GetBoardCatalog(c *fiber.Ctx) error {
	board, err := board(c)
	if err != nil {
		return errhtml(c, err) // TODO: update
	}

	threads, err := DB.Threads(c.Context(), board.ID, 0)
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

	return render(c, board.Title, "board/catalog", fiber.Map{
		"board":   board,
		"threads": posts,

		"showpicker": true,
	})
}

func GetBoardThread(c *fiber.Ctx) error {
	if isStreams(c) {
		return GetBoardNote(c)
	}

	board, err := board(c)
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

	posts, err := DB.Thread(c.Context(), board.ID, op.ID, 0, true)
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

	return render(c, fmt.Sprintf("/%s/%d", board.ID, op.ID), "board/thread", fiber.Map{
		"board": board,
		"posts": posts,

		"nposts":  nposts,
		"posters": posters,

		"showpicker": true,
	})
}

func GetBoardReport(c *fiber.Ctx) error {
	board, err := board(c)
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

	return render(c, fmt.Sprintf("Report Post /%s/%d", board.ID, pid), "board/report", fiber.Map{
		"board": board,
		"post":  post,
	})
}

func PostBoardReport(c *fiber.Ctx) error {
	board, err := board(c)
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
	rep := database.Report{
		Source: getIP(c),
		Board:  board.ID,
		Post:   database.PostID(pid),
		Reason: reason,
		Date:   time.Now().UTC(),
	}

	if err := DB.FileReport(c.Context(), rep); err != nil {
		return errhtml(c, err)
	}

	go rep.Notify(DB)

	// Redirect back to the index
	return c.Redirect("/" + board.ID)
}

func GetDelete(c *fiber.Ctx) error {
	// Need privileges
	ok := hasPriv(c, database.ModTypeJanitor)
	if !ok {
		return c.Redirect("/admin/login")
	}

	boardReq := strings.TrimSpace(c.Query("board"))
	postReq := strings.TrimSpace(c.Query("post"))

	board, err := DB.Board(c.Context(), boardReq)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return errhtmlc(c, "That board does not exist.", 404, "/admin")
	} else if err != nil {
		return errhtml(c, err, "/admin")
	}

	pid, err := strconv.Atoi(postReq)
	if err != nil {
		return errhtmlc(c, "Bad post number.", 400, fmt.Sprintf("/%s", board.ID))
	}

	// Check if it's a valid post
	post, err := DB.Post(c.Context(), board.ID, database.PostID(pid))
	if err != nil {
		return errhtmlc(c, "The post you are looking for doesn't exist.", 404, fmt.Sprintf("/%s", board.ID))
	}

	hasConfirmed := strings.TrimSpace(c.Query("confirm", "")) == "1"
	if hasConfirmed {
		if post.Thread == 0 {
			if err := DB.DeleteThread(c.Context(), board.ID, post.ID, database.ModerationAction{
				Author: c.Locals("username").(string),
				Type:   database.ModActionDelete,
				Board:  board.ID,
				Post:   post.ID,
				Reason: c.Query("reason", "No reason provided."),
				Date:   time.Now().UTC(),
			}); err != nil {
				return errhtml(c, err)
			}
		} else {
			if err := DB.DeletePost(c.Context(), board.ID, post.ID, database.ModerationAction{
				Author: c.Locals("username").(string),
				Type:   database.ModActionDelete,
				Board:  board.ID,
				Post:   post.ID,
				Reason: c.Query("reason", "No reason provided."),
				Date:   time.Now().UTC(),
			}); err != nil {
				return errhtml(c, err)
			}
		}

		// Tell everyone else if it's local
		if post.IsLocal() {
			go func() {
				if err := fedi.PostDel(context.Background(), board, post); err != nil {
					log.Printf("fedi.PostDel for /%s/%d: error: %s", board.ID, post.ID, err)
				}
			}()
		}

		return c.Redirect("/" + board.ID)
	}

	return render(c, fmt.Sprintf("Delete Post /%s/%d", board.ID, pid), "admin/delete", fiber.Map{
		"board": board,
		"post":  post,
	})
}
