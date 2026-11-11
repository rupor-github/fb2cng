package publication

import (
	"github.com/beevik/etree"

	"fbc/content"
)

// Markup selects content syntax independently of the actual output format used
// by templates and normalization. It does not select a container or image policy.
type Markup uint8

const (
	// XHTML11 uses the existing EPUB2-style documents and floating-note markup.
	XHTML11 Markup = iota
	// HTML5 uses HTML5 documents, aside footnotes, noterefs and backlink roles.
	HTML5
)

// renderState keeps the chosen dialect local to one build, without changing the
// prepared Content's output identity or adding backend state to other converters.
type renderState struct {
	*content.Content
	markup    Markup
	backlinks map[*etree.Element]string
}
