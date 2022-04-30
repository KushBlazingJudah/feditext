package config

import (
	"crypto/rand"
	"os"
	"runtime/debug"
)

var (
	// FQDN is the domain of the server, such as example.com or foo.example.com.
	FQDN string = "localhost"

	// Title is the title of this server. It is used in titles.
	Title string = "Feditext"

	// Version is the version of the server that was built.
	// This can be used to determine what version a server is running without
	// needing to call upon the administrator.
	// Do not set manually.
	Version string = "unknown"

	// DatabaseEngine is the engine that is used as a database.
	// Right now, this is only SQLite3.
	DatabaseEngine string

	// DatabaseArg is an argument passed to a database's InitFunc.
	// This can be as simple as a file path (SQLite3) or if we had Postgres,
	// whatever this is: https://www.postgresql.org/docs/11/libpq-connect.html#LIBPQ-CONNSTRING
	DatabaseArg string

	// ListenAddress is where the HTTP server will be listening.
	// By default, it is :8080, or, 0.0.0.0:8080.
	ListenAddress string = ":8080"

	// JWTSecret is used to sign authorization tokens.
	// This is 64 random bytes generated upon startup.
	JWTSecret []byte = make([]byte, 64)

	// RandAdmin sets the "admin" user in the database to a random password that is printed in the console.
	RandAdmin bool = true
)

func init() {
	// Most of everything here is fatal anyway so just panic

	if _, err := os.Stat("./jwtsecret"); err == nil {
		JWTSecret, err = os.ReadFile("./jwtsecret")
		if err != nil {
			panic(err)
		}
	} else {
		rand.Read(JWTSecret)

		fp, err := os.OpenFile("./jwtsecret", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0400)
		if err != nil {
			panic(err)
		}
		defer fp.Close()
		if _, err := fp.Write(JWTSecret); err != nil {
			panic(err)
		}
	}

	// Use Go 1.18 features to obtain information about this build
	// Inspiration from this PR: https://github.com/tailscale/tailscale/pull/4185/files

	info, ok := debug.ReadBuildInfo()
	if !ok {
		Version = "error"
		return
	}

	dirty := ""
	commit := "unknown"

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) > 9 {
				commit = s.Value[:9]
			} else {
				commit = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}

	Version = commit + dirty
}
