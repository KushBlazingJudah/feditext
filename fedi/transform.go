package fedi

import (
	"context"
	"fmt"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
)

func TransformBoard(board database.Board) Actor {
	u := fmt.Sprintf("%s://%s/%s", config.TransportProtocol, config.FQDN, board.ID)

	return Actor{
		ID:                u,
		Type:              "Service",
		Inbox:             u + "/inbox",
		Outbox:            u + "/outbox",
		Following:         u + "/following",
		Followers:         u + "/followers",
		Name:              board.ID,
		PreferredUsername: board.Title,
		Summary:           board.Description,
		Restricted:        true,

		// No support yet:
		// PublicKey:         &PublicKey{},
	}
}

// TransformPost converts our native post structure to ActivityPub's Note.
func TransformPost(ctx context.Context, actor Actor, p database.Post, irt Note, fetchReplies bool) (Note, error) {
	// This part is (database.Post).AsPost backwards
	attTo := p.Name
	if attTo == "Anonymous" {
		// FChannel uses Anonymous by default.
		attTo = ""
	}

	a := actor.ID
	if strings.HasPrefix(p.Source, "http") {
		a = p.Source
	}

	n := Note{
		ID:           p.APID,
		Type:         "Note",
		Actor:        a,
		AttributedTo: attTo,
		Tripcode:     p.Tripcode,
		Subject:      p.Subject,
		Content:      p.Raw, // Don't send already formatted posts
		Published:    p.Date,
		Replies:      nil,

		// We don't bother with Updated.
	}

	if irt.ID != "" {
		// Trim off a lot of the fat.
		irt = Note{
			ID:    irt.ID,
			Type:  irt.Type,
			Actor: irt.Actor,
		}

		n.InReplyTo = append(n.InReplyTo, irt)
	}

	if fetchReplies {
		reps, err := DB.Replies(ctx, actor.Name, p.ID)
		if err != nil {
			return n, err
		}

		if len(reps) > 0 {
			n.Replies = &OrderedNoteCollection{
				Type:       "OrderedCollection",
				TotalItems: len(reps),
			}

			for _, reply := range reps {
				// We throw away the error value as it will always be nil if we don't touch the database.
				// This is true when we tell it to ignore replies.
				rep, _ := TransformPost(ctx, actor, reply, n, false)
				n.Replies.OrderedItems = append(n.Replies.OrderedItems, rep)
			}
		}
	}

	return n, nil
}
