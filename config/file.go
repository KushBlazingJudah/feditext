package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
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
				panic(fmt.Errorf("config: transport: expected http or https, got %s", value))
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
				}
			}
		default:
			log.Printf("unknown config key \"%s\"", key)
		}
	}

	return nil
}
