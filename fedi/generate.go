package fedi

import (
	"context"
	"strings"

	"github.com/KushBlazingJudah/feditext/database"
)

func GenerateOutbox(ctx context.Context, board database.Board) (Outbox, error) {
	actor := TransformBoard(board)

	ob := Outbox{
		Context: Context,
		Actor:   &actor,
	}

	threads, err := DB.Threads(ctx, board.ID)
	if err != nil {
		return ob, err
	}

	ob.OrderedItems = []LinkObject{}

	for _, thread := range threads {
		if strings.HasPrefix(thread.Source, "http") {
			continue // external, don't put in outbox
		}

		n, err := TransformPost(ctx, &actor, thread, Object{}, false)
		if err != nil {
			return ob, err
		}

		posts, err := DB.Thread(ctx, board.ID, thread.ID, 0)
		if err != nil {
			return ob, err
		}

		if l := len(posts) - 1; l > 0 { // -1 for OP
			n.Replies = &OrderedCollection{
				Object:       &Object{Type: "OrderedCollection"},
				TotalItems:   l,
				OrderedItems: make([]LinkObject, 0, l),
			}

			for _, post := range posts[1:] { // Skip OP
				nn, err := TransformPost(ctx, &actor, post, n, true)
				if err != nil {
					return ob, err
				}

				n.Replies.OrderedItems = append(n.Replies.OrderedItems, LinkObject(nn))
			}
		}

		ob.OrderedItems = append(ob.OrderedItems, LinkObject(n))
	}

	ob.TotalItems = len(ob.OrderedItems)
	return ob, nil
}
