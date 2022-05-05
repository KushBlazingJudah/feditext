package fedi

import (
	"fmt"
	"log"
	"time"

	"github.com/KushBlazingJudah/feditext/database"
)

func (n Object) AsPost() (database.Post, error) {
	if n.Type != "Note" {
		// We need an actor to save.
		return database.Post{}, fmt.Errorf("Object.AsPost: invalid type; expected Note, got %s", n.Type)
	}

	published := time.Now()
	if n.Published != nil && !n.Published.IsZero() {
		published = *n.Updated
	}

	updated := time.Now()
	if n.Updated != nil && !n.Updated.IsZero() {
		updated = *n.Updated
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

	// TODO: Sanitize

	return database.Post{
		// Thread and ID aren't really possible to fill out, nor should we care.
		// These two just serve as unique identifiers on the database end.
		// We don't really care about them, it determines ordering and that's about it.
		// I'm not sure how FChannel mangles ordering, but this is just how it is.
		// The approach of doing nothing is the simplest and most people won't care too much anyway.

		Name:     name,
		Tripcode: n.Tripcode,
		Subject:  n.Subject,
		Date:     published,
		Bumpdate: updated,
		Raw:      n.Content,
		Source:   actor,
		APID:     n.ID,
	}, nil
}

func (n Object) AsThread() ([]database.Post, error) {
	var posts []database.Post

	if n.Replies == nil || n.Replies.TotalItems < 1 {
		// No more work needs to be done
		p, err := n.AsPost()
		return []database.Post{p}, err
	}

	op, err := n.AsPost()
	if err != nil {
		return nil, err
	}

	posts = make([]database.Post, 0, n.Replies.TotalItems+1) // +1 for OP
	posts = append(posts, op)

	for _, note := range n.Replies.OrderedItems[1:] {
		if note, err := Object(note).AsPost(); err != nil {
			log.Printf("error importing %s: %s", note.ID, err)
			continue
		} else {
			posts = append(posts, note)
		}
	}

	return posts, nil
}
