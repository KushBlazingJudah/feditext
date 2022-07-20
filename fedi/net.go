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
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/KushBlazingJudah/feditext/database"
)

const streams = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`

var wfRegex = regexp.MustCompile(`(https?):\/\/([0-9a-z\-\.]*\.[0-9a-z]+(?::\d+)?)\/([0-9a-z]+)`)
var webfingerCache = map[string]Actor{}

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
		return Actor{}, fmt.Errorf("Finger: invalid format; %s", actor)
	}
	tp, host, id := match[1], match[2], match[3]

	uri := fmt.Sprintf("%s://%s", tp, host)
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/.well-known/webfinger?resource=acct:%s@%s", uri, id, host), nil)
	if err != nil {
		return Actor{}, err
	}

	res, err := Proxy.Do(req)
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

	res, err = Proxy.Do(req)
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
	if len(act.To) == 0 {
		// There's nothing to do
		return nil
	}

	if act.Actor == nil || act.Actor.PublicKey == nil {
		return fmt.Errorf("invalid activity; missing actor or public key")
	}

	data, err := json.Marshal(act)
	if err != nil {
		return err
	}

	if config.Debug {
		log.Printf("sending an activity of type %s to %d different actors", act.Type, len(act.To))
		log.Printf("marshalled json for activity: %s", string(data))
	}

	wg := sync.WaitGroup{}

	for _, to := range act.To {
		if to.Type != "Link" {
			continue
		}

		// Reasonable amount of time for everything here to complete.
		ctx, cancel := context.WithTimeout(ctx, config.MaxReqTime)
		defer cancel()

		actor, err := Finger(ctx, to.ID)
		if err != nil {
			log.Printf("failed to finger %s: %v", to.ID, err)
			continue
		}

		if actor.Inbox != "" {
			req, err := http.NewRequestWithContext(ctx, "POST", actor.Inbox, bytes.NewBuffer(data))
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

			sig, err := Sign(act.Actor.Name, data) // TODO: Bad.
			if err != nil {
				return err
			}

			req.Header.Set("Content-Type", streams)
			req.Header.Set("Date", date)
			req.Header.Set("Signature", fmt.Sprintf(`keyId="%s",headers="(request-target) host date",signature="%s"`, act.Actor.PublicKey.ID, sig))
			req.Host = u.Host

			wg.Add(1)
			go func(to LinkObject) {
				res, err := Proxy.Do(req)
				if err != nil {
					log.Printf("failed sending activity to %s: %v", to.ID, err)
					return
				}

				if res.Body != nil {
					// Can be nil in some cases
					defer res.Body.Close()
				}

				if res.StatusCode != 200 {
					log.Printf("failed sending activity to %s: non-200 status code %d", to.ID, res.StatusCode)

					if config.Debug && res.Body != nil {
						// Write to stderr
						fmt.Fprintf(os.Stderr, "Response body (at most 4096 bytes) for failure on %s for %s follows", act.Object.ID, to.ID)
						io.Copy(os.Stderr, io.LimitReader(res.Body, 4096))
						fmt.Fprint(os.Stderr, "\n")
					}
				}

				wg.Done()
			}(to)
		}
	}

	wg.Wait()

	return nil
}

func FetchOutbox(ctx context.Context, actorUrl string) (Outbox, error) {
	actor, err := Finger(ctx, actorUrl)
	if err != nil {
		return Outbox{}, err
	}

	if actor.Outbox == "" {
		return Outbox{}, fmt.Errorf("actor returned no outbox")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", actor.Outbox, nil)
	if err != nil {
		return Outbox{}, err
	}

	res, err := Proxy.Do(req)
	if err != nil {
		return Outbox{}, err
	}
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	outbox := Outbox{}
	err = decoder.Decode(&outbox)
	return outbox, err
}

func MergeOutbox(ctx context.Context, board string, ob Outbox) error {
	for _, thread := range ob.OrderedItems {
		if thread.Type != "Note" {
			log.Printf("encountered unknown type %s in outbox", thread.Type)
			continue
		}

		t, err := Object(thread).AsThread(ctx, board)
		if err != nil {
			log.Printf("error converting object %s to thread: %s", thread.ID, err)
			continue
		}

		// Import it into the database
		op := t[0]

		// Check if we have the OP already in the database
		if post, err := DB.FindAPID(ctx, board, op.APID); err != nil {
			// We (probably) don't.
			if err := DB.SavePost(ctx, board, &op); err != nil {
				// Log it to the console.
				log.Printf("unable to import %s: %s", op.APID, err)
				continue
			}
		} else {
			// We do have it in the database so we can ignore the first one.
			op = post
		}

		for _, post := range t[1:] {
			post.Thread = op.ID

			// First, check if it's in the database.
			// We'll save it if it isn't.
			if _, err := DB.FindAPID(ctx, board, post.APID); err != nil {
				// We're probably safe to save it into the database.
				// Most likely fatal if it isn't.
				if err := DB.SavePost(ctx, board, &post); err != nil {
					log.Printf("unable to save %s: %s", post.APID, err)
					continue
				}

				if config.Debug {
					log.Printf("added %s to %s", post.APID, board)
				}
			}

		}
	}

	return nil
}

func activityBase(ctx context.Context, board database.Board) (Activity, error) {
	actor := TransformBoard(board)
	actor.NoCollapse = true
	lactor := LinkActor(actor)

	followers, err := DB.Followers(ctx, board.ID)
	if err != nil {
		return Activity{}, err
	}

	flo := make([]LinkObject, 0, len(followers))
	for _, follower := range followers {
		flo = append(flo, LinkObject{Type: "Link", ID: follower})
	}

	return Activity{
		Object: &Object{
			Context: Context,
			Actor:   &lactor,
			To:      flo,
		},
	}, nil
}

// PostOut sends a post out to federated servers.
func PostOut(ctx context.Context, board database.Board, post database.Post) error {
	actor := TransformBoard(board)
	act, err := activityBase(ctx, board)
	if err != nil {
		return err
	}

	irt := Object{}
	if post.Thread != 0 {
		thread, err := DB.Post(ctx, board.ID, post.Thread)
		if err != nil {
			return err
		}

		irt.Type = "Note"
		irt.ID = thread.APID

		// Add the actor of the thread into To, if it's not already there
		// thread.Source is the Actor for posts that aren't local to us.
		if !thread.IsLocal() {
			ok := true
			for _, v := range act.To {
				if v.ID == thread.Source {
					ok = false
					break
				}
			}
			if ok {
				act.To = append(act.To, LinkObject{Type: "Link", ID: thread.Source})
			}
		}
	}

	note, err := TransformPost(ctx, &actor, post, irt, false, true) // Won't ever have replies
	if err != nil {
		return err
	}

	act.Object.Type = "Create"
	act.ObjectProp = &note

	return SendActivity(ctx, act)
}

func PostDel(ctx context.Context, board database.Board, post database.Post) error {
	actor := TransformBoard(board)
	lactor := LinkActor(actor)

	act, err := activityBase(ctx, board)
	if err != nil {
		return err
	}

	act.Object.Type = "Delete"
	act.ObjectProp = &Object{
		ID:         post.APID,
		Type:       "Note",
		Actor:      &lactor,
		NoCollapse: true,
	}

	return SendActivity(ctx, act)
}
