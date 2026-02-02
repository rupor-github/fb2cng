package kfx

import (
	"fmt"
	"strings"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

// maxStorylineSplitDepth controls the maximum section depth at which titled sections
// become separate storylines. Sections at this depth or shallower with titles get
// their own storyline; deeper sections are processed inline regardless of title.
//
// Value of 2 means:
//   - Depth 1 (top-level sections): titled sections → separate storylines
//   - Depth 2 (nested in depth-1): titled sections → separate storylines
//   - Depth 3+: processed inline (same storyline as parent)
const maxStorylineSplitDepth = 2

// processStorylineSectionContent processes FB2 section content for a single storyline.
//
// depth is the effective heading depth (1..n) used for:
// - choosing wrapper/title heading level
// - deciding whether a titled nested section becomes a separate storyline
//
// Unlike raw FB2 nesting depth, this depth does NOT increase for untitled <section>
// wrappers. KP3 effectively ignores untitled sections for heading level/margins.
// We model that by only incrementing depth when entering a titled section.
//
// isStorylineRoot indicates whether this section is the root section for its storyline.
// KP3 uses slightly different wrapper margin normalization for the first nested title
// in a storyline vs. nested titles that appear inline deeper in the same storyline.
// Titled nested sections up to maxStorylineSplitDepth are collected for the caller to
// process as separate storylines, while deeper sections are processed inline.
//
// Parameters:
//   - nestedTitledSections: receives titled nested sections that should become separate storylines
//   - directChildTOC: receives TOC entries for sections processed inline
func processStorylineSectionContent(c *content.Content, section *fb2.Section, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, depth int, storylineRootDepth int, isStorylineRoot bool, directChildTOC *[]*TOCEntry, nestedTitledSections *[]sectionWorkItem, idToEID eidByFB2ID) error {
	// Enter section container for margin collapsing tracking.
	// Section content is a standard container (no title-block mode).
	//
	// Important: we do NOT push "section" into sectionCtx below to avoid .section overriding
	// descendant paragraph styles via the cascade.
	sb.EnterContainer(ContainerSection, 0)
	defer sb.ExitContainer()

	if section == nil {
		return nil
	}

	// KP3 does not always materialize .section { margin: 1em 0 } onto the first/last element.
	// In particular, for image-only sections (images + empty-lines), KP3 does not apply the
	// section container margins to the first image.
	//
	// We model this by only applying section container margins when the section has
	// non-trivial (non-image) content.
	if sectionHasNonTrivialContent(section) {
		styles.EnsureBaseStyle("section")
		sb.SetContainerMargins(NewStyleContext(styles).ExtractContainerMargins("div", "section"))
	}

	if section.ID != "" {
		if _, exists := idToEID[section.ID]; !exists {
			idToEID[section.ID] = sb.NextEID()
		}
	}

	if section.Image != nil {
		imgID := strings.TrimPrefix(section.Image.Href, "#")
		if imgInfo, ok := imageResources[imgID]; ok {
			ctx := NewStyleContext(styles)
			resolved, isFloatImage := ctx.ResolveImageWithDimensions(ImageBlock, imgInfo.Width, imgInfo.Height, "image")
			eid := sb.AddImage(imgInfo.ResourceName, resolved, section.Image.Alt, isFloatImage)
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
			// KP3 normalizes nested section title wrapper margins relative to the depth where the
			// storyline started. For storylines that start at depth=2, wrapper levels are shifted
			// down by 1 (depth 3 -> wrapper --h2, etc.). For storylines that start at depth=1,
			// wrapper levels match depth (depth 3 -> wrapper --h3).
			if depth == 2 && isStorylineRoot {
				// KP3 uses base wrapper (section-title) for the root titled section of a depth=2 storyline.
				wrapperClass = "section-title"
			} else {
				normalized := depth
				if storylineRootDepth > 1 {
					normalized = depth - (storylineRootDepth - 1)
				}
				normalized = max(normalized, 2)
				wrapperClass = fmt.Sprintf("section-title--h%d", min(normalized, 6))
			}
			headerClassBase = "section-title-header"
			// Map depth to heading level: 2->h2, 3->h3, 4->h4, 5->h5, 6+->h6
			headingLevel = min(depth, 6)
			vigTopPos = common.VignettePosSectionTitleTop
			vigBottomPos = common.VignettePosSectionTitleBottom
		}

		// Start wrapper block - this is the KFX equivalent of <div class="chapter-title"> or <div class="section-title">
		// Children get position-based style filtering: first keeps top margin, last keeps bottom, middle loses both.
		sb.StartBlock(wrapperClass, styles, nil)

		// Add top vignette - style resolved with position filtering at build time
		addVignetteImage(c.Book, sb, imageResources, vigTopPos)

		// Add title as single combined heading entry (matches KP3 behavior)
		// Context includes wrapper class for margin inheritance
		titleCtx := NewStyleContext(styles).Push("div", wrapperClass)
		markTitleStylesUsed(wrapperClass, headerClassBase, styles)
		addTitleAsHeading(c, section.Title, titleCtx, headerClassBase, headingLevel, sb, styles, imageResources, ca, idToEID)

		// Add bottom vignette - style resolved with position filtering at build time
		addVignetteImage(c.Book, sb, imageResources, vigBottomPos)

		// End wrapper block
		sb.EndBlock()
	}

	// Compute child depth once per section. Depth increases only if THIS section has a title.
	childDepth := depth
	if section.HasTitle() {
		childDepth = depth + 1
	}

	// Process annotation.
	if section.Annotation != nil {
		// KP3 sometimes emits an actual wrapper entry (content_list) for annotations.
		// Heuristic: use a wrapper when there are multiple block items.
		if len(section.Annotation.Items) > 1 {
			// Wrapper-backed annotation can render its own container margins, so do NOT force
			// transferring mb to the last child. Keeping mb on children prevents KP3-like
			// sibling collapsing (last child mb should bubble up to the wrapper and collapse
			// into the following element's mt).
			wrapperEID := sb.StartBlock("annotation", styles, &BlockOptions{Kind: ContainerAnnotation})

			// If annotation has an ID, assign it to the wrapper entry.
			if section.Annotation.ID != "" {
				if _, exists := idToEID[section.Annotation.ID]; !exists {
					idToEID[section.Annotation.ID] = wrapperEID
				}
			}

			// Store container margins for post-processing (applies to the wrapper-backed container).
			// Use Push (not PushBlock) so container ml/mr remain on the wrapper rather than being
			// inherited by each child paragraph.
			annotationCtx := NewStyleContext(styles).Push("div", "annotation")
			sb.SetContainerMargins(NewStyleContext(styles).ExtractContainerMargins("div", "annotation"))
			for i := range section.Annotation.Items {
				processFlowItem(c, &section.Annotation.Items[i], annotationCtx, "annotation", sb, styles, imageResources, ca, idToEID)
			}
			sb.EndBlock()
		} else {
			// Flat annotation (no wrapper entry): apply styling directly to each paragraph.
			// Enter annotation container for margin collapsing tracking.
			// Use FlagForceTransferMBToLastChild to always transfer container's margin-bottom to the last paragraph.
			sb.EnterContainer(ContainerAnnotation, FlagForceTransferMBToLastChild)

			// If annotation has an ID, assign it to the first content item.
			if section.Annotation.ID != "" {
				if _, exists := idToEID[section.Annotation.ID]; !exists {
					idToEID[section.Annotation.ID] = sb.NextEID()
				}
			}

			annotationCtx := NewStyleContext(styles).PushBlock("div", "annotation")
			sb.SetContainerMargins(annotationCtx.ExtractContainerMargins("div", "annotation"))
			for i := range section.Annotation.Items {
				processFlowItem(c, &section.Annotation.Items[i], annotationCtx, "annotation", sb, styles, imageResources, ca, idToEID)
			}

			sb.ExitContainer() // Exit annotation container
		}
	}

	// Process epigraphs - KFX doesn't use wrapper blocks for epigraphs.
	// Instead, apply epigraph styling directly to each paragraph as flat siblings.
	// This matches KP3 reference output where margin-left is on each paragraph.
	// Position filtering is applied within each epigraph's items.
	for _, epigraph := range section.Epigraphs {
		// Enter epigraph container for margin collapsing tracking.
		// Use FlagTransferMBToLastChild so that epigraph's margin-bottom goes to its last child
		// (text-author if present) rather than bubbling up to sibling collapsing.
		sb.EnterContainer(ContainerEpigraph, FlagTransferMBToLastChild)

		// If epigraph has an ID, assign it to the first content item
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				// NextEID returns the EID that will be assigned to the next content item
				idToEID[epigraph.Flow.ID] = sb.NextEID()
			}
		}

		epigraphCtx := NewStyleContext(styles).PushBlock("div", "epigraph")
		// Store container margins for post-processing
		sb.SetContainerMargins(epigraphCtx.ExtractContainerMargins("div", "epigraph"))

		for i := range epigraph.Flow.Items {
			processFlowItem(c, &epigraph.Flow.Items[i], epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithImages(c, &epigraph.TextAuthors[i], epigraphCtx, "text-author", 0, sb, styles, imageResources, ca, idToEID)
		}

		sb.ExitContainer() // Exit epigraph container
	}

	// Process content items
	// Note: We don't push "section" into the style context because the .section CSS class
	// has margins meant for the section CONTAINER, not for elements INSIDE the section.
	// If we pushed "section", its margin-bottom (1em) would incorrectly override paragraph
	// margins via the class cascade. Section is purely structural in FB2.
	sectionCtx := NewStyleContext(styles)
	var lastTitledEntry *TOCEntry
	lastTitledDepth := 0

	for i := range section.Content {
		item := &section.Content[i]
		if item.Kind == fb2.FlowSection && item.Section != nil {
			nestedSection := item.Section

			// KP3 effectively treats untitled wrapper sections as belonging to the most
			// recently seen titled sibling section (TOC nesting behaves the same way).
			//
			// This impacts heading level / title wrapper margins: a titled section found
			// inside such an untitled wrapper should be one level deeper than that last
			// titled sibling.
			nextDepth := childDepth
			if !nestedSection.HasTitle() && lastTitledDepth > 0 {
				nextDepth = lastTitledDepth + 1
			}

			// Only split storylines for titled sections up to maxStorylineSplitDepth.
			// Deeper sections are processed inline regardless of title.
			shouldSplit := nestedSection.HasTitle() && depth < maxStorylineSplitDepth

			if shouldSplit {
				// Titled section within split depth -> becomes a separate storyline
				// Add to the work queue for the caller to process
				*nestedTitledSections = append(*nestedTitledSections, sectionWorkItem{
					section:     nestedSection,
					depth:       childDepth,
					parentEntry: nil, // Will be set by caller
					isTopLevel:  false,
				})
				lastTitledDepth = childDepth
			} else {
				// Process inline in this storyline:
				// - Untitled sections at any depth
				// - Titled sections at depth 3+
				firstEID := sb.NextEID()
				if nestedSection.ID != "" {
					if _, exists := idToEID[nestedSection.ID]; !exists {
						idToEID[nestedSection.ID] = firstEID
					}
				}

				// Process nested section content recursively (same storyline)
				var childTOC []*TOCEntry
				var childTitledSections []sectionWorkItem
				if err := processStorylineSectionContent(c, nestedSection, sb, styles, imageResources, ca, nextDepth, storylineRootDepth, false, &childTOC, &childTitledSections, idToEID); err != nil {
					return err
				}

				// Add section-end vignette for inline titled sections (depth > 1)
				// This mirrors EPUB behavior at convert/epub/xhtml.go:797-798
				// Section-end vignette appears at the end of an inline titled section.
				if nestedSection.HasTitle() && childDepth > 1 {
					addVignetteImage(c.Book, sb, imageResources, common.VignettePosSectionEnd)
				}

				// Any titled sections within split depth found in children still need separate storylines
				*nestedTitledSections = append(*nestedTitledSections, childTitledSections...)

				// Create TOC entry for nested section if it has a title
				if nestedSection.HasTitle() {
					titleText := nestedSection.AsTitleText("")
					tocEntry := &TOCEntry{
						ID:           nestedSection.ID,
						Title:        titleText,
						FirstEID:     firstEID,
						IncludeInTOC: true,
						Children:     childTOC,
					}
					*directChildTOC = append(*directChildTOC, tocEntry)
					lastTitledEntry = tocEntry
					lastTitledDepth = nextDepth
				} else if len(childTOC) > 0 {
					// Untitled section - nest children under last titled sibling
					if lastTitledEntry != nil {
						lastTitledEntry.Children = append(lastTitledEntry.Children, childTOC...)
					} else {
						*directChildTOC = append(*directChildTOC, childTOC...)
					}
				}
			}
		} else {
			processFlowItem(c, item, sectionCtx, "section", sb, styles, imageResources, ca, idToEID)
		}
	}

	return nil
}

// sectionHasNonTrivialContent reports whether a section has content that should cause
// .section container margins to be applied.
//
// For KP3 parity, sections consisting only of images and empty-lines should not force
// section container margins to materialize on the first/last image.
func sectionHasNonTrivialContent(section *fb2.Section) bool {
	if section == nil {
		return false
	}
	if section.HasTitle() {
		return true
	}
	if len(section.Epigraphs) > 0 {
		return true
	}
	if section.Annotation != nil && len(section.Annotation.Items) > 0 {
		return true
	}
	for i := range section.Content {
		item := &section.Content[i]
		switch item.Kind {
		case fb2.FlowParagraph, fb2.FlowSubtitle, fb2.FlowPoem, fb2.FlowCite, fb2.FlowTable:
			return true
		case fb2.FlowImage, fb2.FlowEmptyLine:
			// Not considered non-trivial content for section margins.
		case fb2.FlowSection:
			if item.Section != nil && sectionHasNonTrivialContent(item.Section) {
				return true
			}
		}
	}
	return false
}

// processFlowItem processes a flow item using ContentAccumulator.
// ctx tracks the full ancestor context chain for CSS cascade emulation.
// contextName is the immediate context name (e.g., "section", "cite") used for subtitle naming.
func processFlowItem(c *content.Content, item *fb2.FlowItem, ctx StyleContext, contextName string, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID) {
	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil {
			// Consume any pending empty-line margin from StyleContext.
			// This margin was stored for detecting empty-line-before-image cases.
			// When text content comes between empty-line and image, clear it so
			// the image doesn't mistakenly think it was preceded by an empty-line.
			ctx.ConsumePendingMargin()
			// Use full context chain for CSS cascade emulation.
			// Pass context directly - it may have position for margin filtering.
			// Extra classes come from optional custom class in FB2.
			addParagraphWithImages(c, item.Paragraph, ctx, item.Paragraph.Style, 0, sb, styles, imageResources, ca, idToEID)
		}

	case fb2.FlowSubtitle:
		if item.Subtitle != nil {
			// Consume any pending empty-line margin from StyleContext (see FlowParagraph comment).
			ctx.ConsumePendingMargin()
			// Subtitles use <p> in EPUB, so use p as base here too.
			// Context-specific subtitle style adds alignment, margins.
			// Pass the subtitle class via extraClasses so it's applied directly to the paragraph.
			extraClasses := contextName + "-subtitle"
			if item.Subtitle.Style != "" {
				extraClasses = extraClasses + " " + item.Subtitle.Style
			}
			addParagraphWithImages(c, item.Subtitle, ctx, extraClasses, 0, sb, styles, imageResources, ca, idToEID)
		}

	case fb2.FlowEmptyLine:
		// KP3 mostly doesn't create content entries for empty-lines. Instead, it adds
		// the empty-line's margin to the following element's margin-top.
		// Additionally, the preceding element's margin-bottom is stripped.
		// This prevents double-spacing (previous mb + empty-line mt).
		//
		// Exception (KP3 behavior): between two block images, KP3 emits a real
		// container spacer entry with margin-top (see AddEmptyLineSpacer).
		// In that case we must NOT strip the previous image's margin-bottom.
		if !sb.PreviousEntryIsImage() {
			sb.MarkPreviousEntryStripMB()
		}
		// Extract the margin value from the emptyline style and store it for the next entry.
		// The margin is stored in StorylineBuilder (for text) and StyleContext (for images).
		// For text elements: stored in ContentRef.EmptyLineMarginTop, applied during post-processing.
		// For images: stored via SetPreviousEntryEmptyLineMarginBottom (special handling).
		styleSpec := "emptyline"
		margin := ctx.GetEmptyLineMargin(styleSpec, styles)
		if margin > 0 {
			// Store in both places:
			// - StorylineBuilder for the next text entry (applied in addEntry)
			// - StyleContext for FlowImage detection (consumed in FlowImage case)
			sb.SetPendingEmptyLineMarginTop(margin)
			ctx.AddEmptyLineMargin(margin)
		}

	case fb2.FlowPoem:
		if item.Poem != nil {
			// KFX doesn't use wrapper blocks for poems - content is flat with styling applied directly.
			// If poem has an ID, assign it to the first content item.
			if item.Poem.ID != "" {
				if _, exists := idToEID[item.Poem.ID]; !exists {
					idToEID[item.Poem.ID] = sb.NextEID()
				}
			}
			// processPoem handles PushBlock internally with proper item counting
			processPoem(c, item.Poem, ctx, sb, styles, imageResources, ca, idToEID)
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
			// processCite handles PushBlock internally with proper item counting
			processCite(c, item.Cite, ctx, sb, styles, imageResources, ca, idToEID)
		}

	case fb2.FlowTable:
		if item.Table != nil {
			eid := sb.AddTable(c, item.Table, styles, ca, imageResources, idToEID)
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
		// When empty-line precedes an image, KP3 puts the empty-line margin on the
		// PREVIOUS element (as margin-bottom) rather than on the image (as margin-top).
		// This is different from empty-line followed by text, where the margin goes
		// to the next element's margin-top.
		pendingMargin := ctx.ConsumePendingMargin()
		if pendingMargin > 0 {
			// KP3 special case: image + empty-line + image -> emits a spacer container
			// between the images rather than turning the empty-line into image margin.
			if sb.PreviousEntryIsImage() {
				// Clear pending empty-line mt on the builder (the spacer will consume it)
				sb.consumePendingEmptyLineMarginTop()
				sb.AddEmptyLineSpacer(pendingMargin, styles)
			} else {
				// Default: empty-line before image becomes previous element's mb.
				sb.SetPreviousEntryEmptyLineMarginBottom(pendingMargin)
				// Also clear the pending margin from StorylineBuilder since we consumed it
				// (it was set for both places in FlowEmptyLine case)
				sb.consumePendingEmptyLineMarginTop()
			}
		}
		resolved, isFloatImage := ctx.ResolveImageWithDimensions(ImageBlock, imgInfo.Width, imgInfo.Height, "image")
		eid := sb.AddImage(imgInfo.ResourceName, resolved, item.Image.Alt, isFloatImage)
		if item.Image.ID != "" {
			if _, exists := idToEID[item.Image.ID]; !exists {
				idToEID[item.Image.ID] = eid
			}
		}

	case fb2.FlowSection:
		// Nested sections handled in processStorylineSectionContent
	}
}

// processPoem processes poem content using ContentAccumulator.
// Matches EPUB's appendPoemElement handling: title, epigraphs, subtitles, stanzas, text-authors, date.
// ctx contains the ancestor context chain (e.g., may already include "cite" if poem is inside cite).
func processPoem(c *content.Content, poem *fb2.Poem, ctx StyleContext, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID) {
	// Enter poem container for margin collapsing tracking.
	// Poem is a standard container (no title-block mode).
	sb.EnterContainer(ContainerPoem, 0)
	defer sb.ExitContainer()

	// Enter poem container for block-level margin accumulation
	poemCtx := ctx.PushBlock("div", "poem")
	// Store container margins for post-processing
	sb.SetContainerMargins(poemCtx.ExtractContainerMargins("div", "poem"))

	// Process poem title - uses addTitleAsParagraphs for proper -first/-next styling
	// (matches EPUB's appendTitleAsDiv pattern)
	if poem.Title != nil {
		addTitleAsParagraphs(c, poem.Title, poemCtx, "poem-title", 0, sb, styles, imageResources, ca, idToEID)
	}

	// Process poem epigraphs - nested container inside poem
	for _, epigraph := range poem.Epigraphs {
		// Enter epigraph container for margin collapsing tracking.
		// Use FlagTransferMBToLastChild so that epigraph's margin-bottom goes to its last child
		// (text-author if present) rather than bubbling up to sibling collapsing.
		sb.EnterContainer(ContainerEpigraph, FlagTransferMBToLastChild)

		// If epigraph has an ID, assign it to the first content item
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = sb.NextEID()
			}
		}

		epigraphCtx := poemCtx.PushBlock("div", "epigraph")
		// Store container margins for post-processing
		sb.SetContainerMargins(epigraphCtx.ExtractContainerMargins("div", "epigraph"))

		for i := range epigraph.Flow.Items {
			processFlowItem(c, &epigraph.Flow.Items[i], epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithImages(c, &epigraph.TextAuthors[i], epigraphCtx, "text-author", 0, sb, styles, imageResources, ca, idToEID)
		}

		sb.ExitContainer() // Exit epigraph container
	}

	// Process poem subtitles (matches EPUB's poem.Subtitles handling)
	for i := range poem.Subtitles {
		addParagraphWithImages(c, &poem.Subtitles[i], poemCtx, "poem-subtitle", 0, sb, styles, imageResources, ca, idToEID)
	}

	// Process stanzas - nested container inside poem
	for _, stanza := range poem.Stanzas {
		// Enter stanza container for margin collapsing tracking.
		// Stanzas use:
		// - FlagStripMiddleMarginBottom: removes mb from all verses except the last
		// - FlagTransferMBToLastChild: stanza's mb goes TO last verse (not FROM it)
		// This matches KP3 behavior where spacing between verses comes from margin-top,
		// and the last verse carries the accumulated stanza margin-bottom.
		sb.EnterContainer(ContainerStanza, FlagStripMiddleMarginBottom|FlagTransferMBToLastChild)

		// Use stanza context for block-level margin accumulation
		stanzaCtx := poemCtx.PushBlock("div", "stanza")
		// Store container margins for post-processing
		sb.SetContainerMargins(stanzaCtx.ExtractContainerMargins("div", "stanza"))

		// Stanza title - uses addTitleAsParagraphs for proper -first/-next styling
		// (matches EPUB's appendTitleAsDiv pattern)
		if stanza.Title != nil {
			addTitleAsParagraphs(c, stanza.Title, stanzaCtx, "stanza-title", 0, sb, styles, imageResources, ca, idToEID)
		}
		// Stanza subtitle (matches EPUB's "stanza-subtitle" class)
		if stanza.Subtitle != nil {
			addParagraphWithImages(c, stanza.Subtitle, stanzaCtx, "stanza-subtitle", 0, sb, styles, imageResources, ca, idToEID)
		}
		// Verses
		for i := range stanza.Verses {
			addParagraphWithImages(c, &stanza.Verses[i], stanzaCtx, "verse", 0, sb, styles, imageResources, ca, idToEID)
		}

		sb.ExitContainer() // Exit stanza container
	}

	// Process text authors
	for i := range poem.TextAuthors {
		addParagraphWithImages(c, &poem.TextAuthors[i], poemCtx, "text-author", 0, sb, styles, imageResources, ca, idToEID)
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
			// Use immediate resolution with container context for proper margin handling.
			resolvedStyle := poemCtx.Resolve("p", "date")
			contentName, offset := ca.Add(dateText)
			sb.AddContent(SymText, contentName, offset, "", resolvedStyle)
		}
	}
}

// processCite processes cite content using ContentAccumulator.
// Matches EPUB's appendCiteElement handling: processes all flow items with "cite" context,
// followed by text-authors.
// ctx contains the ancestor context chain (already includes "cite" pushed by caller).
func processCite(c *content.Content, cite *fb2.Cite, ctx StyleContext, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID) {
	// Enter cite container for margin collapsing tracking.
	// Use FlagTransferMBToLastChild so that the cite's margin-bottom goes to its last child
	// (text-author if present) rather than bubbling up to sibling collapsing. This matches
	// KP3 behavior where the last element of a cite keeps the container's margin-bottom.
	sb.EnterContainer(ContainerCite, FlagTransferMBToLastChild)
	defer sb.ExitContainer()

	// Enter cite container for block-level margin accumulation
	citeCtx := ctx.PushBlock("blockquote", "cite")
	// Store container margins for post-processing
	sb.SetContainerMargins(citeCtx.ExtractContainerMargins("blockquote", "cite"))

	// Process all cite flow items with full context chain
	for i := range cite.Items {
		processFlowItem(c, &cite.Items[i], citeCtx, "cite", sb, styles, imageResources, ca, idToEID)
	}

	// Process text authors
	for i := range cite.TextAuthors {
		addParagraphWithImages(c, &cite.TextAuthors[i], citeCtx, "text-author", 0, sb, styles, imageResources, ca, idToEID)
	}
}

// processFootnoteSectionContent processes a footnote section content.
// This mirrors EPUB's appendFootnoteSectionContent function, handling all section elements:
// title, epigraphs, image, annotation, and content with special footnote handling
// (more paragraphs indicator and backlinks).
//
// Parameters:
//   - section: the FB2 section to process
//   - addMoreIndicator: function to add "more paragraphs" indicator to first paragraph
//   - addBacklinks: function to add backlink paragraph at the end
func processFootnoteSectionContent(
	c *content.Content,
	section *fb2.Section,
	sb *StorylineBuilder,
	styles *StyleRegistry,
	imageResources imageResourceInfoByID,
	ca *ContentAccumulator,
	idToEID eidByFB2ID,
	addMoreIndicator func(c *content.Content, para *fb2.Paragraph, ctx StyleContext, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID),
	addBacklinks func(c *content.Content, sectionID string, sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator, idToEID eidByFB2ID),
) {
	// Enter footnote section container for margin collapsing tracking.
	sb.EnterContainer(ContainerFootnote, 0)
	defer sb.ExitContainer()

	// Process section title FIRST (without registering ID yet)
	// This matches EPUB behavior where the ID is on the body content, not the title
	// Unlike body titles, section titles are styled paragraphs, not semantic headings
	// (matching EPUB's appendTitleAsDiv pattern with "footnote-title" class)
	if section.Title != nil && len(section.Title.Items) > 0 {
		titleCtx := NewStyleContext(styles).PushBlock("div", "footnote-title")
		if styles != nil {
			styles.EnsureBaseStyle("footnote-title")
		}
		addTitleAsParagraphs(c, section.Title, titleCtx, "footnote-title", 0, sb, styles, imageResources, ca, idToEID)
	}

	// NOW register the section ID - it will point to first body content EID
	// This ensures footnote popups show the body content, not the title
	if section.ID != "" {
		if _, exists := idToEID[section.ID]; !exists {
			idToEID[section.ID] = sb.NextEID()
		}
	}

	// Process epigraphs - same pattern as regular sections
	// KFX doesn't use wrapper blocks for epigraphs; apply styling directly to each paragraph.
	for _, epigraph := range section.Epigraphs {
		// Enter epigraph container for margin collapsing tracking.
		// Use FlagTransferMBToLastChild so that epigraph's margin-bottom goes to its last child
		// (text-author if present) rather than bubbling up to sibling collapsing.
		sb.EnterContainer(ContainerEpigraph, FlagTransferMBToLastChild)

		// If epigraph has an ID, assign it to the first content item
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = sb.NextEID()
			}
		}

		epigraphCtx := NewStyleContext(styles).PushBlock("div", "epigraph")
		// Store container margins for post-processing
		sb.SetContainerMargins(epigraphCtx.ExtractContainerMargins("div", "epigraph"))

		for i := range epigraph.Flow.Items {
			processFlowItem(c, &epigraph.Flow.Items[i], epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID)
		}
		for i := range epigraph.TextAuthors {
			addParagraphWithImages(c, &epigraph.TextAuthors[i], epigraphCtx, "text-author", 0, sb, styles, imageResources, ca, idToEID)
		}

		sb.ExitContainer() // Exit epigraph container
	}

	// Process section image if present
	if section.Image != nil {
		imgID := strings.TrimPrefix(section.Image.Href, "#")
		if imgInfo, ok := imageResources[imgID]; ok {
			ctx := NewStyleContext(styles)
			resolved, isFloatImage := ctx.ResolveImageWithDimensions(ImageBlock, imgInfo.Width, imgInfo.Height, "image")
			eid := sb.AddImage(imgInfo.ResourceName, resolved, section.Image.Alt, isFloatImage)
			if section.Image.ID != "" {
				if _, exists := idToEID[section.Image.ID]; !exists {
					idToEID[section.Image.ID] = eid
				}
			}
		}
	}

	// Process annotation if present
	if section.Annotation != nil {
		// Enter annotation container for margin collapsing tracking.
		// Use FlagForceTransferMBToLastChild to always transfer container's margin-bottom to the last paragraph.
		sb.EnterContainer(ContainerAnnotation, FlagForceTransferMBToLastChild)

		if section.Annotation.ID != "" {
			if _, exists := idToEID[section.Annotation.ID]; !exists {
				idToEID[section.Annotation.ID] = sb.NextEID()
			}
		}
		annotationCtx := NewStyleContext(styles).PushBlock("div", "annotation")
		// Store container margins for post-processing
		sb.SetContainerMargins(annotationCtx.ExtractContainerMargins("div", "annotation"))
		for i := range section.Annotation.Items {
			processFlowItem(c, &section.Annotation.Items[i], annotationCtx, "annotation", sb, styles, imageResources, ca, idToEID)
		}

		sb.ExitContainer() // Exit annotation container
	}

	// Count paragraphs in section content to determine if "more" indicator is needed
	// Skip if footnote-more style has display: none in CSS
	paragraphCount := countFootnoteParagraphs(section.Content)
	moreIndicatorHidden := styles != nil && styles.IsHidden("footnote-more")
	needMoreIndicator := paragraphCount > 1 && c.MoreParaStr != "" && addMoreIndicator != nil && !moreIndicatorHidden
	isFirstParagraph := true

	// Process section content (paragraphs, poems, etc.)
	footnoteCtx := NewStyleContext(styles).PushBlock("div", "footnote")
	for i := range section.Content {
		item := &section.Content[i]

		// Add "more paragraphs" indicator to the first paragraph if needed
		if needMoreIndicator && isFirstParagraph && item.Paragraph != nil {
			addMoreIndicator(c, item.Paragraph, footnoteCtx, sb, styles, imageResources, ca, idToEID)
			isFirstParagraph = false
			continue
		}

		processFlowItem(c, item, footnoteCtx, "footnote", sb, styles, imageResources, ca, idToEID)
		// Mark first paragraph as processed (for poems/cites that may contain paragraphs)
		if item.Paragraph != nil {
			isFirstParagraph = false
		}
	}

	// Add backlink paragraph if this footnote was referenced
	if addBacklinks != nil && section.ID != "" {
		addBacklinks(c, section.ID, sb, styles, ca, idToEID)
	}
}
