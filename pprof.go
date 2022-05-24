package feditext

// Ripped right out of
// https://github.com/gofiber/fiber/blob/master/middleware/pprof/pprof.go. This
// is to allow "authentication" if you could call it that. Creates a key and
// logs it to stdout, and you point pprof at `/<key>/debug/pprof`. pprof
// doesn't support authentication but I still want to be able to probe live
// instances of my own as edge cases will be found there.

import (
	"net/http/pprof"
	"math/rand"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// Set pprof adaptors
var (
	pprofIndex        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Index)
	pprofCmdline      = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Cmdline)
	pprofProfile      = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Profile)
	pprofSymbol       = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Symbol)
	pprofTrace        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Trace)
	pprofAllocs       = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("allocs").ServeHTTP)
	pprofBlock        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("block").ServeHTTP)
	pprofGoroutine    = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("goroutine").ServeHTTP)
	pprofHeap         = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("heap").ServeHTTP)
	pprofMutex        = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("mutex").ServeHTTP)
	pprofThreadcreate = fasthttpadaptor.NewFastHTTPHandlerFunc(pprof.Handler("threadcreate").ServeHTTP)
)

func pprofNew() fiber.Handler {
	// Generate key
	// Sucks but it's easy
	k := base64.URLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", rand.Int63())))
	pfx := fmt.Sprintf("/%s/debug/pprof", k)
	log.Printf("pprof available at %s", pfx)

	// Return new handler
	return func(c *fiber.Ctx) error {
		path := c.Path()
		if !strings.HasPrefix(path, pfx) {
			return c.Next()
		}

		// Switch to original path without stripped slashes
		switch path {
		case pfx+"/":
			pprofIndex(c.Context())
		case pfx+"/cmdline":
			pprofCmdline(c.Context())
		case pfx+"/profile":
			pprofProfile(c.Context())
		case pfx+"/symbol":
			pprofSymbol(c.Context())
		case pfx+"/trace":
			pprofTrace(c.Context())
		case pfx+"/allocs":
			pprofAllocs(c.Context())
		case pfx+"/block":
			pprofBlock(c.Context())
		case pfx+"/goroutine":
			pprofGoroutine(c.Context())
		case pfx+"/heap":
			pprofHeap(c.Context())
		case pfx+"/mutex":
			pprofMutex(c.Context())
		case pfx+"/threadcreate":
			pprofThreadcreate(c.Context())
		default:
			// pprof index only works with trailing slash
			if strings.HasSuffix(path, "/") {
				path = strings.TrimRight(path, "/")
			} else {
				path = pfx+"/"
			}

			return c.Redirect(path, fiber.StatusFound)
		}
		return nil
	}
}
