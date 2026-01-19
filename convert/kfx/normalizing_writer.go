package kfx

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// normalizingWriter accumulates text with whitespace normalization.
// It collapses consecutive whitespace to single spaces while tracking
// the rune count for style event offsets. Leading and trailing whitespace
// is automatically trimmed using the pendingSpace approach.
type normalizingWriter struct {
	buf               strings.Builder
	runeCount         int
	pendingSpace      bool // Deferred space - only written if followed by non-space
	preserveWS        bool // When true, write text as-is (for code blocks)
	suppressNextSpace bool // Suppress next pending space (after structural breaks)
}

// newNormalizingWriter creates a new normalizing writer.
func newNormalizingWriter() *normalizingWriter {
	return &normalizingWriter{}
}

// WriteString writes text, normalizing whitespace unless preserveWS is set.
// Returns the rune count of what was actually written.
func (nw *normalizingWriter) WriteString(s string) int {
	if s == "" {
		return 0
	}

	if nw.preserveWS {
		// In preserve mode, write any pending space first (unless suppressed)
		if nw.pendingSpace && !nw.suppressNextSpace {
			nw.buf.WriteRune(' ')
			nw.runeCount++
		}
		nw.pendingSpace = false
		nw.suppressNextSpace = false
		nw.buf.WriteString(s)
		count := utf8.RuneCountInString(s)
		nw.runeCount += count
		return count
	}

	written := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			// Only mark pending space if we've written content and not suppressing.
			// Keep suppressNextSpace active until we hit non-whitespace content,
			// so all whitespace after structural breaks is suppressed.
			if (nw.buf.Len() > 0 || nw.pendingSpace) && !nw.suppressNextSpace {
				nw.pendingSpace = true
			}
			// Note: DO NOT clear suppressNextSpace here - it stays active until
			// we encounter actual content (non-whitespace).
		} else {
			// Write pending space before this non-space character (unless suppressed)
			if nw.pendingSpace && !nw.suppressNextSpace {
				nw.buf.WriteRune(' ')
				nw.runeCount++
				written++
			}
			nw.pendingSpace = false
			nw.suppressNextSpace = false // Only clear when we write actual content
			nw.buf.WriteRune(r)
			nw.runeCount++
			written++
		}
	}
	return written
}

// SetPreserveWhitespace sets whether to preserve whitespace (for code blocks).
func (nw *normalizingWriter) SetPreserveWhitespace(preserve bool) {
	nw.preserveWS = preserve
}

// WriteRaw writes a string directly without normalization.
// Used for structural characters like newlines between title paragraphs.
// Discards any pending space and suppresses the next pending space
// (to prevent " " after structural breaks like "\n").
func (nw *normalizingWriter) WriteRaw(s string) {
	nw.pendingSpace = false     // Discard pending space before structural break
	nw.suppressNextSpace = true // Suppress leading space after break
	nw.buf.WriteString(s)
	nw.runeCount += utf8.RuneCountInString(s)
}

// String returns the accumulated text. No trimming needed since pending space
// approach ensures no leading/trailing whitespace is written.
func (nw *normalizingWriter) String() string {
	return nw.buf.String()
}

// Len returns the byte length of accumulated text.
func (nw *normalizingWriter) Len() int {
	return nw.buf.Len()
}

// RuneCount returns the current rune count (matches string length in runes).
func (nw *normalizingWriter) RuneCount() int {
	return nw.runeCount
}

// Reset clears the writer for reuse.
func (nw *normalizingWriter) Reset() {
	nw.buf.Reset()
	nw.runeCount = 0
	nw.pendingSpace = false
	nw.preserveWS = false
}

// HasPendingSpace returns true if there's a pending space that hasn't been written.
func (nw *normalizingWriter) HasPendingSpace() bool {
	return nw.pendingSpace
}

// ConsumePendingSpace returns true and clears pending space if there was one,
// otherwise returns false. Used for preserving spaces across flush boundaries.
func (nw *normalizingWriter) ConsumePendingSpace() bool {
	if nw.pendingSpace {
		nw.pendingSpace = false
		return true
	}
	return false
}

// SetPendingSpace marks that a space should precede the next non-space content.
func (nw *normalizingWriter) SetPendingSpace() {
	nw.pendingSpace = true
}
