package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"regexp"
	"strings"

	"github.com/KushBlazingJudah/feditext/captcha"
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
)

// Prevents an import cycle.
var DB database.Database

var Tmpl *html.Engine

var citeRegex = regexp.MustCompile("&gt;&gt;(\\d+)")
var quoteRegex = regexp.MustCompile("(?m)^&gt;(.+?)$")

func init() {
	Tmpl = html.New("./views", ".html")
	Tmpl.Debug(true)

	Tmpl.AddFunc("unescape", func(s string) template.HTML {
		return template.HTML(s)
	})

	Tmpl.AddFunc("format", func(s string) template.HTML {
		s = template.HTMLEscapeString(s)

		// TODO: Handle links not on page.
		// I would've done this myself if ReplaceAllStringFunc would expand.
		s = citeRegex.ReplaceAllString(s, `<a href="#p$1" class="cite">&gt;&gt;$1</a>`)

		s = quoteRegex.ReplaceAllString(s, `<span class="quote">&gt;$1</span>`)
		s = strings.ReplaceAll(s, "\n", "<br/>")
		return template.HTML(s)
	})

	Tmpl.AddFunc("summarize", func(s string) template.HTML {
		s = strings.ReplaceAll(s, "\n", " ")
		s = template.HTMLEscapeString(s)
		if len(s) > 240 {
			return template.HTML(s[:240] + "...")
		}

		return template.HTML(s)
	})

	Tmpl.AddFunc("captcha", func() template.HTML {
		name, err := captcha.Fetch(context.TODO())
		if err != nil {
			log.Printf("while retreving captcha: %v", err)
			return template.HTML("<b>unable to retrieve captcha; please refresh</b>")
		}

		return template.HTML(fmt.Sprintf(`<img src="/captcha/%s"></img><br><input type="text" name="solution" id="solution" placeholder="Captcha solution"><input type="hidden" name="captcha" id="captcha" value="%s">`, name, name))
	})
}

// convenience function since html/template kinda sucks
func render(c *fiber.Ctx, title, tmpl string, f fiber.Map) error {
	if title == "" {
		title = config.Title
	} else {
		title = fmt.Sprintf("%s | %s", title, config.Title)
	}

	boards, err := DB.Boards(c.Context())
	if err != nil {
		return err
	}

	m := fiber.Map{
		"boards":   boards,
		"fqdn":     config.FQDN,
		"name":     config.Title,
		"title":    title,
		"version":  config.Version,
		"username": c.Locals("username"),
		"privs":    c.Locals("privs"),
	}

	// merge map
	for k, v := range f {
		m[k] = v
	}

	return c.Render(tmpl, m)
}

func redirBanned(c *fiber.Ctx) (bool, error) {
	// Skip if logged in
	if c.Locals("privs") != nil {
		return true, nil
	}

	ok, _, _, err := DB.Banned(c.Context(), c.IP())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	if !ok {
		return false, c.Redirect("/banned")
	}

	return true, nil
}

func errResp(c *fiber.Ctx, msg string, status int, ret ...string) error {
	retu := ""
	if len(ret) == 1 {
		retu = ret[0]
	}

	if err := c.SendStatus(status); err != nil {
		return err
	}

	return render(c, "Error", "error", fiber.Map{
		"error":  msg,
		"return": retu,
	})
}
