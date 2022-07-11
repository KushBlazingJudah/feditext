package crypto

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/golang-jwt/jwt/v4"
)

func JwtKeyfunc(t *jwt.Token) (interface{}, error) {
	if t.Method.Alg() != "HS256" {
		return nil, fmt.Errorf("unexpected jwt signing method=%v", t.Header["alg"])
	}

	return config.JWTSecret, nil
}

func DoTrip(name string) (string, string) {
	toks := strings.SplitN(name, "#", 2)

	if len(toks) > 0 {
		// Prevent empty names

		toks[0] = strings.TrimSpace(toks[0])
		if len(toks[0]) == 0 {
			toks[0] = "Anonymous"
		}
	}

	if len(toks) == 1 {
		return toks[0], ""
	} else if toks[1][0] == '#' {
		return toks[0], SecureTrip(toks[1][1:])
	} else {
		return toks[0], Trip(toks[1])
	}
}

func SecureTrip(pass string) string {
	// I didn't know the difference between a secure trip and just a normal one until today.
	// I thought it was just like the function below for either one and one did something different?
	// I don't know what I was thinking.
	// Anyways.

	// Special case for moderator trip
	if pass == "mod" {
		return "mod"
	}

	buf := make([]byte, len(pass)+len(config.TripSecret))
	copy(buf, pass)
	copy(buf[len(pass):], config.TripSecret)

	hash := sha1.Sum(buf)
	return "!!" + base64.URLEncoding.EncodeToString(hash[:])[:10]
}

func Trip(pass string) string {
	hash := sha1.Sum([]byte(pass))
	return "!" + base64.URLEncoding.EncodeToString(hash[:])[:10]
}
