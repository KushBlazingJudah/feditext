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

- <=3000 SLOC in the main codebase according to cloc
  - This limit was chosen to keep the codebase drastically simple where needed.
  - Textboards will never be as complex as an imageboard.
  - If this limit proves too hard to keep under, it may be increased.
  FChannel was approaching 6,000 lines when I started work on it and I found it
  hard to comprehend at times.
- Tons of comments, good documentation
- Sane moderation
  - FChannel says it won't keep IPs, we will.
    Not keeping them is good for privacy but not good when you have bad actors,
    and it's surprising that FChannel hasn't had any (yet, or that I know of).
    Or at least intentionally.
  - Public moderation log.
- Tor as a first-class citizen
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
