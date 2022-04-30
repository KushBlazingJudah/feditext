package config

import (
	"bufio"
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
		default:
			log.Printf("unknown config key \"%s\"", key)
		}
	}

	return nil
}
