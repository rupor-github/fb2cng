package kfx

import (
	"fmt"
	"maps"
	"strings"

	"fbc/common"
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

// BuildStoryline creates a $259 storyline fragment.
// Based on reference KFX, storyline has:
// - Named FID (like "l1", "l2", etc.) - simple decimal format for readability
// - $176 (story_name) as symbol reference
// - $146 (content_list) with content entries
//
// Naming pattern: "l{N}" where N is sequential (e.g., l1, l2, l3).
// Uses simple format instead of base36 for better human readability during debugging.
func BuildStoryline(storyName string, contentEntries []any) *Fragment {
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

// BuildSection creates a $260 section fragment.
// Based on reference KFX, section has:
// - Named FID (like "c0", "c1", etc.) - simple decimal format for readability
// - $174 (section_name) as symbol reference
// - $141 (page_templates) with layout entries for storylines
//
// Naming pattern: "c{N}" where N is sequential starting from 0 (e.g., c0, c1, c2).
// Uses simple format instead of base36 for better human readability during debugging.
func BuildSection(sectionName string, pageTemplates []any) *Fragment {
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
// Based on KPV reference: {$155: eid, $159: $269, $176: storyline_name}
// The page template simply references the storyline by name and uses text type.
func NewPageTemplateEntry(eid int, storylineName string) StructValue {
	return NewStruct().
		SetInt(SymUniqueID, int64(eid)).               // $155 = id
		SetSymbol(SymType, SymText).                   // $159 = type = $269 (text)
		Set(SymStoryName, SymbolByName(storylineName)) // $176 = story_name ref
}

// ContentRef represents a reference to content within a storyline.
type ContentRef struct {
	EID           int             // Element ID ($155)
	Type          KFXSymbol       // Content type symbol ($269=text, $270=container, $271=image, etc.)
	ContentName   string          // Name of the content fragment
	ContentOffset int             // Offset within content fragment ($403)
	ResourceName  string          // For images: external_resource fragment id/name ($175)
	AltText       string          // For images: alt text ($584)
	Style         string          // Optional style name
	StyleEvents   []StyleEventRef // Optional inline style events ($142)
	Children      []any           // Optional nested content for containers
	HeadingLevel  int             // For headings: 1-6 for h1-h6 ($790), 0 means not a heading
}

// StyleEventRef represents a style event for inline formatting ($142).
type StyleEventRef struct {
	Offset         int    // $143 - start offset
	Length         int    // $144 - length
	Style          string // $157 - style name
	LinkTo         string // $179 - link target (optional)
	IsFootnoteLink bool   // If true, adds $616: $617 (yj.display: yj.note)
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

	// Heading level for h1-h6 elements
	if ref.HeadingLevel > 0 {
		entry.SetInt(SymYjHeadingLevel, int64(ref.HeadingLevel)) // $790 = yj.semantics.heading_level
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
			if se.IsFootnoteLink {
				ev.SetSymbol(SymYjDisplay, SymYjNote) // $616 = $617 (yj.display = yj.note)
			}
			events = append(events, ev)
		}
		entry.SetList(SymStyleEvents, events) // $142 = style_events
	}

	if ref.Type == SymImage {
		entry.Set(SymResourceName, SymbolByName(ref.ResourceName)) // $175 = resource_name (symbol reference)
		entry.SetString(SymAltText, ref.AltText)                   // $584 = alt_text
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
func (sb *StorylineBuilder) AddContent(contentType KFXSymbol, contentName string, contentOffset int, style string) int {
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

// AddContentAndEvents adds content with style events.
func (sb *StorylineBuilder) AddContentAndEvents(contentType KFXSymbol, contentName string, contentOffset int, style string, events []StyleEventRef) int {
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

// AddContentWithHeading adds content with style events and heading level.
func (sb *StorylineBuilder) AddContentWithHeading(contentType KFXSymbol, contentName string, contentOffset int, style string, events []StyleEventRef, headingLevel int) int {
	eid := sb.eidCounter
	sb.eidCounter++

	sb.contentEntries = append(sb.contentEntries, ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		Style:         style,
		StyleEvents:   events,
		HeadingLevel:  headingLevel,
	})

	return eid
}

func (sb *StorylineBuilder) AddImage(resourceName, style, altText string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	sb.contentEntries = append(sb.contentEntries, ContentRef{
		EID:          eid,
		Type:         SymImage,
		ResourceName: resourceName,
		Style:        style,
		AltText:      altText,
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
func (sb *StorylineBuilder) Build() (*Fragment, *Fragment) {
	// Build content entries for storyline
	entries := make([]any, 0, len(sb.contentEntries))
	for _, ref := range sb.contentEntries {
		entries = append(entries, NewContentEntry(ref))
	}

	// Create storyline fragment
	storylineFrag := BuildStoryline(sb.name, entries)

	// Create page template entry for section - uses dedicated EID
	pageTemplates := []any{
		NewPageTemplateEntry(sb.pageTemplateEID, sb.name),
	}

	// Create section fragment
	sectionFrag := BuildSection(sb.sectionName, pageTemplates)

	return storylineFrag, sectionFrag
}

// generateStoryline creates storyline and section fragments from an FB2 book.
// It uses the provided StyleRegistry to reference styles by name.
// Returns fragments, next EID, section names for document_data, TOC entries, per-section EID sets,
// and mapping of original FB2 IDs to EIDs (for $266 anchors).
func generateStoryline(book *fb2.FictionBook, styles *StyleRegistry,
	imageResources imageResourceInfoByID, startEID int, footnotesIndex fb2.FootnoteRefs,
) (*FragmentList, int, sectionNameList, []*TOCEntry, sectionEIDsBySectionName, eidByFB2ID, error) {
	fragments := NewFragmentList()
	eidCounter := startEID
	sectionNames := make(sectionNameList, 0)
	tocEntries := make([]*TOCEntry, 0)
	sectionEIDs := make(sectionEIDsBySectionName)
	idToEID := make(eidByFB2ID)

	// All content fragments will be collected here
	allContentFragments := make(map[string][]string)
	contentCount := 0

	// Default screen width for image style calculations
	defaultWidth := 600

	sectionCount := 0

	coverAdded := false

	// Collect footnote bodies for processing at the end
	var footnoteBodies []*fb2.Body

	// Process each body
	for i := range book.Bodies {
		body := &book.Bodies[i]

		// Collect footnote bodies for later processing (at the end, like EPUB does)
		if body.Footnotes() {
			footnoteBodies = append(footnoteBodies, body)
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
				cover := &book.Description.TitleInfo.Coverpage[0]
				coverID := strings.TrimPrefix(cover.Href, "#")
				if imgInfo, ok := imageResources[coverID]; ok {
					// Cover uses full width style
					resolved := styles.ResolveImageStyle(imgInfo.Width, defaultWidth)
					sb.AddImage(imgInfo.ResourceName, resolved, cover.Alt)
					coverAdded = true
				}
			}

			if err := processBodyIntroContent(book, body, sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex); err != nil {
				return nil, 0, nil, nil, nil, nil, err
			}

			// Collect content fragments
			maps.Copy(allContentFragments, ca.Finish())

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
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, err
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

			if err := processStorylineContent(book, section, sb, styles, imageResources, ca, 1, &nestedTOCEntries, idToEID, defaultWidth, footnotesIndex); err != nil {
				return nil, 0, nil, nil, nil, nil, err
			}

			// Collect content fragments
			maps.Copy(allContentFragments, ca.Finish())

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
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, err
			}
		}
	}

	// Process footnote bodies at the end - each footnote section becomes part of a single storyline
	// This ensures footnote IDs (n_1, n_2, etc.) are registered in idToEID for anchor generation
	if len(footnoteBodies) > 0 {
		sectionCount++
		storyName := fmt.Sprintf("l%d", sectionCount)
		sectionName := fmt.Sprintf("c%d", sectionCount-1)
		sectionNames = append(sectionNames, sectionName)

		sb := NewStorylineBuilder(storyName, sectionName, eidCounter)
		contentCount++
		ca := NewContentAccumulator(contentCount)

		// Process all footnote bodies into a single storyline
		for _, body := range footnoteBodies {
			// Process body title if present
			if body.Title != nil {
				for _, item := range body.Title.Items {
					if item.Paragraph != nil {
						addParagraphWithImages(item.Paragraph, "footnote-title", sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
					}
				}
			}

			// Process each section in the footnote body
			for j := range body.Sections {
				section := &body.Sections[j]
				// Register the section ID for anchor generation
				if section.ID != "" {
					if _, exists := idToEID[section.ID]; !exists {
						idToEID[section.ID] = sb.NextEID()
					}
				}

				// Process section title
				if section.Title != nil {
					for _, item := range section.Title.Items {
						if item.Paragraph != nil {
							addParagraphWithImages(item.Paragraph, "footnote-title", sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
						}
					}
				}

				// Process section content (paragraphs, poems, etc.)
				for k := range section.Content {
					processFlowItem(&section.Content[k], "footnote", sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
				}
			}
		}

		maps.Copy(allContentFragments, ca.Finish())
		sectionEIDs[sectionName] = sb.AllEIDs()

		// Create TOC entry for footnotes (not included in main TOC)
		tocEntry := &TOCEntry{
			ID:           sectionName,
			Title:        "Notes",
			SectionName:  sectionName,
			StoryName:    storyName,
			FirstEID:     sb.FirstEID(),
			IncludeInTOC: false, // Don't include footnotes in TOC
		}
		tocEntries = append(tocEntries, tocEntry)

		eidCounter = sb.NextEID()
		storylineFrag, sectionFrag := sb.Build()

		if err := fragments.Add(storylineFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, err
		}
		if err := fragments.Add(sectionFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, err
		}
	}

	// Create content fragments from accumulated content
	for name, contentList := range allContentFragments {
		contentFrag := buildContentFragmentByName(name, contentList)
		if err := fragments.Add(contentFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, err
		}
	}

	return fragments, eidCounter, sectionNames, tocEntries, sectionEIDs, idToEID, nil
}

// processBodyIntroContent processes body intro content using ContentAccumulator.
func addVignetteImage(book *fb2.FictionBook, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, pos common.VignettePos, screenWidth int) {
	if book == nil || !book.IsVignetteEnabled(pos) {
		return
	}
	vigID := book.VignetteIDs[pos]
	if vigID == "" {
		return
	}
	imgInfo, ok := imageResources[vigID]
	if !ok {
		return
	}
	resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
	sb.AddImage(imgInfo.ResourceName, resolved, "") // Vignettes are decorative, no alt text
}

func processBodyIntroContent(book *fb2.FictionBook, body *fb2.Body, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) error {
	if body.Image != nil {
		imgID := strings.TrimPrefix(body.Image.Href, "#")
		if imgInfo, ok := imageResources[imgID]; ok {
			resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
			eid := sb.AddImage(imgInfo.ResourceName, resolved, body.Image.Alt)
			if body.Image.ID != "" {
				if _, exists := idToEID[body.Image.ID]; !exists {
					idToEID[body.Image.ID] = eid
				}
			}
		}
	}

	// Process body title
	if body.Title != nil {
		if body.Main() {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosBookTitleTop, screenWidth)
		}
		for _, item := range body.Title.Items {
			if item.Paragraph != nil {
				addParagraphWithImages(item.Paragraph, "body-title", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
			}
		}
		if body.Main() {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosBookTitleBottom, screenWidth)
		}
	}

	// Process body epigraphs
	for _, epigraph := range body.Epigraphs {
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = sb.NextEID()
			}
		}
		for i := range epigraph.Flow.Items {
			processFlowItem(&epigraph.Flow.Items[i], "epigraph", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithImages(&epigraph.TextAuthors[i], "p text-author", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	return nil
}

// processStorylineContent processes FB2 section content using ContentAccumulator.
func processStorylineContent(book *fb2.FictionBook, section *fb2.Section, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, depth int, nestedTOC *[]*TOCEntry, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) error {
	if section != nil && section.ID != "" {
		if _, exists := idToEID[section.ID]; !exists {
			idToEID[section.ID] = sb.NextEID()
		}
	}

	if section.Image != nil {
		imgID := strings.TrimPrefix(section.Image.Href, "#")
		if imgInfo, ok := imageResources[imgID]; ok {
			resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
			eid := sb.AddImage(imgInfo.ResourceName, resolved, section.Image.Alt)
			if section.Image.ID != "" {
				if _, exists := idToEID[section.Image.ID]; !exists {
					idToEID[section.Image.ID] = eid
				}
			}
		}
	}

	// Process title
	if section.Title != nil {
		if depth == 1 {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosChapterTitleTop, screenWidth)
		} else {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosSectionTitleTop, screenWidth)
		}

		styleName := fmt.Sprintf("h%d", min(depth, 6))
		for _, item := range section.Title.Items {
			if item.Paragraph != nil {
				addParagraphWithImages(item.Paragraph, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
			}
		}

		if depth == 1 {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosChapterTitleBottom, screenWidth)
		} else {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosSectionTitleBottom, screenWidth)
		}
	}

	// Process annotation
	if section.Annotation != nil {
		if section.Annotation.ID != "" {
			if _, exists := idToEID[section.Annotation.ID]; !exists {
				idToEID[section.Annotation.ID] = sb.NextEID()
			}
		}
		for i := range section.Annotation.Items {
			processFlowItem(&section.Annotation.Items[i], "annotation", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	// Process epigraphs
	for _, epigraph := range section.Epigraphs {
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = sb.NextEID()
			}
		}
		for i := range epigraph.Flow.Items {
			processFlowItem(&epigraph.Flow.Items[i], "epigraph", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithImages(&epigraph.TextAuthors[i], "p text-author", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	// Process content items
	for i := range section.Content {
		item := &section.Content[i]
		if item.Kind == fb2.FlowSection && item.Section != nil {
			// Nested section - track for TOC hierarchy
			nestedSection := item.Section
			titleText := nestedSection.AsTitleText("")

			// Track the EID where this nested section starts
			firstEID := sb.NextEID()

			// Process nested section content recursively
			var childTOC []*TOCEntry
			if err := processStorylineContent(book, nestedSection, sb, styles, imageResources, ca, depth+1, &childTOC, idToEID, screenWidth, footnotesIndex); err != nil {
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
			processFlowItem(item, "section", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	if depth == 1 {
		addVignetteImage(book, sb, styles, imageResources, common.VignettePosChapterEnd, screenWidth)
	} else {
		addVignetteImage(book, sb, styles, imageResources, common.VignettePosSectionEnd, screenWidth)
	}

	return nil
}

// processFlowItem processes a flow item using ContentAccumulator.
// context parameter specifies the parent context (e.g., "section", "cite", "annotation", "epigraph")
// and is used for context-specific subtitle styles like EPUB does.
func processFlowItem(item *fb2.FlowItem, context string, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	addContent := func(text, styleName string) int {
		resolved := styles.ResolveStyle(styleName)
		contentName, offset := ca.Add(text)
		return sb.AddContent(SymText, contentName, offset, resolved)
	}

	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil {
			// Always start with base "p" style, add context style for styled contexts,
			// then add custom class if present. This mimics CSS cascade.
			styleName := "p"
			// Add context style for contexts that have specific paragraph styling
			// (epigraph, annotation, cite all have font/margin styles that apply to contained paragraphs)
			switch context {
			case "epigraph", "annotation", "cite":
				styleName = styleName + " " + context
			}
			if item.Paragraph.Style != "" {
				styleName = styleName + " " + item.Paragraph.Style
			}
			addParagraphWithImages(item.Paragraph, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowSubtitle:
		if item.Subtitle != nil {
			// Use context-specific subtitle style like EPUB (e.g., "section-subtitle", "cite-subtitle")
			// Start with base "p" to get text-align, line-height etc, then add context-subtitle
			styleName := "p " + context + "-subtitle"
			if item.Subtitle.Style != "" {
				styleName = styleName + " " + item.Subtitle.Style
			}
			addParagraphWithImages(item.Subtitle, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowEmptyLine:
		addContent("\n", "emptyline") // Matches CSS ".emptyline" class

	case fb2.FlowPoem:
		if item.Poem != nil {
			if item.Poem.ID != "" {
				if _, exists := idToEID[item.Poem.ID]; !exists {
					idToEID[item.Poem.ID] = sb.NextEID()
				}
			}
			processPoem(item.Poem, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowCite:
		if item.Cite != nil {
			if item.Cite.ID != "" {
				if _, exists := idToEID[item.Cite.ID]; !exists {
					idToEID[item.Cite.ID] = sb.NextEID()
				}
			}
			processCite(item.Cite, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowTable:
		if item.Table != nil {
			if item.Table.ID != "" {
				if _, exists := idToEID[item.Table.ID]; !exists {
					idToEID[item.Table.ID] = sb.NextEID()
				}
			}
			text := tableToText(item.Table)
			if text != "" {
				eid := addContent(text, "table")
				if item.Table.ID != "" {
					if _, exists := idToEID[item.Table.ID]; !exists {
						idToEID[item.Table.ID] = eid
					}
				}
			}
		}

	case fb2.FlowImage:
		if item.Image == nil {
			return
		}
		imgID := strings.TrimPrefix(item.Image.Href, "#")
		imgInfo, ok := imageResources[imgID]
		if !ok {
			return
		}
		resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
		eid := sb.AddImage(imgInfo.ResourceName, resolved, item.Image.Alt)
		if item.Image.ID != "" {
			if _, exists := idToEID[item.Image.ID]; !exists {
				idToEID[item.Image.ID] = eid
			}
		}

	case fb2.FlowSection:
		// Nested sections handled in processStorylineContent
	}
}

// processPoem processes poem content using ContentAccumulator.
// Matches EPUB's appendPoemElement handling: title, epigraphs, subtitles, stanzas, text-authors, date.
func processPoem(poem *fb2.Poem, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	// Process poem title
	if poem.Title != nil {
		for _, item := range poem.Title.Items {
			if item.Paragraph != nil {
				addParagraphWithImages(item.Paragraph, "p poem-title", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
			}
		}
	}

	// Process poem epigraphs
	for _, epigraph := range poem.Epigraphs {
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = sb.NextEID()
			}
		}
		for i := range epigraph.Flow.Items {
			processFlowItem(&epigraph.Flow.Items[i], "epigraph", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithImages(&epigraph.TextAuthors[i], "p text-author", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	// Process poem subtitles (matches EPUB's poem.Subtitles handling)
	for i := range poem.Subtitles {
		addParagraphWithImages(&poem.Subtitles[i], "p poem-subtitle", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}

	// Process stanzas
	for _, stanza := range poem.Stanzas {
		// Stanza title (matches EPUB's "stanza-title" class)
		if stanza.Title != nil {
			for _, item := range stanza.Title.Items {
				if item.Paragraph != nil {
					addParagraphWithImages(item.Paragraph, "p stanza-title", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
				}
			}
		}
		// Stanza subtitle (matches EPUB's "stanza-subtitle" class)
		if stanza.Subtitle != nil {
			addParagraphWithImages(stanza.Subtitle, "p stanza-subtitle", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		// Verses
		for i := range stanza.Verses {
			addParagraphWithImages(&stanza.Verses[i], "p verse", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	// Process text authors
	for i := range poem.TextAuthors {
		addParagraphWithImages(&poem.TextAuthors[i], "p text-author", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}

	// Process poem date (matches EPUB's ".date" class)
	if poem.Date != nil {
		var dateText string
		if poem.Date.Display != "" {
			dateText = poem.Date.Display
		} else if !poem.Date.Value.IsZero() {
			dateText = poem.Date.Value.Format("2006-01-02")
		}
		if dateText != "" {
			resolved := styles.ResolveStyle("p date")
			contentName, offset := ca.Add(dateText)
			sb.AddContent(SymText, contentName, offset, resolved)
		}
	}
}

// processCite processes cite content using ContentAccumulator.
// Matches EPUB's appendCiteElement handling: processes all flow items with "cite" context,
// followed by text-authors.
func processCite(cite *fb2.Cite, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	// Process all cite flow items with "cite" context (enables cite-subtitle, etc.)
	for i := range cite.Items {
		processFlowItem(&cite.Items[i], "cite", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}

	// Process text authors
	for i := range cite.TextAuthors {
		addParagraphWithImages(&cite.TextAuthors[i], "p text-author", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}
}

func addParagraphWithImages(para *fb2.Paragraph, styleName string, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	var (
		buf    strings.Builder
		events []StyleEventRef
	)

	// Determine heading level from style name
	headingLevel := styleToHeadingLevel(styleName)

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		resolved := styles.ResolveStyle(styleName)
		contentName, offset := ca.Add(buf.String())
		var eid int
		if headingLevel > 0 {
			eid = sb.AddContentWithHeading(SymText, contentName, offset, resolved, events, headingLevel)
		} else {
			eid = sb.AddContentAndEvents(SymText, contentName, offset, resolved, events)
		}
		if para.ID != "" {
			if _, exists := idToEID[para.ID]; !exists {
				idToEID[para.ID] = eid
			}
		}
		buf.Reset()
		events = nil
	}

	var walk func(seg *fb2.InlineSegment, inlineStyle string)
	walk = func(seg *fb2.InlineSegment, inlineStyle string) {
		// Handle inline images - flush current content and add image
		if seg.Kind == fb2.InlineImageSegment {
			flush()
			if seg.Image == nil {
				return
			}
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			imgInfo, ok := imageResources[imgID]
			if !ok {
				return
			}
			resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
			sb.AddImage(imgInfo.ResourceName, resolved, seg.Image.Alt)
			return
		}

		// Determine style for this segment based on its kind
		var segStyle string
		switch seg.Kind {
		case fb2.InlineStrong:
			segStyle = "strong"
		case fb2.InlineEmphasis:
			segStyle = "emphasis"
		case fb2.InlineStrikethrough:
			segStyle = "strikethrough"
		case fb2.InlineSub:
			segStyle = "sub"
		case fb2.InlineSup:
			segStyle = "sup"
		case fb2.InlineCode:
			segStyle = "code"
		case fb2.InlineNamedStyle:
			segStyle = seg.Style
		case fb2.InlineLink:
			// Links use link-footnote or link-internal style
			if strings.HasPrefix(seg.Href, "#") {
				segStyle = "link-footnote"
			} else {
				segStyle = "link-external"
			}
		}

		// Track position for style event
		start := buf.Len()

		// Add text content
		buf.WriteString(seg.Text)

		// Process children with current style context
		for i := range seg.Children {
			walk(&seg.Children[i], segStyle)
		}

		end := buf.Len()

		// Create style event if we have styled content
		if segStyle != "" && end > start {
			resolved := styles.ResolveStyle(segStyle)
			event := StyleEventRef{
				Offset: start,
				Length: end - start,
				Style:  resolved,
			}
			// Add link target for links and detect footnote links
			if seg.Kind == fb2.InlineLink {
				linkTo := strings.TrimPrefix(seg.Href, "#")
				if linkTo != "" && linkTo != seg.Href {
					event.LinkTo = linkTo
					// Check if this is a footnote link using FootnotesIndex (like EPUB does)
					if _, isFootnote := footnotesIndex[linkTo]; isFootnote {
						event.IsFootnoteLink = true
					}
				}
			}
			events = append(events, event)
		}
	}

	for i := range para.Text {
		walk(&para.Text[i], "")
	}
	flush()
}

// styleToHeadingLevel extracts heading level from style name.
// Returns 1-6 for heading styles, 0 for non-heading styles.
// Recognized patterns:
//   - "body-title" -> 1 (book title)
//   - "h1", "h2", ..., "h6" -> 1, 2, ..., 6 (section titles by depth)
//   - "chapter-title" -> 1
//   - "section-title" -> 2
func styleToHeadingLevel(styleName string) int {
	// Check for body-title (h1)
	if strings.Contains(styleName, "body-title") || strings.Contains(styleName, "chapter-title") {
		return 1
	}

	// Check for h1-h6 patterns
	for i := 1; i <= 6; i++ {
		pattern := fmt.Sprintf("h%d", i)
		if strings.Contains(styleName, pattern) {
			return i
		}
	}

	// section-title defaults to h2
	if strings.Contains(styleName, "section-title") {
		return 2
	}

	return 0
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

// BuildNavigation creates a $389 book_navigation fragment from TOC entries.
// This creates a hierarchical TOC structure similar to epub's NCX/nav.
// The $389 fragment value is a list of reading order navigation entries.
func BuildNavigation(tocEntries []*TOCEntry, startEID int) *Fragment {
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
