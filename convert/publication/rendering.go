package publication

import (
	"github.com/beevik/etree"

	"fbc/content"
	"fbc/fb2"
)

// Options selects syntax and optional adapter-owned rendering behavior. The
// prepared output identity remains available to templates, but never selects hooks.
type Options struct {
	Markup            Markup
	Text              TextDecorator
	FloatingFootnotes FloatingFootnoteRenderer
	RewriteStylesheet StylesheetRewriter
}

// TextDecorator adds reader-specific wrappers without owning source traversal,
// page boundaries, links or images. Each build must receive its own instance.
type TextDecorator interface {
	DocumentRoot(body *etree.Element) *etree.Element
	NextParagraph()
	SavePosition() func()
	InlineParent(parent *etree.Element) *etree.Element
	IsWrapper(element *etree.Element) bool
	AppendText(parent *etree.Element, chunk string)
}

// FloatingFootnoteRenderer owns a reader's alternative floating-note structure.
// Shared rendering still registers references and resolves publication links.
type FloatingFootnoteRenderer interface {
	ReferenceBodies(bodies []*fb2.Body) []*fb2.Body
	Render(parent *etree.Element, section *fb2.Section, rendering FootnoteRendering) error
	RewriteLinks(chapters []Chapter, prepared *content.Content, rendered map[string]string)
}

// FootnoteRendering exposes the shared operations a custom note renderer needs.
// These callbacks are valid only during the current rendering call.
type FootnoteRendering struct {
	Content          *content.Content
	AppendBacklinks  func(parent *etree.Element, sectionID, separator string) error
	AppendPageMarker func(parent *etree.Element)
}

func (c *renderState) nextParagraph() {
	if c.text != nil {
		c.text.NextParagraph()
	}
}

func (c *renderState) inlineParent(parent *etree.Element) *etree.Element {
	if c.text != nil {
		return c.text.InlineParent(parent)
	}
	return parent
}

func (c *renderState) renderFloatingFootnote(parent *etree.Element, section *fb2.Section) error {
	return c.footnotes.Render(parent, section, FootnoteRendering{
		Content: c.Content,
		AppendBacklinks: func(parent *etree.Element, sectionID, separator string) error {
			_, err := appendFootnoteBacklinks(parent, c, sectionID, footnoteBacklinkOptions{
				LeadingSeparator: separator, EPUB3Role: c.markup == HTML5,
			})
			return err
		},
		AppendPageMarker: func(parent *etree.Element) { appendPageMarker(parent, c) },
	})
}
