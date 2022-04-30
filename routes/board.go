package routes

import (
	"fmt"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/gofiber/fiber/v2"
)

func board(c *fiber.Ctx) (database.Board, error) {
	board := database.Board{}

	boards, err := DB.Boards(c.Context())
	if err != nil {
		return board, err
	}

	for _, v := range boards {
		if v.ID == c.Params("board") {
			board = v
			break
		}
	}

	if board.ID == "" {
		// TODO: 404
		return board, c.SendStatus(404)
	}

	return board, nil
}

// GetBoardIndex ("/:board") returns a summary of all of the threads on the board.
func GetBoardIndex(c *fiber.Ctx) error {
	// TODO: This calls DB.Boards twice.

	boards, err := DB.Boards(c.Context())
	if err != nil {
		return err
	}

	board, err := board(c)
	if err != nil {
		return err
	}

	threads, err := DB.Threads(c.Context(), board.ID)
	if err != nil {
		return err
	}

	return c.Render("board", fiber.Map{
		"title":   config.Title,
		"version": config.Version,
		"boards":  boards,
		"board":   board,
		"threads": threads,
	})
}

func PostBoardIndex(c *fiber.Ctx) error {
	board, err := board(c)
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
