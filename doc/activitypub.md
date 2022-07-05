# Feditext's ActivityPub Implementation

**This document is a work in progress and is likely missing crucial
information.**

FChannel's implementation of the ActivityPub spec is broken, however for
compatibility Feditext follows it where possible while trying to better it at
the same time.

Full compatibility with the ActivityPub spec is not a primary goal of FChannel
and by extension Feditext at this time.
FChannel's developer says that given time, it will become more compliant with
the actual spec however "base features and usability" comes first.

Feditext internally uses structures similar to FChannel's however a lot of the
cruft is thrown away and some types are flexible.
Feditext will accept a link in some places where FChannel will not.
It also ditches many properties that are normally null, unless they aren't.
It may cause some breakage in programs that expect these properties to be there
even though they're empty. FChannel handles it just fine however.

The changes on our end are minimal but impactful:

- We don't care about the preview/attachments so we throw it away.
  Remember, this is a **text**board.
  - Obviously enough, if you read from this board you will not get any previews
    or attachments because we literally store no information about it.
- 2-level deep replies and inReplyTo objects are stripped off.
  (Essentially: thread -> replies -> note -> replies. The final replies object
  has no inReplyTo attribute internally, and every Note in those replies has no
  replies attribute internally.)
  - I also have no idea on the impact this will have.
    I assume it will be minimal because I doubt at any point in the FChannel
    codebase it looks for the replies to a reply of a reply of a thread.
    FChannel seems to mangle our data just fine.
  - It also saves a huge amount of bandwidth. An earlier implementation halved
    the amount of data sent out by the outbox.
- Many optional structures have been made pointers; along with the `omitempty`
  option, when nil they will be stripped off. **Devs, watch out!**
- Replies has the "OrderedCollection" type. Not compliant with the spec but
  important.

This document doesn't serve as an implementation guide for ActivityPub; refer
to the official spec for that.
It simply lists out the differences and gotchas.
Use this as a reference if you plan to talk to Feditext or FChannel.

## Differences

Generally you only really care about a subset of things in the ActivityPub spec.
Right now, Feditext only has a concept of these few things:

- Actor
- Activity
  - Follow
  - Create
- Note
- OrderedCollection

Many things are missing on this list that FChannel supports, we will too
eventually.

Along with this, if you think of disjointing anything, **don't**.
Feditext will most likely parse it just fine but it will probably get thrown out
if it is disjointed.
Federation is hugely a work in progress, and full compliance is not a priority.

## Actor

Actor has the following added properties:

- `restricted` (boolean): Marks the board SFW if true.

These properties have a different meaning:

- `name` (text): Identifier of the board. (the `prog` part of `/prog/`)
- `preferredUsername` (text): Title of the board.

## Note

Note has the following added properties:

- `tripcode` (string): An identifier for a user if they so choose to use it.
  - Note: This very well may be arbitrary text, so watch out. Tripcodes start
    with a `!` and capcodes with a `#`.
- `Sticky` (boolean): We don't deal with this. <!-- TODO -->
- `Locked` (boolean): Nor this. <!-- TODO -->

It differs in that:

- `replies` is an OrderedCollection
- `attributedTo` is arbitrary text

You can ignore pretty much every attribute in here asides from a few basic ones:

- `id`, `actor` (link)
- `content` (text)
  - Numeric cites are rewritten on post submission from our end to fit
    FChannel's schema of `>>activitypub id`.
- `replies` (OrderedCollection)
- `inReplyTo` (list of Notes)

On incoming messages (activities), the `option` (list of strings) field shows up
and so far contains only one thing of value: `sage`.
