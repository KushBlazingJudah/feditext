package feditext

import (
	"fmt"

	"github.com/KushBlazingJudah/feditext/config"
	"github.com/golang-jwt/jwt/v4"
)

func jwtKeyfunc(t *jwt.Token) (interface{}, error) {
	if t.Method.Alg() != "HS256" {
		return nil, fmt.Errorf("Unexpected jwt signing method=%v", t.Header["alg"])
	}

	return config.JWTSecret, nil
}
