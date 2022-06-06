package routes

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"log"

	"github.com/KushBlazingJudah/feditext/captcha"
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/crypto"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/fedi"
	"github.com/KushBlazingJudah/feditext/util"
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
		"code":  name,
		"image": base64.StdEncoding.EncodeToString(img),
	})
}

func Post(c *fiber.Ctx) error {
	isBot := isStreams(c)

	// Check ban status
	if ok, err := redirBanned(c); err != nil || !ok {
		// User was redirected already
		return err
	}

	// NOTE: returnTo in FChannel works differently than here.
	// We just point you back at whatever you specify.
	returnTo := c.FormValue("returnTo", "/")

	// Check captcha
	if ok := checkCaptcha(c); !ok {
		if isBot {
			return errjsonc(c, 400, "Bad captcha response.")
		}

		return errhtmlc(c, "Bad captcha response.", 400, returnTo)
	}

	boardName := c.FormValue("boardName")
	if boardName == "" {
		if isBot {
			return errjsonc(c, 400, "Specify boardName.")
		}

		return errhtmlc(c, "boardName was not specified.", 400, returnTo)
	}

	board, err := DB.Board(c.Context(), boardName)
	if err != nil {
		if isBot {
			return errjson(c, err) // TODO: update
		}

		return errhtmlc(c, "boardName points to an unknown board.", 400, returnTo)
	}

	name := util.Trim(c.FormValue("name", "Anonymous"), config.NameCutoff)
	subject := util.Trim(c.FormValue("subject"), config.SubjectCutoff)
	content := util.Trim(c.FormValue("comment"), config.PostCutoff)
	if content == "" {
		if isBot {
			return errjsonc(c, 400, "Comment must not be empty.")
		}

		return errhtmlc(c, "Comment must not be empty.", 400, returnTo)
	}

	inReplyTo := c.FormValue("inReplyTo")
	options := c.FormValue("options", "noko")

	var trip string
	name, trip = crypto.DoTrip(name)

	post := database.Post{
		Bumpdate: time.Now().UTC(), // TODO: sage
		Name:     name,
		Raw:      content,
		Source:   getIP(c),
		Subject:  subject,
		Tripcode: trip,
	}

	// If inReplyTo is specified, grab the thread ID if it's there.
	// If not, fail.
	if inReplyTo != "" {
		thread, err := resolvePost(c, board, inReplyTo)
		if err != nil {
			if isBot {
				return errjson(c, err)
			}

			return errhtml(c, err, returnTo)
		}

		if thread.Thread != 0 {
			if isBot {
				return errjsonc(c, 400, "The thread you are posting to is actually a post.")
			}

			return errhtmlc(c, "The thread you are posting to doesn't exist.", 400, returnTo)
		}

		post.Thread = thread.ID
	}

	if err := DB.SavePost(c.Context(), board.ID, &post); err != nil {
		// TODO: update

		if isBot {
			return errjson(c, err)
		} else {
			return errhtml(c, err)
		}
	}

	// TODO: FBI anon asks for outputting the AP object upon response.

	go func() {
		if err := fedi.PostOut(context.Background(), board, post); err != nil {
			log.Printf("fedi.PostOut for /%s/%d: error: %s", board.ID, post.ID, err)
		}
	}()

	// Redirect to the newly created thread if not a bot, and noko
	if !isBot && strings.HasPrefix(options, "noko") {
		return c.Redirect(fmt.Sprintf("/%s/%d", board.ID, post.ID))
	} else if !isBot {
		return c.Redirect(fmt.Sprintf("/%s", board.ID))
	}

	return c.SendStatus(200)
}
