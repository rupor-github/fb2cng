package kfx

import (
	"fmt"
	"strings"

	"fbc/fb2"
)

// TOCEntry represents a table of contents entry with hierarchical structure.
// This mirrors the chapterData structure in epub for consistent TOC generation.
type TOCEntry struct {
	ID           string      // Unique ID for this entry
	Title        string      // Display title for TOC
	SectionName  string      // KFX section name (e.g., "c0")
	StoryName    string      // KFX storyline name (e.g., "l1")
	FirstEID     int         // First content EID for navigation target
	IncludeInTOC bool        // Whether to include in TOC
	Children     []*TOCEntry // Nested TOC entries for subsections
}

// MaxContentFragmentSize is the maximum size in bytes for a content fragment's content_list.
// KFXInput validates that content fragments don't exceed 8192 bytes.
// This is separate from the container's ChunkSize ($412) which is used for streaming/compression.
const MaxContentFragmentSize = 8192

// ContentAccumulator manages content fragments with automatic chunking.
// Each paragraph/text entry is a separate item in content_list.
// When accumulated size exceeds MaxContentFragmentSize, a new fragment is created.
// ContentAccumulator accumulates paragraph content into named content fragments,
// automatically splitting into chunks when size limits are exceeded.
//
// Fragment naming pattern: "content_{N}" where N is sequential (e.g., content_1, content_2).
// When chunked: "content_{N}_{M}" where M is chunk number (e.g., content_1_1, content_1_2).
// This human-readable format is used instead of base36 for better debuggability.
type ContentAccumulator struct {
	baseCounter  int                 // Base counter for naming (e.g., 1 for content_1)
	currentName  string              // Current content fragment name
	currentList  []string            // Current content list (each entry is one paragraph)
	currentSize  int                 // Current accumulated size in bytes
	fragments    map[string][]string // All completed content fragments
	chunkCounter int                 // Counter for chunk suffixes (0 = no suffix)
}

// NewContentAccumulator creates a new content accumulator.
// Fragment names follow pattern "content_{baseCounter}" for readability.
func NewContentAccumulator(baseCounter int) *ContentAccumulator {
	name := fmt.Sprintf("content_%d", baseCounter)
	return &ContentAccumulator{
		baseCounter:  baseCounter,
		currentName:  name,
		currentList:  make([]string, 0),
		currentSize:  0,
		fragments:    make(map[string][]string),
		chunkCounter: 0,
	}
}

// CurrentName returns the current content fragment name.
func (ca *ContentAccumulator) CurrentName() string {
	return ca.currentName
}

// CurrentOffset returns the current offset (index) within the current content fragment.
func (ca *ContentAccumulator) CurrentOffset() int {
	return len(ca.currentList)
}

// Add adds a paragraph/text entry to the accumulator.
// Each call adds one entry to content_list. Creates new chunk if size limit exceeded.
// Returns the content name and offset for the added text.
func (ca *ContentAccumulator) Add(text string) (name string, offset int) {
	textSize := len(text)

	// Check if we need to start a new chunk
	// Start new chunk if current is non-empty and adding this would exceed limit
	if ca.currentSize > 0 && ca.currentSize+textSize > MaxContentFragmentSize {
		ca.finishCurrentChunk()
	}

	name = ca.currentName
	offset = len(ca.currentList)
	ca.currentList = append(ca.currentList, text)
	ca.currentSize += textSize

	return name, offset
}

// finishCurrentChunk saves the current chunk and starts a new one.
func (ca *ContentAccumulator) finishCurrentChunk() {
	if len(ca.currentList) > 0 {
		ca.fragments[ca.currentName] = ca.currentList
	}

	ca.chunkCounter++
	ca.currentName = fmt.Sprintf("content_%d_%d", ca.baseCounter, ca.chunkCounter)
	ca.currentList = make([]string, 0)
	ca.currentSize = 0
}

// Finish completes accumulation and returns all content fragments.
func (ca *ContentAccumulator) Finish() map[string][]string {
	// Save current chunk if it has content
	if len(ca.currentList) > 0 {
		ca.fragments[ca.currentName] = ca.currentList
	}
	return ca.fragments
}

// BuildStorylineFragment creates a $259 storyline fragment.
// Based on reference KFX, storyline has:
// - Named FID (like "l1", "l2", etc.) - simple decimal format for readability
// - $176 (story_name) as symbol reference
// - $146 (content_list) with content entries
//
// Naming pattern: "l{N}" where N is sequential (e.g., l1, l2, l3).
// Uses simple format instead of base36 for better human readability during debugging.
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
// - Named FID (like "c0", "c1", etc.) - simple decimal format for readability
// - $174 (section_name) as symbol reference
// - $141 (page_templates) with layout entries for storylines
//
// Naming pattern: "c{N}" where N is sequential starting from 0 (e.g., c0, c1, c2).
// Uses simple format instead of base36 for better human readability during debugging.
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
		SetInt(SymUniqueID, int64(eid)).                // $155 = id
		Set(SymStoryName, SymbolByName(storylineName)). // $176 = story_name ref
		SetInt(SymWidth, int64(width)).                 // $66 = width
		SetInt(SymHeight, int64(height)).               // $67 = height
		SetSymbol(SymLayout, SymScaleFit).              // $156 = layout = $326 (scale_fit)
		SetSymbol(SymFloat, SymCenter).                 // $140 = float = $320 (center)
		SetSymbol(SymType, SymContainer)                // $159 = type = $270 (container)
}

// ContentRef represents a reference to content within a storyline.
type ContentRef struct {
	EID           int             // Element ID ($155)
	Type          int             // Content type symbol ($269=text, $270=container, $271=image, etc.)
	ContentName   string          // Name of the content fragment
	ContentOffset int             // Offset within content fragment ($403)
	ResourceName  string          // For images: external_resource fragment id/name ($175)
	Style         string          // Optional style name
	StyleEvents   []StyleEventRef // Optional inline style events ($142)
	Children      []any           // Optional nested content for containers
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
		SetInt(SymUniqueID, int64(ref.EID)). // $155 = id
		SetSymbol(SymType, ref.Type)         // $159 = type

	if ref.Style != "" {
		entry.Set(SymStyle, SymbolByName(ref.Style)) // $157 = style as symbol
	}

	// Style events for inline formatting
	if len(ref.StyleEvents) > 0 {
		events := make([]any, 0, len(ref.StyleEvents))
		for _, se := range ref.StyleEvents {
			ev := NewStruct().
				SetInt(SymOffset, int64(se.Offset)). // $143 = offset
				SetInt(SymLength, int64(se.Length))  // $144 = length
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

	if ref.Type == SymImage {
		entry.Set(SymResourceName, SymbolByName(ref.ResourceName)) // $175 = resource_name (symbol reference)
	} else {
		// Content reference - nested struct with name and offset
		contentRef := map[string]any{
			"name": SymbolByName(ref.ContentName),
			"$403": ref.ContentOffset,
		}
		entry.Set(SymContent, contentRef) // $145 = content
	}

	// Nested children for containers
	if len(ref.Children) > 0 {
		entry.SetList(SymContentList, ref.Children) // $146 = content_list
	}

	return entry
}

// StorylineBuilder helps build storyline content incrementally.
type StorylineBuilder struct {
	name            string // Storyline name (e.g., "l1")
	sectionName     string // Associated section name (e.g., "c0")
	contentEntries  []ContentRef
	eidCounter      int
	pageTemplateEID int // Separate EID for page template container
}

// AllEIDs returns all EIDs used by this section (page template + content entries).
func (sb *StorylineBuilder) AllEIDs() []int {
	eids := make([]int, 0, len(sb.contentEntries)+1)
	eids = append(eids, sb.pageTemplateEID)
	for _, ref := range sb.contentEntries {
		eids = append(eids, ref.EID)
	}
	return eids
}

// NewStorylineBuilder creates a new storyline builder.
// Allocates the first EID for the page template container.
func NewStorylineBuilder(storyName, sectionName string, startEID int) *StorylineBuilder {
	return &StorylineBuilder{
		name:            storyName,
		sectionName:     sectionName,
		pageTemplateEID: startEID,     // First EID goes to page template
		eidCounter:      startEID + 1, // Content EIDs start after page template
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

func (sb *StorylineBuilder) AddImage(resourceName string, style string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	sb.contentEntries = append(sb.contentEntries, ContentRef{
		EID:          eid,
		Type:         SymImage,
		ResourceName: resourceName,
		Style:        style,
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
// Returns fragments, next EID, section names for document_data, TOC entries, and per-section EID sets.
func GenerateStorylineFromBook(book *fb2.FictionBook, styles *StyleRegistry, imageResourceNames map[string]string, startEID int) (*FragmentList, int, []string, []*TOCEntry, map[string][]int, error) {
	fragments := NewFragmentList()
	eidCounter := startEID
	sectionNames := make([]string, 0)
	tocEntries := make([]*TOCEntry, 0)
	sectionEIDs := make(map[string][]int)

	// All content fragments will be collected here
	allContentFragments := make(map[string][]string)
	contentCount := 0

	// Default dimensions (will be adjusted by Kindle)
	defaultWidth := 600
	defaultHeight := 800

	sectionCount := 0

	coverAdded := false

	// Process each body
	for i := range book.Bodies {
		body := &book.Bodies[i]
		if body.Footnotes() {
			continue
		}

		// Process body intro content (title, epigraphs, image) as separate storyline
		// This mirrors epub's bodyIntroToXHTML which creates a separate chapter for body intro
		if body.Title != nil {
			sectionCount++
			storyName := fmt.Sprintf("l%d", sectionCount)
			sectionName := fmt.Sprintf("c%d", sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline builder for body intro
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter)

			// Process body intro content with accumulator
			contentCount++
			ca := NewContentAccumulator(contentCount)

			// Add cover image once at the very beginning (references external_resource)
			if !coverAdded && len(book.Description.TitleInfo.Coverpage) > 0 {
				coverID := strings.TrimPrefix(book.Description.TitleInfo.Coverpage[0].Href, "#")
				if resName, ok := imageResourceNames[coverID]; ok {
					styles.EnsureStyle("image")
					sb.AddImage(resName, "image")
					coverAdded = true
				}
			}

			if err := processBodyIntroContentWithAccum(body, sb, styles, imageResourceNames, ca); err != nil {
				return nil, 0, nil, nil, nil, err
			}

			// Collect content fragments
			for name, list := range ca.Finish() {
				allContentFragments[name] = list
			}

			sectionEIDs[sectionName] = sb.AllEIDs()

			// Create TOC entry for body intro
			title := body.Title.AsTOCText("Untitled")
			tocEntry := &TOCEntry{
				ID:           sectionName,
				Title:        title,
				SectionName:  sectionName,
				StoryName:    storyName,
				FirstEID:     sb.FirstEID(),
				IncludeInTOC: true,
			}
			tocEntries = append(tocEntries, tocEntry)

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build(defaultWidth, defaultHeight)

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, err
			}
		}

		// Process top-level sections as chapters (like epub does)
		for j := range body.Sections {
			section := &body.Sections[j]
			sectionCount++

			// Generate names: "l1", "l2", ... for storylines; "c0", "c1", ... for sections
			storyName := fmt.Sprintf("l%d", sectionCount)
			sectionName := fmt.Sprintf("c%d", sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline builder
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter)

			// Process section content with accumulator
			contentCount++
			ca := NewContentAccumulator(contentCount)

			// Track nested section info for TOC hierarchy
			var nestedTOCEntries []*TOCEntry

			if err := processStorylineContentWithAccum(section, sb, styles, imageResourceNames, ca, 1, &nestedTOCEntries); err != nil {
				return nil, 0, nil, nil, nil, err
			}

			// Collect content fragments
			for name, list := range ca.Finish() {
				allContentFragments[name] = list
			}

			sectionEIDs[sectionName] = sb.AllEIDs()

			// Create TOC entry for this section
			title := section.AsTitleText("")
			tocEntry := &TOCEntry{
				ID:           section.ID,
				Title:        title,
				SectionName:  sectionName,
				StoryName:    storyName,
				FirstEID:     sb.FirstEID(),
				IncludeInTOC: title != "",
				Children:     nestedTOCEntries,
			}
			tocEntries = append(tocEntries, tocEntry)

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build(defaultWidth, defaultHeight)

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, err
			}
		}
	}

	// Create content fragments from accumulated content
	for name, contentList := range allContentFragments {
		contentFrag := buildContentFragmentByName(name, contentList)
		if err := fragments.Add(contentFrag); err != nil {
			return nil, 0, nil, nil, nil, err
		}
	}

	return fragments, eidCounter, sectionNames, tocEntries, sectionEIDs, nil
}

// processBodyIntroContentWithAccum processes body intro content using ContentAccumulator.
func processBodyIntroContentWithAccum(body *fb2.Body, sb *StorylineBuilder, styles *StyleRegistry, imageResourceNames map[string]string, ca *ContentAccumulator) error {
	if body.Image != nil {
		imgID := strings.TrimPrefix(body.Image.Href, "#")
		if resName, ok := imageResourceNames[imgID]; ok {
			styles.EnsureStyle("image")
			sb.AddImage(resName, "image")
		}
	}

	// Process body title
	if body.Title != nil {
		for _, item := range body.Title.Items {
			if item.Paragraph != nil {
				addParagraphWithInlineImages(item.Paragraph, "body-title", sb, styles, imageResourceNames, ca)
			}
		}
	}

	// Process body epigraphs
	for _, epigraph := range body.Epigraphs {
		for _, item := range epigraph.Flow.Items {
			processFlowItemWithAccum(&item, sb, styles, imageResourceNames, ca, 1)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithInlineImages(&epigraph.TextAuthors[i], "text-author", sb, styles, imageResourceNames, ca)
		}
	}

	return nil
}

// processStorylineContentWithAccum processes FB2 section content using ContentAccumulator.
func processStorylineContentWithAccum(section *fb2.Section, sb *StorylineBuilder, styles *StyleRegistry, imageResourceNames map[string]string, ca *ContentAccumulator, depth int, nestedTOC *[]*TOCEntry) error {
	if section.Image != nil {
		imgID := strings.TrimPrefix(section.Image.Href, "#")
		if resName, ok := imageResourceNames[imgID]; ok {
			styles.EnsureStyle("image")
			sb.AddImage(resName, "image")
		}
	}

	// Process title
	if section.Title != nil {
		styleName := fmt.Sprintf("h%d", min(depth, 6))
		for _, item := range section.Title.Items {
			if item.Paragraph != nil {
				addParagraphWithInlineImages(item.Paragraph, styleName, sb, styles, imageResourceNames, ca)
			}
		}
	}

	// Process annotation
	if section.Annotation != nil {
		for _, item := range section.Annotation.Items {
			processFlowItemWithAccum(&item, sb, styles, imageResourceNames, ca, depth)
		}
	}

	// Process epigraphs
	for _, epigraph := range section.Epigraphs {
		for _, item := range epigraph.Flow.Items {
			processFlowItemWithAccum(&item, sb, styles, imageResourceNames, ca, depth)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithInlineImages(&epigraph.TextAuthors[i], "text-author", sb, styles, imageResourceNames, ca)
		}
	}

	// Process content items
	for _, item := range section.Content {
		if item.Kind == fb2.FlowSection && item.Section != nil {
			// Nested section - track for TOC hierarchy
			nestedSection := item.Section
			titleText := nestedSection.AsTitleText("")

			// Track the EID where this nested section starts
			firstEID := sb.NextEID()

			// Process nested section content recursively
			var childTOC []*TOCEntry
			if err := processStorylineContentWithAccum(nestedSection, sb, styles, imageResourceNames, ca, depth+1, &childTOC); err != nil {
				return err
			}

			// Create TOC entry for nested section
			if titleText != "" {
				tocEntry := &TOCEntry{
					ID:           nestedSection.ID,
					Title:        titleText,
					FirstEID:     firstEID,
					IncludeInTOC: true,
					Children:     childTOC,
				}
				*nestedTOC = append(*nestedTOC, tocEntry)
			} else if len(childTOC) > 0 {
				// Section without title - promote children to parent level
				*nestedTOC = append(*nestedTOC, childTOC...)
			}
		} else {
			processFlowItemWithAccum(&item, sb, styles, imageResourceNames, ca, depth)
		}
	}

	return nil
}

// processFlowItemWithAccum processes a flow item using ContentAccumulator.
func processFlowItemWithAccum(item *fb2.FlowItem, sb *StorylineBuilder, styles *StyleRegistry, imageResourceNames map[string]string, ca *ContentAccumulator, depth int) {
	addContent := func(text, styleName string) {
		styles.EnsureStyle(styleName)
		contentName, offset := ca.Add(text)
		sb.AddContent(SymText, contentName, offset, styleName)
	}

	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil {
			styleName := item.Paragraph.Style
			if styleName == "" {
				styleName = "paragraph"
			}
			addParagraphWithInlineImages(item.Paragraph, styleName, sb, styles, imageResourceNames, ca)
		}

	case fb2.FlowSubtitle:
		if item.Subtitle != nil {
			addParagraphWithInlineImages(item.Subtitle, "subtitle", sb, styles, imageResourceNames, ca)
		}

	case fb2.FlowEmptyLine:
		addContent("\n", "empty-line")

	case fb2.FlowPoem:
		if item.Poem != nil {
			processPoemWithAccum(item.Poem, sb, styles, imageResourceNames, ca)
		}

	case fb2.FlowCite:
		if item.Cite != nil {
			processCiteWithAccum(item.Cite, sb, styles, imageResourceNames, ca)
		}

	case fb2.FlowTable:
		if item.Table != nil {
			text := tableToText(item.Table)
			if text != "" {
				addContent(text, "table")
			}
		}

	case fb2.FlowImage:
		if item.Image == nil {
			return
		}
		imgID := strings.TrimPrefix(item.Image.Href, "#")
		resName, ok := imageResourceNames[imgID]
		if !ok {
			return
		}
		styles.EnsureStyle("image")
		sb.AddImage(resName, "image")

	case fb2.FlowSection:
		// Nested sections handled in processStorylineContentWithAccum
	}
}

// processPoemWithAccum processes poem content using ContentAccumulator.
func processPoemWithAccum(poem *fb2.Poem, sb *StorylineBuilder, styles *StyleRegistry, imageResourceNames map[string]string, ca *ContentAccumulator) {
	if poem.Title != nil {
		for _, item := range poem.Title.Items {
			if item.Paragraph != nil {
				addParagraphWithInlineImages(item.Paragraph, "poem-title", sb, styles, imageResourceNames, ca)
			}
		}
	}

	for _, stanza := range poem.Stanzas {
		if stanza.Title != nil {
			for _, item := range stanza.Title.Items {
				if item.Paragraph != nil {
					addParagraphWithInlineImages(item.Paragraph, "poem-title", sb, styles, imageResourceNames, ca)
				}
			}
		}
		for i := range stanza.Verses {
			addParagraphWithInlineImages(&stanza.Verses[i], "verse", sb, styles, imageResourceNames, ca)
		}
	}

	for i := range poem.TextAuthors {
		addParagraphWithInlineImages(&poem.TextAuthors[i], "text-author", sb, styles, imageResourceNames, ca)
	}
}

// processCiteWithAccum processes cite content using ContentAccumulator.
func processCiteWithAccum(cite *fb2.Cite, sb *StorylineBuilder, styles *StyleRegistry, imageResourceNames map[string]string, ca *ContentAccumulator) {
	for _, item := range cite.Items {
		if item.Kind == fb2.FlowParagraph && item.Paragraph != nil {
			addParagraphWithInlineImages(item.Paragraph, "cite", sb, styles, imageResourceNames, ca)
		}
	}

	for i := range cite.TextAuthors {
		addParagraphWithInlineImages(&cite.TextAuthors[i], "text-author", sb, styles, imageResourceNames, ca)
	}
}

func addParagraphWithInlineImages(para *fb2.Paragraph, styleName string, sb *StorylineBuilder, styles *StyleRegistry, imageResourceNames map[string]string, ca *ContentAccumulator) {
	addText := func(text string) {
		if text == "" {
			return
		}
		styles.EnsureStyle(styleName)
		contentName, offset := ca.Add(text)
		sb.AddContent(SymText, contentName, offset, styleName)
	}

	var buf strings.Builder
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		addText(buf.String())
		buf.Reset()
	}

	var walk func(seg *fb2.InlineSegment)
	walk = func(seg *fb2.InlineSegment) {
		if seg.Kind == fb2.InlineImageSegment {
			flush()
			if seg.Image == nil {
				return
			}
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			resName, ok := imageResourceNames[imgID]
			if !ok {
				return
			}
			styles.EnsureStyle("image")
			sb.AddImage(resName, "image")
			return
		}

		buf.WriteString(seg.Text)
		for _, child := range seg.Children {
			walk(&child)
		}
	}

	for _, seg := range para.Text {
		walk(&seg)
	}
	flush()
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
// The name parameter comes from ContentAccumulator and follows the pattern "content_{N}"
// or "content_{N}_{M}" for chunked content. This human-readable naming convention
// is maintained throughout the conversion for easier debugging and inspection.
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

// BuildNavigationFragment creates a $389 book_navigation fragment from TOC entries.
// This creates a hierarchical TOC structure similar to epub's NCX/nav.
// The $389 fragment value is a list of reading order navigation entries.
func BuildNavigationFragment(tocEntries []*TOCEntry, startEID int) *Fragment {
	// Build TOC entries recursively
	entries := buildNavEntries(tocEntries, startEID)

	// Create TOC navigation container
	tocContainer := NewTOCContainer(entries)

	// Create nav_containers list with just TOC for now
	// Could add landmarks, page_list later
	navContainers := []any{tocContainer}

	// Build reading order navigation entry
	// Structure: {$178: $351 (default), $392: [nav_containers]}
	readingOrderNav := NewStruct().
		SetSymbol(SymReadOrderName, SymDefault). // $178 = default reading order
		SetList(SymNavContainers, navContainers) // $392 = nav_containers

	// $389 is a list of reading order navigation entries
	bookNavList := []any{readingOrderNav}

	return &Fragment{
		FType: SymBookNavigation,
		FID:   SymBookNavigation, // Root fragment - FID == FType
		Value: bookNavList,
	}
}

// buildNavEntries recursively builds navigation unit entries from TOC entries.
func buildNavEntries(tocEntries []*TOCEntry, startEID int) []any {
	entries := make([]any, 0, len(tocEntries))

	for _, entry := range tocEntries {
		if !entry.IncludeInTOC {
			continue
		}

		// Create target position pointing to the first EID of the content
		targetPos := NewStruct().SetInt(SymUniqueID, int64(entry.FirstEID)) // $155 = id

		// Create nav unit with label and target
		navUnit := NewNavUnit(entry.Title, targetPos)

		// Add nested entries for children (hierarchical TOC)
		if len(entry.Children) > 0 {
			childEntries := buildNavEntries(entry.Children, startEID)
			if len(childEntries) > 0 {
				navUnit.SetList(SymEntries, childEntries) // $247 = nested entries
			}
		}

		entries = append(entries, navUnit)
	}

	return entries
}
