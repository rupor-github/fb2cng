package common

import (
	"fmt"
	"strings"
	"unicode"
)

// NormalizeASIN normalizes and validates an Amazon ASIN.
//
// For books, ASIN is a 10-character alphanumeric identifier and is often the same
// as ISBN-10 (including an 'X' check digit).
func NormalizeASIN(in string) (string, error) {
	s := strings.TrimSpace(in)
	if s == "" {
		return "", nil
	}

	// Be forgiving about common separators.
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '-' || unicode.IsSpace(r):
			return -1
		default:
			return r
		}
	}, s)
	s = strings.ToUpper(s)

	if len(s) != 10 {
		return "", fmt.Errorf("asin must be 10 characters, got %d", len(s))
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		isDigit := c >= '0' && c <= '9'
		isUpper := c >= 'A' && c <= 'Z'
		if !isDigit && !isUpper {
			return "", fmt.Errorf("asin must be alphanumeric A-Z0-9, got %q", c)
		}
	}

	return s, nil
}
