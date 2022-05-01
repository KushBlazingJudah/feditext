package captcha

import (
	"context"
	"math/rand"

	"github.com/KushBlazingJudah/feditext/database"
)

var DB database.Database

// Get returns a JPEG image as the captcha and the solution.
func Get() ([]byte, string) {
	text := captchaText()
	img := newImage(200, 40, text)
	return img, text
}

// Make creates a captcha and inserts it into the database.
func Make(ctx context.Context) error {
	img, sol := Get()
	id := captchaID()

	return DB.SaveCaptcha(ctx, id, sol, img)
}

// Fetch grabs a captcha from the database.
// If there are no captchas, some will be created.
func Fetch(ctx context.Context) (string, error) {
start:
	caps, err := DB.Captchas(ctx)
	if err != nil {
		return "", err
	}

	if len(caps) == 0 {
		for i := 0; i < 50; i++ {
			if err := Make(ctx); err != nil {
				return "", err
			}
		}

		goto start
	}

	id := caps[rand.Intn(len(caps))]
	return id, nil
}
