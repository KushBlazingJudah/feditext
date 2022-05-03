package main

// A lot of things here are explicitly pointers so they get dropped when there
// are no values to go in there, and omitempty wouldn't normally omit it.
// This applies for any struct.
// If there's a struct in there that may not matter, make it a pointer.
// encoding/json will handle nils just fine, will you?

import "time"

// Actor represents a user, or in our case, a board, which is actually a
// service account according to ActivityPub.
type Actor struct {
	ID   string `json:"id"`
	Type string `json:"type"`

	Inbox     string `json:"inbox"`
	Outbox    string `json:"outbox"`
	Following string `json:"following"`
	Followers string `json:"followers"`

	Name              string `json:"name"`
	PreferredUsername string `json:"preferredUsername"`
	Summary           string `json:"summary"`

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
	ID   string `json:"id"`
	Type string `json:"type"`

	Actor   string `json:"actor"`
	Content string `json:"content"`

	Published time.Time  `json:"published"`
	Updated   *time.Time `json:"updated,omitempty"` // Usually always nil

	Replies   *OrderedNoteCollection `json:"replies,omitempty"` // Sometimes nil
	InReplyTo []*Note                `json:"inReplyTo,omitempty"`

	// Preview/attachment is ignored since we don't do images
}

// OrderedNoteCollection is an OrderedCollection of notes.
type OrderedNoteCollection struct {
	Type         string  `json:"type"`
	TotalItems   int     `json:"totalItems"`
	OrderedItems []*Note `json:"orderedItems"`
}

type Outbox struct {
	Context string `json:"@context"`

	Actor Actor `json:"actor"`
	*OrderedNoteCollection
}
