package routes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"os"
	"path/filepath"
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

var themes []string

func init() {
	Tmpl = html.New("./views", ".html")

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

	Tmpl.AddFunc("br", func(s string) template.HTML {
		s = template.HTMLEscapeString(s)
		s = strings.ReplaceAll(s, "\n", "<br/>")
		return template.HTML(s)
	})

	Tmpl.AddFunc("fancyname", func(p database.Post) template.HTML {
		var name, trip, domain, domainFull string
		name = fmt.Sprintf(`<span class="name">%s</span>`, template.HTMLEscapeString(p.Name))

		if strings.HasPrefix(p.Source, "http") {
			// Treat as "external"
			u, err := url.Parse(p.Source)
			if err != nil {
				log.Printf("failed parsing %s as source: %s", p.Source, err)
				domain = "(unknown)"
				domainFull = "(unknown)"
			} else {
				// Check if the domain is too long and shorten it otherwise
				domain = u.Hostname()
				domainFull = u.Host
				e := strings.LastIndexAny(domain, ".")
				if e == -1 {
					e = len(domain)
				}

				tld := domain[e:]

				if e > 19 { // Arbitrary number; was 32, then 16, then 19 because 16 + len("...") = 19
					domain = domain[:12] + "..." + domain[e-4:e] + tld
				}
			}

			name += fmt.Sprintf(`<a href="%s" class="external" title="%s">@%s</a>`, template.HTMLEscapeString(p.APID), template.HTMLEscapeString(domainFull), template.HTMLEscapeString(domain))
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

		return template.HTML(fmt.Sprintf(`<img src="/captcha/%s"></img><br><input type="text" name="captcha" id="captcha" maxlength="%d" placeholder="Captcha solution"><input type="hidden" name="captchaCode" id="captchaCode" value="%s">`, name, captcha.CaptchaLen, name))
	})

	Tmpl.AddFunc("time", func(t time.Time) template.HTML {
		if t.IsZero() {
			return template.HTML("")
		}

		s := t.Format("01/02/06(Mon)15:04:05")
		return template.HTML(fmt.Sprintf(`<span data-utc="%d" class="date">%s</span>`, t.Unix(), s))
	})

	// read themes directory
	dir, err := os.ReadDir("./static/css/themes")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		panic(err)
	}

	for _, v := range dir {
		if ext := filepath.Ext(v.Name()); !v.IsDir() && ext == ".css" {
			themes = append(themes, v.Name()[:len(v.Name())-len(ext)])
		}
	}
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
		"boards": boards,
		"fqdn":   config.FQDN,
		"name":   config.Title,
		"title":  title,

		"version": config.Version,
		"postMax": config.PostCutoff,
		"nameMax": config.NameCutoff,
		"subMax":  config.SubjectCutoff,
		"repMax":  config.ReportCutoff,
		"private": config.Private,
		"themes":  themes,
	}

	// merge map
	for k, v := range f {
		m[k] = v
	}

	return c.Render(tmpl, m)
}

func redirBanned(c *fiber.Ctx) (bool, error) {
	// Skip if logged in, or private mode is on
	// We can't ban people in private mode
	if c.Locals("privs") != nil || config.Private {
		return true, nil
	}

	ok, _, _, err := DB.Banned(c.Context(), c.IP())
	if err != nil {
		return false, err
	}

	if !ok {
		return false, c.Redirect("/banned")
	}

	return true, nil
}

func errhtml(c *fiber.Ctx, err error, ret ...string) error {
	retu := ""
	if len(ret) == 1 {
		retu = ret[0]
	}

	if err == nil {
		panic("nil err passed to errjson")
	}

	text := "An unknown error has occurred."
	status := 500

	if errors.Is(err, sql.ErrNoRows) || strings.HasPrefix(err.Error(), "no such table") {
		status = 404
		text = "Not found."
	} else if errors.Is(err, database.ErrPostContents) {
		status = 400
		text = "Your post contents are invalid. Is your post zero characters long?"
	} else if errors.Is(err, database.ErrPostRejected) {
		status = 400
		text = "Your post was rejected!"
	} else if errors.Is(err, ErrInvalidID) {
		status = 404
		text = "Invalid post ID."
	} else {
		// TODO: More filters.
		// TODO: RSA verification error
		// TODO: JSON
		log.Printf("uncaught error on %s: %s", c.Path(), err)
		text = "An internal server error has occurred."
	}

	if err := render(c.Status(status), "Error", "error", fiber.Map{
		"error":  text,
		"return": retu,
	}); err != nil {
		return err
	}

	return err
}

func errhtmlc(c *fiber.Ctx, msg string, status int, ret ...string) error {
	retu := ""
	if len(ret) == 1 {
		retu = ret[0]
	}

	if err := c.SendStatus(status); err != nil {
		return err
	}

	if err := render(c, "Error", "error", fiber.Map{
		"error":  msg,
		"return": retu,
	}); err != nil {
		return err
	}

	return fmt.Errorf("errhtmlc: %s", msg)
}

func getIP(c *fiber.Ctx) string {
	if config.Private {
		return "127.0.0.1"
	}

	return c.IP()
}
