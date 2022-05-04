package fedi

// A lot of things here are explicitly pointers so they get dropped when there
// are no values to go in there, and omitempty wouldn't normally omit it.
// This applies for any struct.
// If there's a struct in there that may not matter, make it a pointer.
// encoding/json will handle nils just fine, will you?
//
// This is also not exactly compliant, i.e. we take what we care about and nothing else.
// However, attachments and previews will never be used by us as we're a textboard.

import (
	"time"

	"github.com/KushBlazingJudah/feditext/database"
)

// Didn't want to make a new file for it
var DB database.Database

const Context = "https://www.w3.org/ns/activitystreams"

type Object struct {
	Context string `json:"@context,omitempty"`

	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`

	Name         string `json:"name,omitempty"`
	AttributedTo string `json:"attributedTo,omitempty"`

	Content string `json:"content,omitempty"`
	Summary string `json:"summary,omitempty"`

	Published *time.Time `json:"published,omitempty"` // Usually never nil
	Updated   *time.Time `json:"updated,omitempty"`   // Usually always nil

	Replies   *OrderedNoteCollection `json:"replies,omitempty"` // Sometimes nil
	InReplyTo []Note                 `json:"inReplyTo,omitempty"`

	Actor string `json:"actor,omitempty"`

	To []string `json:"to,omitempty"`
}

// Actor represents a user, or in our case, a board, which is actually a
// service account according to ActivityPub.
type Actor struct {
	Object

	Inbox     string `json:"inbox"`
	Outbox    string `json:"outbox"`
	Following string `json:"following"`
	Followers string `json:"followers"`

	PreferredUsername string `json:"preferredUsername"`

	PublicKey  *PublicKey `json:"publicKey,omitempty"`
	Restricted bool       `json:"restricted"`
}

// PublicKey holds information pertaining to the public key of an actor.
// It is used to verify posts from an actor.
type PublicKey struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
	Pem   string `json:"publicKeyPem"`
}

// Note is an object representing a single post.
// This could be a thread, or a reply.
type Note struct {
	Object

	Actor string `json:"actor"`

	Tripcode string `json:"tripcode,omitempty"`
	Subject  string `json:"name,omitempty"`

	// Preview/attachment is ignored since we don't do images
}

// OrderedNoteCollection is an OrderedCollection of notes.
type OrderedNoteCollection struct {
	Object

	TotalItems   int    `json:"totalItems"`
	OrderedItems []Note `json:"orderedItems"`
}

type Outbox struct {
	Object

	Actor Actor `json:"actor"`
	*OrderedNoteCollection
}

type Collection struct {
	Object

	TotalItems int      `json:"totalItems"`
	Items      []Object `json:"items"`
}

type Activity struct {
	Object
	Actor *Actor `json:"actor,omitempty"`

	ObjectProp *Object `json:"object,omitempty"`
}
