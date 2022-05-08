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

	var pkey *publicKey

	pubKey, err := PublicKey(board.ID)
	if err == nil {
		pkey = &publicKey{
			ID:    u + "#key",
			Owner: u,
			Pem:   pubKey,
		}
	}

	return Actor{
		Object: &Object{
			ID:   u,
			Type: "Group",
			Name: board.ID,

			Summary: board.Description,
		},

		Inbox:             u + "/inbox",
		Outbox:            u + "/outbox",
		Following:         u + "/following",
		Followers:         u + "/followers",
		PreferredUsername: board.Title,
		Restricted:        true,

		PublicKey: pkey,
	}
}

// TransformPost converts our native post structure to ActivityPub's Note.
func TransformPost(ctx context.Context, actor *Actor, p database.Post, irt Object, fetchReplies bool) (Object, error) {
	// This part is (database.Post).AsPost backwards
	var attTo *LinkObject
	if p.Name != "" && p.Name != "Anonymous" {
		// FChannel uses Anonymous by default.
		attTo = &LinkObject{Type: "Link", ID: p.Name}
	}

	a := &LinkActor{Object: &Object{Type: "Group", ID: actor.ID}}
	if strings.HasPrefix(p.Source, "http") {
		a.ID = p.Source
	}

	n := Object{
		ID:           p.APID,
		Type:         "Note",
		AttributedTo: attTo, // FChannel misuses this
		Content:      p.Raw, // Don't send already formatted posts

		Published: &p.Date,

		Replies:  nil,
		Actor:    a,
		Tripcode: p.Tripcode,
		Name:     p.Subject, // don't worry I don't understand either

		// TODO: We don't bother with Updated as it is used as a sage marker for posts.
		// FChannel doesn't even seem to use it all that much except for threads?
	}

	if irt.ID != "" {
		// Trim off a lot of the fat.
		irt = Object{
			ID:    irt.ID,
			Type:  irt.Type,
			Actor: irt.Actor,
		}

		n.InReplyTo = append(n.InReplyTo, LinkObject(irt))
	}

	if fetchReplies {
		reps, err := DB.Replies(ctx, actor.Name, p.ID)
		if err != nil {
			return n, err
		}

		if len(reps) > 0 {
			n.Replies = &OrderedCollection{
				Object:     &Object{Type: "OrderedCollection"},
				TotalItems: len(reps),
			}

			for _, reply := range reps {
				// We throw away the error value as it will always be nil if we don't touch the database.
				// This is true when we tell it to ignore replies.
				rep, _ := TransformPost(ctx, actor, reply, n, false)
				n.Replies.OrderedItems = append(n.Replies.OrderedItems, LinkObject(rep))
			}
		}
	}

	return n, nil
}
