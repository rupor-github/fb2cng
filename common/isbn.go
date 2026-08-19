package common

import (
	"fmt"
	"strings"
	"unicode"
)

// ISBNKind identifies the validated ISBN length/checksum variant.
type ISBNKind string

const (
	ISBNKind10 ISBNKind = "ISBN-10"
	ISBNKind13 ISBNKind = "ISBN-13"
)

// NormalizeISBN strips common separators and validates ISBN-10/ISBN-13 checksums.
func NormalizeISBN(in string) (string, ISBNKind, error) {
	s := strings.Map(func(r rune) rune {
		switch {
		case r == '-' || unicode.IsSpace(r):
			return -1
		default:
			return r
		}
	}, strings.TrimSpace(in))
	s = strings.ToUpper(s)
	if s == "" {
		return "", "", nil
	}

	switch len(s) {
	case 10:
		if validISBN10(s) {
			return s, ISBNKind10, nil
		}
		return "", "", fmt.Errorf("invalid ISBN-10 checksum: %q", s)
	case 13:
		if validISBN13(s) {
			return s, ISBNKind13, nil
		}
		return "", "", fmt.Errorf("invalid ISBN-13 checksum: %q", s)
	}
	return "", "", fmt.Errorf("isbn must be 10 or 13 characters after normalization, got %d", len(s))
}

func validISBN10(s string) bool {
	sum := 0
	for i := range 10 {
		c := s[i]
		var digit int
		switch {
		case c >= '0' && c <= '9':
			digit = int(c - '0')
		case i == 9 && c == 'X':
			digit = 10
		default:
			return false
		}
		sum += (10 - i) * digit
	}
	return sum%11 == 0
}

func validISBN13(s string) bool {
	sum := 0
	for i := range 13 {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		weight := 1
		if i%2 == 1 {
			weight = 3
		}
		sum += weight * int(c-'0')
	}
	return sum%10 == 0
}
