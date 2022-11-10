package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

type WebHook struct {
	Endpoint string
}

func (w *WebHook) Call(ctx context.Context, p Payload) {
	b := &bytes.Buffer{}

	for i := 0; i < 3; i++ {
		b.Reset()
		if err := json.NewEncoder(b).Encode(p); err != nil {
			panic(err)
		}

		_, err := http.Post(w.Endpoint, "application/json", b)
		if err != nil {
			continue
		}
		break
	}
}
