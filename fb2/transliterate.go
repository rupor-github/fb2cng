package fb2

import (
	"strings"
	"unicode"

	"github.com/gosimple/slug"
)

// Transliterate converts non-ASCII characters to their ASCII equivalents
// while preserving spaces and original capitalization.
// For example: "Война и мир" -> "Voina i mir"
func Transliterate(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		words[i] = transliterateWord(word)
	}
	return strings.Join(words, " ")
}

// transliterateWord transliterates a single word preserving its capitalization pattern.
func transliterateWord(word string) string {
	if word == "" {
		return ""
	}

	runes := []rune(word)
	firstUpper := unicode.IsUpper(runes[0])
	allUpper := isAllUpper(runes)

	// Use slug for transliteration (temporarily disable lowercase)
	slug.Lowercase = false
	trans := slug.Make(word)
	slug.Lowercase = true

	if trans == "" {
		return word
	}

	transRunes := []rune(trans)

	if allUpper {
		// Restore all uppercase
		for i := range transRunes {
			transRunes[i] = unicode.ToUpper(transRunes[i])
		}
	} else if firstUpper {
		// Restore first letter uppercase
		transRunes[0] = unicode.ToUpper(transRunes[0])
	}

	return string(transRunes)
}

// isAllUpper checks if all letters in the rune slice are uppercase.
func isAllUpper(runes []rune) bool {
	hasLetter := false
	for _, r := range runes {
		if unicode.IsLetter(r) {
			hasLetter = true
			if !unicode.IsUpper(r) {
				return false
			}
		}
	}
	return hasLetter
}

// Slugify converts text to a URL-friendly slug format.
// Spaces become hyphens, text is lowercased and transliterated.
// For example: "Война и мир" -> "voina-i-mir"
func Slugify(s string) string {
	return slug.Make(s)
}
