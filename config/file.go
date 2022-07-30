package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

// Load loads a configuration file using a simple key value format.
// See doc/config.example.
func Load(path string) error {
	fp, err := os.Open(path)
	if err != nil {
		return err
	}

	defer fp.Close()

	s := bufio.NewScanner(fp)

	for s.Scan() {
		line := strings.TrimSpace(s.Text())

		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// We're looking for both a key and a value so split at most twice
		tokens := strings.SplitN(line, " ", 2)
		if len(tokens) != 2 {
			continue
		}

		key, value := tokens[0], strings.TrimSpace(tokens[1])
		switch strings.ToLower(key) {
		case "fqdn":
			FQDN = value
		case "dbengine":
			DatabaseEngine = value
		case "dbarg":
			DatabaseArg = value
		case "listen":
			ListenAddress = value
		case "title":
			Title = value
		case "randadmin":
			RandAdmin = value == "true"
		case "transport":
			value = strings.ToLower(value)
			if value != "http" && value != "https" {
				log.Fatalf("Error: transport expects http or https for a value. (Current value: \"%s\")", value)
			}
			TransportProtocol = value
		case "private":
			Private = value == "true"
		//case "publicaudit":
		//	PublicAudit = value == "true"
		case "local":
			AllowLocal = value == "true"
		case "onion":
			AllowOnion = value == "true"
		case "debug":
			Debug = value == "true"
		case "proxy":
			ProxyUrl = value
		case "pprof":
			Pprof = true
		case "textlimit":
			var err error
			PostCutoff, err = strconv.Atoi(value)
			if err != nil {
				log.Fatalf("Error parsing textlimit: %s", err)
				// Exited here
			}

			if PostCutoff <= 0 {
				log.Fatalf("Error: textlimit is set to a negative number or zero. Please set it to a number *above* zero.")
			} else if PostCutoff > 4000 {
				log.Printf("Warning: textlimit is set to over 4000; you may experience problems federating with long posts.")
			}
		case "donate":
			toks := strings.SplitN(value, " ", 2)
			if len(toks) != 2 {
				log.Fatalf("Error: bad value for donate. Expected two values, got %d.", len(toks))
			}

			Donate[toks[0]] = toks[1]
		case "unstable":
			// You should not set any options here.
			// These are features implemented but currently unusable, or half baked.
			// Nothing here is to ever be mentioned in the config file, if you
			// find yourself here you should know what you're doing anyway.

			toks := strings.Split(strings.ToLower(value), ",")
			for _, tok := range toks {
				switch tok {
				case "unfollow":
					// Uses Undo Follow activities instead of toggling with Follow.
					// COMPAT: FChannel nor we support it yet.
					UnstableUnfollow = true
				case "tor2web":
					// Allows tor2web clients to access Feditext.
					// They are otherwise immediately denied access.
					NoT2W = false
				case "textlimit":
					log.Printf("textlimit is no longer an unstable option.")
					log.Printf("The default is now 4000, however you can change this with the \"textlimit\" setting, which accepts any positive number.")
					/*
						// COMPAT: FChannel silently rejects posts with text lengths greater than 2000.
						// This unstable option moves it back to the original 4000
						// limit, however as of writing you will be unable to send
						// posts longer than 2000 chars with FChannel instances.
						PostCutoff = 4000
					*/
				default:
					log.Printf("Unknown unstable option \"%s\"", tok)
				}
			}
		default:
			log.Printf("unknown config key \"%s\"", key)
		}
	}

	return nil
}
