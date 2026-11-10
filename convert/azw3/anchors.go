package azw3

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/beevik/etree"
)

// assignTextAnchors supplies reader-facing element identities independently of
// publication IDs and kindle:pos byte coordinates. KindleGen assigns these to
// HTML content, including note references, aside boundaries and return links.
// Reconstructed XHTML hides them, so preserving epub:type alone is insufficient
// evidence of matching reader input.
func assignTextAnchors(ctx context.Context, doc *pendingDocument, body *etree.Element, fid uint32, next *uint32) error {
	type candidate struct {
		element *etree.Element
		table   bool
	}
	stack := []candidate{{element: body}}
	count := 0
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		element := item.element
		if namespace := element.NamespaceURI(); namespace != "" && namespace != "http://www.w3.org/1999/xhtml" {
			continue // SVG/MathML have their own element identities.
		}
		inTable := item.table || element.Tag == "table"
		children := element.ChildElements()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, candidate{element: children[i], table: inTable})
		}
		// These are compiler-owned attributes, not authored publication anchors.
		element.RemoveAttr("aid")
		if element == body {
			element.CreateAttr("aid", base32(fid, 4))
			count++
			continue
		}
		// KindleGen omits aids inside tables; adding them there is known to
		// disturb Kindle's table layout. Native page-break divs have no aid too.
		if inTable || explicitPageBreak(element) || !anchorElement(element.Tag) {
			continue
		}
		if *next == math.MaxUint32 {
			return errors.New("too many native element anchor identifiers")
		}
		element.CreateAttr("aid", base32(*next, 4))
		*next++
		count++
	}
	doc.Changes = append(doc.Changes, markupChange{
		Tag: "body", Action: fmt.Sprintf("assigned native element anchor identifiers: %d", count),
	})
	return nil
}

func anchorElement(tag string) bool {
	return slices.Contains([]string{
		"a", "abbr", "address", "article", "aside", "audio", "b", "bdo", "blockquote", "button", "cite", "code", "dd", "del",
		"details", "dfn", "div", "dl", "dt", "em", "fieldset", "figcaption", "figure", "footer", "h1", "h2", "h3", "h4", "h5", "h6",
		"header", "hgroup", "i", "ins", "kbd", "label", "legend", "li", "map", "mark", "meter", "nav", "ol", "output", "p", "pre",
		"progress", "q", "rp", "rt", "samp", "section", "select", "small", "span", "strong", "sub", "summary", "sup", "textarea",
		"time", "ul", "var", "video",
	}, tag)
}
