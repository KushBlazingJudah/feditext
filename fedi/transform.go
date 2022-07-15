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
func TransformPost(ctx context.Context, actor *Actor, p database.Post, irt Object, fetchReplies bool, fetchInReplyTo bool) (Object, error) {
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

	bd := &p.Bumpdate
	if bd.IsZero() || bd.Unix() == 0 || bd.Unix() == 1 {
		// Unix time is because we used to record 0/1 for bumpdate.
		bd = nil
	}

	n := Object{
		ID:           p.APID,
		Type:         "Note",
		AttributedTo: attTo, // FChannel misuses this
		Content:      p.Raw, // Don't send already formatted posts

		Published: &p.Date,
		Updated:   bd,

		Replies:  nil,
		Actor:    a,
		Tripcode: p.Tripcode,
		Name:     p.Subject, // don't worry I don't understand either
	}

	if irt.ID != "" {
		// Trim off a lot of the fat.
		irt = Object{
			ID:         irt.ID,
			Type:       irt.Type,
			Actor:      irt.Actor,
			NoCollapse: true, // COMPAT: See a bit further below
		}

		n.InReplyTo = append(n.InReplyTo, LinkObject(irt))
	}

	if fetchInReplyTo {
		reps, err := DB.Replies(ctx, actor.Name, p.ID, true)
		if err != nil {
			return n, err
		}

		if len(reps) > 0 {
			for _, reply := range reps {
				// Toss if this is already in InReplyTo
				for _, v := range n.InReplyTo {
					if v.ID == reply.APID {
						goto next
					}
				}
				// We throw away the error value as it will always be nil if we don't touch the database.
				// This is true when we tell it to ignore replies.
				rep, _ := TransformPost(ctx, actor, reply, n, false, false)

				// Also kill some other properties we don't care about.
				// They can stay there, but to save CPU cycles...
				// Ideally, I'd toss away Content too and just get by with
				// LinkObjects but pretty sure FChannel needs the struct.
				// Pretty sure it does anyway.
				rep.InReplyTo = nil
				rep.Replies = nil
				rep.Updated = nil
				rep.Published = nil

				n.InReplyTo = append(n.InReplyTo, LinkObject(rep))
				next:
			}
		}
	}

	if len(n.InReplyTo) == 0 {
		// COMPAT: FChannel will choke if we send them a post with no inReplyTo.
		// The only time this should be true is when we make a thread.
		// Either way, it comes at hopefully no cost.

		// Create an empty Object.
		n.InReplyTo = append(n.InReplyTo, LinkObject{NoCollapse: true})
	}

	if fetchReplies {
		var reps []database.Post
		var err error = nil
		if len(p.Replies) > 0 {
			// Replies have already been passed to us.
			reps = p.Replies
		} else {
			reps, err = DB.Replies(ctx, actor.Name, p.ID, false)
			if err != nil {
				return n, err
			}
		}

		if len(reps) > 0 {
			n.Replies = &OrderedCollection{
				Object:     &Object{Type: "OrderedCollection"},
				TotalItems: len(reps),
			}

			for _, reply := range reps {
				// We throw away the error value as it will always be nil if we don't touch the database.
				// This is true when we tell it to ignore replies.
				rep, _ := TransformPost(ctx, actor, reply, n, false, false)

				// Also kill some other properties we don't care about.
				// They can stay there, but to save CPU cycles...
				// Ideally, I'd toss away Content too and just get by with
				// LinkObjects but pretty sure FChannel needs the struct.
				// Pretty sure it does anyway.
				rep.InReplyTo = nil
				rep.Replies = nil
				rep.Updated = nil
				rep.Published = nil

				n.Replies.OrderedItems = append(n.Replies.OrderedItems, LinkObject(rep))
			}
		}
	}

	return n, nil
}
