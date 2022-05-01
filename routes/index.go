package routes

import (
	"github.com/gofiber/fiber/v2"
)

func GetIndex(c *fiber.Ctx) error {
	news, err := DB.News(c.Context())
	if err != nil {
		return err
	}

	return render(c, "", "index", fiber.Map{
		"news": news,
	})
}

func GetAudit(c *fiber.Ctx) error {
	audits, err := DB.Audits(c.Context())
	if err != nil {
		return err
	}

	return render(c, "Audit Log", "audit", fiber.Map{
		"audits": audits,
	})
}
