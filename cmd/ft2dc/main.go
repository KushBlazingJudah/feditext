package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/etaaa/go-webhooks"
)

var (
	addr = flag.String("addr", "localhost:8081", "address to listen on")
	wh   = os.Getenv("WEBHOOK_URL")
)

type payload struct {
	ID   string
	Date time.Time
	Data map[string]interface{}
}

func main() {
	s := http.Server{
		Addr: *addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pl := payload{}
			if err := json.NewDecoder(io.TeeReader(r.Body, os.Stdout)).Decode(&pl); err != nil {
				panic(err)
			}

			if pl.ID == "post.created" {
				pd := pl.Data["post"].(map[string]interface{})
				d := webhooks.Webhook{
					Content:  pd["raw"].(string),
					Username: fmt.Sprintf("/%s/ id %.0f thread %.0f - %s", pl.Data["board"], pd["id"], pd["thread"], pd["source"]),
				}

				webhooks.SendWebhook(wh, d, true)
			}
		}),
	}

	log.Fatal(s.ListenAndServe())
}
