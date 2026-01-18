package kfx

import (
	"strings"

	"fbc/common"
	"fbc/fb2"
)

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

	// Process title with wrapper (mirrors EPUB's <div class="chapter-title"> or <div class="section-title">)
	if section.Title != nil {
		// Determine wrapper class, header class base, and heading level based on depth
		var wrapperClass, headerClassBase string
		var headingLevel int
		var vigTopPos, vigBottomPos common.VignettePos
		if depth == 1 {
			wrapperClass = "chapter-title"
			headerClassBase = "chapter-title-header"
			headingLevel = 1
			vigTopPos = common.VignettePosChapterTitleTop
			vigBottomPos = common.VignettePosChapterTitleBottom
		} else {
			wrapperClass = "section-title"
			headerClassBase = "section-title-header"
			// Map depth to heading level: 2->h2, 3->h3, 4->h4, 5->h5, 6+->h6
			headingLevel = min(depth, 6)
			vigTopPos = common.VignettePosSectionTitleTop
			vigBottomPos = common.VignettePosSectionTitleBottom
		}

		// Calculate element positions for this title block
		positions := calcChapterTitlePositions(book, vigTopPos, vigBottomPos)

		// Start wrapper block - this is the KFX equivalent of <div class="chapter-title"> or <div class="section-title">
		// Body/chapter titles (depth 1) use PositionMiddle() to preserve margin-bottom
		// Section titles (depth 2+) use PositionLast() to exclude margin-bottom (KP3 behavior)
		wrapperPos := PositionMiddle()
		if depth > 1 {
			wrapperPos = PositionLast()
		}
		sb.StartBlock(wrapperClass, styles, wrapperPos)

		// Add top vignette with position-aware style
		addVignetteImage(book, sb, styles, imageResources, vigTopPos, screenWidth, positions.VignetteTop)

		// Add title as single combined heading entry (matches KP3 behavior)
		// Context includes wrapper class for margin inheritance
		titleCtx := NewStyleContext().Push("div", wrapperClass, styles)
		markTitleStylesUsed(wrapperClass, headerClassBase, styles)
		addTitleAsHeading(section.Title, titleCtx, headerClassBase, headingLevel, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex, positions.Title)

		// Add bottom vignette with position-aware style
		addVignetteImage(book, sb, styles, imageResources, vigBottomPos, screenWidth, positions.VignetteBottom)

		// End wrapper block
		sb.EndBlock()
	}

	// Process annotation - KFX doesn't use wrapper blocks for annotations.
	// Content is flat with styling applied directly to each paragraph.
	if section.Annotation != nil {
		// If annotation has an ID, assign it to the first content item
		if section.Annotation.ID != "" {
			if _, exists := idToEID[section.Annotation.ID]; !exists {
				idToEID[section.Annotation.ID] = sb.NextEID()
			}
		}
		annotationCtx := NewStyleContext().PushBlock("div", "annotation", styles)
		for i := range section.Annotation.Items {
			processFlowItem(&section.Annotation.Items[i], annotationCtx, "annotation", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	// Process epigraphs - KFX doesn't use wrapper blocks for epigraphs.
	// Instead, apply epigraph styling directly to each paragraph as flat siblings.
	// This matches KP3 reference output where margin-left is on each paragraph.
	epigraphCtx := NewStyleContext().PushBlock("div", "epigraph", styles)
	for _, epigraph := range section.Epigraphs {
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

	// Process content items
	sectionCtx := NewStyleContext().Push("div", "section", styles)
	var lastTitledEntry *TOCEntry

	// Pre-count nested sections for position-aware styling
	nestedSectionCount := 0
	for i := range section.Content {
		if section.Content[i].Kind == fb2.FlowSection && section.Content[i].Section != nil {
			nestedSectionCount++
		}
	}
	_ = nestedSectionCount // May be used for future position-aware styling

	for i := range section.Content {
		item := &section.Content[i]
		if item.Kind == fb2.FlowSection && item.Section != nil {
			// Nested section - track for TOC hierarchy
			// KFX doesn't use wrapper blocks for sections - content is flat.
			nestedSection := item.Section

			// Assign section ID to the first content item that will be created
			firstEID := sb.NextEID()
			if nestedSection.ID != "" {
				if _, exists := idToEID[nestedSection.ID]; !exists {
					idToEID[nestedSection.ID] = firstEID
				}
			}

			// Process nested section content recursively (no wrapper block)
			var childTOC []*TOCEntry
			if err := processStorylineContent(book, nestedSection, sb, styles, imageResources, ca, depth+1, &childTOC, idToEID, screenWidth, footnotesIndex); err != nil {
				return err
			}

			// Create TOC entry for nested section only if it has a title
			if nestedSection.HasTitle() {
				titleText := nestedSection.AsTitleText("")
				tocEntry := &TOCEntry{
					ID:           nestedSection.ID,
					Title:        titleText,
					FirstEID:     firstEID,
					IncludeInTOC: true,
					Children:     childTOC,
				}
				*nestedTOC = append(*nestedTOC, tocEntry)
				lastTitledEntry = tocEntry
			} else if len(childTOC) > 0 {
				// Section without title - nest children inside the last titled sibling if one exists
				if lastTitledEntry != nil {
					lastTitledEntry.Children = append(lastTitledEntry.Children, childTOC...)
				} else {
					// No preceding titled sibling, promote children to parent level
					*nestedTOC = append(*nestedTOC, childTOC...)
				}
			}
		} else {
			processFlowItem(item, sectionCtx, "section", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	if depth == 1 {
		addVignetteImage(book, sb, styles, imageResources, common.VignettePosChapterEnd, screenWidth, PositionMiddle())
	} else {
		addVignetteImage(book, sb, styles, imageResources, common.VignettePosSectionEnd, screenWidth, PositionMiddle())
	}

	return nil
}

// processFlowItem processes a flow item using ContentAccumulator.
// ctx tracks the full ancestor context chain for CSS cascade emulation.
// contextName is the immediate context name (e.g., "section", "cite") used for subtitle naming.
func processFlowItem(item *fb2.FlowItem, ctx StyleContext, contextName string, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil {
			// Use full context chain for CSS cascade emulation.
			// Base "p" + ancestor contexts + optional custom class from FB2.
			styleName := ctx.Resolve("p", "", styles)
			if item.Paragraph.Style != "" {
				styleName = styleName + " " + item.Paragraph.Style
			}
			addParagraphWithImages(item.Paragraph, styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowSubtitle:
		if item.Subtitle != nil {
			// Subtitles use <p> in EPUB, so use p as base here too
			// Context-specific subtitle style adds alignment, margins
			var styleName string
			if contextName == "section" {
				if styles != nil {
					styles.EnsureBaseStyle("section--section-subtitle")
				}
				styleName = "section--section-subtitle"
			} else {
				styleName = ctx.Resolve("p", contextName+"-subtitle", styles)
			}
			if item.Subtitle.Style != "" {
				styleName = styleName + " " + item.Subtitle.Style
			}
			addParagraphWithImages(item.Subtitle, styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowEmptyLine:
		// Empty lines in KFX are handled via block margins on surrounding content,
		// not via explicit newline content entries. Reference KFX files from
		// Kindle Previewer don't have standalone "\n" content entries.
		// The emptyline style should set appropriate margin-top/margin-bottom
		// on adjacent elements instead.
		return

	case fb2.FlowPoem:
		if item.Poem != nil {
			// KFX doesn't use wrapper blocks for poems - content is flat with styling applied directly.
			// If poem has an ID, assign it to the first content item.
			if item.Poem.ID != "" {
				if _, exists := idToEID[item.Poem.ID]; !exists {
					idToEID[item.Poem.ID] = sb.NextEID()
				}
			}
			processPoem(item.Poem, ctx.PushBlock("div", "poem", styles), sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowCite:
		if item.Cite != nil {
			// KFX doesn't use wrapper blocks for cites - content is flat with styling applied directly.
			// If cite has an ID, assign it to the first content item.
			if item.Cite.ID != "" {
				if _, exists := idToEID[item.Cite.ID]; !exists {
					idToEID[item.Cite.ID] = sb.NextEID()
				}
			}
			processCite(item.Cite, ctx.PushBlock("blockquote", "cite", styles), sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowTable:
		if item.Table != nil {
			eid := sb.AddTable(item.Table, styles, ca)
			if item.Table.ID != "" {
				if _, exists := idToEID[item.Table.ID]; !exists {
					idToEID[item.Table.ID] = eid
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
// ctx contains the ancestor context chain (e.g., may already include "cite" if poem is inside cite).
func processPoem(poem *fb2.Poem, ctx StyleContext, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	// Process poem title - uses current context + poem-title
	if poem.Title != nil {
		for _, item := range poem.Title.Items {
			if item.Paragraph != nil {
				styleName := ctx.Resolve("p", "poem-title", styles)
				addParagraphWithImages(item.Paragraph, styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
			}
		}
	}

	// Process poem epigraphs - KFX doesn't use wrapper blocks for epigraphs.
	// Instead, apply epigraph styling directly to each paragraph as flat siblings.
	// This matches KP3 reference output where margin-left is on each paragraph.
	epigraphCtx := ctx.PushBlock("div", "epigraph", styles)
	for _, epigraph := range poem.Epigraphs {
		// If epigraph has an ID, assign it to the first content item
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
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

	// Process poem subtitles (matches EPUB's poem.Subtitles handling)
	for i := range poem.Subtitles {
		styleName := ctx.Resolve("p", "poem-subtitle", styles)
		addParagraphWithImages(&poem.Subtitles[i], styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}

	// Process stanzas - KFX doesn't use wrapper blocks for stanzas.
	// Content is flat with styling applied directly to each verse line.
	stanzaCtx := ctx.PushBlock("div", "stanza", styles)
	for _, stanza := range poem.Stanzas {
		// Stanza title (matches EPUB's "stanza-title" class)
		if stanza.Title != nil {
			for _, item := range stanza.Title.Items {
				if item.Paragraph != nil {
					styleName := stanzaCtx.Resolve("p", "stanza-title", styles)
					addParagraphWithImages(item.Paragraph, styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
				}
			}
		}
		// Stanza subtitle (matches EPUB's "stanza-subtitle" class)
		if stanza.Subtitle != nil {
			styleName := stanzaCtx.Resolve("p", "stanza-subtitle", styles)
			addParagraphWithImages(stanza.Subtitle, styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		// Verses - use stanza context
		for i := range stanza.Verses {
			styleName := stanzaCtx.Resolve("p", "verse", styles)
			addParagraphWithImages(&stanza.Verses[i], styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	// Process text authors
	for i := range poem.TextAuthors {
		styleName := ctx.Resolve("p", "text-author", styles)
		addParagraphWithImages(&poem.TextAuthors[i], styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
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
			styleName := ctx.Resolve("p", "date", styles)
			resolved := styles.ResolveStyle(styleName)
			contentName, offset := ca.Add(dateText)
			sb.AddContent(SymText, contentName, offset, resolved)
		}
	}
}

// processCite processes cite content using ContentAccumulator.
// Matches EPUB's appendCiteElement handling: processes all flow items with "cite" context,
// followed by text-authors.
// ctx contains the ancestor context chain (already includes "cite" pushed by caller).
func processCite(cite *fb2.Cite, ctx StyleContext, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	// Process all cite flow items with full context chain
	for i := range cite.Items {
		processFlowItem(&cite.Items[i], ctx, "cite", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}

	// Process text authors
	for i := range cite.TextAuthors {
		styleName := ctx.Resolve("p", "text-author", styles)
		addParagraphWithImages(&cite.TextAuthors[i], styleName, 0, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}
}
