package kfx

import (
	"fmt"
	"strings"

	"fbc/content"
	"fbc/fb2"
)

// addTitleAsHeading creates a single combined content entry for multi-paragraph titles.
// This matches KP3 behavior where all title lines are combined with newlines and
// style events are used for -first/-next styling within the combined entry.
// The heading level ($790) is applied only to this combined entry.
// ctx provides the style context (wrapper class like "body-title") for descendant rules and inheritance.
// Position-based style filtering is deferred to build time when the title is inside a wrapper block.
func addTitleAsHeading(c *content.Content, title *fb2.Title, ctx StyleContext, headerStyleBase string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID) {
	if title == nil || len(title.Items) == 0 {
		return
	}

	// Title style events use the base header styles directly (e.g. "section-title-header-first").
	// Wrapper context is already provided by the StyleContext (ctx) for any descendant selectors.
	descendantPrefix := headerStyleBase
	headingTag := fmt.Sprintf("h%d", headingLevel)

	// Check if title contains inline images - if so, fall back to separate paragraphs
	// since KFX can't mix text and images in a single content entry
	if titleHasInlineImages(title) {
		addTitleAsParagraphs(c, title, ctx, headerStyleBase, headingLevel, sb, styles, imageResources, ca, idToEID)
		return
	}

	// Create inline style context by pushing the heading tag and base style.
	// This ensures inline styles (like sub/sup) inherit font-size from the heading.
	inlineCtx := ctx.Push(headingTag, headerStyleBase)

	var (
		nw               = newNormalizingWriter() // Normalizes whitespace and tracks rune count
		events           []StyleEventRef
		firstParagraph   = true
		prevWasEmptyLine = false
		firstParaID      string                  // Store ID of first paragraph for EID mapping
		backlinkRefIDs   []BacklinkRefWithOffset // Backlink refs to register after EID is assigned
	)

	// inlineStyleInfo tracks style and optional link info during inline walks.
	type inlineStyleInfo struct {
		Style          string
		LinkTo         string // Link target (internal anchor ID or external link anchor ID)
		IsFootnoteLink bool
	}

	// processSegment walks inline segments recursively, tracking style context for nested styles.
	// styleContext contains the stack of ancestor inline styles (e.g., [code] when inside a code block).
	// When styles are nested, the inner event's style is merged with all context styles,
	// and link info is inherited from ancestor link segments.
	var processSegment func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo)
	processSegment = func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo) {
		// Inline images should have been filtered out by titleHasInlineImages check
		if seg.Kind == fb2.InlineImageSegment {
			return
		}

		// Determine style for this segment based on its kind
		var segStyle string
		var isLink bool
		var linkTo string
		var isFootnoteLink bool
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
			nw.SetPreserveWhitespace(true) // Preserve whitespace in code
		case fb2.InlineNamedStyle:
			segStyle = seg.Style
		case fb2.InlineLink:
			// Links use different styles based on target type:
			// - link-footnote: internal links to footnote bodies (superscript style)
			// - link-internal: internal links to other content (no decoration)
			// - link-external: external URLs (underlined)
			isLink = true
			if after, found := strings.CutPrefix(seg.Href, "#"); found {
				linkTo = after
				if _, isNote := c.FootnotesIndex[linkTo]; isNote {
					segStyle = "link-footnote"
					isFootnoteLink = true
					// Register this footnote reference for backlink generation
					ref := c.AddFootnoteBackLinkRef(linkTo)
					// Collect RefID to register with EID after the element is created (offset set below)
					backlinkRefIDs = append(backlinkRefIDs, BacklinkRefWithOffset{RefID: ref.RefID})
				} else {
					segStyle = "link-internal"
				}
			} else {
				segStyle = "link-external"
				// Register external URL and get anchor ID for $179 link_to
				linkTo = styles.RegisterExternalLink(seg.Href)
			}
		}

		// Track position for style event using rune count (KFX uses character offsets).
		// Use GetPseudoStartText to account for ::before content.
		startText := GetPseudoStartText(seg, segStyle, styles)
		start := nw.ContentStartOffset(startText)

		// Now that start is known, set the offset on the backlink ref we just collected
		if isFootnoteLink && len(backlinkRefIDs) > 0 {
			backlinkRefIDs[len(backlinkRefIDs)-1].Offset = start
		}

		// Inject ::before content (inherits styling from base element)
		InjectPseudoBefore(segStyle, styles, nw)

		// Add text content (normalizingWriter handles whitespace and rune counting)
		nw.WriteString(seg.Text)

		// Build new style context for children (if this segment has a style)
		childContext := styleContext
		if segStyle != "" {
			info := inlineStyleInfo{Style: segStyle}
			if isLink {
				info.LinkTo = linkTo
				info.IsFootnoteLink = isFootnoteLink
			}
			childContext = append(append([]inlineStyleInfo(nil), styleContext...), info)
		}

		// Process children with updated style context
		for i := range seg.Children {
			processSegment(&seg.Children[i], childContext)
		}

		// Restore whitespace handling after code block
		if seg.Kind == fb2.InlineCode {
			nw.SetPreserveWhitespace(false)
		}

		// Use RuneCountAfterPendingSpace to include trailing whitespace inside
		// the styled element. KP3 includes such whitespace in the style span.
		end := nw.RuneCountAfterPendingSpace()

		// Inject ::after content (inherits styling from base element)
		// Always update end to include ::after in the main style span
		if InjectPseudoAfter(segStyle, styles, nw) {
			end = nw.RuneCountAfterPendingSpace()
		}

		// Create style event if we have styled content
		if segStyle != "" && end > start {
			// Merge context styles with current style for nested inline elements.
			// E.g., link inside code gets "code link-footnote" merged.
			var styleNames []string
			for _, sctx := range styleContext {
				styleNames = append(styleNames, sctx.Style)
			}
			styleNames = append(styleNames, segStyle)

			// Resolve inline style using delta-only approach (KP3 behavior).
			// Style events contain only properties that differ from the parent
			// (heading style). Block-level properties are excluded.
			mergedSpec := strings.Join(styleNames, " ")
			mergedStyle := inlineCtx.ResolveInlineDelta(mergedSpec)

			// Note: MarkUsage is called later after SegmentNestedStyleEvents(),
			// to avoid marking styles that get deduplicated during segmentation.
			event := StyleEventRef{
				Offset: start,
				Length: end - start,
				Style:  mergedStyle,
			}

			// Inherit link properties from context if not a link itself
			// This ensures nested elements like <a><sup>text</sup></a> get link info
			if isLink {
				event.LinkTo = linkTo
				event.IsFootnoteLink = isFootnoteLink
			} else {
				// Check context for link info (innermost link wins)
				for i := len(styleContext) - 1; i >= 0; i-- {
					if styleContext[i].LinkTo != "" {
						event.LinkTo = styleContext[i].LinkTo
						event.IsFootnoteLink = styleContext[i].IsFootnoteLink
						break
					}
				}
			}

			events = append(events, event)
		}
	}

	// Count paragraphs to determine if we need -first/-next style events.
	// For single-paragraph titles without inline styles, KP3 doesn't add style events.
	paragraphCount := 0
	for _, item := range title.Items {
		if item.Paragraph != nil {
			paragraphCount++
		}
	}
	needParagraphStyleEvents := paragraphCount > 1

	// Process each title item
	for _, item := range title.Items {
		if item.Paragraph != nil {
			// Add newline between paragraphs with -break style (but not before first, not after empty line)
			if !firstParagraph && !prevWasEmptyLine {
				breakStart := nw.RuneCount()
				nw.WriteRaw("\n") // Use WriteRaw for structural newline

				// Add style event for the break newline (like EPUB's <br class="...-break">)
				// Use ResolveInlineDelta to only include properties that differ from the
				// heading's base style, avoiding redundant line-height in style events.
				breakStyle := descendantPrefix + "-break"
				resolved := inlineCtx.ResolveInlineDelta(breakStyle)
				events = append(events, StyleEventRef{
					Offset: breakStart,
					Length: 1,
					Style:  resolved,
				})
			}

			// Determine style for this paragraph (-first or -next)
			var paraStyle string
			if firstParagraph {
				paraStyle = headerStyleBase + "-first"
				firstParaID = item.Paragraph.ID
				firstParagraph = false
			} else {
				paraStyle = headerStyleBase + "-next"
			}

			// Add style event for entire paragraph span
			paraStart := nw.RuneCount()

			// Process paragraph content
			for i := range item.Paragraph.Text {
				processSegment(&item.Paragraph.Text[i], nil) // Start with empty style context
			}

			paraEnd := nw.RuneCount()

			// Add paragraph-level style event only for multi-paragraph titles.
			// For single-paragraph titles, the main element style is sufficient
			// and KP3 doesn't add style events in this case.
			// Use ResolveInlineDelta to only include properties that differ from the
			// heading's base style, avoiding redundant line-height in style events.
			if needParagraphStyleEvents && paraEnd > paraStart {
				styleName := descendantPrefix + suffixFromParaStyle(paraStyle, headerStyleBase)
				resolved := inlineCtx.ResolveInlineDelta(styleName)
				events = append(events, StyleEventRef{
					Offset: paraStart,
					Length: paraEnd - paraStart,
					Style:  resolved,
				})
			}

			prevWasEmptyLine = false
		} else if item.EmptyLine {
			// Add newline for empty line with style event (like EPUB's <br class="...-emptyline">)
			emptylineStart := nw.RuneCount()
			nw.WriteRaw("\n") // Use WriteRaw for structural newline

			// Add style event for the emptyline character
			// Use ResolveInlineDelta to only include properties that differ from the
			// heading's base style, avoiding redundant line-height in style events.
			emptylineStyle := descendantPrefix + "-emptyline"
			resolved := inlineCtx.ResolveInlineDelta(emptylineStyle)
			events = append(events, StyleEventRef{
				Offset: emptylineStart,
				Length: 1,
				Style:  resolved,
			})

			prevWasEmptyLine = true
		}
	}

	// Create the combined content entry with heading level
	if nw.Len() == 0 {
		return
	}

	// Build styleSpec for deferred resolution.
	// Only include the element's own tag and class - the wrapper context is handled
	// by StyleContext in resolveChildStyles(). This avoids merging wrapper margins.
	styleSpec := headingTag + " " + headerStyleBase
	contentName, offset := ca.Add(nw.String())
	totalLen := nw.RuneCount() // Total content length for gap filling

	// Apply segmentation to eliminate overlapping style events (KP3 requirement)
	segmentedEvents := SegmentNestedStyleEvents(events)

	// KP3 fills gaps in title style events with a base line-height style.
	// This ensures every character has at least line-height: 1.0101lh for proper spacing.
	// Only apply when there are explicit style events to fill.
	if len(segmentedEvents) > 0 {
		var baseLineHeightStyle string
		if styles != nil && totalLen > 0 {
			baseLineHeightStyle = styles.RegisterResolved(map[KFXSymbol]any{
				SymLineHeight: DimensionValue(AdjustedLineHeightLh, SymUnitLh),
			}, styleUsageText, true)
		}
		segmentedEvents = FillStyleEventGaps(segmentedEvents, totalLen, baseLineHeightStyle)
	}

	// Mark usage only for styles that survived segmentation and gap filling
	if styles != nil {
		for _, ev := range segmentedEvents {
			styles.ResolveStyle(ev.Style, styleUsageText)
		}
	}

	eid := sb.AddContentWithHeadingDeferred(SymText, contentName, offset, styleSpec, segmentedEvents, headingLevel)

	// Map first paragraph ID to the combined entry's EID
	if firstParaID != "" {
		if _, exists := idToEID[firstParaID]; !exists {
			idToEID[firstParaID] = anchorTarget{EID: eid}
		}
	}

	// Register any backlink ref IDs collected during processing
	for _, ref := range backlinkRefIDs {
		if _, exists := idToEID[ref.RefID]; !exists {
			idToEID[ref.RefID] = anchorTarget{EID: eid, Offset: ref.Offset}
		}
	}
}

func markTitleStylesUsed(wrapperClass, headerBase string, styles *StyleRegistry) {
	if styles == nil {
		return
	}

	// Ensure base and variant styles used by title style events exist.
	styles.EnsureBaseStyle(headerBase)
	styles.EnsureBaseStyle(headerBase + "-first")
	styles.EnsureBaseStyle(headerBase + "-next")
	styles.EnsureBaseStyle(headerBase + "-break")
	styles.EnsureBaseStyle(headerBase + "-emptyline")

	// Ensure wrapper classes exist. wrapperClass may be a class list (space-separated).
	for class := range strings.FieldsSeq(wrapperClass) {
		styles.EnsureBaseStyle(class)
	}
}

func suffixFromParaStyle(paraStyle, headerBase string) string {
	return strings.TrimPrefix(paraStyle, headerBase)
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

// addTitleAsParagraphs adds title paragraphs as separate entries.
// This is the KFX equivalent of EPUB's appendTitleAsDiv.
//
// When headingLevel > 0, paragraphs get heading semantics (used as fallback for
// addTitleAsHeading when titles contain inline images).
//
// When headingLevel == 0, paragraphs are styled without heading semantics
// (used for poem, stanza, and footnote section titles).
//
// Parameters:
//   - c: Content context for footnote tracking
//   - title: The FB2 title structure containing paragraphs and empty lines
//   - ctx: Style context for property inheritance
//   - styleBase: Base style name (e.g., "poem-title", "section-title-header") - suffixed with -first/-next
//   - headingLevel: Semantic heading level (1-6), or 0 for no heading semantics
//   - sb: StorylineBuilder to add entries to
//   - styles: StyleRegistry for style resolution
//   - imageResources: Image resource info for inline images
//   - ca: ContentAccumulator for text content
//   - idToEID: Map for ID to EID tracking
//
// Note: EmptyLine items are ignored as spacing is handled via block margins.
func addTitleAsParagraphs(c *content.Content, title *fb2.Title, ctx StyleContext, styleBase string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID) {
	if title == nil || len(title.Items) == 0 {
		return
	}

	firstParagraph := true
	prevWasImageOnlyHeadingParagraph := false
	for _, item := range title.Items {
		if item.Paragraph != nil {
			// Determine style for this paragraph.
			// Include both the base style (e.g., "chapter-title-header" with text-align: center)
			// and the position-specific style (e.g., "chapter-title-header-first" with display: inline).
			// This ensures the resolved style gets properties from both CSS classes.
			var paraStyle string
			if firstParagraph {
				paraStyle = styleBase + " " + styleBase + "-first"
				firstParagraph = false
			} else {
				paraStyle = styleBase + " " + styleBase + "-next"
			}

			hasTextContent := paragraphHasTextContent(item.Paragraph)
			hasInlineImages := paragraphHasInlineImages(item.Paragraph)
			isImageOnlyHeadingParagraph := headingLevel > 0 && hasInlineImages && !hasTextContent
			isTitleTextFollowingImageOnlyHeading := prevWasImageOnlyHeadingParagraph && hasTextContent
			if isTitleTextFollowingImageOnlyHeading {
				paraStyle = strings.TrimSpace(paraStyle + " title-after-image")
			}

			// Pass paraStyle via extraClasses to apply styling directly to the paragraph.
			// Context (ctx) provides inheritance chain for descendant selector matching.
			addParagraphWithImages(c, item.Paragraph, ctx, paraStyle, headingLevel, sb, styles, imageResources, ca, idToEID)

			prevWasImageOnlyHeadingParagraph = isImageOnlyHeadingParagraph
		}
		// EmptyLine items are ignored - spacing is handled via block margins like regular flow content
	}
}

// titleFromStrings creates an fb2.Title from one or more strings.
// Each non-empty string becomes a paragraph in the title.
// This is useful for generated content (like TOC page titles) that needs
// to be processed through addTitleAsHeading.
func titleFromStrings(lines ...string) *fb2.Title {
	var items []fb2.TitleItem
	for _, line := range lines {
		if line != "" {
			items = append(items, fb2.TitleItem{
				Paragraph: &fb2.Paragraph{
					Text: []fb2.InlineSegment{{Text: line}},
				},
			})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &fb2.Title{Items: items}
}
