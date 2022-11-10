package hook

import (
	"context"
	"time"
)

// All database.Post types are written as "interface{}" to prevent an import cycle.
// Shit fix, I know, but it works.

type PostCreatePayload struct {
	Board string      `json:"board"`
	Post  interface{} `json:"post"`
}

type PostDeletePayload struct {
	Board  string      `json:"board"`
	Post   interface{} `json:"post"`
	Reason string      `json:"reason"`
	Actor  string      `json:"actor"`
}

func PostCreate(ctx context.Context, board string, p interface{}) {
	pl := Payload{
		ID:   Created,
		Date: time.Now(),
		Data: PostCreatePayload{
			Board: board,
			Post:  p,
		},
	}

	call(ctx, pl)
}

func PostDelete(ctx context.Context, board string, p interface{}, actor, reason string) {
	pl := Payload{
		ID:   Deleted,
		Date: time.Now(),
		Data: PostDeletePayload{
			Board:  board,
			Post:   p,
			Actor:  actor,
			Reason: reason,
		},
	}

	call(ctx, pl)
}
