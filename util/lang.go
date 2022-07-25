package util

import (
	"unicode"
)

var jap = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x3000, 0x303F, 1}, // Punctuation
		{0x3041, 0x3096, 1}, // Hiragana
		{0x30A0, 0x30FF, 1}, // Katakana
		{0x4E00, 0x9FCB, 1}, // Unified CJK
	},
}

// IsJapanese returns true if any of the characters inside of the string are
// within Japanese unicode ranges.
func IsJapanese(s string) bool {
	for _, r := range s {
		if unicode.In(r, jap) {
			return true
		}
	}
	return false
}
