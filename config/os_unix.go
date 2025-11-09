//go:build !windows

package config

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// CleanFileName removes not allowed characters form file name.
func CleanFileName(in string) string {
	out := strings.TrimLeft(strings.Map(func(sym rune) rune {
		if strings.ContainsRune(string(os.PathSeparator)+string(os.PathListSeparator), sym) {
			return -1
		}
		return sym
	}, in), ".")
	if len(out) == 0 {
		out = "_bad_file_name_"
	}
	return out
}

// EnableColorOutput checks if colorized output is possible.
func EnableColorOutput(stream *os.File) bool {
	return term.IsTerminal(int(stream.Fd()))
}
