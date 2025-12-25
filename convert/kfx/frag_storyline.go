package kfx

import (
	"fmt"
	"strings"

	"fbc/fb2"
)

// Symbol constants for structure - additions to main symbols.go
const (
	SymPageTemplates = 141 // page_templates - list of content locations in section
	SymPage          = 326 // page - layout value
)

// BuildStorylineFragment creates a $259 storyline fragment.
// Based on reference KFX, storyline has:
// - Named FID (like "l1", "l2", etc.)
// - $176 (story_name) as symbol reference
// - $146 (content_list) with content entries
func BuildStorylineFragment(storyName string, contentEntries []any) *Fragment {
	storyline := NewStruct().
		Set(SymStoryName, SymbolByName(storyName)) // $176 = story_name as symbol

	if len(contentEntries) > 0 {
		storyline.SetList(SymContentList, contentEntries) // $146 = content_list
	}

	return &Fragment{
		FType:   SymStoryline,
		FIDName: storyName,
		Value:   storyline,
	}
}

// BuildSectionFragment creates a $260 section fragment.
// Based on reference KFX, section has:
// - Named FID (like "c0", "c1", etc.)
// - $174 (section_name) as symbol reference
// - $141 (page_templates) with layout entries for storylines
func BuildSectionFragment(sectionName string, pageTemplates []any) *Fragment {
	section := NewStruct().
		Set(SymSectionName, SymbolByName(sectionName)) // $174 = section_name as symbol

	if len(pageTemplates) > 0 {
		section.SetList(SymPageTemplates, pageTemplates) // $141 = page_templates
	}

	return &Fragment{
		FType:   SymSection,
		FIDName: sectionName,
		Value:   section,
	}
}

// NewPageTemplateEntry creates a page template entry for section's $141.
// Based on reference: {$155: eid, $176: storyline_name, $66: w, $67: h, $156: layout, $140: float, $159: $270}
func NewPageTemplateEntry(eid int, storylineName string, width, height int) StructValue {
	return NewStruct().
		SetInt(SymUniqueID, int64(eid)).           // $155 = id
		Set(SymStoryName, SymbolByName(storylineName)). // $176 = story_name ref
		SetInt(SymWidth, int64(width)).            // $66 = width
		SetInt(SymHeight, int64(height)).          // $67 = height
		SetSymbol(SymLayout, SymPage).             // $156 = layout = $326 (page)
		SetSymbol(SymFloat, SymCenter).            // $140 = float = $320 (center)
		SetSymbol(SymType, SymContainer)           // $159 = type = $270 (container)
}

// ContentRef represents a reference to content within a storyline.
type ContentRef struct {
	EID           int    // Element ID ($155)
	Type          int    // Content type symbol ($269=text, $270=container, $271=image, etc.)
	ContentName   string // Name of the content fragment
	ContentOffset int    // Offset within content fragment ($403)
	Style         string // Optional style name
	StyleEvents   []StyleEventRef // Optional inline style events ($142)
	Children      []any  // Optional nested content for containers
}

// StyleEventRef represents a style event for inline formatting ($142).
type StyleEventRef struct {
	Offset int    // $143 - start offset
	Length int    // $144 - length
	Style  string // $157 - style name
	LinkTo string // $179 - link target (optional)
}

// NewContentEntry creates a content entry for storyline's $146.
// Based on reference: {$155: eid, $157: style, $159: type, $145: {name: content_X, $403: offset}}
func NewContentEntry(ref ContentRef) StructValue {
	entry := NewStruct().
		SetInt(SymUniqueID, int64(ref.EID)).   // $155 = id
		SetSymbol(SymType, ref.Type)           // $159 = type

	if ref.Style != "" {
		entry.Set(SymStyle, SymbolByName(ref.Style)) // $157 = style as symbol
	}

	// Style events for inline formatting
	if len(ref.StyleEvents) > 0 {
		events := make([]any, 0, len(ref.StyleEvents))
		for _, se := range ref.StyleEvents {
			ev := NewStruct().
				SetInt(SymOffset, int64(se.Offset)). // $143 = offset
				SetInt(SymLength, int64(se.Length)) // $144 = length
			if se.Style != "" {
				ev.Set(SymStyle, SymbolByName(se.Style)) // $157 = style
			}
			if se.LinkTo != "" {
				ev.Set(SymLinkTo, SymbolByName(se.LinkTo)) // $179 = link_to
			}
			events = append(events, ev)
		}
		entry.SetList(SymStyleEvents, events) // $142 = style_events
	}

	// Content reference - nested struct with name and offset
	contentRef := map[string]any{
		"name":  SymbolByName(ref.ContentName),
		"$403":  ref.ContentOffset,
	}
	entry.Set(SymContent, contentRef) // $145 = content

	// Nested children for containers
	if len(ref.Children) > 0 {
		entry.SetList(SymContentList, ref.Children) // $146 = content_list
	}

	return entry
}

// StorylineBuilder helps build storyline content incrementally.
type StorylineBuilder struct {
	name            string       // Storyline name (e.g., "l1")
	sectionName     string       // Associated section name (e.g., "c0")
	contentEntries  []ContentRef
	eidCounter      int
	pageTemplateEID int          // Separate EID for page template container
}

// NewStorylineBuilder creates a new storyline builder.
// Allocates the first EID for the page template container.
func NewStorylineBuilder(storyName, sectionName string, startEID int) *StorylineBuilder {
	return &StorylineBuilder{
		name:            storyName,
		sectionName:     sectionName,
		pageTemplateEID: startEID,      // First EID goes to page template
		eidCounter:      startEID + 1,  // Content EIDs start after page template
	}
}

// AddContent adds a content reference to the storyline.
func (sb *StorylineBuilder) AddContent(contentType int, contentName string, contentOffset int, style string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	sb.contentEntries = append(sb.contentEntries, ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		Style:         style,
	})

	return eid
}

// AddContentWithEvents adds content with style events.
func (sb *StorylineBuilder) AddContentWithEvents(contentType int, contentName string, contentOffset int, style string, events []StyleEventRef) int {
	eid := sb.eidCounter
	sb.eidCounter++

	sb.contentEntries = append(sb.contentEntries, ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		Style:         style,
		StyleEvents:   events,
	})

	return eid
}

// FirstEID returns the first EID used by this storyline content.
func (sb *StorylineBuilder) FirstEID() int {
	if len(sb.contentEntries) > 0 {
		return sb.contentEntries[0].EID
	}
	return sb.eidCounter
}

// PageTemplateEID returns the EID allocated for the page template.
func (sb *StorylineBuilder) PageTemplateEID() int {
	return sb.pageTemplateEID
}

// NextEID returns the next EID that will be assigned.
func (sb *StorylineBuilder) NextEID() int {
	return sb.eidCounter
}

// Build creates the storyline and section fragments.
// Returns storyline fragment, section fragment.
func (sb *StorylineBuilder) Build(width, height int) (*Fragment, *Fragment) {
	// Build content entries for storyline
	entries := make([]any, 0, len(sb.contentEntries))
	for _, ref := range sb.contentEntries {
		entries = append(entries, NewContentEntry(ref))
	}

	// Create storyline fragment
	storylineFrag := BuildStorylineFragment(sb.name, entries)

	// Create page template entry for section - uses dedicated EID
	pageTemplates := []any{
		NewPageTemplateEntry(sb.pageTemplateEID, sb.name, width, height),
	}

	// Create section fragment
	sectionFrag := BuildSectionFragment(sb.sectionName, pageTemplates)

	return storylineFrag, sectionFrag
}

// GenerateStorylineFromBook creates storyline and section fragments from an FB2 book.
// It uses the provided StyleRegistry to reference styles by name.
// Returns fragments, next EID, and section names for document_data.
func GenerateStorylineFromBook(book *fb2.FictionBook, styles *StyleRegistry, startEID int) (*FragmentList, int, []string, error) {
	fragments := NewFragmentList()
	eidCounter := startEID
	sectionNames := make([]string, 0)

	// Content accumulator - collects text into content fragments
	contentFragments := make(map[string][]string)
	contentCount := 0

	// Default dimensions (will be adjusted by Kindle)
	defaultWidth := 600
	defaultHeight := 800

	sectionCount := 0

	// Process each body
	for i := range book.Bodies {
		body := &book.Bodies[i]
		if body.Footnotes() {
			continue
		}

		// Process sections in the body
		for j := range body.Sections {
			section := &body.Sections[j]
			sectionCount++

			// Generate names: "l1", "l2", ... for storylines; "c0", "c1", ... for sections
			storyName := fmt.Sprintf("l%d", sectionCount)
			sectionName := fmt.Sprintf("c%d", sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline builder
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter)

			// Process section content
			contentCount++
			contentName := fmt.Sprintf("content_%d", contentCount)
			var contentList []string

			if err := processStorylineContent(section, sb, styles, &contentList, contentName, 1); err != nil {
				return nil, 0, nil, err
			}

			// Store content fragment data
			if len(contentList) > 0 {
				contentFragments[contentName] = contentList
			}

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build(defaultWidth, defaultHeight)

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, err
			}
		}
	}

	// Create content fragments from accumulated content
	for name, contentList := range contentFragments {
		contentFrag := buildContentFragmentByName(name, contentList)
		if err := fragments.Add(contentFrag); err != nil {
			return nil, 0, nil, err
		}
	}

	return fragments, eidCounter, sectionNames, nil
}

// processStorylineContent processes FB2 section content into storyline entries.
func processStorylineContent(section *fb2.Section, sb *StorylineBuilder, styles *StyleRegistry, contentList *[]string, contentName string, depth int) error {
	addContent := func(text, styleName string) {
		styles.EnsureStyle(styleName) // Register style if not already present
		offset := len(*contentList)
		*contentList = append(*contentList, text)
		sb.AddContent(SymText, contentName, offset, styleName)
	}

	// Process title
	if section.Title != nil {
		styleName := fmt.Sprintf("h%d", min(depth, 6))
		for _, item := range section.Title.Items {
			if item.Paragraph != nil {
				text := paragraphToText(item.Paragraph)
				if text != "" {
					addContent(text, styleName)
				}
			}
		}
	}

	// Process epigraphs
	for _, epigraph := range section.Epigraphs {
		for _, item := range epigraph.Flow.Items {
			processFlowItemToStoryline(&item, sb, styles, contentList, contentName, depth)
		}
		for i := range epigraph.TextAuthors {
			text := paragraphToText(&epigraph.TextAuthors[i])
			if text != "" {
				addContent(text, "text-author")
			}
		}
	}

	// Process content items
	for _, item := range section.Content {
		processFlowItemToStoryline(&item, sb, styles, contentList, contentName, depth)
	}

	return nil
}

// processFlowItemToStoryline processes a flow item into storyline content.
func processFlowItemToStoryline(item *fb2.FlowItem, sb *StorylineBuilder, styles *StyleRegistry, contentList *[]string, contentName string, depth int) {
	addContent := func(text, styleName string) {
		styles.EnsureStyle(styleName) // Register style if not already present
		offset := len(*contentList)
		*contentList = append(*contentList, text)
		sb.AddContent(SymText, contentName, offset, styleName)
	}

	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil {
			styleName := item.Paragraph.Style
			if styleName == "" {
				styleName = "paragraph"
			}
			text := paragraphToText(item.Paragraph)
			if text != "" {
				addContent(text, styleName)
			}
		}

	case fb2.FlowSubtitle:
		if item.Subtitle != nil {
			text := paragraphToText(item.Subtitle)
			if text != "" {
				addContent(text, "subtitle")
			}
		}

	case fb2.FlowEmptyLine:
		addContent("\n", "empty-line")

	case fb2.FlowPoem:
		if item.Poem != nil {
			processPoemToStoryline(item.Poem, sb, styles, contentList, contentName)
		}

	case fb2.FlowCite:
		if item.Cite != nil {
			processCiteToStoryline(item.Cite, sb, styles, contentList, contentName)
		}

	case fb2.FlowTable:
		if item.Table != nil {
			text := tableToText(item.Table)
			if text != "" {
				addContent(text, "table")
			}
		}

	case fb2.FlowImage:
		// Images need special handling - TODO
		// For now skip

	case fb2.FlowSection:
		// Nested sections - process recursively
		if item.Section != nil {
			if item.Section.Title != nil {
				styleName := fmt.Sprintf("h%d", min(depth+1, 6))
				for _, titleItem := range item.Section.Title.Items {
					if titleItem.Paragraph != nil {
						text := paragraphToText(titleItem.Paragraph)
						if text != "" {
							addContent(text, styleName)
						}
					}
				}
			}
			for _, subItem := range item.Section.Content {
				processFlowItemToStoryline(&subItem, sb, styles, contentList, contentName, depth+1)
			}
		}
	}
}

// processPoemToStoryline processes poem content into storyline.
func processPoemToStoryline(poem *fb2.Poem, sb *StorylineBuilder, styles *StyleRegistry, contentList *[]string, contentName string) {
	addContent := func(text, styleName string) {
		styles.EnsureStyle(styleName) // Register style if not already present
		offset := len(*contentList)
		*contentList = append(*contentList, text)
		sb.AddContent(SymText, contentName, offset, styleName)
	}

	if poem.Title != nil {
		for _, item := range poem.Title.Items {
			if item.Paragraph != nil {
				text := paragraphToText(item.Paragraph)
				if text != "" {
					addContent(text, "poem-title")
				}
			}
		}
	}

	for _, stanza := range poem.Stanzas {
		if stanza.Title != nil {
			for _, item := range stanza.Title.Items {
				if item.Paragraph != nil {
					text := paragraphToText(item.Paragraph)
					if text != "" {
						addContent(text, "poem-title")
					}
				}
			}
		}
		for i := range stanza.Verses {
			text := paragraphToText(&stanza.Verses[i])
			if text != "" {
				addContent(text, "verse")
			}
		}
	}

	for i := range poem.TextAuthors {
		text := paragraphToText(&poem.TextAuthors[i])
		if text != "" {
			addContent(text, "text-author")
		}
	}
}

// processCiteToStoryline processes cite content into storyline.
func processCiteToStoryline(cite *fb2.Cite, sb *StorylineBuilder, styles *StyleRegistry, contentList *[]string, contentName string) {
	addContent := func(text, styleName string) {
		styles.EnsureStyle(styleName) // Register style if not already present
		offset := len(*contentList)
		*contentList = append(*contentList, text)
		sb.AddContent(SymText, contentName, offset, styleName)
	}

	for _, item := range cite.Items {
		if item.Kind == fb2.FlowParagraph && item.Paragraph != nil {
			text := paragraphToText(item.Paragraph)
			if text != "" {
				addContent(text, "cite")
			}
		}
	}

	for i := range cite.TextAuthors {
		text := paragraphToText(&cite.TextAuthors[i])
		if text != "" {
			addContent(text, "text-author")
		}
	}
}

// paragraphToText extracts plain text from a paragraph.
func paragraphToText(para *fb2.Paragraph) string {
	var buf strings.Builder
	for _, seg := range para.Text {
		extractText(&seg, &buf)
	}
	return buf.String()
}

// extractText recursively extracts text from inline segments.
func extractText(seg *fb2.InlineSegment, buf *strings.Builder) {
	buf.WriteString(seg.Text)
	for _, child := range seg.Children {
		extractText(&child, buf)
	}
}

// tableToText extracts text representation from a table.
func tableToText(table *fb2.Table) string {
	var buf strings.Builder
	for _, row := range table.Rows {
		for i, cell := range row.Cells {
			if i > 0 {
				buf.WriteString("\t")
			}
			buf.WriteString(cell.AsPlainText())
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

// buildContentFragmentByName creates a content ($145) fragment with string name.
func buildContentFragmentByName(name string, contentList []string) *Fragment {
	// Use string-keyed map for content with local symbol names
	// The "name" field value should be a symbol, not a string
	content := map[string]any{
		"$146": anySlice(contentList), // content_list
		"name": SymbolByName(name),    // name as symbol value
	}

	return &Fragment{
		FType:   SymContent,
		FIDName: name,
		Value:   content,
	}
}

// anySlice converts []string to []any
func anySlice(s []string) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}
