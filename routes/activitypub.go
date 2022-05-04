package routes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/fedi"
	"github.com/gofiber/fiber/v2"
)

const streams = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

// False positive for application/ld+ld, application/activity+ld, application/json+json
var streamsRegex = regexp.MustCompile(`application/(ld|json|activity)\+(ld|json)`)

type link struct {
	Rel  string `json:"rel"`
	Type string `json:"type"`
	Href string `json:"href"`
}

type webfingerResp struct {
	Subject string `json:"subject"`
	Links   []link
}

func isStreams(c *fiber.Ctx) bool {
	// Hack, but it works
	str, ok := c.GetReqHeaders()["Accept"]
	if !ok {
		return false
	}

	return streamsRegex.MatchString(str)
}

// (*fiber.Ctx).JSON doesn't let me not escape HTML.
// I also didn't look.
func jsonresp(c *fiber.Ctx, data any) error {
	c.Response().Header.Add("Content-Type", streams)

	encoder := json.NewEncoder(c)
	encoder.SetEscapeHTML(false)

	return encoder.Encode(data)
}

func Webfinger(c *fiber.Ctx) error {
	// Shotty implmentation but it works

	query := c.Query("resource")
	fmt.Println("webfinger", query, c.Request().URI())
	if query == "" {
		// TODO: JSON error
		return c.Status(400).SendString("need a resource query")
	}

	if !strings.HasPrefix(query, "acct:") {
		// TODO: JSON error
		return c.Status(400).SendString("only support acct")
	}

	toks := strings.SplitN(query[5:], "@", 2)
	fmt.Println(toks[0], toks[1])
	if len(toks) != 2 {
		return c.Status(404).SendString("no actor found")
	} else if toks[1] != config.FQDN {
		return c.Status(404).SendString("no actor found")
	}

	if board, err := DB.Board(c.Context(), toks[0]); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(404).SendString("no actor found")
		}

		return err
	} else {
		return c.JSON(webfingerResp{
			Subject: fmt.Sprintf("acct:/%s@%s", board.ID, config.FQDN),
			Links: []link{{
				Rel:  "self",
				Type: "application/activity+json",
				Href: fmt.Sprintf("%s://%s/%s", config.TransportProtocol, config.FQDN, board.ID),
			}},
		})
	}
}

func GetBoardActor(c *fiber.Ctx) error {
	_, board, err := board(c)
	if board.ID == "" || err != nil {
		return err
	}

	return jsonresp(c, fedi.TransformBoard(board))
}

func GetBoardOutbox(c *fiber.Ctx) error {
	_, board, err := board(c)
	if board.ID == "" || err != nil {
		return err
	}

	c.Response().Header.Add("Content-Type", streams)

	outbox, err := fedi.GenerateOutbox(c.Context(), board)
	if err != nil {
		return err
	}

	return jsonresp(c, outbox)
}

func GetBoardNote(c *fiber.Ctx) error {
	// Correct me if I'm wrong, but I don't think we're supposed to return an OrderedNoteCollection here.
	// Anyways, this is what FChannel does.
	// I thought we should return the Note representation...?

	_, board, err := board(c)
	if board.ID == "" || err != nil {
		return err
	}

	actor := fedi.TransformBoard(board)

	var post database.Post

	pid, err := strconv.Atoi(c.Params("thread"))
	if err != nil {
		// It's probably ActivityPub
		// TODO: Does FChannel resolve for post IDs not its own? If so, that sucks.
		q := fmt.Sprintf("%s://%s/%s/%s", config.TransportProtocol, config.FQDN, board.ID, c.Params("thread"))
		post, err = DB.FindAPID(c.Context(), board.ID, q)
		if err != nil {
			return err
		}
	} else {
		// It's a post number of ours
		post, err = DB.Post(c.Context(), board.ID, database.PostID(pid))
		if err != nil {
			return err
		}
	}

	// I could probably make this much more dense but eh

	if post.Thread == 0 {
		// It's a thread
		// Fetch the replies and send

		thread, err := DB.Thread(c.Context(), board.ID, database.PostID(pid), 0)
		if err != nil {
			return err
		}

		if l := len(thread); l > 1 {
			out := fedi.OrderedNoteCollection{
				Type:         "OrderedCollection",
				TotalItems:   l,
				OrderedItems: make([]fedi.Note, 0, l),
			}

			t := fedi.Note{}

			for i, post := range thread { // Shadow post
				if i == 1 { // HACK
					t = out.OrderedItems[0]
				}

				nn, err := fedi.TransformPost(c.Context(), actor, post, t, true)
				if err != nil {
					return err
				}

				out.OrderedItems = append(out.OrderedItems, nn)
			}

			return jsonresp(c, out)
		}

		// No posts in thread
		// Ignoring err because we don't touch the DB
		// There are no replies to care about if there are no posts
		op, _ := fedi.TransformPost(c.Context(), actor, post, fedi.Note{}, false)

		return jsonresp(c, fedi.OrderedNoteCollection{
			Type:         "OrderedCollection",
			TotalItems:   1,
			OrderedItems: []fedi.Note{op},
		})
	}

	// It's a post if we got here
	thread, err := DB.Post(c.Context(), board.ID, post.Thread)
	if err != nil {
		return err
	}

	// Not accessing DB so err doesn't matter
	nthread, _ := fedi.TransformPost(c.Context(), actor, thread, fedi.Note{}, false)

	op, err := fedi.TransformPost(c.Context(), actor, post, nthread, true)
	if err != nil {
		return err
	}

	fmt.Println(post.APID, post.Raw)

	// TODO: Set inReplyTo for thread

	// No replies
	return jsonresp(c, fedi.OrderedNoteCollection{
		Type:         "OrderedCollection",
		TotalItems:   1,
		OrderedItems: []fedi.Note{op},
	})
}

func GetBoardFollowers(c *fiber.Ctx) error {
	return c.JSON(fedi.Followers{
		Context:    fedi.Context,
		Type:       "Collection",
		TotalItems: 0,
		Follower:   []fedi.Follower{},
	})
}
