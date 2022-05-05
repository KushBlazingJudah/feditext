# feditext

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

In its current state, it probably works quite well as a textboard even though
it's not complete.
The idea behind doing it this way is so I have a core engine that more or less
works fine, a nice foundation for implementing ActivityPub on top of.
You can probably rip ActivityPub out of this quite easily and use it as a
standalone textboard, which is what I've intended.

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

- <=4000 SLOC in the main codebase according to cloc
  - Previously it was 3,000 lines but the core engine came close to 2,000 so I
    bumped it. I don't think I will hit 4,000 but will most likely exceed 3,000.
  - FChannel was approaching 6,000 lines when I started work on it and I found it
    hard to comprehend at times. I don't want it to be like this.
- ~~Tons of comments~~ (lol), good documentation
  - Comments were kinda thrown out the window but it's pretty easy to figure out
    what's going on if you're even remotely proficient in SQL or Golang
- Sane moderation
  - FChannel says it won't keep IPs, we will.
    Not keeping them is good for privacy but not good when you have bad actors,
    and it's surprising that FChannel hasn't had any (yet, or that I know of).
    Or at least intentionally.
  - Public moderation log. It's a little broken right now but it's there.
- Later, but good Tor support. I don't want to rent a VPS.

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

As it stands, the core engine of Feditext is relatively complete and will not be
facing many changes.
However, I advise against using this for any serious purposes until the first
proper release, which is when I'll most likely fire up an instance of my own.

Feditext can be built easily using the included Makefile:

- `build` builds a **release** build; this strips out some data that is
  necessary for debugging.
- `dev` builds a **developer** build; debugging information is left intact.
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
