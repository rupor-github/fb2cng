package publication

import (
	"strings"

	"github.com/beevik/etree"

	"fbc/common"
	"fbc/content/text"
	"fbc/fb2"
)

// appendFloatFootnoteSectionContentAZW3 follows the single-paragraph aside
// structure of Kindle Method 1, retaining our existing backlink semantics.
// Authored markup is reduced to text; newlines are content, not pretty printing.
func appendFloatFootnoteSectionContentAZW3(parent *etree.Element, c *renderState, section *fb2.Section) error {
	aside := parent.CreateElement("aside")
	aside.CreateAttr("epub:type", "footnote")
	if section.ID != "" {
		aside.CreateAttr("id", section.ID)
	}
	if section.Lang != "" {
		aside.CreateAttr("xml:lang", section.Lang)
	}
	p := aside.CreateElement("p")
	p.CreateAttr("class", "footnote")

	var title, body azw3FootnoteText
	title.title(section.Title)
	body.section(section, false)
	label := strings.TrimSpace(strings.Join(title.lines, "\n"))
	// Distinguish every original title from the flattened body, independently
	// of return navigation. Renumbered labels retain label_template output.
	if c.FootnotesMode == common.FootnotesModeFloat && label != "" &&
		!(strings.HasPrefix(label, "[") && strings.HasSuffix(label, "]")) {
		label = "[" + label + "]"
	}
	refs := c.BackLinkIndex[section.ID]
	if len(refs) > 0 && label != "" {
		leading := p.CreateElement("a")
		leading.CreateAttr("href", refs[0].Href)
		leading.CreateAttr("epub:type", "backlink")
		leading.CreateAttr("role", "doc-backlink")
		// This caption is the note label, not a backlink-template/location label.
		leading.SetText(label)
	} else {
		p.CreateText(label)
	}
	if label != "" && len(body.lines) > 0 {
		p.CreateText(text.NBSP)
	}
	visible := 0
	for _, line := range body.lines {
		if line != "" {
			visible++
		}
	}
	if visible > 1 && c.MoreParaStr != "" {
		// Native compilation resolves CSS visibility and lowers this temporary
		// marker to plain text (or removes it). No span survives in the AZW3 note.
		more := p.CreateElement("span")
		more.CreateAttr("class", "footnote-more")
		more.CreateAttr("data-fbc-footnote-more", "")
		more.SetText(c.MoreParaStr)
	}
	p.CreateText(strings.Join(body.lines, "\n"))

	// Keep approximate page anchors at note boundaries, never inside its plain
	// paragraph. Generated return captions do not introduce page-marker spans.
	tracking := c.PageTrackingEnabled
	c.PageTrackingEnabled = false
	_, err := appendEPUBFootnoteBacklinks(p, c, section.ID, epubFootnoteBacklinkOptions{LeadingSeparator: "\n", EPUB3Role: true})
	c.PageTrackingEnabled = tracking
	if err != nil {
		return err
	}
	_, markers := c.SplitTextByPage(azw3FootnoteInlineTextFromElement(p))
	for _, marker := range markers {
		if marker {
			appendPageMarker(parent, c)
		}
	}
	return nil
}

func azw3FootnoteInlineTextFromElement(element *etree.Element) string {
	var result strings.Builder
	for _, token := range element.Child {
		switch child := token.(type) {
		case *etree.CharData:
			result.WriteString(child.Data)
		case *etree.Element:
			result.WriteString(azw3FootnoteInlineTextFromElement(child))
		}
	}
	return result.String()
}

// azw3FootnoteText is intentionally local: the other formats' extraction rules
// differ, notably for links, image fallbacks and block separators.
type azw3FootnoteText struct {
	lines []string
}

func (out *azw3FootnoteText) line(value string) {
	// Collapse source HTML whitespace, not meaningful Unicode spaces such as NBSP.
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	})
	if value = strings.Join(fields, " "); value != "" {
		out.lines = append(out.lines, value)
	}
}

func (out *azw3FootnoteText) paragraph(p *fb2.Paragraph) {
	if p != nil {
		out.line(azw3FootnoteInlineText(p.Text))
	}
}

func (out *azw3FootnoteText) title(title *fb2.Title) {
	if title == nil {
		return
	}
	for _, item := range title.Items {
		if item.EmptyLine {
			out.lines = append(out.lines, "")
		} else {
			out.paragraph(item.Paragraph)
		}
	}
}

func (out *azw3FootnoteText) epigraphs(epigraphs []fb2.Epigraph) {
	for _, epigraph := range epigraphs {
		out.flow(epigraph.Flow.Items)
		out.authors(epigraph.TextAuthors)
	}
}

func (out *azw3FootnoteText) authors(authors []fb2.Paragraph) {
	for i := range authors {
		out.paragraph(&authors[i])
	}
}

func (out *azw3FootnoteText) section(section *fb2.Section, includeTitle bool) {
	if section == nil {
		return
	}
	if includeTitle {
		out.title(section.Title)
	}
	out.epigraphs(section.Epigraphs)
	if section.Image != nil {
		out.line(azw3FootnoteImageText(section.Image.Alt))
	}
	if section.Annotation != nil {
		out.flow(section.Annotation.Items)
	}
	out.flow(section.Content)
}

func (out *azw3FootnoteText) flow(items []fb2.FlowItem) {
	for _, item := range items {
		switch item.Kind {
		case fb2.FlowParagraph:
			out.paragraph(item.Paragraph)
		case fb2.FlowSubtitle:
			out.paragraph(item.Subtitle)
		case fb2.FlowImage:
			if item.Image != nil {
				out.line(azw3FootnoteImageText(item.Image.Alt))
			}
		case fb2.FlowEmptyLine:
			out.lines = append(out.lines, "")
		case fb2.FlowSection:
			out.section(item.Section, true)
		case fb2.FlowCite:
			if item.Cite != nil {
				out.flow(item.Cite.Items)
				out.authors(item.Cite.TextAuthors)
			}
		case fb2.FlowPoem:
			out.poem(item.Poem)
		case fb2.FlowTable:
			if item.Table != nil {
				for _, row := range item.Table.Rows {
					cells := make([]string, 0, len(row.Cells))
					for _, cell := range row.Cells {
						cells = append(cells, azw3FootnoteInlineText(cell.Content))
					}
					out.line(strings.Join(cells, " "))
				}
			}
		}
	}
}

func (out *azw3FootnoteText) poem(poem *fb2.Poem) {
	if poem == nil {
		return
	}
	out.title(poem.Title)
	out.epigraphs(poem.Epigraphs)
	out.authors(poem.Subtitles)
	for _, stanza := range poem.Stanzas {
		out.title(stanza.Title)
		out.paragraph(stanza.Subtitle)
		out.authors(stanza.Verses)
	}
	out.authors(poem.TextAuthors)
	if poem.Date != nil {
		out.line(poem.Date.Display)
	}
}

func azw3FootnoteInlineText(segments []fb2.InlineSegment) string {
	parts := appendAZW3FootnoteInlineParts(nil, segments)
	var result strings.Builder
	for i, part := range parts {
		if part.linkPlaceholder && i > 0 && i+1 < len(parts) &&
			strings.HasSuffix(strings.TrimSpace(parts[i-1].text), "[") &&
			strings.HasPrefix(strings.TrimSpace(parts[i+1].text), "]") {
			// Reuse source brackets surrounding the link, even across formatting
			// boundaries. Only remove brackets we generated, never authored text.
			label := strings.TrimSpace(part.text)
			start := strings.Index(part.text, label)
			part.text = part.text[:start] + label[1:len(label)-1] + part.text[start+len(label):]
		}
		result.WriteString(part.text)
	}
	return result.String()
}

type azw3FootnoteInlinePart struct {
	text            string
	linkPlaceholder bool
}

func appendAZW3FootnoteInlineParts(parts []azw3FootnoteInlinePart, segments []fb2.InlineSegment) []azw3FootnoteInlinePart {
	for _, segment := range segments {
		var part azw3FootnoteInlinePart
		switch segment.Kind {
		case fb2.InlineImageSegment:
			if segment.Image != nil {
				part.text = azw3FootnoteImageText(segment.Image.Alt)
			}
		case fb2.InlineLink:
			caption := segment.Text + azw3FootnoteInlineText(segment.Children)
			part = azw3FootnoteLinkPart(caption, segment.Href)
		default:
			part.text = segment.Text
		}
		if part.text != "" {
			if len(parts) > 0 && !part.linkPlaceholder && !parts[len(parts)-1].linkPlaceholder {
				parts[len(parts)-1].text += part.text
			} else {
				parts = append(parts, part)
			}
		}
		if segment.Kind != fb2.InlineLink && segment.Kind != fb2.InlineImageSegment {
			parts = appendAZW3FootnoteInlineParts(parts, segment.Children)
		}
	}
	return parts
}

func azw3FootnoteImageText(alt string) string {
	return "[img:" + strings.TrimSpace(alt) + "]"
}

func azw3FootnoteLinkPart(caption, href string) azw3FootnoteInlinePart {
	label := strings.TrimSpace(caption)
	if label == "" {
		caption = href
		label = strings.TrimSpace(caption)
	}
	if label == "" || (strings.HasPrefix(label, "[") && strings.HasSuffix(label, "]")) {
		return azw3FootnoteInlinePart{text: caption}
	}
	// Keep authored surrounding whitespace outside the placeholder so adjacent
	// inline text still reads naturally. The caller emits escaped XML text only.
	start := strings.Index(caption, label)
	return azw3FootnoteInlinePart{
		text:            caption[:start] + "[link:" + label + "]" + caption[start+len(label):],
		linkPlaceholder: true,
	}
}

// Paragraphs and other authored elements no longer have individual anchors in a
// flattened note. Preserve incoming navigation by targeting the containing note.
func redirectAZW3FootnoteLinks(chapters []Chapter, c *renderState, rendered idToFileMap) {
	redirects := make(map[string]string)
	for id, ref := range c.IDsIndex {
		if _, exists := rendered[id]; exists {
			continue
		}
		for _, ancestor := range ref.Path {
			if section, ok := ancestor.(*fb2.Section); ok {
				if _, note := c.FootnotesIndex[section.ID]; note && rendered[section.ID] != "" {
					redirects[id] = section.ID
					break
				}
			}
		}
	}
	for _, chapter := range chapters {
		if chapter.Doc == nil {
			continue
		}
		for _, link := range chapter.Doc.FindElements("//a[@href]") {
			if id, local := strings.CutPrefix(link.SelectAttrValue("href", ""), "#"); local {
				if noteID := redirects[id]; noteID != "" {
					link.CreateAttr("href", "#"+noteID)
				}
			}
		}
	}
}
