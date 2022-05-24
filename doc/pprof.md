# pprof

Feditext supports pprof and will happily serve it for you if you tell it to.

Enabling it is simple:

```
pprof true
```

Feditext will spit out a path for you to visit, where you can point either a web
browser or pprof itself at.

[Read this if you don't know how to use pprof.](https://go.dev/blog/pprof)

This is used on my own instance **right now**, and could be a useful tool for
tuning Feditext, finding chokepoints, and other various nasty things.
