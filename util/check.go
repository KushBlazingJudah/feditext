package util

import (
	"math"
	"regexp"
)

var IsAlpha = regexp.MustCompile(`^[[:alpha:]]+$`).MatchString
var IsAlnum = regexp.MustCompile(`^[[:alnum:]]+$`).MatchString

func IMax(a, b int) int {
	return int(math.Max(float64(a), float64(b)))
}

func IMin(a, b int) int {
	return int(math.Min(float64(a), float64(b)))
}

func Clamp(a, b, c int) int {
	return IMin(IMax(b, a), c)
}
