package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

const (
	// NameCutoff is the max length for names.
	NameCutoff = 64

	// SubjectCutoff is the max length for subjects.
	SubjectCutoff = 64

	// ReportCutoff is the max length for reports.
	ReportCutoff = 240

	RequestTimeout = 30 * time.Second
	MaxReqTime     = 60 * time.Second

	MaxRetries      = 5
	RetryDelay      = 10 * time.Second
	RetryMultiplyer = 3

	ThreadsPerPage = 10

	Major = 0
	Minor = 1
	Patch = 2
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
	// This is 64 random bytes generated upon startup, and pulled from ./jwtsecret.
	JWTSecret []byte = make([]byte, 64)

	// TripSecret is used to create tripcodes.
	// This is 16 random bytes generated upon startup and pulled from ./tripsecret.
	TripSecret []byte = make([]byte, 16)

	// RandAdmin sets the "admin" user in the database to a random password that is printed in the console.
	RandAdmin bool = false

	// TransportProtocol is the protocol that is used to access this board.
	// It should be one of http or https. Preferably https.
	TransportProtocol string = "http"

	// Private turns off IP logging, and banning.
	// You should have this off unless you're hosting this through Tor, as
	// there is no point to log them there.
	// It will not clear stored IPs.
	Private bool = false

	// PublicAudit enables the public audit log. Disabled, the audit log will be inaccessible.
	// Flipping this arbitrarily will not delete the entries in the audit log.
	PublicAudit bool = false

	// AllowLocal allows requests to local IPs to be made.
	// You should turn this off in a server pointing to the outside.
	AllowLocal bool = false

	// AllowOnion allows requests to Tor Hidden Services to be made.
	// You should keep this off unless you are running this behind a Tor proxy,
	// in which case you should also turn on private mode.
	AllowOnion bool = false

	// ProxyUrl is the URL representation of the proxy going out.
	// This could be something like "socks5://127.0.0.1:9050".
	// You should only use this if you accept Tor connections.
	ProxyUrl string = ""

	// Debug prints out extra information on ActivityPub requests.
	Debug bool = false

	// Pprof installs the pprof handler at /debug/pprof.
	// This is publicly available **with almost no authentication.**
	// pprof does not support authentication, so the most that can be done is
	// putting a key in the path.
	Pprof bool = false

	// PostCutoff is the max length for posts.
	PostCutoff = 4000

	// EmailServer points to an SMTP server where Feditext can send mail
	// notifying whoever on new incoming mail.
	EmailServer string = ""

	// EmailUsername is the username that is used to authenticate to the
	// SMTP server.
	// This is usually your email address, so if left blank, EmailAddress
	// will be used.
	EmailUsername string = ""

	// EmailAddress is the email address that Feditext will use to send
	// email.
	EmailAddress string = ""

	// EmailPassword is the password that is used to authenticate to the
	// SMTP server.
	EmailPassword string = ""

	// EmailFrom is what goes in the From field on outgoing emails.
	// If left blank, "Feditext <EmailAddress>" is used.
	EmailFrom string = ""

	// Donate is a list of places to donate, such as XMR -> address.
	// This is to aid in easily allowing instance admins to have a spot to
	// accept donations without needing to modify source code.
	Donate map[string]string = map[string]string{}

	// NoT2W disallows any tor2web proxy identifying as such with the
	// "X-tor2web" header from accessing Feditext.
	// Every request will be immediately denied with a message telling users to
	// download the Tor Browser Bundle.
	NoT2W bool = true

	// UnstableUnfollow uses Undo activities instead of sending another Follow,
	// which as of writing FChannel will use to toggle you on and off the
	// followers list.
	// Undo Follow is not implemented in FChannel, nor here (yet).
	// **Only set if you know what you're doing.**
	UnstableUnfollow bool = false
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

	// Same thing but for TripSecret
	if _, err := os.Stat("./tripsecret"); err == nil {
		TripSecret, err = os.ReadFile("./tripsecret")
		if err != nil {
			panic(err)
		}
	} else {
		rand.Read(TripSecret)

		fp, err := os.OpenFile("./tripsecret", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0400)
		if err != nil {
			panic(err)
		}
		defer fp.Close()
		if _, err := fp.Write(TripSecret); err != nil {
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

	Version = fmt.Sprintf("v%d.%d.%d-%s%s", Major, Minor, Patch, commit, dirty)
}
