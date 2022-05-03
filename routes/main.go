package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/KushBlazingJudah/feditext/captcha"
	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
)

// Prevents an import cycle.
var DB database.Database

var Tmpl *html.Engine

func init() {
	Tmpl = html.New("./views", ".html")
	Tmpl.Debug(true)

	Tmpl.AddFunc("unescape", func(s string) template.HTML {
		return template.HTML(s)
	})

	Tmpl.AddFunc("summarize", func(s string) template.HTML {
		s = strings.ReplaceAll(s, "\n", " ")
		s = template.HTMLEscapeString(s)
		if len(s) > 240 {
			return template.HTML(s[:160] + "...")
		}

		return template.HTML(s)
	})

	Tmpl.AddFunc("fancyname", func(p database.Post) template.HTML {
		var name, trip, domain string
		name = fmt.Sprintf(`<span class="name">%s</span>`, template.HTMLEscapeString(p.Name))

		if strings.HasPrefix(p.Source, "http") {
			// Treat as "external"
			u, err := url.Parse(p.Source)
			if err != nil {
				log.Printf("failed parsing %s as source: %s", p.Source, err)
				domain = "(unknown)"
			} else {
				domain = u.Hostname()
				if len(domain) > 32 { // Arbitrary number
					domain = domain[len(domain)-32:]
				}
			}
			name += fmt.Sprintf(`<span class="external">@%s</span>`, domain)
		}

		if p.Tripcode != "" {
			trip = fmt.Sprintf(`<span class="tripcode">%s</span>`, template.HTMLEscapeString(p.Tripcode)) // TODO: Clip
		}

		return template.HTML(name + trip)
	})

	Tmpl.AddFunc("captcha", func() template.HTML {
		name, err := captcha.Fetch(context.TODO())
		if err != nil {
			log.Printf("while retreving captcha: %v", err)
			return template.HTML("<b>unable to retrieve captcha; please refresh</b>")
		}

		return template.HTML(fmt.Sprintf(`<img src="/captcha/%s"></img><br><input type="text" name="solution" id="solution" maxlength="%d" placeholder="Captcha solution"><input type="hidden" name="captcha" id="captcha" value="%s">`, name, captcha.CaptchaLen, name))
	})

	Tmpl.AddFunc("time", func(t time.Time) template.HTML {
		if t.IsZero() {
			return template.HTML("")
		}

		s := t.Format("01/02/06(Mon)15:04:05")
		return template.HTML(fmt.Sprintf(`<span data-utc="%d" class="date">%s</span>`, t.Unix(), s))
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
		"boards":  boards,
		"fqdn":    config.FQDN,
		"name":    config.Title,
		"title":   title,
		"version": config.Version,

		"postMax": config.PostCutoff,
		"nameMax": config.NameCutoff,
		"subMax":  config.SubjectCutoff,
		"repMax":  config.ReportCutoff,
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
