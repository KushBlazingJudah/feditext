package routes

import (
	"fmt"
	"strconv"
	"time"

	"github.com/KushBlazingJudah/feditext/config"
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

// GetBoardIndex ("/:board") returns a summary of all of the threads on the board.
func GetBoardIndex(c *fiber.Ctx) error {
	boards, board, err := board(c)
	if err != nil {
		return err
	}

	threads, err := DB.Threads(c.Context(), board.ID)
	if err != nil {
		return err
	}

	return c.Render("board", fiber.Map{
		"title":   config.Title,
		"boards":  boards,
		"board":   board,
		"threads": threads,
	})
}

func PostBoardIndex(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
	}

	var newPost struct {
		Name, Content string
	}

	if err := c.BodyParser(&newPost); err != nil {
		return err
	}

	// TODO: Sanitize!

	if newPost.Name == "" {
		newPost.Name = "Anonymous"
	}

	post := database.Post{
		Name:    newPost.Name,
		Content: newPost.Content,
		Source:  c.IP(),
	}

	if err := DB.SavePost(c.Context(), board.ID, &post); err != nil {
		return err
	}

	// Redirect to the newly created thread
	return c.Redirect(fmt.Sprintf("/%s/%d", board.ID, post.ID))
}

func GetBoardThread(c *fiber.Ctx) error {
	boards, board, err := board(c)
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

	return c.Render("thread", fiber.Map{
		"title":  fmt.Sprintf("/%s/%d | %s", board.ID, posts[0].ID, config.Title),
		"boards": boards,
		"board":  board,
		"posts":  posts,
	})
}

func PostBoardThread(c *fiber.Ctx) error {
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

	var newPost struct {
		Name, Content string
	}

	if err := c.BodyParser(&newPost); err != nil {
		return err
	}

	// TODO: Sanitize!

	if newPost.Name == "" {
		newPost.Name = "Anonymous"
	}

	// Reusing the post variable
	post = database.Post{
		Thread:  database.PostID(tid),
		Name:    newPost.Name,
		Content: newPost.Content,
		Source:  c.IP(),
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
		Action: database.ModActionDelete,
		Board:  board.ID,
		Post:   post.ID,
		Reason: "TODO",
		Time:   time.Now(),
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
		Action: database.ModActionDelete,
		Board:  board.ID,
		Post:   post.ID,
		Reason: "TODO",
		Time:   time.Now(),
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

	pid, err := strconv.Atoi(c.Params("post"))
	if err != nil {
		return err
	}

	post, err := DB.Post(c.Context(), board.ID, database.PostID(pid)) // unsafe?
	if err != nil {
		return err
	}

	return c.Render("report", fiber.Map{
		"title": fmt.Sprintf("Report Post | %s", config.Title),
		"board": board,
		"post":  post,
	})
}

func PostBoardReport(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return err
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
