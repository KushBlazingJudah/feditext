package fedi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/KushBlazingJudah/feditext/database"
	"github.com/KushBlazingJudah/feditext/util"
)

func (n Object) AsPost(ctx context.Context, board string) (database.Post, error) {
	if n.Type != "Note" {
		// We need an actor to save.
		return database.Post{}, fmt.Errorf("Object.AsPost: invalid type; expected Note, got %s", n.Type)
	}

	published := time.Now().UTC()
	if n.Published != nil && !n.Published.IsZero() {
		published = *n.Published
	}

	var updated time.Time
	if util.Has("sage", n.Options) {
		// TODO: This can be used in threads, however the database methods
		// (which should ignore this value on new threads) save us here.
		updated = time.Time{}
	} else if n.Updated != nil && !n.Updated.IsZero() {
		updated = *n.Updated
	} else {
		updated = time.Now().UTC()
	}

	name := ""
	if n.AttributedTo == nil || n.AttributedTo.ID == "" {
		name = "Anonymous"
	} else {
		name = n.AttributedTo.ID
	}

	actor := ""
	if n.Actor == nil || n.Actor.ID == "" {
		// We need an actor to save.
		return database.Post{}, fmt.Errorf("Object.AsPost: no actor")
	}
	actor = n.Actor.ID

	// The post coming in may have a thread attached to it.
	// Look for it.
	thread := database.PostID(0)
	ok := false

	if len(n.InReplyTo) > 0 {
		for _, t := range n.InReplyTo {
			if t.ID != "" {
				// Check if it exists in the database
				th, err := DB.FindAPID(ctx, board, t.ID)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return database.Post{}, err
				} else if errors.Is(err, sql.ErrNoRows) {
					continue
				}

				if th.Thread == 0 {
					thread = th.ID
					ok = true
					break
				}
			} else {
				// FChannel implementation bug
				thread = 0
				ok = true
				break
			}
		}
	} else {
		// Not in reply to anything so it's obviously a thread
		thread = 0
		ok = true
	}

	if !ok {
		// No post in the database, ignore!
		return database.Post{}, fmt.Errorf("no suitable thread in the database to reply to, ignoring")
	}

	// TODO: Sanitize

	return database.Post{
		// Thread and ID aren't really possible to fill out, nor should we care.
		// These two just serve as unique identifiers on the database end.
		// We don't really care about them, it determines ordering and that's about it.
		// I'm not sure how FChannel mangles ordering, but this is just how it is.
		// The approach of doing nothing is the simplest and most people won't care too much anyway.

		Name:     name,
		Thread:   thread,
		Tripcode: n.Tripcode,
		Subject:  n.Name,
		Date:     published,
		Bumpdate: updated,
		Raw:      n.Content,
		Source:   actor,
		APID:     n.ID,
	}, nil
}

func (n Object) AsThread(ctx context.Context, board string) ([]database.Post, error) {
	var posts []database.Post

	if n.Replies == nil || n.Replies.TotalItems < 1 {
		// No more work needs to be done
		p, err := n.AsPost(ctx, board)
		return []database.Post{p}, err
	}

	op, err := n.AsPost(ctx, board)
	if err != nil {
		return nil, err
	}

	posts = make([]database.Post, 0, n.Replies.TotalItems+1) // +1 for OP
	posts = append(posts, op)

	for _, note := range n.Replies.OrderedItems[1:] {
		// Kill InReplyTo here, we don't need it and it only makes things more painful.
		note.InReplyTo = nil // "whoops"
		if nnn, err := Object(note).AsPost(ctx, board); err != nil {
			log.Printf("error importing %s: %s", note.ID, err)
			continue
		} else {
			posts = append(posts, nnn)
		}
	}

	return posts, nil
}
