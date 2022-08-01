package util

import (
	"math"
	"regexp"
)

var IsAlpha = regexp.MustCompile(`^[[:alpha:]]+$`).MatchString
var IsAlnum = regexp.MustCompile(`^[[:alnum:]]+$`).MatchString

func IsAlnumRune(r rune) bool {
	return ('A' <= r && r <= 'Z') ||
		('a' <= r && r <= 'z') ||
		('0' <= r && r <= '9')
}

func IMax(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}

func IMin(a, b int) int {
	return int(math.Min(float64(a), float64(b)))
}

func Clamp(a, b, c int) int {
	return IMin(IMax(b, a), c)
}
