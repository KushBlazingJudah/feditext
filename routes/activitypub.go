package routes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/fedi"
	"github.com/gofiber/fiber/v2"
)

const streams = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

// False positive for application/ld+ld, application/activity+ld, application/json+json
var streamsRegex = regexp.MustCompile(`application/(ld|json|activity)\+(ld|json)`)

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

			for _, post := range thread { // Shadow post
				nn, err := fedi.TransformPost(c.Context(), actor, post, true)
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
		op, _ := fedi.TransformPost(c.Context(), actor, post, false)

		return jsonresp(c, fedi.OrderedNoteCollection{
			Type:         "OrderedCollection",
			TotalItems:   1,
			OrderedItems: []fedi.Note{op},
		})
	}

	// It's a post if we got here

	op, err := fedi.TransformPost(c.Context(), actor, post, true)
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
