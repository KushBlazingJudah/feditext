# Feditext's ActivityPub Implementation

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
