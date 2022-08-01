# ![logo](logo.png) feditext

A textboard for ~~the fediverse~~
[FChannel](https://github.com/FChannel0/FChannel-Server) and itself.

This project aims to federate with existing FChannel instances and itself,
while maintaining a strictly text-only interface.

## Achtung!

**Federation is a work in progress, and by no means stable nor good.**
Between FChannel and Feditext, federation has been battle tested and works *more
or less*.
However, do not expect any federation between the outer Fediverse.

## Goals

Feditext's goals are simple and to the point:

- Sane moderation
  - FChannel says it won't keep IPs, we will, optionally.
    Not keeping them is good for privacy but not good when you have bad actors,
    and it's surprising that FChannel hasn't had any (yet, or that I know of).
    Or at least intentionally.
  - If you're running a Tor instance, you can choose to not keep IPs.
    You should because they will all probably be 127.0.0.1 anyway.
  - Public moderation log. It's a little broken right now but it's there.
    Can also be turned off.
- Textboard first, ActivityPub later
  - Feditext relies on numeric IDs internally to do its bookkeeping.
    Where possible, ActivityPub is simply put on top of the core engine,
    possibly making it extremely easy to just rip it out if you wish to just use
    the textboard part.
- Implemented in a simple and good fashion where possible.
  - The code attempts to be clean and simple to understand, however in places it
    needs comments or just needs work in general.
  - Database is kept simple, and while there's a list of functions you need to
    implement, a port to another DBMS is more than doable in an afternoon.

### Non-goals

- Attachments and previews
  - It's a textboard.
    Our ActivityPub implementation doesn't even have anywhere to hold these, and
    neither does my disk.
- Proper ActivityPub support
  - While work was done to try to make it not choke on some data it takes in,
    Feditext will not be 100% compliant until FChannel is.

This list is bound to grow and shrink given time.

## Dependencies

- Go 1.18+

If you build with SQLite3:

- a C compiler
- SQLite3 itself and its headers

## Running

Feditext can be built easily using the included Makefile:

- `build` builds a **release** build; this strips out some data that is
  necessary for debugging, but it's also several megabytes smaller.
- `dev` builds a **developer** build; debugging information is left intact.
- `run` runs a **developer** build.
- `dist` creates a tarball from the release build for easy deployment.
  Created out of convenience for me.

Less important targets:

- `check` runs tests.
  There are none working right now, I neglected to update the database tests.
- `tidy` tidies up everything, runs gofmt and goimports if they're there.

The following variables will be useful to you:

- The standard `GO*` variables if you're cross compiling (you probably aren't)
- `TAGS` builds with certain features included or excluded; `sqlite3` is the
  default value.
  - Always have a database in here, if you don't your build will be entirely
    useless.

Once you've built Feditext, copy `doc/config.example` to `./feditext.config` and
**read the whole thing**.
From there, you should be good to run Feditext with a simple `./feditext` or
`make run`.

Feditext is extremely limited in what you can do from the CLI, but you can:

- load different configuration files with `-config ...`
- create a user with `create`
  - See `./feditext create -help` for more information

Or, if you just want to start it, run it with no arguments.
