package routes

import (
	"encoding/base64"

	"github.com/KushBlazingJudah/feditext/captcha"
	"github.com/gofiber/fiber/v2"
)

func GetCaptchaID(c *fiber.Ctx) error {
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

func GetCaptcha(c *fiber.Ctx) error {
	// We just need to fetch a random captcha
	name, err := captcha.Fetch(c.Context())
	if err != nil {
		return errjson(c, err)
	}

	img, _, err := DB.Captcha(c.Context(), name)
	if err != nil {
		return errjson(c, err)
	}

	return c.JSON(map[string]string{
		"code": name,
		"image": base64.StdEncoding.EncodeToString(img),
	})
}
