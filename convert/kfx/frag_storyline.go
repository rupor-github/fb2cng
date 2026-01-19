package kfx

import (
	"strings"

	"fbc/common"
	"fbc/fb2"
)

// generateStoryline creates storyline and section fragments from an FB2 book.
// It uses the provided StyleRegistry to reference styles by name.
// Returns fragments, next EID, section names for document_data, TOC entries, per-section EID sets,
// and mapping of original FB2 IDs to EIDs (for $266 anchors).
func generateStoryline(book *fb2.FictionBook, styles *StyleRegistry,
	imageResources imageResourceInfoByID, startEID int, footnotesIndex fb2.FootnoteRefs,
) (*FragmentList, int, sectionNameList, []*TOCEntry, sectionEIDsBySectionName, eidByFB2ID, LandmarkInfo, error) {
	fragments := NewFragmentList()
	eidCounter := startEID
	sectionNames := make(sectionNameList, 0)
	tocEntries := make([]*TOCEntry, 0)
	sectionEIDs := make(sectionEIDsBySectionName)
	idToEID := make(eidByFB2ID)
	landmarks := LandmarkInfo{}

	// Single shared content accumulator for the entire book.
	// KP3 consolidates content across all storylines into fewer, larger fragments.
	ca := NewContentAccumulator(1)

	// Default screen width for image style calculations
	defaultWidth := 600

	sectionCount := 0

	// Collect footnote bodies for processing at the end
	var footnoteBodies []*fb2.Body

	// Create separate cover section at the very beginning (like reference KFX)
	// Cover is a container-type section with just the image, not embedded in body intro
	if len(book.Description.TitleInfo.Coverpage) > 0 {
		cover := &book.Description.TitleInfo.Coverpage[0]
		coverID := strings.TrimPrefix(cover.Href, "#")
		if imgInfo, ok := imageResources[coverID]; ok {
			sectionCount++
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline with just the cover image
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)
			// Use minimal cover style - no width constraints since page template defines dimensions
			resolved := styles.ResolveCoverImageStyle()
			sb.AddImage(imgInfo.ResourceName, resolved, cover.Alt)

			sectionEIDs[sectionName] = sb.AllEIDs()
			eidCounter = sb.NextEID()

			// Build cover storyline fragment
			storylineFrag := sb.BuildStorylineOnly()
			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}

			// Build cover section with container type and image dimensions
			// Reference KFX: {$140: $320, $155: eid, $156: $326, $159: $270, $176: storyline, $66: width, $67: height}
			// The page template EID should be unique (not the same as the image content EID)
			pageTemplateEID := sb.PageTemplateEID()
			pageTemplate := NewCoverPageTemplateEntry(pageTemplateEID, storyName, imgInfo.Width, imgInfo.Height)
			sectionFrag := BuildSection(sectionName, []any{pageTemplate})
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}

			// Track cover EID for landmarks
			landmarks.CoverEID = pageTemplateEID
		}
	}

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
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline builder for body intro
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

			if err := processBodyIntroContent(book, body, sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
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
			idToEID[anchorID] = sb.FirstEID()

			// Track start reading location (first body intro is the "Start" landmark)
			if landmarks.StartEID == 0 {
				landmarks.StartEID = sb.FirstEID()
			}

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
		}

		// Process top-level sections as chapters (like epub does)
		for j := range body.Sections {
			section := &body.Sections[j]
			sectionCount++

			// Generate names using base36: "l1", "l2", ... "lA", "lB", ... for storylines
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline builder
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

			// Track nested section info for TOC hierarchy
			var nestedTOCEntries []*TOCEntry

			if err := processStorylineContent(book, section, sb, styles, imageResources, ca, 1, &nestedTOCEntries, idToEID, defaultWidth, footnotesIndex); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
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
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
		}
	}

	// Process footnote bodies at the end - each footnote body gets its own storyline
	// This ensures footnote IDs (n_1, n_2, etc.) are registered in idToEID for anchor generation
	// Reference KFX creates separate storylines for each footnote body (notes, comments, etc.)
	for _, body := range footnoteBodies {
		sectionCount++
		storyName := "l" + toBase36(sectionCount)
		sectionName := "c" + toBase36(sectionCount-1)
		sectionNames = append(sectionNames, sectionName)

		sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

		// Process body title if present with proper heading semantics
		// Uses footnote-title style directly (gets layout-hints: [treat_as_title])
		if body.Title != nil && len(body.Title.Items) > 0 {
			// Create context for title
			titleCtx := NewStyleContext().PushBlock("div", "footnote-title", styles)
			if styles != nil {
				styles.EnsureBaseStyle("footnote-title")
			}

			// Use addTitleAsHeading for proper heading level and layout-hints
			addTitleAsHeading(body.Title, titleCtx, "footnote-title", 2, sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex, PositionFirst())
		}

		// Process each section in the footnote body
		// KFX doesn't use wrapper blocks for footnotes - content is flat with styling applied directly.
		for j := range body.Sections {
			section := &body.Sections[j]

			// Process section title FIRST (without registering ID yet)
			// This matches EPUB behavior where the ID is on the body content, not the title
			// Unlike body titles, section titles are styled paragraphs, not semantic headings
			// (matching EPUB's appendTitleAsDiv pattern and KP3 reference output)
			if section.Title != nil && len(section.Title.Items) > 0 {
				titleCtx := NewStyleContext().PushBlock("div", "footnote-section-title", styles)
				if styles != nil {
					styles.EnsureBaseStyle("footnote-section-title")
				}
				addTitleAsParagraphs(section.Title, titleCtx, "footnote-section-title", 0, sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
			}

			// NOW register the section ID - it will point to first body content EID
			// This ensures footnote popups show the body content, not the title
			if section.ID != "" {
				if _, exists := idToEID[section.ID]; !exists {
					idToEID[section.ID] = sb.NextEID()
				}
			}

			// Process section content (paragraphs, poems, etc.)
			footnoteCtx := NewStyleContext().PushBlock("div", "footnote", styles)
			for k := range section.Content {
				processFlowItem(&section.Content[k], footnoteCtx, "footnote", sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
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
		}
		tocEntries = append(tocEntries, tocEntry)
		idToEID[anchorID] = sb.FirstEID()

		eidCounter = sb.NextEID()
		storylineFrag, sectionFrag := sb.Build()

		if err := fragments.Add(storylineFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, err
		}
		if err := fragments.Add(sectionFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, err
		}
	}

	// Create content fragments from accumulated content (single shared accumulator)
	for name, contentList := range ca.Finish() {
		contentFrag := buildContentFragmentByName(name, contentList)
		if err := fragments.Add(contentFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, err
		}
	}

	return fragments, eidCounter, sectionNames, tocEntries, sectionEIDs, idToEID, landmarks, nil
}

// titleBlockPositions calculates element positions for a title block.
// A title block may contain: vignette-top (optional), title (required), vignette-bottom (optional).
// Returns positions for: vignetteTop, title, vignetteBottom.
type titleBlockPositions struct {
	VignetteTop    ElementPosition
	Title          ElementPosition
	VignetteBottom ElementPosition
}

func calcTitleBlockPositions(book *fb2.FictionBook, hasTopVignette, hasBottomVignette bool) titleBlockPositions {
	// Count actual elements
	count := 1 // title is always present
	if hasTopVignette && book.IsVignetteEnabled(common.VignettePosBookTitleTop) {
		count++
	}
	if hasBottomVignette && book.IsVignetteEnabled(common.VignettePosBookTitleBottom) {
		count++
	}

	// Single element: first and last
	if count == 1 {
		return titleBlockPositions{
			Title: PositionFirstAndLast(),
		}
	}

	// Two elements: first + last
	if count == 2 {
		if hasTopVignette && book.IsVignetteEnabled(common.VignettePosBookTitleTop) {
			return titleBlockPositions{
				VignetteTop: PositionFirst(),
				Title:       PositionLast(),
			}
		}
		return titleBlockPositions{
			Title:          PositionFirst(),
			VignetteBottom: PositionLast(),
		}
	}

	// Three elements: first, middle, last
	return titleBlockPositions{
		VignetteTop:    PositionFirst(),
		Title:          PositionMiddle(),
		VignetteBottom: PositionLast(),
	}
}

// calcChapterTitlePositions calculates positions for chapter/section title blocks.
func calcChapterTitlePositions(book *fb2.FictionBook, vigTopPos, vigBottomPos common.VignettePos) titleBlockPositions {
	hasTop := book != nil && book.IsVignetteEnabled(vigTopPos)
	hasBottom := book != nil && book.IsVignetteEnabled(vigBottomPos)

	count := 1 // title always present
	if hasTop {
		count++
	}
	if hasBottom {
		count++
	}

	if count == 1 {
		return titleBlockPositions{Title: PositionFirstAndLast()}
	}
	if count == 2 {
		if hasTop {
			return titleBlockPositions{VignetteTop: PositionFirst(), Title: PositionLast()}
		}
		return titleBlockPositions{Title: PositionFirst(), VignetteBottom: PositionLast()}
	}
	return titleBlockPositions{
		VignetteTop:    PositionFirst(),
		Title:          PositionMiddle(),
		VignetteBottom: PositionLast(),
	}
}

// addVignetteImage adds a vignette image to the storyline if enabled.
func addVignetteImage(book *fb2.FictionBook, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, vigPos common.VignettePos, screenWidth int, elemPos ElementPosition) {
	if book == nil || !book.IsVignetteEnabled(vigPos) {
		return
	}
	_ = screenWidth
	vigID := book.VignetteIDs[vigPos]
	if vigID == "" {
		return
	}
	imgInfo, ok := imageResources[vigID]
	if !ok {
		return
	}

	// Resolve vignette style with position-aware filtering
	resolved := ""
	if styles != nil {
		resolved = styles.ResolveVignetteImageStyleWithPosition(elemPos)
	}
	sb.AddImage(imgInfo.ResourceName, resolved, "") // Vignettes are decorative, no alt text
}

// processBodyIntroContent processes body intro content (title, epigraphs, image).
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

	// Process body title with wrapper (mirrors EPUB's <div class="body-title">)
	if body.Title != nil {
		// Calculate element positions for this title block
		positions := calcTitleBlockPositions(book, body.Main(), body.Main())

		// Start wrapper block - this is the KFX equivalent of <div class="body-title">
		// Use PositionMiddle() to preserve both margin-top and margin-bottom since content follows
		sb.StartBlock("body-title", styles, PositionMiddle())

		if body.Main() {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosBookTitleTop, screenWidth, positions.VignetteTop)
		}
		// Add title as single combined heading entry (matches KP3 behavior)
		// Uses body-title-header as base for -first/-next styles, heading level 1
		// Context includes wrapper class for margin inheritance
		titleCtx := NewStyleContext().Push("div", "body-title", styles)
		markTitleStylesUsed("body-title", "body-title-header", styles)
		addTitleAsHeading(body.Title, titleCtx, "body-title-header", 1, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex, positions.Title)
		if body.Main() {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosBookTitleBottom, screenWidth, positions.VignetteBottom)
		}

		// End wrapper block
		sb.EndBlock()
	}

	// Process body epigraphs - KFX doesn't use wrapper blocks for epigraphs.
	// Instead, apply epigraph styling directly to each paragraph as flat siblings.
	// This matches KP3 reference output where margin-left is on each paragraph.
	epigraphCtx := NewStyleContext().PushBlock("div", "epigraph", styles)
	for _, epigraph := range body.Epigraphs {
		// If epigraph has an ID, assign it to the first content item
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				// NextEID returns the EID that will be assigned to the next content item
				idToEID[epigraph.Flow.ID] = sb.NextEID()
			}
		}

		for i := range epigraph.Flow.Items {
			processFlowItem(&epigraph.Flow.Items[i], epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		for i := range epigraph.TextAuthors {
			styleName := epigraphCtx.Resolve("p", "text-author", styles)
			addParagraphWithImages(&epigraph.TextAuthors[i], styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	return nil
}
