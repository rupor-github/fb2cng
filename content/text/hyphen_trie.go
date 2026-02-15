package text

import (
	"strings"
	"unicode"
)

// addPatternString specialized function for TeX-style hyphenation patterns.
// Accepts strings of the form '.hy2p'. The value it stores is of type []int
func (p *trie) addPatternString(s string) {

	v := []int{}

	const zero = '0'

	// Convert to runes once to avoid byte-offset vs rune-index confusion
	// (range over string yields byte offsets, not rune indices).
	runes := []rune(s)

	for i, sym := range runes {

		if unicode.IsDigit(sym) {
			if i == 0 {
				// This is a prefix number
				v = append(v, int(sym-zero))
			}
			// this is a number referring to the previous character, and has
			// already been handled
			continue
		}

		if i < len(runes)-1 {
			// look ahead to see if it's followed by a number
			next := runes[i+1]
			if unicode.IsDigit(next) {
				// next char is the hyphenation value for this char
				v = append(v, int(next-zero))
			} else {
				// hyphenation for this char is an implied zero
				v = append(v, 0)
			}
		} else {
			// last character gets an implied zero
			v = append(v, 0)
		}
	}

	pure := strings.Map(func(sym rune) rune {
		if unicode.IsDigit(sym) {
			return -1
		}
		return sym
	}, s)

	leaf := p.addRunes(strings.NewReader(pure))
	if leaf == nil {
		return
	}
	leaf.value = v
}
