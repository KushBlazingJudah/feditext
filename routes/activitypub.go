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
	"time"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/fedi"
	"github.com/KushBlazingJudah/feditext/util"
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

func errjson(c *fiber.Ctx, err error) error {
	if err == nil {
		panic("nil err passed to errjson")
	}

	if config.Debug {
		log.Printf("error on %s: %s", c.Path(), err)
	}

	if errors.Is(err, sql.ErrNoRows) || strings.HasPrefix(err.Error(), "no such table") {
		_ = c.Status(404).JSON(map[string]string{
			"error": "not found",
		})
	} else if errors.Is(err, database.ErrPostContents) {
		_ = c.Status(400).JSON(map[string]string{
			"error": "invalid post contents",
		})
	} else if errors.Is(err, database.ErrPostRejected) {
		_ = c.Status(400).JSON(map[string]string{
			"error": "post was rejected",
		})
	} else {
		// TODO: More filters.
		// TODO: RSA verification error
		// TODO: JSON
		log.Printf("uncaught error on %s: %s", c.Path(), err)
		_ = c.Status(500).JSON(map[string]string{
			"error": "an internal server error has occurred",
		})
	}

	return err
}

func errjsonc(c *fiber.Ctx, code int, err string) error {
	if config.Debug {
		log.Printf("custom error on %s: %s", c.Path(), err)
	}

	_ = c.Status(code).JSON(map[string]string{
		"error": err,
	})

	return fmt.Errorf("errjsonc: %s", err)
}

func Webfinger(c *fiber.Ctx) error {
	// Shotty implmentation but it works

	query := c.Query("resource")
	if query == "" {
		return errjsonc(c, 400, "need a resource query")
	}

	if !strings.HasPrefix(query, "acct:") {
		return errjsonc(c, 400, "only acct is supported")
	}

	toks := strings.SplitN(query[5:], "@", 2)
	if len(toks) != 2 {
		return errjsonc(c, 404, "not found")
	} else if toks[1] != config.FQDN {
		return errjsonc(c, 404, "not found")
	}

	if board, err := DB.Board(c.Context(), toks[0]); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errjsonc(c, 404, "not found")
		}

		return errjson(c, err)
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
	if err != nil {
		return errjson(c, err)
	}

	act := fedi.Activity{}
	if err := json.Unmarshal(c.Body(), &act); err != nil {
		return errjson(c, err)
	}

	if act.Actor == nil || act.Actor.ID == "" || act.Object == nil {
		return errjsonc(c, 400, "missing attributes")
	}

	log.Printf("received activity from %s: %s", c.IP(), string(c.Body()))

	// Another sanity check
	if err := fedi.CheckHeaders(c, act.Actor.ID); err != nil {
		return errjson(c, err)
	}

	if act.Type == "Follow" {
		if act.ObjectProp == nil {
			return errjsonc(c, 400, "need target")
		}

		// Accept it
		// TODO: Blacklist
		if err := DB.AddFollow(c.Context(), act.Actor.ID, board.ID); err != nil {
			return errjson(c, err)
		}

		// FChannel doesn't send back an Accept, it just is what it is
		if err := DB.AddFollowing(c.Context(), board.ID, act.Actor.ID); err != nil {
			return errjson(c, err)
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
				Actor: &fedi.LinkActor{Object: &fedi.Object{Type: "Group", ID: act.Actor.ID}},
				Type:  "Follow",
			},
		}

		if err := fedi.SendActivity(c.Context(), accept); err != nil {
			return errjson(c, err)
		}
	} else if act.Type == "Create" {
		if act.Object == nil || act.To == nil || act.ObjectProp == nil {
			return errjsonc(c, 400, "missing needed attributes")
		}
		// TODO: Should we ignore from places that aren't marked as following?

		// Do a quick sanity check
		if act.ObjectProp != nil {
			if !util.EqualDomains(act.Actor.ID, act.ObjectProp.ID) {
				// TODO: Reject
				return errjsonc(c, 400, "rejecting; may be spoofed")
			}
		}

		// Check what board it should go to
		// TODO: Improve upon this. It kinda sucks.
		boards := []database.Board{}
		start := fmt.Sprintf("%s://%s/", config.TransportProtocol, config.FQDN)
		for _, t := range act.To {
			if strings.HasPrefix(t.ID, start) {
				// That's us!
				board, err := DB.Board(c.Context(), t.ID[len(start):])
				if err != nil {
					return errjson(c, err)
				}

				boards = append(boards, board)
			}
		}

		if len(boards) == 0 {
			return errjsonc(c, 404, "not found")
		}

		// This does some checking to ensure that the thread exists if it's in reply to one.
		// We don't care about threads we don't know about.
		post, err := act.ObjectProp.AsPost(c.Context(), board.ID)
		if err != nil {
			return errjson(c, err)
		}

		for _, board := range boards {
			if err := DB.SavePost(c.Context(), board.ID, &post); err != nil {
				return errjson(c, err)
			}
			post.ID = 0
		}
	} else if act.Type == "Delete" {
		if act.Object == nil || act.Actor == nil || act.To == nil || act.ObjectProp == nil || act.ObjectProp.ID == "" {
			return errjsonc(c, 400, "missing needed attributes")
		}
		// TODO: Should we ignore from places that aren't marked as following?

		// Check what board it should go to
		// TODO: Improve upon this. It kinda sucks.
		boards := []database.Board{}
		start := fmt.Sprintf("%s://%s/", config.TransportProtocol, config.FQDN)
		for _, t := range act.To {
			if strings.HasPrefix(t.ID, start) {
				// That's us!
				board, err := DB.Board(c.Context(), t.ID[len(start):])
				if err != nil {
					return errjson(c, err)
				}

				boards = append(boards, board)
			}
		}

		if len(boards) == 0 {
			return errjsonc(c, 404, "not found")
		}

		// Check if the post exists in our database.
		post, err := DB.FindAPID(c.Context(), board.ID, act.ObjectProp.ID)
		if err != nil {
			return errjson(c, err)
		}

		if !util.EqualDomains(post.APID, act.Actor.ID) {
			// TODO: Reject
			return errjsonc(c, 403, "attempted to delete object that you don't own")
		}

		// Delete it
		action := database.ModerationAction{
			Author: act.Actor.ID,
			Type:   database.ModActionDelete,
			Board:  board.ID,
			Post:   post.ID,
			Reason: "Externally deleted.",
			Date:   time.Now().UTC(),
		}

		if post.Thread == 0 {
			fmt.Println("delete thread")
			err = DB.DeleteThread(c.Context(), board.ID, post.ID, action)
		} else {
			err = DB.DeletePost(c.Context(), board.ID, post.ID, action)
		}

		if err != nil {
			return errjson(c, err)
		} else {
			return c.SendStatus(200)
		}
	} else { // TODO: FChannel doesn't send back an accept, so assume it's fine?
		log.Printf("%s sent unknown activity type %s", c.IP(), act.Type)

		_, err = os.Stdout.Write(c.Body())
	}

	return err
}

func GetBoardActor(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return errjson(c, err)
	}

	return jsonresp(c, fedi.TransformBoard(board))
}

func GetBoardOutbox(c *fiber.Ctx) error {
	_, board, err := board(c)
	if err != nil {
		return errjson(c, err)
	}

	c.Response().Header.Add("Content-Type", streams)

	outbox, err := fedi.GenerateOutbox(c.Context(), board)
	if err != nil {
		return errjson(c, err)
	}

	return jsonresp(c, outbox)
}

func GetBoardNote(c *fiber.Ctx) error {
	// Correct me if I'm wrong, but I don't think we're supposed to return an OrderedCollection here.
	// Anyways, this is what FChannel does.
	// I thought we should return the Note representation...?

	_, board, err := board(c)
	if err != nil {
		return errjson(c, err)
	}

	actor := fedi.TransformBoard(board)

	var post database.Post

	pid, err := strconv.Atoi(c.Params("thread"))
	if err != nil {
		// It's probably ActivityPub
		q := fmt.Sprintf("%s://%s/%s/%s", config.TransportProtocol, config.FQDN, board.ID, c.Params("thread"))
		post, err = DB.FindAPID(c.Context(), board.ID, q)
		if err != nil {
			return errjson(c, err)
		}
	} else {
		// It's a post number of ours
		post, err = DB.Post(c.Context(), board.ID, database.PostID(pid))
		if err != nil {
			return errjson(c, err)
		}
	}

	// I could probably make this much more dense but eh

	if post.Thread == 0 {
		// It's a thread
		// Fetch the replies and send

		op, err := fedi.TransformPost(c.Context(), &actor, post, fedi.Object{}, false)
		if err != nil {
			return errjson(c, err)
		}

		posts, err := DB.Thread(c.Context(), board.ID, post.ID, 0)
		if err != nil {
			return errjson(c, err)
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
					return errjson(c, err)
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
		return errjson(c, err)
	}

	// Not accessing DB so err doesn't matter
	nthread, _ := fedi.TransformPost(c.Context(), &actor, thread, fedi.Object{}, false)

	op, err := fedi.TransformPost(c.Context(), &actor, post, nthread, true)
	if err != nil {
		return errjson(c, err)
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
	if err != nil {
		return errjson(c, err)
	}

	followers, err := DB.Followers(c.Context(), board.ID)
	if err != nil {
		return errjson(c, err)
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
	if err != nil {
		return errjson(c, err)
	}

	following, err := DB.Following(c.Context(), board.ID)
	if err != nil {
		return errjson(c, err)
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
