# feditext

A textboard for the fediverse.

This project aims to federate with existing FChannel instances, barring images,
since text is much easier to moderate, as fun as images are.

## Achtung!

This is largely a work in progress, and most likely will be for a while.
This is exacerbated by the fact that I probably will not work on this as often
as I would for other projects.

**This does not actually have federation support yet, and probably won't until I
get the underlying engine worked out and stable.**

In its current state, it probably works quite well as a textboard even though
it's not complete.
The idea behind doing it this way is so I have a core engine that more or less
works fine, a nice foundation for implementing ActivityPub on top of.

## Rationale

I have previously done a great deal of work on [FChannel's
codebase](https://github.com/FChannel0/FChannel-Server), however I very quickly
lost interest in the project after doing a huge amount of work converting a good
portion of the code to Fiber and restructuring a good chunk of the codebase.
I was generally burnt out of programming at that time so I turned my attention
to other projects outside of programming, however eventually got the courage
back and worked on other programming projects and pushed FChannel off to the
side.

FChannel is an ambitious project and I am glad I got to play a role in its
development, and I hope that more projects like FChannel pop up to improve upon
each other's mistakes while having their own uniqueness to it.
This is one such project I'd like to see.

I have a huge amount of respect for the developer, however I'm afraid he made
some awful design choices while writing FChannel, so this serves to be a project
to expand upon later and to use to improve the main codebase of FChannel, or
maybe even my unfinished Fiber branch which I may eventually finish.
I'm not saying my choices will be much better.

## Goals

Feditext's goals are simple and to the point:

- <=4000 SLOC in the main codebase according to cloc
  - The core textboard engine is approaching 2000 lines, by then it will
    probably be complete, and may even shrink once I do some optimization.
    I have bumped the line limit to 4000 lines from 3000 as a result.
  - FChannel was approaching 6,000 lines when I started work on it and I found it
    hard to comprehend at times. I don't want it to be like this.
- ~~Tons of comments~~ (lol), good documentation
  - Comments were kinda thrown out the window but it's pretty easy to figure out
    what's going on if you're even remotely proficient in SQL or Golang
  - I will document how the ActivityPub implementation works when the time
    comes.
- Sane moderation
  - FChannel says it won't keep IPs, we will.
    Not keeping them is good for privacy but not good when you have bad actors,
    and it's surprising that FChannel hasn't had any (yet, or that I know of).
    Or at least intentionally.
  - Public moderation log.
- Tor as a first-class citizen
  - If I get this off the ground, my instance will be ran over Tor mainly for
    privacy reasons. I hope that others prop up instances.
  - Optionally, all requests will be made through a proxy.
    Hosting over Tor and don't want to expose the server's IP? Sure!
- Able to federate with FChannel, of course without images.

Any changes I may need to make to FChannel (hopefully few if not none!) I will
upstream.
The developer says that he's interested in adding textboards to FChannel, so
maybe this will give just the push we all need.

## Dependencies

- Go 1.18+

If you build with SQLite3:

- a C compiler
- SQLite3 itself and its headers

## Running

Feditext is not in a very usable state, however a Makefile is included.

- `build` builds a **release** build; this strips out some data that is
  necessary for debugging.
- `dev` builds a **developer** build; debugging information is left intact and
  the race detector is turned on.
- `run` runs a **developer** build.

Less important targets:

- `check` runs tests.
- `tidy` tidies up everything.

The following variables will be useful to you:

- The standard `GO*` variables if you're cross compiling (you probably aren't)
- `TAGS` builds with certain features included or excluded; `sqlite3` is the
  default value.
  - Always have a database in here, if you don't your build will be entirely
    useless.
