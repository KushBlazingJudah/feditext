package fedi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/KushBlazingJudah/feditext/crypto"
)

var P Proxy

const streams = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

type Proxy interface {
	Request(ctx context.Context, method, url string, body io.Reader) (*http.Response, error)
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}

type NullProxy struct {
	client http.Client
}

func (n NullProxy) Request(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	return n.client.Do(req)
}

func (n NullProxy) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return n.client.Do(req)
}

func Finger(ctx context.Context, actor string) (Actor, error) {
	return Actor{}, nil
}

func SendActivity(ctx context.Context, act Activity) error {
	if act.Actor == nil || act.Actor.PublicKey == nil {
		return fmt.Errorf("invalid activity; missing actor or public key")
	}

	data, err := json.Marshal(act)
	if err != nil {
		return err
	}

	for _, to := range act.To {
		if to.Type != "Link" {
			continue
		}

		actor, err := Finger(ctx, to.ID)
		if err != nil {
			log.Printf("failed to finger %s: %v", to, err)
			continue
		}

		if actor.Inbox != "" {
			req, err := http.NewRequest("POST", actor.Inbox, bytes.NewBuffer(data))
			if err != nil {
				log.Printf("unable to generate request for %s: %v", to, err)
				continue
			}

			u, err := url.Parse(actor.Inbox)
			if err != nil {
				log.Printf("failed to parse inbox url for %s: %v", to, err)
				continue
			}

			date := time.Now().UTC().Format(time.RFC1123)
			data := fmt.Sprintf("(request-target): post %s\nhost: %s\ndate: %s\n", u.Path, u.Host, date)

			sig, err := crypto.Sign(act.Actor.Name, data) // TODO: Bad.
			if err != nil {
				return err
			}

			req.Header.Set("Content-Type", streams)
			req.Header.Set("Date", date)
			req.Header.Set("Signature", fmt.Sprintf(`keyId="%s",headers="(request-target) host date",signature="%s"`, act.Actor.PublicKey.ID, sig))
			req.Host = u.Host

			_, err = P.Do(ctx, req)
			if err != nil {
				log.Printf("failed sending activity to %s: %v", to, err)
				continue
			}
		}
	}

	return nil
}
