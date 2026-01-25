package kfx

import (
	"strconv"
	"strings"

	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
)

func nextContentBaseCounter(fragments *FragmentList) int {
	maxN := 0
	for _, f := range fragments.GetByType(SymContent) {
		name := f.FIDName
		if !strings.HasPrefix(name, "content_") {
			continue
		}
		rest := strings.TrimPrefix(name, "content_")
		parts := strings.SplitN(rest, "_", 2)
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}
	return maxN + 1
}

func nextSectionIndex(sectionNames sectionNameList) int {
	maxN := -1
	for _, s := range sectionNames {
		if !strings.HasPrefix(s, "c") {
			continue
		}
		rest := strings.TrimPrefix(s, "c")
		// Try base36 first (handles "cA", "cB", etc.)
		n64, err := strconv.ParseInt(rest, 36, 64)
		if err != nil {
			continue
		}
		n := int(n64)
		if n > maxN {
			maxN = n
		}
	}
	return maxN + 1
}

// tocPageEntry represents a single TOC page entry with link information.
type tocPageEntry struct {
	Title    string          // Display text
	AnchorID string          // Anchor ID to link to (FB2 section ID)
	FirstEID int             // Target EID for the anchor
	Children []*tocPageEntry // Nested entries (for hierarchy)
}

// buildTOCEntryTree returns TOC entries as a tree structure for hierarchical list generation.
func buildTOCEntryTree(entries []*TOCEntry, includeUntitled bool) []*tocPageEntry {
	var build func(es []*TOCEntry) []*tocPageEntry
	build = func(es []*TOCEntry) []*tocPageEntry {
		var out []*tocPageEntry
		for _, e := range es {
			if e == nil {
				continue
			}
			if !e.IncludeInTOC && !includeUntitled {
				continue
			}
			t := e.Title
			if t == "" {
				t = "Untitled"
			}
			entry := &tocPageEntry{
				Title:    t,
				AnchorID: e.ID,
				FirstEID: e.FirstEID,
			}
			if len(e.Children) > 0 {
				entry.Children = build(e.Children)
			}
			out = append(out, entry)
		}
		return out
	}
	return build(entries)
}

// collectTOCAnchors recursively collects anchor IDs from a TOC entry tree.
func collectTOCAnchors(entries []*tocPageEntry, idToEID eidByFB2ID) {
	for _, e := range entries {
		if e.AnchorID != "" && e.FirstEID > 0 {
			idToEID[e.AnchorID] = e.FirstEID
		}
		if len(e.Children) > 0 {
			collectTOCAnchors(e.Children, idToEID)
		}
	}
}

// tocListBuilder helps build hierarchical TOC lists with proper EID allocation.
type tocListBuilder struct {
	eidCounter   int
	styles       *StyleRegistry
	ca           *ContentAccumulator
	styleContext StyleContext // Tracks ancestor styles for proper cascade
}

// buildTOCList builds a hierarchical list structure for TOC entries.
// Returns the list StructValue and all EIDs used (for position_id_map).
func (b *tocListBuilder) buildTOCList(entries []*tocPageEntry, isNested bool) (StructValue, []int) {
	if len(entries) == 0 {
		return nil, nil
	}

	listEID := b.eidCounter
	b.eidCounter++

	var allEIDs []int
	allEIDs = append(allEIDs, listEID)

	// Push list context for nested lists
	// Use "ol" as element tag to inherit HTML ol defaults (margin-top/bottom: 1em)
	// from the stylemap. Top-level list uses "toc-list", nested use "toc-nested".
	var listContext StyleContext
	if isNested {
		listContext = b.styleContext.Push("ol", "toc-nested")
	} else {
		listContext = b.styleContext.Push("ol", "toc-list")
	}

	// Save current context and set new one for children
	savedContext := b.styleContext
	b.styleContext = listContext

	var items []any
	for _, entry := range entries {
		item, itemEIDs := b.buildTOCListItem(entry)
		items = append(items, item)
		allEIDs = append(allEIDs, itemEIDs...)
	}

	// Restore context
	b.styleContext = savedContext

	// Build list entry: {$100: $343 (numeric), $146: [...], $155: eid, $159: $276 (list)}
	list := NewStruct().
		SetInt(SymUniqueID, int64(listEID)).
		SetSymbol(SymType, SymList).                 // $159 = $276 (list)
		SetSymbol(SymListStyle, SymListStyleNumber). // $100 = $343 (numeric list style)
		SetList(SymContentList, items)               // $146 = content_list

	// Only top-level list gets the "ol" style with margins.
	// Nested lists don't have a style - they inherit from parent context.
	// This matches KP3 reference where only the outermost TOC list has margin styling.
	if !isNested {
		listStyle := listContext.Resolve("ol", "toc-list")
		if b.styles != nil {
			b.styles.ResolveStyle(listStyle, styleUsageWrapper)
		}
		if listStyle != "" {
			list.Set(SymStyle, SymbolByName(listStyle))
		}
	}

	return list, allEIDs
}

// buildTOCListItem builds a single listitem with optional nested list.
func (b *tocListBuilder) buildTOCListItem(entry *tocPageEntry) (StructValue, []int) {
	itemEID := b.eidCounter
	b.eidCounter++

	var allEIDs []int
	allEIDs = append(allEIDs, itemEID)

	// Build text content with link
	textEntry, textEID := b.buildTOCTextEntry(entry)
	allEIDs = append(allEIDs, textEID)

	var children []any
	children = append(children, textEntry)

	// If this entry has children, build nested list
	if len(entry.Children) > 0 {
		nestedList, nestedEIDs := b.buildTOCList(entry.Children, true)
		children = append(children, nestedList)
		allEIDs = append(allEIDs, nestedEIDs...)
	}

	// Build listitem entry: {$146: [...], $155: eid, $159: $277 (listitem)}
	item := NewStruct().
		SetInt(SymUniqueID, int64(itemEID)).
		SetSymbol(SymType, SymListItem).  // $159 = $277 (listitem)
		SetList(SymContentList, children) // $146 = content_list

	return item, allEIDs
}

// buildTOCTextEntry builds a text entry with link for a TOC item.
func (b *tocListBuilder) buildTOCTextEntry(entry *tocPageEntry) (StructValue, int) {
	textEID := b.eidCounter
	b.eidCounter++

	// Add text to content accumulator
	contentName, offset := b.ca.Add(entry.Title)

	// Build content reference
	contentRef := map[string]any{
		"name": SymbolByName(contentName),
		"$403": offset,
	}

	// Resolve styles using context - accumulates ancestor styles
	// Context has toc-list (and optionally toc-nested for nested items)
	// Item style adds "toc-item toc-section" to the context
	itemStyle := b.styleContext.Resolve("", "toc-item toc-section")
	if b.styles != nil {
		b.styles.ResolveStyle(itemStyle, styleUsageText)
	}
	// Link style also inherits context for proper cascade
	linkStyle := b.styleContext.Resolve("", "link-toc")
	if b.styles != nil {
		b.styles.ResolveStyle(linkStyle, styleUsageText)
	}

	// Build style event for link (covers entire text)
	textLen := len([]rune(entry.Title))
	event := NewStruct().
		SetInt(SymOffset, 0).
		SetInt(SymLength, int64(textLen))
	if linkStyle != "" {
		event.Set(SymStyle, SymbolByName(linkStyle))
	}
	if entry.AnchorID != "" {
		event.Set(SymLinkTo, SymbolByName(entry.AnchorID))
	}

	// Build text entry
	text := NewStruct().
		SetInt(SymUniqueID, int64(textEID)).
		SetSymbol(SymType, SymText).          // $159 = $269 (text)
		Set(SymContent, contentRef).          // $145 = content
		SetList(SymStyleEvents, []any{event}) // $142 = style_events
	if itemStyle != "" {
		text.Set(SymStyle, SymbolByName(itemStyle))
	}

	return text, textEID
}

// addTOCList adds the complete TOC list structure to the storyline.
func addTOCList(sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator, entries []*tocPageEntry) {
	if len(entries) == 0 {
		return
	}

	builder := &tocListBuilder{
		eidCounter:   sb.NextEID(),
		styles:       styles,
		ca:           ca,
		styleContext: NewStyleContext(styles),
	}

	list, _ := builder.buildTOCList(entries, false)

	// Update storyline's EID counter
	sb.SetNextEID(builder.eidCounter)

	// Add the list as a raw entry
	sb.AddRawEntry(list)
}

// addGeneratedSections optionally injects generated sections (annotation page and/or TOC page).
//
// Returns (in order): updated sectionNames, updated tocEntries, updated sectionEIDs, nextEID, updated landmarks, updated idToEID, error.
// It also appends the necessary fragments (content/storyline/section) into fragments.
func addGeneratedSections(c *content.Content, cfg *config.DocumentConfig,
	styles *StyleRegistry, fragments *FragmentList, sectionNames sectionNameList,
	tocEntries []*TOCEntry, sectionEIDs sectionEIDsBySectionName, nextEID int, landmarks LandmarkInfo, idToEID eidByFB2ID,
	imageResources imageResourceInfoByID, log *zap.Logger,
) (sectionNameList, []*TOCEntry, sectionEIDsBySectionName, int, LandmarkInfo, eidByFB2ID, error) {
	annotationEnabled := cfg.Annotation.Enable && c.Book.Description.TitleInfo.Annotation != nil
	tocPageEnabled := cfg.TOCPage.Placement != common.TOCPagePlacementNone

	before := make(sectionNameList, 0)
	after := make(sectionNameList, 0)

	sectionIdx := nextSectionIndex(sectionNames)
	storyIdx := len(sectionNames) + 1
	contentCounter := nextContentBaseCounter(fragments)

	if annotationEnabled {
		storyName := "l" + toBase36(storyIdx)
		sectionName := "c" + toBase36(sectionIdx)
		storyIdx++
		sectionIdx++

		sb := NewStorylineBuilder(storyName, sectionName, nextEID, styles)
		ca := NewContentAccumulator(contentCounter)
		contentCounter++

		// Add annotation title with proper heading semantics (same pattern as TOC title)
		// Note: Annotation title is NOT in a wrapper block, so it uses direct resolution.
		if annotationTitle := titleFromStrings(cfg.Annotation.Title); annotationTitle != nil {
			titleCtx := NewStyleContext(styles).PushBlock("div", "annotation-title")
			markTitleStylesUsed("", "annotation-title", styles)
			addTitleAsHeading(c, annotationTitle, titleCtx, "annotation-title", 1, sb, styles, nil, ca, nil)
		}

		// Process annotation items with full formatting support (links, emphasis, etc.)
		// Use the same processFlowItem mechanism as body content for consistent rendering.
		// Enter annotation container for margin collapsing tracking (same as regular section annotations).
		// Use FlagForceTransferMBToLastChild to always transfer container's margin-bottom to the last paragraph.
		sb.EnterContainer(ContainerAnnotation, FlagForceTransferMBToLastChild)
		annotationCtx := NewStyleContext(styles).PushBlock("div", "annotation")
		// Store container margins for post-processing
		sb.SetContainerMargins(annotationCtx.ExtractContainerMargins("div", "annotation"))
		for i := range c.Book.Description.TitleInfo.Annotation.Items {
			item := &c.Book.Description.TitleInfo.Annotation.Items[i]
			processFlowItem(c, item, annotationCtx, "annotation", sb, styles, imageResources, ca, idToEID)
		}
		sb.ExitContainer() // Exit annotation container

		for name, list := range ca.Finish() {
			if err := fragments.Add(buildContentFragmentByName(name, list)); err != nil {
				return nil, nil, nil, 0, landmarks, nil, err
			}
		}

		sectionEIDs[sectionName] = sb.AllEIDs()
		nextEID = sb.NextEID()

		storyFrag, secFrag := sb.Build()
		if err := fragments.Add(storyFrag); err != nil {
			return nil, nil, nil, 0, landmarks, nil, err
		}
		if err := fragments.Add(secFrag); err != nil {
			return nil, nil, nil, 0, landmarks, nil, err
		}

		annotationEntry := &TOCEntry{
			ID:           "annotation-page",
			Title:        cfg.Annotation.Title,
			SectionName:  sectionName,
			StoryName:    storyName,
			FirstEID:     sb.FirstEID(),
			IncludeInTOC: cfg.Annotation.InTOC,
		}
		if cfg.Annotation.InTOC {
			tocEntries = append([]*TOCEntry{annotationEntry}, tocEntries...)
		}
		before = append(before, sectionName)
	}

	var tocSectionName string
	if tocPageEnabled {
		storyName := "l" + toBase36(storyIdx)
		tocSectionName = "c" + toBase36(sectionIdx)
		storyIdx++
		sectionIdx++

		sb := NewStorylineBuilder(storyName, tocSectionName, nextEID, styles)
		ca := NewContentAccumulator(contentCounter)
		contentCounter++

		// Build TOC title using titleFromStrings + addTitleAsHeading
		// This creates a proper fb2.Title structure from book title and optional authors
		var authors string
		if cfg.TOCPage.AuthorsTemplate != "" {
			expanded, err := c.Book.ExpandTemplateMetainfo(config.AuthorsTemplateFieldName, cfg.TOCPage.AuthorsTemplate, c.SrcName, c.OutputFormat)
			if err != nil {
				log.Warn("Unable to prepare list of authors for TOC", zap.Error(err))
			} else {
				authors = expanded
			}
		}
		bookTitle := c.Book.Description.TitleInfo.BookTitle.Value
		if tocTitle := titleFromStrings(bookTitle, authors); tocTitle != nil {
			titleCtx := NewStyleContext(styles).PushBlock("div", "toc-title")
			markTitleStylesUsed("", "toc-title", styles)
			addTitleAsHeading(c, tocTitle, titleCtx, "toc-title", 1, sb, styles, nil, ca, nil)
		}

		entries := buildTOCEntryTree(tocEntries, cfg.TOCPage.ChaptersWithoutTitle)
		// Register TOC entry anchors in idToEID for anchor fragment generation
		collectTOCAnchors(entries, idToEID)
		// Build hierarchical list structure
		addTOCList(sb, styles, ca, entries)

		for name, list := range ca.Finish() {
			if err := fragments.Add(buildContentFragmentByName(name, list)); err != nil {
				return nil, nil, nil, 0, landmarks, nil, err
			}
		}

		sectionEIDs[tocSectionName] = sb.AllEIDs()
		nextEID = sb.NextEID()

		storyFrag, secFrag := sb.Build()
		if err := fragments.Add(storyFrag); err != nil {
			return nil, nil, nil, 0, landmarks, nil, err
		}
		if err := fragments.Add(secFrag); err != nil {
			return nil, nil, nil, 0, landmarks, nil, err
		}

		// Track TOC EID for landmarks
		landmarks.TOCEID = sb.FirstEID()
		landmarks.TOCLabel = c.Book.Description.TitleInfo.BookTitle.Value

		if cfg.TOCPage.Placement == common.TOCPagePlacementBefore {
			before = append(sectionNameList{tocSectionName}, before...)
		} else {
			after = append(after, tocSectionName)
		}
	}

	// Build the final section order.
	// The first section typically contains the cover image and should remain first.
	// "before" sections (TOC/annotation) go AFTER the cover but BEFORE the rest of content.
	// "after" sections go at the end.
	newOrder := make(sectionNameList, 0, len(sectionNames)+len(before)+len(after))
	if len(before) > 0 && len(sectionNames) > 0 {
		newOrder = append(newOrder, sectionNames[0]) // Keep cover section first
		newOrder = append(newOrder, before...)       // Insert TOC/annotation after cover
		newOrder = append(newOrder, sectionNames[1:]...)
	} else {
		newOrder = append(newOrder, sectionNames...)
	}
	newOrder = append(newOrder, after...)
	return newOrder, tocEntries, sectionEIDs, nextEID, landmarks, idToEID, nil
}
