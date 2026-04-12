package kfx

import (
	"strings"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

type sectionNameList []string

type sectionEIDsBySectionName map[string][]int

// sectionWorkItem represents a section to be processed as a storyline.
// Used by the work queue to flatten nested titled sections into separate storylines.
type sectionWorkItem struct {
	section      *fb2.Section
	depth        int       // Structural depth in the FB2 hierarchy (1 for top-level, always increments per nesting level)
	titleDepth   int       // Visual depth (only increments when entering a titled section); drives vignette type and heading level
	parentEntry  *TOCEntry // Parent TOC entry for hierarchy tracking
	isTopLevel   bool      // True if this is a direct child of <body>
	isChapterEnd bool      // True if this is the last storyline of a chapter (gets chapter-end vignette)
}

// generateStoryline creates storyline and section fragments from an FB2 book.
// It uses the provided StyleRegistry to reference styles by name.
// Returns fragments, next EID, section names for document_data, TOC entries, per-section EID sets,
// mapping of original FB2 IDs to EIDs (for $266 anchors), landmarks, and chapter-start section names.
// Chapter-start sections are those that correspond to EPUB chapter boundaries (cover, body intro,
// top-level sections, footnotes) - used for page map calculations.
func generateStoryline(c *content.Content, styles *StyleRegistry,
	imageResources imageResourceInfoByID, startEID int,
) (*FragmentList, int, sectionNameList, []*TOCEntry, sectionEIDsBySectionName, eidByFB2ID, LandmarkInfo, map[string]bool, error) {
	fragments := NewFragmentList()
	eidCounter := startEID
	sectionNames := make(sectionNameList, 0)
	tocEntries := make([]*TOCEntry, 0)
	sectionEIDs := make(sectionEIDsBySectionName)
	idToEID := make(eidByFB2ID)
	landmarks := LandmarkInfo{}
	chapterStartSections := make(map[string]bool) // Sections that start new chapters/pages

	// Single shared content accumulator for the entire book.
	// KP3 consolidates content across all storylines into fewer, larger fragments.
	ca := NewContentAccumulator(1)

	sectionCount := 0

	// Collect footnote bodies for processing at the end
	var footnoteBodies []*fb2.Body

	// Create separate cover section at the very beginning (like reference KFX)
	// Cover is a container-type section with just the image, not embedded in body intro
	if len(c.Book.Description.TitleInfo.Coverpage) > 0 {
		cover := &c.Book.Description.TitleInfo.Coverpage[0]
		coverID := strings.TrimPrefix(cover.Href, "#")
		if imgInfo, ok := imageResources[coverID]; ok {
			sectionCount++
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)
			chapterStartSections[sectionName] = true // Cover is a chapter boundary

			// Create storyline with just the cover image
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)
			// Use minimal cover style - no width constraints since page template defines dimensions
			resolved := styles.ResolveCoverImageStyle()
			sb.AddImage(imgInfo.ResourceName, resolved, cover.Alt, false)

			sectionEIDs[sectionName] = sb.AllEIDs()
			eidCounter = sb.NextEID()

			// Build cover storyline fragment
			storylineFrag := sb.BuildStorylineOnly()
			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}

			// Build cover section with container type and image dimensions
			// Reference KFX: {$140: $320, $155: eid, $156: $326, $159: $270, $176: storyline, $66: width, $67: height}
			// The page template EID should be unique (not the same as the image content EID)
			pageTemplateEID := sb.PageTemplateEID()
			pageTemplate := NewCoverPageTemplateEntry(pageTemplateEID, storyName, imgInfo.Width, imgInfo.Height)
			sectionFrag := BuildSection(sectionName, []any{pageTemplate})
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}

			// Track cover EID for landmarks
			landmarks.CoverEID = pageTemplateEID
		}
	}

	// Process each body
	for i := range c.Book.Bodies {
		body := &c.Book.Bodies[i]

		// Collect footnote bodies for later processing (at the end, like EPUB does)
		if body.Footnotes() {
			footnoteBodies = append(footnoteBodies, body)
			continue
		}

		// Process body intro content (title, epigraphs, image) as separate storyline
		// This mirrors epub's bodyIntroToXHTML which creates a separate chapter for body intro
		if body.Title != nil {
			// When CSS has page-break-before:always on .body-title and the body has
			// both an image and a title, place the image in its own storyline/section.
			// This creates a structural page break between them, matching KP3 behavior.
			splitBodyImage := body.Image != nil && c.Book.BodyTitleNeedsBreak()

			if splitBodyImage {
				sectionCount++
				imgStoryName := "l" + toBase36(sectionCount)
				imgSectionName := "c" + toBase36(sectionCount-1)
				sectionNames = append(sectionNames, imgSectionName)
				chapterStartSections[imgSectionName] = true

				imgSB := NewStorylineBuilder(imgStoryName, imgSectionName, eidCounter, styles)
				processBodyImageOnly(body, imgSB, styles, imageResources, idToEID)

				sectionEIDs[imgSectionName] = imgSB.AllEIDs()

				// Track start reading location
				if landmarks.StartEID == 0 {
					landmarks.StartEID = imgSB.FirstEID()
				}

				eidCounter = imgSB.NextEID()

				storylineFrag, sectionFrag := imgSB.Build()
				if err := fragments.Add(storylineFrag); err != nil {
					return nil, 0, nil, nil, nil, nil, landmarks, nil, err
				}
				if err := fragments.Add(sectionFrag); err != nil {
					return nil, 0, nil, nil, nil, nil, landmarks, nil, err
				}
			}

			sectionCount++
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)
			chapterStartSections[sectionName] = true // Body intro is a chapter boundary

			// Create storyline builder for body intro
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

			if err := processBodyIntroContent(c, body, sb, styles, imageResources, ca, idToEID, splitBodyImage); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}

			sectionEIDs[sectionName] = sb.AllEIDs()

			// Create TOC entry for body intro
			// Use "a-" prefix for anchor ID to avoid collision with section fragment ID
			title := body.Title.AsTOCText("Untitled")
			anchorID := "a-" + sectionName
			tocEntry := &TOCEntry{
				ID:           anchorID,
				Title:        title,
				SectionName:  sectionName,
				StoryName:    storyName,
				FirstEID:     sb.FirstEID(),
				IncludeInTOC: true,
			}
			tocEntries = append(tocEntries, tocEntry)
			idToEID[anchorID] = anchorTarget{EID: sb.FirstEID()}

			// Track start reading location (first body intro is the "Start" landmark)
			if landmarks.StartEID == 0 {
				landmarks.StartEID = sb.FirstEID()
			}

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}
		}

		// Process top-level sections and their nested titled sections as separate storylines.
		// Use a work queue to flatten the hierarchy - each titled section becomes its own storyline.
		// This matches kfxlib behavior where chapter granularity determines storyline boundaries.
		var workQueue []sectionWorkItem

		// Initialize queue with top-level sections.
		// Each top-level section is initially marked as isChapterEnd=true.
		// If the section has nested titled sections that become separate storylines,
		// isChapterEnd is transferred to the LAST nested section (see below).
		for j := range body.Sections {
			workQueue = append(workQueue, sectionWorkItem{
				section:      &body.Sections[j],
				depth:        1,
				titleDepth:   1,
				parentEntry:  nil,
				isTopLevel:   true,
				isChapterEnd: true, // May be transferred to last nested storyline
			})
		}

		// Process sections from the queue
		for len(workQueue) > 0 {
			// Pop first item (FIFO for document order)
			work := workQueue[0]
			workQueue = workQueue[1:]

			section := work.section
			sectionCount++

			// Generate names using base36: "l1", "l2", ... "lA", "lB", ... for storylines
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Mark top-level sections as chapter boundaries (matches EPUB chapter splits)
			if work.isTopLevel {
				chapterStartSections[sectionName] = true
			}

			// Create storyline builder
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

			// Process section content, collecting nested titled sections for later processing.
			var nestedTitledSections []sectionWorkItem
			var directChildTOC []*TOCEntry

			if err := processStorylineSectionContent(c, section, sb, styles, imageResources, ca, work.depth, work.titleDepth, work.depth, &directChildTOC, &nestedTitledSections, idToEID); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}

			// Add end vignettes.
			//
			// Section-end vignette:
			// - Added for titled nested section storylines (depth > 1)
			// - Only when this storyline is a leaf (no nested titled sections split into their own storylines)
			//   because otherwise the section continues in subsequent storylines.
			//
			// Chapter-end vignette:
			// - Added only for the LAST storyline of the chapter (work.isChapterEnd)
			// - Only when this storyline is a leaf, because a following nested storyline inherits chapter-end.
			//
			// KP3/EPUB behavior: if the chapter ends on a nested section storyline, the output includes BOTH:
			// section-end vignette first, then chapter-end vignette.
			isLeafStoryline := len(nestedTitledSections) == 0
			if isLeafStoryline && section.HasTitle() && work.titleDepth > 1 {
				addEndVignette(c.Book, sb, styles, imageResources, common.VignettePosSectionEnd)
			}
			if work.isChapterEnd && isLeafStoryline && section.HasTitle() {
				addEndVignette(c.Book, sb, styles, imageResources, common.VignettePosChapterEnd)
			}

			sectionEIDs[sectionName] = sb.AllEIDs()

			// Create TOC entry for this section
			includeInTOC := section.HasTitle()
			title := ""
			if includeInTOC {
				title = section.AsTitleText("")
			}
			tocEntry := &TOCEntry{
				ID:           section.ID,
				Title:        title,
				SectionName:  sectionName,
				StoryName:    storyName,
				FirstEID:     sb.FirstEID(),
				IncludeInTOC: includeInTOC,
				Children:     directChildTOC, // Direct children (untitled nested sections)
			}

			// Link to parent TOC hierarchy
			if work.parentEntry != nil {
				work.parentEntry.Children = append(work.parentEntry.Children, tocEntry)
			} else {
				tocEntries = append(tocEntries, tocEntry)
			}

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, nil, err
			}

			// Add nested titled sections to queue for processing as separate storylines.
			// They become children in the TOC hierarchy of the current entry.
			// IMPORTANT: Prepend to queue (not append) to maintain document order.
			// With append, sibling sections would be processed before nested children.
			// With prepend, we process depth-first which preserves FB2 document order.
			//
			// Transfer isChapterEnd to nested sections.
			// When the parent is untitled (wrapper section), ALL children are independent chapters
			// and each gets isChapterEnd=true (mirrors EPUB behavior where each split section
			// gets its own chapter-end vignette based on titleDepth).
			// When the parent is titled, only the LAST nested section inherits isChapterEnd,
			// since the chapter continues through all nested storylines and ends at the last one.
			parentIsUntitled := !section.HasTitle()
			for i := range nestedTitledSections {
				nestedTitledSections[i].parentEntry = tocEntry
				if parentIsUntitled {
					// Untitled wrapper: each child is an independent chapter
					nestedTitledSections[i].isChapterEnd = work.isChapterEnd
				} else {
					// Titled parent: only the last nested section inherits isChapterEnd
					nestedTitledSections[i].isChapterEnd = work.isChapterEnd && (i == len(nestedTitledSections)-1)
				}
			}
			if len(nestedTitledSections) > 0 {
				workQueue = append(nestedTitledSections, workQueue...)
			}
		}
	}

	// Process footnote bodies at the end - each footnote body gets its own storyline
	// This ensures footnote IDs (n_1, n_2, etc.) are registered in idToEID for anchor generation
	// Reference KFX creates separate storylines for each footnote body (notes, comments, etc.)
	//
	// In default mode (not float), footnotes behave like regular sections:
	// - No backlinks are generated
	// - Individual footnote sections appear as nested TOC entries under the footnote body
	isFloatMode := c.FootnotesMode.IsFloat()

	for _, body := range footnoteBodies {
		sectionCount++
		storyName := "l" + toBase36(sectionCount)
		sectionName := "c" + toBase36(sectionCount-1)
		sectionNames = append(sectionNames, sectionName)
		chapterStartSections[sectionName] = true // Footnotes body is a chapter boundary

		sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

		// Process body title if present with proper heading semantics
		// Uses body-title-header style (same as EPUB, matching KP3 reference)
		// This gets layout-hints: [treat_as_title] via shouldHaveLayoutHints
		//
		// KP3 reference structure for footnote body titles:
		//   content_list: [{wrapper with margin-top from body-title}]
		//     content_list: [{title without margin-top}]
		//
		// The wrapper block provides the vertical spacing (margin-top: 2em from body-title CSS),
		// while the title inside has no margin-top (stripped via TitleBlock position).
		if body.Title != nil && len(body.Title.Items) > 0 {
			// Start wrapper block with body-title style (has margin-top: 2em)
			sb.StartBlock("body-title", styles, nil)

			// Create context for title inside wrapper - Push() for inheritance only
			// Position-based style filtering is deferred to build time
			titleCtx := NewStyleContext(styles).Push("div", "body-title")
			markTitleStylesUsed("", "body-title-header", styles)

			addTitleAsHeading(c, body.Title, titleCtx, "body-title-header", 1, sb, styles, imageResources, ca, idToEID)

			sb.EndBlock()
		}

		// Collect child TOC entries for individual footnote sections (default mode only)
		var childTOCEntries []*TOCEntry

		// Process each section in the footnote body using the unified processing function.
		// This ensures consistent handling of all section elements: title, epigraphs,
		// image, annotation, and content.
		for j := range body.Sections {
			section := &body.Sections[j]

			// In default mode (not float): record EID before processing for nested TOC entry
			var sectionFirstEID int
			if !isFloatMode {
				sectionFirstEID = sb.NextEID()
			}

			// Create backlinks callback - only used in float mode
			// In default mode, footnotes behave like regular sections (no backlinks)
			var addBacklinks func(c *content.Content, sectionID string, sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator, idToEID eidByFB2ID)
			if isFloatMode {
				addBacklinks = func(c *content.Content, sectionID string, sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator, idToEID eidByFB2ID) {
					if c.BacklinkStr == "" {
						return
					}
					if refs, ok := c.BackLinkIndex[sectionID]; ok && len(refs) > 0 {
						addBacklinkParagraph(c, refs, sb, styles, ca, idToEID)
					}
				}
			}

			processFootnoteSectionContent(c, section, sb, styles, imageResources, ca, idToEID, addBacklinks)

			// In default mode: create nested TOC entry for this footnote section
			// This mirrors EPUB behavior where individual footnote sections appear under footnote body
			if !isFloatMode {
				sectionTitle := section.AsTitleText("")
				if sectionTitle != "" {
					childEntry := &TOCEntry{
						ID:           section.ID,
						Title:        sectionTitle,
						SectionName:  sectionName,
						StoryName:    storyName,
						FirstEID:     sectionFirstEID,
						IncludeInTOC: true,
					}
					childTOCEntries = append(childTOCEntries, childEntry)
				}
			}
		}

		sectionEIDs[sectionName] = sb.AllEIDs()

		// Create TOC entry for this footnote body
		// Use "a-" prefix for anchor ID to avoid collision with section fragment ID
		// Use body title if available (e.g., "Примечания"), fallback to body.Name (e.g., "notes")
		// Include in TOC only if body has a meaningful title (matching EPUB behavior)
		anchorID := "a-" + sectionName
		bodyTitle := body.AsTitleText(body.Name)
		tocEntry := &TOCEntry{
			ID:           anchorID,
			Title:        bodyTitle,
			SectionName:  sectionName,
			StoryName:    storyName,
			FirstEID:     sb.FirstEID(),
			IncludeInTOC: bodyTitle != "", // Include in TOC if body has a title
			Children:     childTOCEntries, // Nested entries for individual footnote sections (default mode)
		}
		tocEntries = append(tocEntries, tocEntry)
		idToEID[anchorID] = anchorTarget{EID: sb.FirstEID()}

		eidCounter = sb.NextEID()
		storylineFrag, sectionFrag := sb.Build()

		if err := fragments.Add(storylineFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, nil, err
		}
		if err := fragments.Add(sectionFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, nil, err
		}
	}

	// Create content fragments from accumulated content (single shared accumulator)
	for name, contentList := range ca.Finish() {
		contentFrag := buildContentFragmentByName(name, contentList)
		if err := fragments.Add(contentFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, nil, err
		}
	}

	return fragments, eidCounter, sectionNames, tocEntries, sectionEIDs, idToEID, landmarks, chapterStartSections, nil
}

// addVignetteImage adds a vignette image to the storyline if enabled.
// Uses deferred style resolution - the "image-vignette" style will be resolved
// with position filtering at build time when inside a wrapper block.
func addVignetteImage(book *fb2.FictionBook, sb *StorylineBuilder, imageResources imageResourceInfoByID, vigPos common.VignettePos) {
	if book == nil || !book.IsVignetteEnabled(vigPos) {
		return
	}
	vigID := book.VignetteIDs[vigPos]
	if vigID == "" {
		return
	}
	imgInfo, ok := imageResources[vigID]
	if !ok {
		return
	}

	// Use deferred resolution - position filtering applied at build time
	sb.AddImageDeferred(imgInfo.ResourceName, "image-vignette", "") // Vignettes are decorative, no alt text
}

// addEndVignette adds an end-of-section vignette image directly to the storyline.
// Unlike title vignettes, end vignettes are not in a wrapper block and use explicit
// position filtering to remove margin-top (spacing comes from preceding element).
func addEndVignette(book *fb2.FictionBook, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, vigPos common.VignettePos) {
	if book == nil || !book.IsVignetteEnabled(vigPos) {
		return
	}
	vigID := book.VignetteIDs[vigPos]
	if vigID == "" {
		return
	}
	imgInfo, ok := imageResources[vigID]
	if !ok {
		return
	}

	// Resolve end vignette style - uses image-vignette-end which has no margin-top
	// End vignettes get their spacing from the preceding element's margin-bottom
	resolved := ""
	if styles != nil {
		resolved = styles.ResolveStyle("image-vignette-end", styleUsageText)
	}
	sb.AddImage(imgInfo.ResourceName, resolved, "", false) // Vignettes are decorative, no alt text
}

// processBodyImageOnly adds just the body image to a storyline builder.
// Used when the body image is split into its own storyline/section.
func processBodyImageOnly(body *fb2.Body, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, idToEID eidByFB2ID) {
	if body.Image == nil {
		return
	}
	imgID := strings.TrimPrefix(body.Image.Href, "#")
	imgInfo, ok := imageResources[imgID]
	if !ok {
		return
	}
	ctx := NewStyleContext(styles)
	resolved, isFloatImage := ctx.ResolveImageWithDimensions(ImageBlock, imgInfo.Width, imgInfo.Height, "image")
	eid := sb.AddImage(imgInfo.ResourceName, resolved, body.Image.Alt, isFloatImage)
	if body.Image.ID != "" {
		if _, exists := idToEID[body.Image.ID]; !exists {
			idToEID[body.Image.ID] = anchorTarget{EID: eid}
		}
	}
}

// processBodyIntroContent processes body intro content (title, epigraphs, image).
// When skipImage is true, the body image is not added (it was already placed in
// a separate storyline via processBodyImageOnly).
func processBodyIntroContent(c *content.Content, body *fb2.Body, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, skipImage bool) error {
	if !skipImage && body.Image != nil {
		imgID := strings.TrimPrefix(body.Image.Href, "#")
		if imgInfo, ok := imageResources[imgID]; ok {
			ctx := NewStyleContext(styles)
			resolved, isFloatImage := ctx.ResolveImageWithDimensions(ImageBlock, imgInfo.Width, imgInfo.Height, "image")
			eid := sb.AddImage(imgInfo.ResourceName, resolved, body.Image.Alt, isFloatImage)
			if body.Image.ID != "" {
				if _, exists := idToEID[body.Image.ID]; !exists {
					idToEID[body.Image.ID] = anchorTarget{EID: eid}
				}
			}
		}
	}

	// Process body title with wrapper (mirrors EPUB's <div class="body-title">)
	if body.Title != nil {
		// Start wrapper block - this is the KFX equivalent of <div class="body-title">
		// Position-based style filtering is deferred to Build() time when actual position is known
		sb.StartBlock("body-title", styles, nil)

		if body.Main() {
			addVignetteImage(c.Book, sb, imageResources, common.VignettePosBookTitleTop)
		}
		// Add title as single combined heading entry (matches KP3 behavior)
		// Uses body-title-header as base for -first/-next styles, heading level 1
		// Context includes wrapper class for margin inheritance
		titleCtx := NewStyleContext(styles).Push("div", "body-title")
		markTitleStylesUsed("body-title", "body-title-header", styles)
		addTitleAsHeading(c, body.Title, titleCtx, "body-title-header", 1, sb, styles, imageResources, ca, idToEID)
		if body.Main() {
			addVignetteImage(c.Book, sb, imageResources, common.VignettePosBookTitleBottom)
		}

		// End wrapper block
		sb.EndBlock()
	}

	// Process body epigraphs - KFX doesn't use wrapper blocks for epigraphs.
	// Instead, apply epigraph styling directly to each paragraph as flat siblings.
	// This matches KP3 reference output where margin-left is on each paragraph.
	for _, epigraph := range body.Epigraphs {
		// If epigraph has an ID, assign it to the first content item
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				// NextEID returns the EID that will be assigned to the next content item
				idToEID[epigraph.Flow.ID] = anchorTarget{EID: sb.NextEID()}
			}
		}

		epigraphCtx := NewStyleContext(styles).PushBlock("div", "epigraph")

		for i := range epigraph.Flow.Items {
			var next *fb2.FlowItem
			if i+1 < len(epigraph.Flow.Items) {
				next = &epigraph.Flow.Items[i+1]
			}
			processFlowItem(c, &epigraph.Flow.Items[i], next, epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithImages(c, &epigraph.TextAuthors[i], epigraphCtx, "text-author", 0, sb, styles, imageResources, ca, idToEID)
		}
	}

	return nil
}

// addBacklinkParagraph adds a single backlink paragraph at the end of a footnote section.
// All references to the footnote are combined into one paragraph with NBSP separators,
// matching the EPUB implementation.
func addBacklinkParagraph(c *content.Content, refs []content.BackLinkRef, sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator, _ eidByFB2ID) {
	if len(refs) == 0 || c.BacklinkStr == "" {
		return
	}

	// Resolve styles:
	// - Paragraph style: basic paragraph without footnote class (backlink is outside footnote)
	// - Link style: link-backlink with link color properties (for style_events)
	paraStyle := "p"
	linkStyle := "link-backlink"
	if styles != nil {
		styles.EnsureBaseStyle(linkStyle)
	}
	resolvedLink := linkStyle
	if styles != nil {
		// Link style uses StyleContext for style_events (standalone, no container context)
		resolvedLink = NewStyleContext(styles).Resolve("", linkStyle)
	}
	// Don't pre-resolve paraStyle - will be done in Build() with position filtering

	// Build the combined text with NBSP separators between backlinks
	// e.g., "[<]\u00A0[<]\u00A0[<]" for 3 references
	const nbsp = "\u00A0"
	backlinkRunes := []rune(c.BacklinkStr)
	backlinkLen := len(backlinkRunes)

	var textBuilder strings.Builder
	var events []StyleEventRef
	offset := 0

	for i, ref := range refs {
		if i > 0 {
			textBuilder.WriteString(nbsp)
			offset++ // NBSP is 1 rune
		}

		textBuilder.WriteString(c.BacklinkStr)

		// Create style event for this backlink with link to the reference location
		events = append(events, StyleEventRef{
			Offset:         offset,
			Length:         backlinkLen,
			Style:          resolvedLink,
			LinkTo:         ref.RefID,
			IsFootnoteLink: false, // Not a footnote link, it's a backlink
		})

		offset += backlinkLen
	}

	// Add content text
	contentName, contentOffset := ca.Add(textBuilder.String())

	// Mark link style usage (paragraph style will be marked in Build() after position filtering)
	if styles != nil {
		styles.ResolveStyle(resolvedLink, styleUsageText)
	}

	// Add the content entry: paragraph uses container style, events use link style
	// Pass empty resolved style - will be resolved in Build() with position filtering
	sb.AddContentAndEvents(SymText, contentName, contentOffset, paraStyle, "", events)
}

func addStandaloneFootnoteMoreIndicator(c *content.Content, sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator) {
	if c == nil || sb == nil || ca == nil || c.MoreParaStr == "" {
		return
	}

	if styles != nil {
		styles.EnsureBaseStyle("footnote-more")
	}

	contentName, contentOffset := ca.Add(c.MoreParaStr)
	resolved := ""
	if styles != nil {
		resolved = NewStyleContext(styles).Resolve("", "footnote-more")
	}

	// Preserve pending footnote content so the following real body element (e.g. the image)
	// remains the first footnote-content entry.
	hadPendingFootnoteContent := sb.consumePendingFootnoteContent()
	sb.AddContent(SymText, contentName, contentOffset, "", resolved)
	if hadPendingFootnoteContent {
		sb.SetPendingFootnoteContent()
	}
}
