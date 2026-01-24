package kfx

import (
	"fmt"
	"strings"

	"fbc/content"
	"fbc/fb2"
)

// addParagraphWithImages processes a paragraph with potential inline images.
// ctx provides the style context with optional position for margin filtering.
// extraClasses are additional CSS classes to append to the style (e.g., paragraph's custom style).
func addParagraphWithImages(c *content.Content, para *fb2.Paragraph, ctx StyleContext, extraClasses string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID) {
	// Check for single spanning style that can be merged into block style
	// This matches KP3 behavior where <p><em>text</em></p> gets font-style:italic in block style
	spanningStyle := detectSingleSpanningStyle(para.Text)
	spanningClasses := extraClasses
	if spanningStyle != "" {
		// Merge the spanning style into the block style name
		if spanningClasses != "" {
			spanningClasses = spanningClasses + " " + spanningStyle
		} else {
			spanningClasses = spanningStyle
		}
	}

	// Determine the element tag - use heading tag (h1-h6) for heading contexts, otherwise p
	tag := "p"
	if headingLevel > 0 {
		tag = fmt.Sprintf("h%d", headingLevel)
	}

	// Build the full styleSpec for this paragraph
	styleSpec := ctx.StyleSpec(tag, spanningClasses)

	// If headingLevel not passed explicitly, try to detect from style name
	if headingLevel == 0 {
		headingLevel = styleToHeadingLevel(styleSpec)
		// Re-check: if we detected a heading level, rebuild styleSpec with proper tag
		if headingLevel > 0 {
			tag = fmt.Sprintf("h%d", headingLevel)
			styleSpec = ctx.StyleSpec(tag, spanningClasses)
		}
	}

	// Determine if images in this paragraph should be inline or block.
	// If the paragraph has text content mixed with images, images are inline.
	// If the paragraph has only images (no text), images are block-level.
	hasTextContent := paragraphHasTextContent(para)
	hasInlineImages := paragraphHasInlineImages(para)

	// Use mixed content mode for:
	// 1. Paragraphs with BOTH text and inline images - images flow with text
	// 2. Image-only paragraphs in heading context - KP3 wraps these in text entry with content_list
	//    so the image becomes "inline within the title" with render: inline
	if hasInlineImages && (hasTextContent || headingLevel > 0) {
		addParagraphWithMixedContent(c, para, ctx, spanningClasses, headingLevel, sb, styles, imageResources, idToEID)
		return
	}

	// Check for image-only block early - these use ResolveBlockImageStyle() which handles
	// its own style resolution, so we don't need to call ctx.Resolve() for them.
	// This avoids creating unused styles that would become unreferenced fragments.
	imageOnlyBlock := !hasTextContent && hasInlineImages

	// For image-only blocks that are NOT standalone images (i.e., images inside p, footnote, etc.),
	// we need to resolve text-indent and margin-left from the CSS cascade and pass them to
	// ResolveBlockImageStyle. This allows the image to be properly aligned:
	// - text-indent becomes margin-left for alignment with text (KP3 behavior for footnote images)
	// - margin-left from container (e.g., cite) is inherited for proper indentation
	var blockTextIndent any
	var blockMarginLeft any
	if imageOnlyBlock && !strings.Contains(styleSpec, "image") {
		// Resolve text-indent and margin-left from the full CSS cascade (context + tag + classes)
		blockTextIndent = ctx.ResolveProperty(tag, spanningClasses, SymTextIndent)
		blockMarginLeft = ctx.ResolveProperty(tag, spanningClasses, SymMarginLeft)
	}

	// Determine if we should resolve style immediately (with position) or defer
	// If context has position, resolve now with position filtering.
	// Otherwise, defer resolution to Build() time.
	// Skip for image-only blocks - they use ResolveBlockImageStyle() which creates its own style.
	var resolvedStyle string
	var deferredStyleSpec string
	if ctx.HasPosition() && !imageOnlyBlock {
		// Immediate resolution with position filtering
		resolvedStyle = ctx.Resolve(tag, spanningClasses)
	} else if !imageOnlyBlock {
		// Deferred resolution - store styleSpec for Build() time
		deferredStyleSpec = styleSpec
	}

	// Otherwise, use the original approach (text-only or block image paragraphs)
	var (
		nw             = newNormalizingWriter() // Normalizes whitespace and tracks rune count
		events         []StyleEventRef
		backlinkRefIDs []string // RefIDs to register after EID is assigned
	)

	// Create inline style context by pushing the paragraph tag and classes.
	// This ensures inline styles (like sub/sup) inherit font-size from the container.
	inlineCtx := ctx.Push(tag, spanningClasses)

	flush := func() {
		if nw.Len() == 0 {
			return
		}
		contentName, offset := ca.Add(nw.String())

		// Apply segmentation to eliminate overlapping style events (KP3 requirement)
		segmentedEvents := SegmentNestedStyleEvents(events)

		// Mark usage only for styles that survived segmentation
		if styles != nil {
			for _, ev := range segmentedEvents {
				styles.MarkUsage(ev.Style, styleUsageText)
			}
		}

		var eid int
		if headingLevel > 0 {
			eid = sb.AddContentWithHeading(SymText, contentName, offset, deferredStyleSpec, resolvedStyle, segmentedEvents, headingLevel)
		} else {
			eid = sb.AddContentAndEvents(SymText, contentName, offset, deferredStyleSpec, resolvedStyle, segmentedEvents)
		}
		if para.ID != "" {
			if _, exists := idToEID[para.ID]; !exists {
				idToEID[para.ID] = eid
			}
		}
		// Register any backlink ref IDs collected during processing
		for _, refID := range backlinkRefIDs {
			if _, exists := idToEID[refID]; !exists {
				idToEID[refID] = eid
			}
		}
		nw.Reset()
		events = nil
		backlinkRefIDs = nil
	}

	// inlineStyleInfo tracks style and optional link info during inline walks.
	type inlineStyleInfo struct {
		Style          string
		LinkTo         string // Link target (internal anchor ID or external link anchor ID)
		IsFootnoteLink bool
	}

	// walk processes inline segments recursively, tracking style context for nested styles.
	// styleContext contains the stack of ancestor inline styles (e.g., [code] when inside a code block).
	// When styles are nested, the inner event's style is merged with all context styles,
	// and link info is inherited from ancestor link segments.
	//
	// If spanningStyle is non-empty, it means the outermost style(s) were already merged into the
	// block style, so we track depth to skip creating style events at the spanning level.
	//
	// imageOnlyBlock (defined above) indicates this is an image-only paragraph where the image
	// should inherit block-level styling (margins, break properties) from the paragraph style.
	spanningStyleParts := strings.Fields(spanningStyle)
	var walk func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo, spanningDepth int)
	walk = func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo, spanningDepth int) {
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
			// For image-only paragraphs (like subtitles with only an image),
			// inherit block-level styling from the paragraph style.
			// This matches KP3's behavior where such images get margins, break properties, etc.
			// For non-standalone images (inside p, footnote, cite, etc.):
			// - blockTextIndent provides text-indent to use as margin-left for left-alignment
			// - blockMarginLeft provides inherited container margin (e.g., from cite)
			var resolved string
			if imageOnlyBlock {
				resolved = styles.ResolveBlockImageStyle(imgInfo.Width, c.ScreenWidth, styleSpec, blockTextIndent, blockMarginLeft)
			} else {
				resolved = styles.ResolveImageStyle(imgInfo.Width, c.ScreenWidth)
			}
			sb.AddImage(imgInfo.ResourceName, resolved, seg.Image.Alt)
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
					// The ref.RefID becomes the anchor that backlinks point to
					ref := c.AddFootnoteBackLinkRef(linkTo)
					// Collect RefID to register with EID after flush
					backlinkRefIDs = append(backlinkRefIDs, ref.RefID)
					// linkTo stays as the footnote ID (after) - it's what we link TO
				} else {
					segStyle = "link-internal"
				}
			} else {
				segStyle = "link-external"
				// Register external URL and get anchor ID for $179 link_to
				linkTo = styles.RegisterExternalLink(seg.Href)
			}
		}

		// Track position for style event using rune count (KFX uses character offsets)
		start := nw.RuneCount()

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
		// If we're at spanning depth, increment it for children
		childSpanningDepth := spanningDepth
		if spanningDepth < len(spanningStyleParts) && segStyle == spanningStyleParts[spanningDepth] {
			childSpanningDepth++
		}
		for i := range seg.Children {
			walk(&seg.Children[i], childContext, childSpanningDepth)
		}

		// Restore whitespace handling after code block
		if seg.Kind == fb2.InlineCode {
			nw.SetPreserveWhitespace(false)
		}

		end := nw.RuneCount()

		// Create style event if we have styled content
		// Skip if this style was already merged into block style (spanningDepth tracks this)
		isSpanningStyle := spanningDepth < len(spanningStyleParts) && segStyle == spanningStyleParts[spanningDepth]
		if segStyle != "" && end > start && !isSpanningStyle {
			// Merge context styles with current style for nested inline elements.
			// E.g., link inside code gets "code link-footnote" merged.
			var styleNames []string
			for _, sctx := range styleContext {
				styleNames = append(styleNames, sctx.Style)
			}
			styleNames = append(styleNames, segStyle)

			// Resolve inline style using the paragraph context (inlineCtx).
			// This ensures inline styles inherit font-size from the container.
			mergedSpec := strings.Join(styleNames, " ")
			mergedStyle := inlineCtx.ResolveNoMark("", mergedSpec)

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

	for i := range para.Text {
		walk(&para.Text[i], nil, 0) // Start with empty style context and spanning depth 0
	}
	flush()
}

// addParagraphWithMixedContent handles paragraphs that have both text and inline images,
// or image-only paragraphs in heading context.
// It creates a single content entry with content_list that interleaves text and images,
// matching KP3's structure where inline images flow with text.
// For heading contexts, this wraps images in a text-type entry so they render as "inline within title".
// ctx provides the style context with optional position for margin filtering.
// spanningClasses are CSS classes from spanning styles (already detected by caller).
func addParagraphWithMixedContent(c *content.Content, para *fb2.Paragraph, ctx StyleContext, spanningClasses string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, idToEID eidByFB2ID) {
	// Determine the element tag - use heading tag (h1-h6) for heading contexts, otherwise p
	tag := "p"
	if headingLevel > 0 {
		tag = fmt.Sprintf("h%d", headingLevel)
	}

	// Build the full styleSpec for this paragraph
	styleSpec := ctx.StyleSpec(tag, spanningClasses)

	// Determine if we should resolve style immediately (with position) or defer
	var resolvedStyle string
	var deferredStyleSpec string
	if ctx.HasPosition() {
		// Immediate resolution with position filtering
		resolvedStyle = ctx.Resolve(tag, spanningClasses)
	} else {
		// Deferred resolution - store styleSpec for Build() time
		deferredStyleSpec = styleSpec
	}

	// Parse spanningClasses for spanning style depth tracking
	spanningStyleParts := strings.Fields(spanningClasses)
	var (
		items          []InlineContentItem // Collected content items (text and images)
		nw             = newNormalizingWriter()
		events         []StyleEventRef
		backlinkRefIDs []string // RefIDs to register after EID is assigned
	)

	// Create inline style context by pushing the paragraph tag and classes.
	// This ensures inline styles (like sub/sup) inherit font-size from the container.
	inlineCtx := ctx.Push(tag, spanningClasses)

	// Track cumulative rune count across flushes for style_event offsets
	var cumulativeRuneCount int

	// Flush current text to items slice, preserving trailing spaces for inline images
	flushText := func() {
		if nw.Len() == 0 && !nw.HasPendingSpace() {
			return
		}
		text := nw.String()
		hadPendingSpace := nw.ConsumePendingSpace()
		if text != "" || hadPendingSpace {
			// Append trailing space if there was one pending
			if hadPendingSpace {
				text += " "
			}
			items = append(items, InlineContentItem{
				Text: text,
			})
		}
		// Accumulate rune count before resetting
		cumulativeRuneCount += nw.RuneCount()
		nw.Reset()
	}

	// inlineStyleInfo tracks style and optional link info during inline walks.
	type inlineStyleInfo struct {
		Style          string
		LinkTo         string
		IsFootnoteLink bool
	}

	var walk func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo, spanningDepth int)
	walk = func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo, spanningDepth int) {
		// Handle inline images - flush current text and add image item
		if seg.Kind == fb2.InlineImageSegment {
			// Check if there's a pending space that will be added during flush
			hasPendingSpace := nw.HasPendingSpace()
			// Capture the offset before flushing (includes all text so far)
			imgOffset := cumulativeRuneCount + nw.RuneCount()
			// Account for the trailing space that flushText() will add
			if hasPendingSpace {
				imgOffset++
			}
			flushText()
			if seg.Image == nil {
				return
			}
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			imgInfo, ok := imageResources[imgID]
			if !ok {
				return
			}

			// Check if image is inside a link context - if so, create a style_event
			// to make the image clickable. KP3 uses an empty style with link_to.
			// The offset is at the current text position, and length covers the
			// "virtual" space the image occupies in the text stream.
			for i := len(styleContext) - 1; i >= 0; i-- {
				if styleContext[i].LinkTo != "" {
					// Create style_event for the linked image
					// Kindle requires the $157 (style) field to be present for links to work,
					// even if the style is empty.
					emptyStyle := styles.MarkUsage("kfx-link-empty", styleUsageText)
					events = append(events, StyleEventRef{
						Offset:         imgOffset,
						Length:         1,
						Style:          emptyStyle,
						LinkTo:         styleContext[i].LinkTo,
						IsFootnoteLink: styleContext[i].IsFootnoteLink,
					})
					break
				}
			}

			// Create inline image style with em-based dimensions
			imgStyle := styles.ResolveInlineImageStyle(imgInfo.Width, imgInfo.Height)
			items = append(items, InlineContentItem{
				IsImage:      true,
				ResourceName: imgInfo.ResourceName,
				Style:        imgStyle,
				AltText:      seg.Image.Alt,
			})
			// Mark that we're continuing after an inline image.
			// This allows leading whitespace in subsequent text to be preserved
			// (if it exists in the source), but does NOT unconditionally add space.
			nw.SetContinueAfterInline()
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
			nw.SetPreserveWhitespace(true)
		case fb2.InlineNamedStyle:
			segStyle = seg.Style
		case fb2.InlineLink:
			isLink = true
			if after, found := strings.CutPrefix(seg.Href, "#"); found {
				linkTo = after
				if _, isNote := c.FootnotesIndex[linkTo]; isNote {
					segStyle = "link-footnote"
					isFootnoteLink = true
					// Register this footnote reference for backlink generation
					ref := c.AddFootnoteBackLinkRef(linkTo)
					// Collect RefID to register with EID after the element is created
					backlinkRefIDs = append(backlinkRefIDs, ref.RefID)
				} else {
					segStyle = "link-internal"
				}
			} else {
				segStyle = "link-external"
				linkTo = styles.RegisterExternalLink(seg.Href)
			}
		}

		// Track position for style event using rune count (cumulative across flushes)
		start := cumulativeRuneCount + nw.RuneCount()

		// Add text content
		nw.WriteString(seg.Text)

		// Build new style context for children
		childContext := styleContext
		if segStyle != "" {
			info := inlineStyleInfo{Style: segStyle}
			if isLink {
				info.LinkTo = linkTo
				info.IsFootnoteLink = isFootnoteLink
			}
			childContext = append(append([]inlineStyleInfo(nil), styleContext...), info)
		}

		// Process children
		childSpanningDepth := spanningDepth
		if spanningDepth < len(spanningStyleParts) && segStyle == spanningStyleParts[spanningDepth] {
			childSpanningDepth++
		}
		for i := range seg.Children {
			walk(&seg.Children[i], childContext, childSpanningDepth)
		}

		if seg.Kind == fb2.InlineCode {
			nw.SetPreserveWhitespace(false)
		}

		end := cumulativeRuneCount + nw.RuneCount()

		// Create style event if we have styled content
		isSpanningStyle := spanningDepth < len(spanningStyleParts) && segStyle == spanningStyleParts[spanningDepth]
		if segStyle != "" && end > start && !isSpanningStyle {
			var styleNames []string
			for _, sctx := range styleContext {
				styleNames = append(styleNames, sctx.Style)
			}
			styleNames = append(styleNames, segStyle)

			// Resolve inline style using the paragraph context (inlineCtx).
			// This ensures inline styles inherit font-size from the container.
			mergedSpec := strings.Join(styleNames, " ")
			mergedStyle := inlineCtx.ResolveNoMark("", mergedSpec)

			event := StyleEventRef{
				Offset: start,
				Length: end - start,
				Style:  mergedStyle,
			}

			if isLink {
				event.LinkTo = linkTo
				event.IsFootnoteLink = isFootnoteLink
			} else {
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

	// Walk all segments to collect items and events
	for i := range para.Text {
		walk(&para.Text[i], nil, 0)
	}
	// Flush any remaining text, but only if there's actual text content (not just pending space).
	// For image-only paragraphs in heading context, we don't want to add a trailing space.
	if nw.Len() > 0 {
		flushText()
	}

	// Don't pre-resolve the block style - pass styleSpec for deferred resolution with position filtering
	// Apply segmentation to events
	segmentedEvents := SegmentNestedStyleEvents(events)

	// Mark usage for segmented styles
	if styles != nil {
		for _, ev := range segmentedEvents {
			styles.MarkUsage(ev.Style, styleUsageText)
		}
	}

	// Add as mixed content entry with optional heading level
	// Pass deferredStyleSpec and resolvedStyle (one or the other will be non-empty based on position context)
	eid := sb.AddMixedContent(deferredStyleSpec, resolvedStyle, items, segmentedEvents, headingLevel)
	if para.ID != "" {
		if _, exists := idToEID[para.ID]; !exists {
			idToEID[para.ID] = eid
		}
	}
	// Register any backlink ref IDs collected during processing
	for _, refID := range backlinkRefIDs {
		if _, exists := idToEID[refID]; !exists {
			idToEID[refID] = eid
		}
	}
}

// detectSingleSpanningStyle checks if a paragraph's content is entirely wrapped in a single
// inline styling element (like <emphasis> or <strong>). If so, returns the style name to merge
// into the block style. Returns empty string if no single spanning style is found.
//
// This matches KP3 behavior where inline styles that span the entire paragraph content
// are promoted to the block style rather than using style_events.
//
// Leading/trailing whitespace-only text segments are ignored when checking for single container.
//
// Examples:
//   - "<p><em>italic text</em></p>" -> returns "emphasis" (entire content is italic)
//   - "<p>some <em>italic</em> text</p>" -> returns "" (partial coverage)
//   - "<p><strong><em>bold italic</em></strong></p>" -> returns "strong emphasis" (nested)
func detectSingleSpanningStyle(segments []fb2.InlineSegment) string {
	// Find the single non-whitespace segment, ignoring leading/trailing whitespace
	var mainSeg *fb2.InlineSegment
	for i := range segments {
		seg := &segments[i]
		// Skip whitespace-only text segments
		if seg.Kind == fb2.InlineText && strings.TrimSpace(seg.Text) == "" {
			continue
		}
		// If we already found a main segment, there are multiple non-whitespace segments
		if mainSeg != nil {
			return ""
		}
		mainSeg = seg
	}

	if mainSeg == nil {
		return ""
	}

	seg := mainSeg

	// The segment must be a styling element (not plain text, image, or link)
	// Links are excluded because they need style_events for the link_to attribute
	var styleNames []string
	for seg != nil {
		switch seg.Kind {
		case fb2.InlineEmphasis:
			styleNames = append(styleNames, "emphasis")
		case fb2.InlineStrong:
			styleNames = append(styleNames, "strong")
		case fb2.InlineStrikethrough:
			styleNames = append(styleNames, "strikethrough")
		case fb2.InlineCode:
			styleNames = append(styleNames, "code")
		case fb2.InlineNamedStyle:
			if seg.Style != "" {
				styleNames = append(styleNames, seg.Style)
			}
		case fb2.InlineText:
			// Plain text at top level - no spanning style
			if len(styleNames) == 0 {
				return ""
			}
			// Text inside styling - check if there are other siblings
			return strings.Join(styleNames, " ")
		case fb2.InlineLink, fb2.InlineImageSegment, fb2.InlineSub, fb2.InlineSup:
			// Links need style_events for link_to, sub/sup typically aren't paragraph-spanning
			return ""
		default:
			return ""
		}

		// Check if this segment has exactly one child to continue descending
		if len(seg.Children) == 1 {
			seg = &seg.Children[0]
		} else if len(seg.Children) == 0 {
			// Styled element with direct text (e.g., <em>text</em> where text is in seg.Text)
			if seg.Text != "" && len(styleNames) > 0 {
				return strings.Join(styleNames, " ")
			}
			return ""
		} else {
			// Multiple children - need to check if they're all text
			allText := true
			for i := range seg.Children {
				if seg.Children[i].Kind != fb2.InlineText {
					allText = false
					break
				}
			}
			if allText && len(styleNames) > 0 {
				return strings.Join(styleNames, " ")
			}
			return ""
		}
	}

	return strings.Join(styleNames, " ")
}
