package fedi

import (
	"context"
	"fmt"

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
	if !p.IsLocal() {
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
			ID:         irt.ID,
			Type:       irt.Type,
			Actor:      irt.Actor,
			NoCollapse: true, // COMPAT: See below
		}

		n.InReplyTo = append(n.InReplyTo, LinkObject(irt))
	} else {
		// COMPAT: FChannel will choke if we send them a post with no inReplyTo.
		// The only time this should be true is when we make a thread.
		// Either way, it comes at hopefully no cost.

		// Create an empty Object.
		n.InReplyTo = append(n.InReplyTo, LinkObject{NoCollapse: true})
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
