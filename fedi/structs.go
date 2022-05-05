package fedi

/*
	These structures are a lot slower than the previous ones I had.
	By a factor of about 3 if I remember previous times.
	Yes.

	So, if you have any idea on where to improve things, please let me know.
	It would be greatly appreciated as these structures don't lessen my will to
	live like the other ones do.

	I still don't like Collection/OrderedCollection but it's an improvement
	upon OrderedNoteCollection. Be happy, I'll figure something out later.
	This may comply with the standards much more.
*/

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/KushBlazingJudah/feditext/database"
)

var Context = StringList{"https://www.w3.org/ns/activitystreams"}
var DB database.Database

// StringList is a type that takes either a string, or a []string.
// In the case of string, index 0 will be said string.
type StringList []string

func (s StringList) MarshalJSON() ([]byte, error) {
	switch len(s) {
	case 1:
		// We only want to encode the string.
		return json.Marshal(s[0])
	default:
		// We have more than one, so encode them all.
		return json.Marshal([]string(s))
	}
}

func (s *StringList) UnmarshalJSON(data []byte) error {
	// In order to determine if this data is fine or not, we first need to
	// unmarshal this data to an empty interface to do a type switch on.
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	switch v := value.(type) {
	case []string:
		*s = v
	case string:
		*s = []string{v}
	default:
		return fmt.Errorf("stringList: expected []string or string, got %T", v)
	}

	return nil
}

// Link represents a link to an Object.
// This may be just a raw string; in which case, Href will be the only thing set.
// There are other properties in here, but we don't care about them.
type Link struct {
	Href string
	Rel  string
	Name string
}

func (l Link) MarshalJSON() ([]byte, error) {
	if l.Href == "" {
		// It doesn't make much sense to marshal a link when we have no link.
		return json.Marshal(nil)
	} else if l.Rel == "" && l.Name == "" {
		// This is a "thin" link, just a string.
		return json.Marshal(l.Href)
	}

	// Fat link.
	// This sucks, but it's what we need to do.
	m := map[string]string{"href": l.Href}
	if l.Rel != "" {
		m["rel"] = l.Rel
	}
	if l.Name != "" {
		m["name"] = l.Name
	}

	return json.Marshal(m)
}

func (l *Link) UnmarshalJSON(data []byte) error {
	// In order to determine if this data is fine or not, we first need to
	// unmarshal this data to an empty interface to do a type switch on.
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	switch v := value.(type) {
	case string:
		*l = Link{Href: v}
	case map[string]interface{}:
		if rel, ok := v["rel"]; ok {
			l.Rel, ok = rel.(string)
			if !ok {
				return fmt.Errorf("Link: expected string for rel, got %T", rel)
			}
		}

		if href, ok := v["href"]; ok {
			l.Href, ok = href.(string)
			if !ok {
				return fmt.Errorf("Link: expected string for href, got %T", href)
			}
		}

		if name, ok := v["name"]; ok {
			l.Name, ok = name.(string)
			if !ok {
				return fmt.Errorf("Link: expected string for name, got %T", name)
			}
		}
	default:
		return fmt.Errorf("Link: expected string or Link, got %T", v)
	}

	return nil
}

// LinkObject is for cases where there may either be a Link or an Object.
// Links can be a full Link object, or just a string.
// Objects must be a full Object.
type LinkObject Object

func (l *LinkObject) MarshalJSON() ([]byte, error) {
	if l.Type == "" {
		return []byte("null"), nil
	} else if l.Type == "Link" {
		link := Link{Href: l.ID}

		return link.MarshalJSON()
	}

	// We are not a link, so we must be an object.
	return json.Marshal(Object(*l))
}

func (l *LinkObject) UnmarshalJSON(data []byte) error {
	// We can pretty easily determine what we're dealing with simply by looking
	// at the first byte.
	// We do it this way so we can pass it on to Object if it is an Object.
	if data[0] == '{' {
		// It is an Object.
		// It's done this way to prevent recursing.
		type cool LinkObject
		var obj cool
		err := json.Unmarshal(data, &obj)
		*l = LinkObject(obj)

		return err
	}

	// If we made it here, it must be a link.
	var value string
	err := json.Unmarshal(data, &value)

	l.Type = "Link"
	l.ID = value

	return err
}

// LinkActor is for cases where there may either be a Link or an Actor.
// Links can be a full Link object, or just a string.
// Actors must be a full Object.
type LinkActor Actor

func (l *LinkActor) MarshalJSON() ([]byte, error) {
	if l.Type == "" {
		return []byte("null"), nil
	} else if l.Type == "Group" {
		link := Link{Href: l.ID}

		return link.MarshalJSON()
	}

	// We are not a link, so we must be an object.
	return json.Marshal(Actor(*l))
}

func (l *LinkActor) UnmarshalJSON(data []byte) error {
	// We can pretty easily determine what we're dealing with simply by looking
	// at the first byte.
	// We do it this way so we can pass it on to Object if it is an Object.
	if data[0] == '{' {
		// It is an Object.
		// It's done this way to prevent recursing.
		type cool LinkActor
		var obj cool
		err := json.Unmarshal(data, &obj)
		*l = LinkActor(obj)

		return err
	}

	// If we made it here, it must be a link.
	var value string
	err := json.Unmarshal(data, &value)

	l.Type = "Link"
	l.ID = value

	return err
}

// Object is the base type for all things ActivityPub.
// A small subset is used since we don't care about 90% of it.
type Object struct {
	Context StringList `json:"@context,omitempty"`

	ID   string `json:"id,omitempty"`
	Type string `json:"type,omitempty"`

	AttributedTo *LinkObject        `json:"attributedTo,omitempty"`
	Content      string             `json:"content,omitempty"`
	InReplyTo    []LinkObject       `json:"inReplyTo,omitempty"`
	Name         string             `json:"name,omitempty"`
	Published    *time.Time         `json:"published,omitempty"`
	Replies      *OrderedCollection `json:"replies,omitempty"`
	Summary      string             `json:"summary,omitempty"`
	To           []LinkObject       `json:"to,omitempty"`
	Updated      *time.Time         `json:"updated,omitempty"`

	// Extended attributes for other types
	Tripcode string     `json:"tripcode,omitempty"`
	Subject  string     `json:"subject,omitempty"`
	Actor    *LinkActor `json:"actor,omitempty"`
}

type Activity struct {
	*Object

	ObjectProp *Object `json:"object,omitempty"`
}

// Collection is a collection of links or objects.
type Collection struct {
	*Object

	TotalItems int          `json:"totalItems"`
	Items      []LinkObject `json:"items,omitempty"`
}

func (c Collection) MarshalJSON() ([]byte, error) {
	if c.TotalItems == 0 {
		return []byte("null"), nil
	}

	// There's something to encode!
	// Time for some hacks.
	type cool Collection
	return json.Marshal(cool(c))
}

// OrderedCollection is a strictly ordered collection of links or objects.
// It exists solely because encoding/json won't do two JSON keys for one value,
// and I don't want to start pulling keys out of map[string]any returned from
// Unmarshal.
type OrderedCollection struct {
	*Object

	TotalItems   int          `json:"totalItems"`
	OrderedItems []LinkObject `json:"orderedItems,omitempty"`
}

func (c OrderedCollection) MarshalJSON() ([]byte, error) {
	// See Collection.MarshalJSON.
	if c.TotalItems == 0 {
		return []byte("null"), nil
	}

	// There's something to encode!
	// Time for some hacks.
	type cool OrderedCollection
	return json.Marshal(cool(c))
}

type publicKey struct {
	ID    string `json:"id,omitempty"`
	Owner string `json:"owner,omitempty"`
	Pem   string `json:"publicKeyPem,omitempty"`
}

type Actor struct {
	*Object

	PublicKey *publicKey `json:"publicKey,omitempty"`

	Inbox     string `json:"inbox,omitempty"`
	Outbox    string `json:"outbox,omitempty"`
	Following string `json:"following,omitempty"`
	Followers string `json:"followers,omitempty"`

	PreferredUsername string `json:"preferredUsername,omitempty"`
	Restricted        bool   `json:"restricted"`
}

type Outbox struct {
	Context StringList `json:"@context,omitempty"`
	Actor   *Actor     `json:"actor,omitempty"`

	TotalItems   int          `json:"totalItems"`
	OrderedItems []LinkObject `json:"orderedItems,omitempty"`
}
