package routes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/crypto"
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

func PostBoardInbox(c *fiber.Ctx) error {
	_, board, err := board(c)
	if board.ID == "" || err != nil {
		return err
	}

	for k, v := range c.GetReqHeaders() {
		fmt.Printf("%s: %s\n", k, v)
	}
	_, err = os.Stdout.Write(c.Body())

	act := fedi.Activity{}
	if err := json.Unmarshal(c.Body(), &act); err != nil {
		return err
	}

	if act.Actor == nil || act.Actor.ID == "" {
		return fmt.Errorf("need actor")
	} else if act.Actor.PublicKey == nil {
		// TODO: Webfinger
		return fmt.Errorf("need public key")
	} else if act.Actor.PublicKey.Pem == "" {
		// TODO: Webfinger
		return fmt.Errorf("need public key data")
	}

	if err := crypto.CheckHeaders(c, act.Actor.PublicKey.Pem); err != nil {
		return err
	}

	if act.Type == "Follow" {
		if act.ObjectProp == nil {
			return fmt.Errorf("need target")
		}

		// Accept it
		// TODO: Blacklist
		if err := DB.AddFollow(c.Context(), act.Actor.ID, board.ID); err != nil {
			return err
		}

		// FChannel doesn't send back an Accept, it just is what it is
		if err := DB.AddFollowing(c.Context(), board.ID, act.Actor.ID); err != nil {
			return err
		}

		log.Printf("Accepted follow from %s to board %s", act.Actor.ID, board.ID)

		b := fedi.LinkActor(fedi.TransformBoard(board))
		b.NoCollapse = true // FChannel doesn't understand
		accept := fedi.Activity{
			Object: &fedi.Object{
				Context: fedi.Context,
				Type:    "Accept",
				Actor:   &b,
				To:      []fedi.LinkObject{{Type: "Link", ID: act.Actor.ID}},
			},

			ObjectProp: &fedi.Object{
				Actor:      &fedi.LinkActor{Object: &fedi.Object{Type: "Group", ID: act.Actor.ID}},
				Type:       "Follow",
				NoCollapse: true,
			},
		}

		if err := fedi.SendActivity(c.Context(), accept); err != nil {
			return err
		}
	} else if act.Type == "Create" {
		if act.Object == nil || act.Actor == nil || act.To == nil || act.ObjectProp == nil {
			return c.SendStatus(400) // TODO
		}
		// TODO: Should we ignore from places that aren't marked as following?
		// TODO: Check for spoofing?

		// Check what board it should go to
		// TODO: Improve upon this. It kinda sucks.
		boards := []database.Board{}
		start := fmt.Sprintf("%s://%s/", config.TransportProtocol, config.FQDN)
		for _, t := range act.To {
			if strings.HasPrefix(t.ID, start) {
				// That's us!
				board, err := DB.Board(c.Context(), t.ID[len(start):])
				if err != nil {
					return err
				}

				boards = append(boards, board)
			}
		}

		if len(boards) == 0 {
			return c.SendStatus(404) // TODO
		}

		post, err := act.ObjectProp.AsPost(c.Context(), board.ID)
		if err != nil {
			return err
		}

		for _, board := range boards {
			if err := DB.SavePost(c.Context(), board.ID, &post); err != nil {
				return err
			}
			post.ID = 0
		}
	} else { // TODO: FChannel doesn't send back an accept, so assume it's fine?
		log.Printf("%s sent unknown activity type %s", c.IP(), act.Type)
	}

	return err
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
	// Correct me if I'm wrong, but I don't think we're supposed to return an OrderedCollection here.
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

		op, err := fedi.TransformPost(c.Context(), &actor, post, fedi.Object{}, false)
		if err != nil {
			return err
		}

		posts, err := DB.Thread(c.Context(), board.ID, post.ID, 0)
		if err != nil {
			return err
		}

		if l := len(posts) - 1; l > 0 {
			op.Replies = &fedi.OrderedCollection{
				Object:       &fedi.Object{Type: "OrderedCollection"},
				TotalItems:   l,
				OrderedItems: make([]fedi.LinkObject, 0, l),
			}

			for _, post := range posts[1:] {
				p, err := fedi.TransformPost(c.Context(), &actor, post, op, true)
				if err != nil {
					return err
				}

				op.Replies.OrderedItems = append(op.Replies.OrderedItems, fedi.LinkObject(p))
			}
		}

		return jsonresp(c, fedi.OrderedCollection{
			Object:       &fedi.Object{Type: "OrderedCollection"},
			TotalItems:   1,
			OrderedItems: []fedi.LinkObject{fedi.LinkObject(op)},
		})
	}

	// It's a post if we got here
	thread, err := DB.Post(c.Context(), board.ID, post.ID)
	if err != nil {
		return err
	}

	// Not accessing DB so err doesn't matter
	nthread, _ := fedi.TransformPost(c.Context(), &actor, thread, fedi.Object{}, false)

	op, err := fedi.TransformPost(c.Context(), &actor, post, nthread, true)
	if err != nil {
		return err
	}

	// No replies
	return jsonresp(c, fedi.OrderedCollection{
		Object:       &fedi.Object{Type: "OrderedCollection"},
		TotalItems:   1,
		OrderedItems: []fedi.LinkObject{fedi.LinkObject(op)},
	})
}

func GetBoardFollowers(c *fiber.Ctx) error {
	_, board, err := board(c)
	if board.ID == "" || err != nil {
		return err
	}

	followers, err := DB.Followers(c.Context(), board.ID)
	if err != nil {
		return err
	}

	f := make([]fedi.LinkObject, 0, len(followers))

	for _, i := range followers {
		f = append(f, fedi.LinkObject{Type: "Actor", ID: i}) // TODO: What type goes here?
		// Seems to me like Mastodon just uses links here.
		// Done this way solely to prevent it from doing that.
	}

	return c.JSON(fedi.Collection{
		Object:     &fedi.Object{Context: fedi.Context, Type: "Collection"},
		TotalItems: len(f),
		Items:      f,
	})
}

func GetBoardFollowing(c *fiber.Ctx) error {
	_, board, err := board(c)
	if board.ID == "" || err != nil {
		return err
	}

	following, err := DB.Following(c.Context(), board.ID)
	if err != nil {
		return err
	}

	f := make([]fedi.LinkObject, 0, len(following))

	for _, i := range following {
		// See GetBoardFollowers.
		f = append(f, fedi.LinkObject{Type: "Actor", ID: i})
	}

	return c.JSON(fedi.Collection{
		Object:     &fedi.Object{Context: fedi.Context, Type: "Collection"},
		TotalItems: len(f),
		Items:      f,
	})
}
