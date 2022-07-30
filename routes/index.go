package routes

import (
	"database/sql"
	"errors"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/gofiber/fiber/v2"
)

func GetIndex(c *fiber.Ctx) error {
	news, err := DB.News(c.Context())
	if err != nil {
		return err
	}

	return render(c, "", "index", fiber.Map{
		"news":  news,
		"audit": config.PublicAudit,
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

func GetBanned(c *fiber.Ctx) error {
	// This route is disabled in private mode

	ok, exp, reason, err := DB.Banned(c.Context(), c.IP())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	return render(c, "", "banned", fiber.Map{
		"banned":  !ok,
		"expires": exp,
		"reason":  reason,
	})
}

func GetRules(c *fiber.Ctx) error {
	return render(c, "Rules", "rules", nil)
}

func GetFAQ(c *fiber.Ctx) error {
	return render(c, "FAQ", "faq", fiber.Map{
		"donate": config.Donate,
	})
}
