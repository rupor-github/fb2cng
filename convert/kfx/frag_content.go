package kfx

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"fbc/fb2"
)

// ContentGenerator manages content generation state and produces KFX content fragments.
type ContentGenerator struct {
	log          *zap.Logger
	fragments    *FragmentList
	eidCounter   int    // Element ID counter
	contentCount int    // Content fragment counter
	storyName    string // Current storyline name
}

// NewContentGenerator creates a new content generator.
func NewContentGenerator(log *zap.Logger) *ContentGenerator {
	return &ContentGenerator{
		log:        log,
		fragments:  NewFragmentList(),
		eidCounter: 1000, // Start at 1000 to leave room for metadata
		storyName:  "main",
	}
}

// Fragments returns all generated fragments.
func (g *ContentGenerator) Fragments() *FragmentList {
	return g.fragments
}

// nextEID generates the next unique element ID.
func (g *ContentGenerator) nextEID() int {
	eid := g.eidCounter
	g.eidCounter++
	return eid
}

// nextContentName generates the next content fragment name.
func (g *ContentGenerator) nextContentName() string {
	g.contentCount++
	return fmt.Sprintf("content_%d", g.contentCount)
}

// ContentNode represents a KFX content element being built.
type ContentNode struct {
	Type     int            // Content type symbol ($269=text, $270=container, $271=image, etc.)
	ID       int            // Element ID ($155)
	Style    string         // Style name ($157)
	Content  any            // Content value ($145) - string, struct, or content name reference
	Children []*ContentNode // Child content nodes for containers
	Events   []StyleEvent   // Style events for text content
}

// StyleEvent represents a style event for text formatting.
type StyleEvent struct {
	Offset int    // Start offset in text
	Length int    // Length of styled range
	Style  string // Style name to apply
	LinkTo string // Link target for links
}

// GenerateFromBook generates all content fragments from an FB2 book.
func (g *ContentGenerator) GenerateFromBook(book *fb2.FictionBook) error {
	// Process each body
	for i := range book.Bodies {
		body := &book.Bodies[i]
		if body.Footnotes() {
			// Skip footnote bodies for now - handled separately
			continue
		}

		// Process sections in the body
		for j := range body.Sections {
			section := &body.Sections[j]
			if err := g.generateSection(section, 1); err != nil {
				return fmt.Errorf("generate section: %w", err)
			}
		}
	}

	return nil
}

// generateSection generates content fragments for a section.
func (g *ContentGenerator) generateSection(section *fb2.Section, depth int) error {
	// Generate title content if present
	if section.Title != nil {
		if err := g.generateTitle(section.Title, depth); err != nil {
			return err
		}
	}

	// Generate epigraphs
	for _, epigraph := range section.Epigraphs {
		if err := g.generateEpigraph(&epigraph); err != nil {
			return err
		}
	}

	// Generate section content
	for _, item := range section.Content {
		if err := g.generateFlowItem(&item, depth); err != nil {
			return err
		}
	}

	return nil
}

// generateTitle generates content for a title.
func (g *ContentGenerator) generateTitle(title *fb2.Title, depth int) error {
	styleName := fmt.Sprintf("title-h%d", min(depth, 6))

	for _, item := range title.Items {
		if item.Paragraph != nil {
			if err := g.generateParagraph(item.Paragraph, styleName); err != nil {
				return err
			}
		}
		// Empty lines are handled implicitly by paragraph spacing
	}
	return nil
}

// generateEpigraph generates content for an epigraph.
func (g *ContentGenerator) generateEpigraph(epigraph *fb2.Epigraph) error {
	for _, item := range epigraph.Flow.Items {
		if err := g.generateFlowItem(&item, 1); err != nil {
			return err
		}
	}

	// Generate text authors
	for i := range epigraph.TextAuthors {
		if err := g.generateParagraph(&epigraph.TextAuthors[i], "text-author"); err != nil {
			return err
		}
	}

	return nil
}

// generateFlowItem generates content for a flow item.
func (g *ContentGenerator) generateFlowItem(item *fb2.FlowItem, depth int) error {
	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil {
			style := item.Paragraph.Style
			if style == "" {
				style = "paragraph"
			}
			return g.generateParagraph(item.Paragraph, style)
		}

	case fb2.FlowImage:
		if item.Image != nil {
			return g.generateImage(item.Image)
		}

	case fb2.FlowPoem:
		if item.Poem != nil {
			return g.generatePoem(item.Poem)
		}

	case fb2.FlowSubtitle:
		if item.Subtitle != nil {
			return g.generateParagraph(item.Subtitle, "subtitle")
		}

	case fb2.FlowCite:
		if item.Cite != nil {
			return g.generateCite(item.Cite)
		}

	case fb2.FlowEmptyLine:
		// Empty lines are represented by paragraphs with just whitespace/spacing
		return g.generateEmptyLine()

	case fb2.FlowTable:
		if item.Table != nil {
			return g.generateTable(item.Table)
		}

	case fb2.FlowSection:
		if item.Section != nil {
			return g.generateSection(item.Section, depth+1)
		}
	}

	return nil
}

// generateParagraph generates a text content fragment for a paragraph.
func (g *ContentGenerator) generateParagraph(para *fb2.Paragraph, styleName string) error {
	// Build text content and style events
	var textBuilder strings.Builder
	var events []StyleEvent

	for _, seg := range para.Text {
		g.processInlineSegment(&seg, &textBuilder, &events, 0)
	}

	text := textBuilder.String()
	if text == "" {
		return nil // Skip empty paragraphs
	}

	// Create content fragment
	contentName := g.nextContentName()
	eid := g.nextEID()

	content := NewStruct().
		SetInt(SymElementID, int64(eid)). // $185 = eid
		SetSymbol(SymType, SymText).      // $159 = type ($269 = text)
		SetString(SymContent, text)       // $145 = content

	// Apply paragraph style
	if styleName != "" {
		content.SetString(SymStyle, styleName) // $157 = style
	}

	// Add style events for inline formatting
	if len(events) > 0 {
		eventList := make([]any, 0, len(events))
		for _, ev := range events {
			event := NewStruct().
				SetInt(SymOffset, int64(ev.Offset)).
				SetInt(SymLength, int64(ev.Length))
			if ev.Style != "" {
				event.SetString(SymStyle, ev.Style)
			}
			if ev.LinkTo != "" {
				event.SetString(SymLinkTo, ev.LinkTo)
			}
			eventList = append(eventList, event)
		}
		content.SetList(SymStyleEvents, eventList) // $142 = style_events
	}

	// Add content fragment
	frag := &Fragment{
		FType: SymContent, // $145
		FID:   SymbolID(contentName),
		Value: content,
	}
	if err := g.fragments.Add(frag); err != nil {
		return err
	}

	return nil
}

// processInlineSegment processes an inline segment, building text and style events.
func (g *ContentGenerator) processInlineSegment(seg *fb2.InlineSegment, text *strings.Builder, events *[]StyleEvent, baseOffset int) {
	startOffset := text.Len()

	switch seg.Kind {
	case fb2.InlineText:
		text.WriteString(seg.Text)

	case fb2.InlineStrong:
		g.processStyledSegment(seg, text, events, baseOffset, "strong")

	case fb2.InlineEmphasis:
		g.processStyledSegment(seg, text, events, baseOffset, "emphasis")

	case fb2.InlineStrikethrough:
		g.processStyledSegment(seg, text, events, baseOffset, "strikethrough")

	case fb2.InlineSub:
		g.processStyledSegment(seg, text, events, baseOffset, "subscript")

	case fb2.InlineSup:
		g.processStyledSegment(seg, text, events, baseOffset, "superscript")

	case fb2.InlineCode:
		g.processStyledSegment(seg, text, events, baseOffset, "code")

	case fb2.InlineNamedStyle:
		styleName := seg.Name
		if styleName == "" {
			styleName = seg.Style
		}
		g.processStyledSegment(seg, text, events, baseOffset, styleName)

	case fb2.InlineLink:
		// Process link content
		linkStart := text.Len()
		for _, child := range seg.Children {
			g.processInlineSegment(&child, text, events, baseOffset)
		}
		linkEnd := text.Len()

		if linkEnd > linkStart {
			// Add link event
			linkTo := strings.TrimPrefix(seg.Href, "#")
			*events = append(*events, StyleEvent{
				Offset: linkStart,
				Length: linkEnd - linkStart,
				LinkTo: linkTo,
			})
		}

	case fb2.InlineImageSegment:
		// Inline images are complex - for now, use placeholder
		if seg.Image != nil && seg.Image.Alt != "" {
			text.WriteString("[" + seg.Image.Alt + "]")
		} else {
			text.WriteString("[image]")
		}
	}

	// Process children for non-link segments
	if seg.Kind != fb2.InlineLink {
		for _, child := range seg.Children {
			g.processInlineSegment(&child, text, events, baseOffset)
		}
	}

	// Add style event if this segment has styling
	if seg.Kind != fb2.InlineText && seg.Kind != fb2.InlineLink && seg.Kind != fb2.InlineImageSegment {
		endOffset := text.Len()
		if endOffset > startOffset {
			styleName := kindToStyleName(seg.Kind)
			if styleName != "" {
				*events = append(*events, StyleEvent{
					Offset: startOffset,
					Length: endOffset - startOffset,
					Style:  styleName,
				})
			}
		}
	}
}

// processStyledSegment processes a styled segment and its children.
func (g *ContentGenerator) processStyledSegment(seg *fb2.InlineSegment, text *strings.Builder, events *[]StyleEvent, baseOffset int, styleName string) {
	startOffset := text.Len()

	// Add direct text
	text.WriteString(seg.Text)

	// Process children
	for _, child := range seg.Children {
		g.processInlineSegment(&child, text, events, baseOffset)
	}

	endOffset := text.Len()
	if endOffset > startOffset && styleName != "" {
		*events = append(*events, StyleEvent{
			Offset: startOffset,
			Length: endOffset - startOffset,
			Style:  styleName,
		})
	}
}

// kindToStyleName maps inline segment kind to style name.
func kindToStyleName(kind fb2.InlineSegmentKind) string {
	switch kind {
	case fb2.InlineStrong:
		return "strong"
	case fb2.InlineEmphasis:
		return "emphasis"
	case fb2.InlineStrikethrough:
		return "strikethrough"
	case fb2.InlineSub:
		return "subscript"
	case fb2.InlineSup:
		return "superscript"
	case fb2.InlineCode:
		return "code"
	default:
		return ""
	}
}

// generateImage generates content for a block-level image.
func (g *ContentGenerator) generateImage(img *fb2.Image) error {
	eid := g.nextEID()

	// Image href should reference a resource name
	resourceName := strings.TrimPrefix(img.Href, "#")

	content := NewStruct().
		SetInt(SymElementID, int64(eid)).        // $185 = eid
		SetSymbol(SymType, SymImage).            // $159 = type ($271 = image)
		SetString(SymResourceName, resourceName) // $175 = resource_name

	if img.Alt != "" {
		content.SetString(SymAltText, img.Alt) // $584 = alt_text
	}

	contentName := g.nextContentName()
	frag := &Fragment{
		FType: SymContent,
		FID:   SymbolID(contentName),
		Value: content,
	}
	return g.fragments.Add(frag)
}

// generateEmptyLine generates content for an empty line.
func (g *ContentGenerator) generateEmptyLine() error {
	// Empty line is just a paragraph with a newline or nothing
	eid := g.nextEID()

	content := NewStruct().
		SetInt(SymElementID, int64(eid)).
		SetSymbol(SymType, SymText).
		SetString(SymContent, "\n").
		SetString(SymStyle, "empty-line")

	contentName := g.nextContentName()
	frag := &Fragment{
		FType: SymContent,
		FID:   SymbolID(contentName),
		Value: content,
	}
	return g.fragments.Add(frag)
}

// generatePoem generates content for a poem.
func (g *ContentGenerator) generatePoem(poem *fb2.Poem) error {
	// Generate poem title if present
	if poem.Title != nil {
		if err := g.generateTitle(poem.Title, 3); err != nil {
			return err
		}
	}

	// Generate epigraphs
	for _, epigraph := range poem.Epigraphs {
		if err := g.generateEpigraph(&epigraph); err != nil {
			return err
		}
	}

	// Generate stanzas
	for _, stanza := range poem.Stanzas {
		if err := g.generateStanza(&stanza); err != nil {
			return err
		}
	}

	// Generate text authors
	for i := range poem.TextAuthors {
		if err := g.generateParagraph(&poem.TextAuthors[i], "text-author"); err != nil {
			return err
		}
	}

	return nil
}

// generateStanza generates content for a poem stanza.
func (g *ContentGenerator) generateStanza(stanza *fb2.Stanza) error {
	// Generate stanza title if present
	if stanza.Title != nil {
		if err := g.generateTitle(stanza.Title, 4); err != nil {
			return err
		}
	}

	// Generate verses
	for i := range stanza.Verses {
		if err := g.generateParagraph(&stanza.Verses[i], "verse"); err != nil {
			return err
		}
	}

	return nil
}

// generateCite generates content for a citation.
func (g *ContentGenerator) generateCite(cite *fb2.Cite) error {
	// Generate citation flow content
	for _, item := range cite.Items {
		// Override style for citation items
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				if err := g.generateParagraph(item.Paragraph, "cite"); err != nil {
					return err
				}
			}
		default:
			if err := g.generateFlowItem(&item, 1); err != nil {
				return err
			}
		}
	}

	// Generate text authors
	for i := range cite.TextAuthors {
		if err := g.generateParagraph(&cite.TextAuthors[i], "cite-author"); err != nil {
			return err
		}
	}

	return nil
}

// generateTable generates content for a table.
func (g *ContentGenerator) generateTable(table *fb2.Table) error {
	// Tables in KFX use $278 (table) and $279 (table_row) types
	// For now, generate a simplified text representation
	// Full table support would require container elements

	eid := g.nextEID()

	// Build table text representation
	var text strings.Builder
	for _, row := range table.Rows {
		for i, cell := range row.Cells {
			if i > 0 {
				text.WriteString("\t")
			}
			text.WriteString(cell.AsPlainText())
		}
		text.WriteString("\n")
	}

	content := NewStruct().
		SetInt(SymElementID, int64(eid)).
		SetSymbol(SymType, SymText).
		SetString(SymContent, text.String()).
		SetString(SymStyle, "table")

	contentName := g.nextContentName()
	frag := &Fragment{
		FType: SymContent,
		FID:   SymbolID(contentName),
		Value: content,
	}
	return g.fragments.Add(frag)
}
