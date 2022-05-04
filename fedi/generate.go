package fedi

import (
	"context"
	"strings"

	"github.com/KushBlazingJudah/feditext/database"
)

func GenerateOutbox(ctx context.Context, board database.Board) (Outbox, error) {
	actor := TransformBoard(board)

	oc := &OrderedNoteCollection{Object: Object{Type: "OrderedCollection"}}
	ob := Outbox{
		Object:                Object{Context: "https://www.w3.org/ns/activitystreams"},
		Actor:                 actor,
		OrderedNoteCollection: oc,
	}

	threads, err := DB.Threads(ctx, board.ID)
	if err != nil {
		return ob, err
	}

	oc.TotalItems = len(threads)
	oc.OrderedItems = make([]Note, 0, oc.TotalItems)

	for _, thread := range threads {
		if strings.HasPrefix(thread.Source, "http") {
			continue // external, don't put in outbox
		}

		n, err := TransformPost(ctx, actor, thread, Note{}, false)
		if err != nil {
			return ob, err
		}

		posts, err := DB.Thread(ctx, board.ID, thread.ID, 0)
		if err != nil {
			return ob, err
		}

		if l := len(posts) - 1; l > 0 { // -1 for OP
			n.Replies = &OrderedNoteCollection{
				Object:       Object{Type: "OrderedCollection"},
				TotalItems:   l,
				OrderedItems: make([]Note, 0, l),
			}

			for _, post := range posts[1:] { // Skip OP
				nn, err := TransformPost(ctx, actor, post, n, true)
				if err != nil {
					return ob, err
				}

				n.Replies.OrderedItems = append(n.Replies.OrderedItems, nn)
			}
		}

		oc.OrderedItems = append(oc.OrderedItems, n)
	}

	return ob, nil
}
