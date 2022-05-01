package routes

import (
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/gofiber/fiber/v2"
)

func GetIndex(c *fiber.Ctx) error {
	boards, err := DB.Boards(c.Context())
	if err != nil {
		return err
	}

	news, err := DB.News(c.Context())
	if err != nil {
		return err
	}

	return c.Render("index", fiber.Map{
		"title":   config.Title,
		"version": config.Version,
		"boards":  boards,
		"news":    news,
	})
}

func GetAudit(c *fiber.Ctx) error {
	audits, err := DB.Audits(c.Context())
	if err != nil {
		return err
	}

	return c.Render("audit", fiber.Map{
		"title":  config.Title,
		"audits": audits,
	})
}
