package routes

import (
	"fmt"
	"strconv"

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
		Thread:  database.PostID(tid),
		Name:    newPost.Name,
		Content: newPost.Content,
		Source:  c.IP(),
	}

	if err := DB.SavePost(c.Context(), board.ID, &post); err != nil {
		return err
	}

	return c.Redirect("")
}
