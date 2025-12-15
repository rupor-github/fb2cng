package gencfg

import (
	"path/filepath"
)

// joinPath joins path elements into a single path string with proper separators.
func joinPath(elem ...string) string {
	return filepath.Join(elem...)
}
