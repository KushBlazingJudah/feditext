package routes

import "github.com/gofiber/fiber/v2"

func GetCaptcha(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.SendStatus(400)
	}

	img, _, err := DB.Captcha(c.Context(), id)
	if err != nil {
		return err
	}

	c.Context().SetContentType("image/jpeg")
	return c.Send(img)
}
