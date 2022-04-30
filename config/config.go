package config

import (
	"runtime/debug"
)

var (
	// FQDN is the domain of the server, such as example.com or foo.example.com.
	FQDN string = "localhost"

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
)

func init() {
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
