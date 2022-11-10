package hook

import (
	"context"
	"sync"
	"time"
)

const Created = "post.created"
const Deleted = "post.created"

var Hooks []Hook

type Hook interface {
	Call(context.Context, Payload)
}

type Payload struct {
	ID   string      `json:"id"`
	Date time.Time   `json:"date"`
	Data interface{} `json:"data"`
}

func call(ctx context.Context, p Payload) {
	wg := sync.WaitGroup{}

	for _, v := range Hooks {
		wg.Add(1)
		go func(h Hook) {
			defer wg.Done()
			h.Call(ctx, p)
		}(v)
	}

	wg.Wait()
}
