package fedi

import (
	"github.com/KushBlazingJudah/feditext/database"
)

func (n Note) AsPost() database.Post {
	updated := n.Published
	if n.Updated != nil && !n.Updated.IsZero() {
		updated = *n.Updated
	}

	name := n.AttributedTo
	if name == "" {
		name = "Anonymous"
	}

	return database.Post{
		// Thread and ID aren't really possible to fill out, nor should we care.
		// These two just serve as unique identifiers on the database end.
		// We don't really care about them, it determines ordering and that's about it.
		// I'm not sure how FChannel mangles ordering, but this is just how it is.
		// The approach of doing nothing is the simplest and most people won't care too much anyway.

		Name:     name,
		Tripcode: n.Tripcode,
		Subject:  n.Subject,
		Date:     n.Published,
		Bumpdate: updated,
		Raw:      n.Content,
		Source:   n.Actor,
		APID:     n.ID,
	}
}

func (n Note) AsThread() []database.Post {
	var posts []database.Post

	if n.Replies == nil || n.Replies.TotalItems < 1 {
		// No more work needs to be done
		return []database.Post{n.AsPost()}
	}

	posts = make([]database.Post, 0, n.Replies.TotalItems+1) // +1 for OP
	posts = append(posts, n.AsPost())

	for _, note := range n.Replies.OrderedItems[1:] {
		posts = append(posts, note.AsPost())
	}

	return posts
}
