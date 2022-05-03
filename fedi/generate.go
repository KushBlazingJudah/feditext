package fedi

import (
	"context"

	"github.com/KushBlazingJudah/feditext/database"
)

func GenerateOutbox(ctx context.Context, board database.Board) (Outbox, error) {
	actor := TransformBoard(board)

	oc := &OrderedNoteCollection{}
	ob := Outbox{
		Context:               "https://www.w3.org/ns/activitystreams",
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
		n, err := TransformPost(ctx, actor, thread, false)
		if err != nil {
			return ob, err
		}

		posts, err := DB.Thread(ctx, board.ID, thread.ID, 0)
		if err != nil {
			return ob, err
		}

		if l := len(posts); l > 0 {
			n.Replies = &OrderedNoteCollection{
				Type:         "OrderedCollection",
				TotalItems:   l,
				OrderedItems: make([]Note, 0, l),
			}

			for _, post := range posts {
				nn, err := TransformPost(ctx, actor, post, true)
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
