package main

/*
	Proof of concept.

	Keeping only what we care about and throwing away some things that won't
	ever be used, the outbox of fchan's prog shrunk 50%.
	On my machine, both serialization and deserialization take up ~38ms.

	Deserialization using the output from this program is 2x as fast.
	This makes sense because there's literally half as much data.
	Serialization using the output from this program is par with real outbox.

	All tests were conducted with a local file.
	Those done over the net are bound to be slower.
	No reference for /prog/ serialization times, but judging by response times,
	not good.

	This assumes a few things that are likely to be true:
	- Replies doesn't matter when it is inside of a reply to a thread (2
	  replies/inReplyTo deep, outbox -> thread -> reply (stop here))
	- You don't care about preview/attachment (we don't)
	- You're okay with nil values (we are)
*/

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func FixOrderedNotes(o *OrderedNoteCollection, depth int) {
	o.Type = "OrderedCollection"

	for _, note := range o.OrderedItems {
		FixNote(note, depth)

		if note.Replies != nil {
			if note.Replies.OrderedItems == nil {
				note.Replies = nil
			} else {
				FixOrderedNotes(note.Replies, depth+1)
			}
		}
	}
}

func FixNote(n *Note, depth int) {
	if depth > 1 {
		n.InReplyTo = nil
		n.Replies = nil
	} else {
		for _, note := range n.InReplyTo {
			FixNote(note, depth+1)
		}

		if n.Replies != nil && n.Replies.OrderedItems != nil {
			for _, note := range n.Replies.OrderedItems {
				FixNote(note, depth+1)
			}
		}
	}
}

func main() {
	// A bulk of the slowness is the connection speed of the main instance and
	// what I'd have to assume is the sheer amount of time it takes to gather
	// everything up and serialize it.
	res, err := http.Get("https://fchan.xyz/prog/outbox")

	//fp, err := os.Open("./outbox")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	//defer fp.Close()

	dt := time.Now()

	var ob Outbox
	decoder := json.NewDecoder(res.Body)
	// decoder := json.NewDecoder(fp)
	if err := decoder.Decode(&ob); err != nil {
		panic(err)
	}

	FixOrderedNotes(ob.OrderedNoteCollection, 0)

	df := time.Now()

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "") // remove to save 250k :)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(ob); err != nil {
		panic(err)
	}

	fmt.Println("deserialization", df.Sub(dt))
	fmt.Println("serialization", time.Since(df))
}
