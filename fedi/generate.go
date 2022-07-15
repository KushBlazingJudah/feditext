package fedi

import (
	"context"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
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

func GenerateFollow(ctx context.Context, board database.Board, to string) (Activity, error) {
	b := LinkActor(TransformBoard(board))
	b.NoCollapse = true // FChannel doesn't understand
	follow := Activity{
		Object: &Object{
			Context: Context,
			Type:    "Follow",
			Actor:   &b,
			To:      []LinkObject{{Type: "Link", ID: to}},
		},

		ObjectProp: &Object{
			Actor:      &LinkActor{Object: &Object{Type: "Group", ID: to}},
			NoCollapse: true,
		},
	}

	return follow, nil
}

func GenerateUnfollow(ctx context.Context, board database.Board, to string) (Activity, error) {
	// The rest of this function uses an Undo Follow, however FChannel
	// currently only supports toggling on and off with Follow.
	if !config.UnstableUnfollow {
		return GenerateFollow(ctx, board, to)
	}

	b := LinkActor(TransformBoard(board))
	b.NoCollapse = true // FChannel doesn't understand
	c := b
	c.NoCollapse = false // need it to collapse though
	unfollow := Activity{
		Object: &Object{
			Context: Context,
			Type:    "Undo",
			Actor:   &b,
			To:      []LinkObject{{Type: "Link", ID: to}},
		},

		ObjectProp: &Object{
			Type:  "Follow",
			Actor: &c,
			To:    []LinkObject{{Type: "Link", ID: to}},
		},
	}

	return unfollow, nil
}

func GenerateAccept(ctx context.Context, board database.Board, to string, obj *Object) (Activity, error) {
	b := LinkActor(TransformBoard(board))
	b.NoCollapse = true // FChannel doesn't understand
	accept := Activity{
		Object: &Object{
			Context: Context,
			Type:    "Accept",
			Actor:   &b,
			To:      []LinkObject{{Type: "Link", ID: to}},
		},

		ObjectProp: obj,
	}

	return accept, nil
}
