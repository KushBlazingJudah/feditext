# Feditext's ActivityPub Implementation

**This document is a work in progress and is missing crucial information.**

FChannel's implementation of the ActivityPub spec is broken, however for
compatibility Feditext follows it where possible while trying to better it at
the same time.

Full compatibility with the ActivityPub spec is not a primary goal of FChannel
and by extension Feditext at this time.
FChannel's developer says that given time, it will become more compliant with
the actual spec however "base features and usability" comes first.

Feditext uses an optimized version of internal representations of ActivityPub
structures similar to those found in FChannel.
This should come at no cost to federation compatibility while also literally
halving the size of the serialized JSON.

The changes are minimal but impactful:

- We don't care about the preview/attachments so we throw it away.
  Remember, this is a **text**board.
  - I have no idea on the impact this will have on federation.
    This project is largely a testbed anyway. Expect bad stuff.
- 2-level deep replies and inReplyTo objects are stripped off.
  (Essentially: thread -> replies -> note -> replies. The final replies object
  has no inReplyTo attribute internally, and every Note in those replies has no
  replies attribute internally.)
  - I also have no idea on the impact this will have.
    I assume it will be minimal because I doubt at any point in the FChannel
    codebase it looks for the replies to a reply of a reply of a thread.
- Many optional structures have been made pointers; along with the `omitempty`
  option, when nil they will be stripped off.
- Replies has the "OrderedCollection" type. Not compliant with the spec but
  important.

At a later date, I will document some things about FChannel's current dialect of
ActivityPub.
However, if there's anything you should take from this now, it's that FChannel
speaks ActivityPub, just not well.

## Differences

Generally you only really care about a subset of things in the ActivityPub spec.
Right now, Feditext only has a concept of these three things:

- Actor
- Note
- OrderedCollection

Many things are missing on this list that FChannel supports, we will too
eventually.

Along with this, if you think of disjointing anything, **don't**.
The implementation is simpler as a result, and no less compliant going out but
things aren't so great coming in.

## OrderedCollection

No differences.
Refer to the spec.

## Actor

Actor has the following added properties:

- `restricted` (boolean): Marks the board SFW if true.

These properties are used:

- `following`, `followers`, `outbox`, `inbox` (link)
- `name` (text): Identifier of the board. (the `prog` part of `/prog/`)
- `preferredUsername` (text): Title of the board.
- `summary` (text): Description of the board.
- `publicKeyPem` (public key) <!-- TODO -->

## Note

Note has the following added properties:

- `tripcode` (string): An identifier for a user if they so choose to use it.
- `Sticky` (boolean) <!-- TODO -->
- `Locked` (boolean) <!-- TODO -->

It differs in that:

- `replies` is an OrderedCollection
- `attributedTo` is arbitrary text

You can ignore pretty much every attribute in here asides from a few basic ones:

- `id`, `actor` (link)
- `content` (text)
  - FChannel cites are done like this: `>>https://foo.example.com/prog/ABCDEFGH`
  We allow `>>1337` and will rewrite it to fit how FChannel cites.
- `replies` (OrderedCollection)
- `inReplyTo` (list of Notes)
