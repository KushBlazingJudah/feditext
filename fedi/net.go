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
	"regexp"
	"sync"
	"time"

	"github.com/KushBlazingJudah/feditext/crypto"
	"github.com/KushBlazingJudah/feditext/database"
)

var P Proxy = NullProxy{}

const streams = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

var wfRegex = regexp.MustCompile(`(https?):\/\/([0-9a-z\-\.]*\.[0-9a-z]+(?::\d+)?)\/([0-9a-z]+)`)
var webfingerCache = map[string]Actor{}

type Proxy interface {
	Request(ctx context.Context, method, url string, body io.Reader) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
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

func (n NullProxy) Do(req *http.Request) (*http.Response, error) {
	return n.client.Do(req)
}

type finger struct {
	Links []struct {
		Rel  string
		Type string
		Href string
	}
}

func Finger(ctx context.Context, actor string) (Actor, error) {
	// Get from cache if at all possible
	if a, ok := webfingerCache[actor]; ok {
		return a, nil
	}

	// Assumes that the actor is in form of https?://instance/actor.
	match := wfRegex.FindStringSubmatch(actor)
	if match == nil || len(match) != 4 {
		return Actor{}, fmt.Errorf("Finger: invalid format; %v", match)
	}
	tp, host, id := match[1], match[2], match[3]

	uri := fmt.Sprintf("%s://%s", tp, host)
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/.well-known/webfinger?resource=acct:%s@%s", uri, id, host), nil)
	if err != nil {
		return Actor{}, err
	}

	res, err := P.Do(req)
	if err != nil {
		return Actor{}, err
	}

	finger := finger{}
	decoder := json.NewDecoder(res.Body)
	if err := decoder.Decode(&finger); err != nil {
		res.Body.Close()
		return Actor{}, err
	}
	res.Body.Close()

	// Check everything we were sent
	target := ""
	for _, link := range finger.Links {
		// Maybe check more? I don't know.
		// This is one of the few spots where I look at FChannel for an implementation.
		if link.Type == "application/activity+json" {
			// I'm also unsure about this, as this gives it an arbitrary link.
			// Good? Maybe. I'm not sure.
			target = link.Href
			break
		}
	}

	if target == "" {
		return Actor{}, fmt.Errorf("Finger: no suitable response")
	}

	// Finally, do one more request to the server.
	req, err = http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return Actor{}, err
	}

	req.Header.Set("Accept", streams)

	res, err = P.Do(req)
	if err != nil {
		return Actor{}, err
	}
	defer res.Body.Close()

	act := Actor{}
	decoder = json.NewDecoder(res.Body)
	if err := decoder.Decode(&act); err != nil {
		return act, err
	}

	// Throw it into the cache now that we have it
	// This saves two queries to a site
	webfingerCache[actor] = act

	return act, nil
}

func SendActivity(ctx context.Context, act Activity) error {
	if act.Actor == nil || act.Actor.PublicKey == nil {
		return fmt.Errorf("invalid activity; missing actor or public key")
	}

	data, err := json.Marshal(act)
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}

	for _, to := range act.To {
		if to.Type != "Link" {
			continue
		}

		actor, err := Finger(ctx, to.ID)
		if err != nil {
			log.Printf("failed to finger %s: %v", to.ID, err)
			continue
		}

		if actor.Inbox != "" {
			req, err := http.NewRequest("POST", actor.Inbox, bytes.NewBuffer(data))
			if err != nil {
				log.Printf("unable to generate request for %s: %v", to.ID, err)
				continue
			}

			u, err := url.Parse(actor.Inbox)
			if err != nil {
				log.Printf("failed to parse inbox url for %s: %v", to.ID, err)
				continue
			}

			date := time.Now().UTC().Format(time.RFC1123)
			data := fmt.Sprintf("(request-target): post %s\nhost: %s\ndate: %s", u.Path, u.Host, date)

			sig, err := crypto.Sign(act.Actor.Name, data) // TODO: Bad.
			if err != nil {
				return err
			}

			req.Header.Set("Content-Type", streams)
			req.Header.Set("Date", date)
			req.Header.Set("Signature", fmt.Sprintf(`keyId="%s",headers="(request-target) host date",signature="%s"`, act.Actor.PublicKey.ID, sig))
			req.Host = u.Host

			wg.Add(1)

			go func() {
				_, err = P.Do(req)
				if err != nil {
					log.Printf("failed sending activity to %s: %v", to.ID, err)
				}
				wg.Done()
			}()
		}
	}

	wg.Wait()

	return nil
}

// PostOut sends a post out to federated servers.
func PostOut(ctx context.Context, board database.Board, post database.Post) error {
	actor := TransformBoard(board)
	lactor := LinkActor(actor)

	irt := Object{}
	if post.Thread != 0 {
		thread, err := DB.Post(ctx, board.ID, post.Thread)
		if err != nil {
			return err
		}

		irt.Type = "Note"
		irt.ID = thread.APID
	}

	note, err := TransformPost(ctx, &actor, post, irt, false)
	if err != nil {
		return err
	}

	followers, err := DB.Followers(ctx, board.ID)
	if err != nil {
		return err
	}

	flo := make([]LinkObject, 0, len(followers))
	for _, follower := range followers {
		flo = append(flo, LinkObject{Type: "Link", ID: follower})
	}

	activity := Activity{
		Object: &Object{
			Context: Context,
			Type:    "Create",
			Actor:   &lactor,
			To:      flo,
		},

		ObjectProp: &note,
	}

	return SendActivity(ctx, activity)
}
