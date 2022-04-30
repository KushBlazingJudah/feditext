package routes

// TODO: EVERY ROUTE IN HERE NEEDS AUTHENTICATION AND BADLY

import (
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/gofiber/fiber/v2"
)

func GetAdmin(c *fiber.Ctx) error {
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
