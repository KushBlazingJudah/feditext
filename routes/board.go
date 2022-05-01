package routes

import (
	"fmt"
	"strconv"
	"time"

	"github.com/KushBlazingJudah/feditext/database"
	"github.com/gofiber/fiber/v2"
)

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
		// TODO: 404
		return boards, board, c.SendStatus(404)
	}

	return boards, board, nil
}

func checkCaptcha(c *fiber.Ctx) bool {
	if ok := hasPriv(c, database.ModTypeJanitor); !ok {
		capID := c.FormValue("captcha")
		sol := c.FormValue("solution")

		if ok, err := DB.Solve(c.Context(), capID, sol); err == nil {
			return ok
		}

		return false
	}

	// Fall through for those who are authenticated
	return true
}

// GetBoardIndex ("/:board") returns a summary of all of the threads on the board.
func GetBoardIndex(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
	}

	threads, err := DB.Threads(c.Context(), board.ID)
	if err != nil {
		return err
	}

	return render(c, board.Title, "board", fiber.Map{
		"board":   board,
		"threads": threads,
	})
}

func PostBoardIndex(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
	}

	// Check ban status
	if ok, err := redirBanned(c); err != nil || !ok {
		return err
	}

	// Check captcha
	if ok := checkCaptcha(c); !ok {
		return c.SendStatus(401)
	}

	name := c.FormValue("name", "Anonymous")
	content := c.FormValue("content")
	subject := c.FormValue("subject")

	if content == "" {
		return c.SendStatus(400) // no blank posts
	}

	// TODO: Sanitize!

	post := database.Post{
		Name:    name,
		Content: content,
		Subject: subject,
		Source:  c.IP(),
	}

	if err := DB.SavePost(c.Context(), board.ID, &post); err != nil {
		return err
	}

	// Redirect to the newly created thread
	return c.Redirect(fmt.Sprintf("/%s/%d", board.ID, post.ID))
}

func GetBoardThread(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
	}

	pid, err := strconv.Atoi(c.Params("thread"))
	if err != nil {
		return err
	}

	posts, err := DB.Thread(c.Context(), board.ID, database.PostID(pid)) // unsafe?
	if err != nil {
		return err
	}

	return render(c, fmt.Sprintf("/%s/%d", board.ID, pid), "thread", fiber.Map{
		"board": board,
		"posts": posts,
	})
}

func PostBoardThread(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
	}

	// Check ban status
	if ok, err := redirBanned(c); err != nil || !ok {
		return err
	}

	// Check captcha
	if ok := checkCaptcha(c); !ok {
		return c.SendStatus(401)
	}

	tid, err := strconv.Atoi(c.Params("thread"))
	if err != nil {
		return err
	}

	// TODO: Check if it's a valid thread
	post, err := DB.Post(c.Context(), board.ID, database.PostID(tid))
	if err != nil {
		return err
	}

	if post.Thread != 0 {
		return c.SendStatus(400) // TODO: bad thread
	}

	name := c.FormValue("name", "Anonymous")
	content := c.FormValue("content")
	subject := c.FormValue("subject")

	if content == "" {
		return c.SendStatus(400) // no blank posts
	}

	// TODO: Sanitize!

	bumpdate := time.Now()
	if c.FormValue("sage") == "on" {
		bumpdate = time.Time{}
	}

	// Reusing the post variable
	post = database.Post{
		Thread:   database.PostID(tid),
		Name:     name,
		Content:  content,
		Bumpdate: bumpdate,
		Subject:  subject,
		Source:   c.IP(),
	}

	if err := DB.SavePost(c.Context(), board.ID, &post); err != nil {
		return err
	}

	// Redirect back to the thread
	return c.Redirect("")
}

func GetThreadDelete(c *fiber.Ctx) error {
	// Need privileges
	ok := hasPriv(c, database.ModTypeJanitor)
	if !ok {
		return c.Redirect("/admin/login")
	}

	_, board, err := board(c)
	if err != nil {
		return err
	}

	tid, err := strconv.Atoi(c.Params("thread"))
	if err != nil {
		return err
	}

	// TODO: Check if it's a valid thread
	post, err := DB.Post(c.Context(), board.ID, database.PostID(tid))
	if err != nil {
		return err
	}

	if post.Thread != 0 {
		return c.SendStatus(400) // TODO: bad thread
	}

	if err := DB.DeleteThread(c.Context(), board.ID, post.ID, database.ModerationAction{
		Author: c.Locals("username").(string),
		Type:   database.ModActionDelete,
		Board:  board.ID,
		Post:   post.ID,
		Reason: "TODO",
		Date:   time.Now(),
	}); err != nil {
		return err
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
		return err
	}

	pid, err := strconv.Atoi(c.Params("post"))
	if err != nil {
		return err
	}

	// Check if it's a valid post
	post, err := DB.Post(c.Context(), board.ID, database.PostID(pid))
	if err != nil {
		return err
	}

	if post.Thread == 0 {
		// It's a thread
		return c.SendStatus(400)
	}

	if err := DB.DeletePost(c.Context(), board.ID, post.ID, database.ModerationAction{
		Author: c.Locals("username").(string),
		Type:   database.ModActionDelete,
		Board:  board.ID,
		Post:   post.ID,
		Reason: "TODO",
		Date:   time.Now(),
	}); err != nil {
		return err
	}

	// Redirect back to the thread
	return c.Redirect(fmt.Sprintf("/%s/%d", board.ID, post.Thread))
}

func GetBoardReport(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
	}

	// Check ban status
	if ok, err := redirBanned(c); err != nil || !ok {
		return err
	}

	pid, err := strconv.Atoi(c.Params("post"))
	if err != nil {
		return err
	}

	post, err := DB.Post(c.Context(), board.ID, database.PostID(pid)) // unsafe?
	if err != nil {
		return err
	}

	return render(c, fmt.Sprintf("Report Post /%s/%d", board.ID, pid), "report", fiber.Map{
		"board": board,
		"post":  post,
	})
}

func PostBoardReport(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
	}

	// Check ban status
	if ok, err := redirBanned(c); err != nil || !ok {
		return err
	}

	// Check captcha
	if ok := checkCaptcha(c); !ok {
		return c.SendStatus(401)
	}

	pid, err := strconv.Atoi(c.Params("post"))
	if err != nil {
		return err
	}

	// Ensure it exists
	_, err = DB.Post(c.Context(), board.ID, database.PostID(pid))
	if err != nil {
		return err
	}

	reason := c.FormValue("reason")

	if err := DB.FileReport(c.Context(), database.Report{
		Source: c.IP(),
		Board:  board.ID,
		Post:   database.PostID(pid),
		Reason: reason,
		Date:   time.Now(),
	}); err != nil {
		return err
	}

	// Redirect back to the index
	return c.Redirect("/" + board.ID)
}
