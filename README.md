# ![logo](logo.png) feditext

A textboard for ~~the fediverse~~ FChannel and itself.

This project aims to federate with existing FChannel instances, and itself,
while maintaining a strictly text-only interface.

## Achtung!

This is largely a work in progress, and most likely will be for a while.
This is exacerbated by the fact that I probably will not work on this as often
as I would for other projects, but as of writing it's my current interest.

**Federation is a work in progress, and by no means stable nor good.**
You can federate with FChannel (more or less) and other instances of Feditext,
however it is by no means perfect at this time.
I highly doubt you'll have any luck going out to any other servers as Feditext
follows how FChannel does things, which is not entirely compliant but *mostly*
compliant.

In its current state, ActivityPub is implemented on top of the core engine where
possible.
In some places it is hard to avoid reliance on it, however in most it is almost
entirely transparent.
You could probably rip out the ActivityPub features and use it solely as a
textboard if you really care enough.

## Rationale

I have previously done a great deal of work on [FChannel's
codebase](https://github.com/FChannel0/FChannel-Server), however I very quickly
lost interest in the project after doing a lot of work converting a portion of
the code to Fiber and restructuring a good chunk of the codebase.
I was generally burnt out of programming at that time so I turned my attention
to other projects outside of programming, however eventually got the courage
back and worked on other programming projects and pushed FChannel off to the
side.

Then I seen activity on the Git repository once again, and then this project was
started.

I have a huge amount of respect for the developer of FChannel, and this is by no
means made as a killer to his project.
However, this project was also made in response to some... odd design choices in
FChannel to say the least.
I'm guilty of it myself, especially in the Fiber port, and probably here too,
however this aims to implement things better and serve as a potential reference
on things FChannel can improve upon.

FChannel is an ambitious project and I am glad I got to play a role in its
development, and I hope that more projects like FChannel pop up to improve upon
each other's mistakes while having their own uniqueness to it.

When I am largely finished with this project, I plan to contribute some good
differences here into FChannel's actual code base to improve quality of life
there too.

## Goals

Feditext's goals are simple and to the point:

- ~~Tons of comments~~ (lol), good documentation
  - Comments were kinda thrown out the window but it's pretty easy to figure out
    what's going on if you're even remotely proficient in SQL or Golang
- Sane moderation
  - FChannel says it won't keep IPs, we will, optionally.
    Not keeping them is good for privacy but not good when you have bad actors,
    and it's surprising that FChannel hasn't had any (yet, or that I know of).
    Or at least intentionally.
  - If you're running a Tor instance, you can choose to not keep IPs.
    You should because they will all probably be 127.0.0.1 anyway.
  - Public moderation log. It's a little broken right now but it's there.
    Can also be turned off.
- Implemented in a simple and good fashion where possible.
  - FChannel has 88+ database functions, we have 42 and they're all contained in
    the database package.
  - We use Golang contexts in numerous places.

Any changes I may need to make to FChannel (hopefully few if not none!) I will
upstream.
The developer says that he's interested in adding textboards to FChannel, so
maybe this will give just the push we all need.

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

As it stands, the core engine of Feditext is relatively complete and will not be
facing many changes.
The federation portions are what is primarily being worked on and what will be
worked on mostly until that is also complete.

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

