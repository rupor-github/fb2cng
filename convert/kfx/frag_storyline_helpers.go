package kfx

import (
	"strings"
	"unicode"

	"fbc/content"
	"fbc/fb2"
)

// titleHasInlineImages checks if any title paragraph contains inline images.
// Used to decide whether to use combined heading or separate paragraph approach.
func titleHasInlineImages(title *fb2.Title) bool {
	for _, item := range title.Items {
		if item.Paragraph != nil {
			if paragraphHasInlineImages(item.Paragraph) {
				return true
			}
		}
	}
	return false
}

// paragraphHasInlineImages recursively checks if a paragraph has inline images.
func paragraphHasInlineImages(para *fb2.Paragraph) bool {
	for i := range para.Text {
		if segmentHasInlineImages(&para.Text[i]) {
			return true
		}
	}
	return false
}

// segmentHasInlineImages recursively checks if a segment or its children contain images.
func segmentHasInlineImages(seg *fb2.InlineSegment) bool {
	if seg.Kind == fb2.InlineImageSegment {
		return true
	}
	for i := range seg.Children {
		if segmentHasInlineImages(&seg.Children[i]) {
			return true
		}
	}
	return false
}

// paragraphHasTextContent checks if a paragraph contains any actual text (not just images).
// Returns true if there's non-whitespace text, false if the paragraph is image-only.
func paragraphHasTextContent(para *fb2.Paragraph) bool {
	for i := range para.Text {
		if segmentHasTextContent(&para.Text[i]) {
			return true
		}
	}
	return false
}

// segmentHasTextContent recursively checks if a segment contains text.
func segmentHasTextContent(seg *fb2.InlineSegment) bool {
	if seg.Kind == fb2.InlineImageSegment {
		return false
	}
	// Check if this segment has non-whitespace text
	if strings.TrimSpace(seg.Text) != "" {
		return true
	}
	// Check children recursively
	for i := range seg.Children {
		if segmentHasTextContent(&seg.Children[i]) {
			return true
		}
	}
	return false
}

// findFirstText recursively finds the first text with non-whitespace content in a segment tree.
// This is used for accurate style event offset calculation when the parent segment's
// Text field is empty but children have text (e.g., <strong>text</strong> where
// the strong element's Text is "" but Children[0].Text is "text").
// Returns the first text that contains non-whitespace, or empty string if none found.
// This skips whitespace-only text nodes (like "\n          " between XML tags)
// to correctly identify where styled content actually starts.
func findFirstText(seg *fb2.InlineSegment) string {
	// Check this segment's direct text first - only if it has non-whitespace
	if seg.Text != "" && hasNonWhitespace(seg.Text) {
		return seg.Text
	}
	// Recursively check children
	for i := range seg.Children {
		if text := findFirstText(&seg.Children[i]); text != "" {
			return text
		}
	}
	return ""
}

// hasNonWhitespace returns true if the string contains any non-whitespace character.
func hasNonWhitespace(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// cellContentHasText checks if table cell content contains any actual text (not just images).
func cellContentHasText(content []fb2.InlineSegment) bool {
	for i := range content {
		if segmentHasTextContent(&content[i]) {
			return true
		}
	}
	return false
}

// extractCellImages extracts image references from cell content.
// Returns the image hrefs (without # prefix) for all images found.
func extractCellImages(content []fb2.InlineSegment) []string {
	var images []string
	var walk func(seg *fb2.InlineSegment)
	walk = func(seg *fb2.InlineSegment) {
		if seg.Kind == fb2.InlineImageSegment && seg.Image != nil {
			href := strings.TrimPrefix(seg.Image.Href, "#")
			if href != "" {
				images = append(images, href)
			}
		}
		for i := range seg.Children {
			walk(&seg.Children[i])
		}
	}
	for i := range content {
		walk(&content[i])
	}
	return images
}

// inlineStyleInfo tracks style and optional link info during inline segment processing.
type inlineStyleInfo struct {
	Style          string
	LinkTo         string // Link target (internal anchor ID or external link anchor ID)
	IsFootnoteLink bool
}

// MixedContentResult holds the output of processMixedInlineSegments.
type MixedContentResult struct {
	Items          []InlineContentItem // Content items (text chunks and inline images)
	Events         []StyleEventRef     // Style events for inline formatting
	BacklinkRefIDs []string            // RefIDs of backlinks to register with EID after element creation
}

// processMixedInlineSegments processes inline segments that may contain mixed content
// (text interleaved with images). This is used for both paragraphs and table cells.
//
// Parameters:
//   - segments: the inline segments to process
//   - styles: style registry for resolving and registering styles
//   - c: content context for footnote index
//   - inlineCtx: StyleContext with the container element (p, h1, td, etc.) already pushed,
//     so inline styles inherit properties like font-size from the container
//   - imageResources: image resource info for resolving inline images
//
// Returns MixedContentResult with items and style events.
func processMixedInlineSegments(
	segments []fb2.InlineSegment,
	styles *StyleRegistry,
	c *content.Content,
	inlineCtx StyleContext,
	imageResources imageResourceInfoByID,
) MixedContentResult {
	var (
		items          []InlineContentItem
		events         []StyleEventRef
		backlinkRefIDs []string
		nw             = newNormalizingWriter()
	)

	// flushText adds current accumulated text to items
	flushText := func() {
		if nw.Len() == 0 && !nw.HasPendingSpace() {
			return
		}
		text := nw.String()
		hadPendingSpace := nw.ConsumePendingSpace()
		if text != "" || hadPendingSpace {
			if hadPendingSpace {
				text += " "
			}
			items = append(items, InlineContentItem{
				Text: text,
			})
		}
		nw.Reset()
	}

	var walk func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo)
	walk = func(seg *fb2.InlineSegment, styleContext []inlineStyleInfo) {
		// Handle inline images - flush text and add image item
		if seg.Kind == fb2.InlineImageSegment {
			flushText()
			if seg.Image == nil {
				return
			}
			imgID := strings.TrimPrefix(seg.Image.Href, "#")
			imgInfo, ok := imageResources[imgID]
			if !ok {
				// Image not found - fall back to alt text
				if seg.Image.Alt != "" {
					nw.WriteString(seg.Image.Alt)
				} else {
					nw.WriteString("[image]")
				}
				return
			}
			// Create inline image style with em-based dimensions
			imgStyle, _ := inlineCtx.ResolveImageWithDimensions(ImageInline, imgInfo.Width, imgInfo.Height, "")
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
					// Collect RefID to return to caller for EID registration
					backlinkRefIDs = append(backlinkRefIDs, ref.RefID)
				} else {
					segStyle = "link-internal"
				}
			} else {
				segStyle = "link-external"
				linkTo = styles.RegisterExternalLink(seg.Href)
			}
		}

		// Track position for style event using rune count.
		// Use ContentStartOffset to account for pending space that may be written
		// before this text content - the style event should point to where the
		// styled content actually starts, not including the preceding space.
		// When seg.Text is empty (structured elements like <strong>text</strong>),
		// we need to look at what the first child will write.
		startText := seg.Text
		if startText == "" && len(seg.Children) > 0 {
			startText = findFirstText(seg)
		}
		start := nw.ContentStartOffset(startText)

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

		// Process children with updated style context
		for i := range seg.Children {
			walk(&seg.Children[i], childContext)
		}

		// Restore whitespace handling after code block
		if seg.Kind == fb2.InlineCode {
			nw.SetPreserveWhitespace(false)
		}

		end := nw.RuneCount()

		// Create style event if we have styled content
		if segStyle != "" && end > start {
			var styleNames []string
			for _, sctx := range styleContext {
				styleNames = append(styleNames, sctx.Style)
			}
			styleNames = append(styleNames, segStyle)

			// Resolve inline style using delta-only approach (KP3 behavior).
			// Style events contain only properties that differ from the parent
			// (container style). Block-level properties are excluded.
			mergedSpec := strings.Join(styleNames, " ")
			mergedStyle := inlineCtx.ResolveInlineDelta(mergedSpec)

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

	// Process all segments
	for i := range segments {
		walk(&segments[i], nil)
	}

	// Flush any remaining text
	if nw.Len() > 0 {
		flushText()
	}

	return MixedContentResult{
		Items:          items,
		Events:         events,
		BacklinkRefIDs: backlinkRefIDs,
	}
}

// TextOnlyResult holds the result of processing inline segments for text-only content.
type TextOnlyResult struct {
	Events         []StyleEventRef // Style events for inline formatting
	BacklinkRefIDs []string        // Collected backlink RefIDs for footnote references
}

// processInlineSegments processes inline segments for text-only content (no inline images).
// This is a convenience wrapper used when inline images should be converted to alt text.
// For mixed content with actual inline images, use processMixedInlineSegments instead.
//
// Parameters:
//   - c: content context for footnote index
//   - segments: the inline segments to process
//   - nw: normalizing writer for text accumulation
//   - styles: style registry for resolving and registering styles
//   - inlineCtx: StyleContext with the container element already pushed
//
// Returns the accumulated style events and backlink RefIDs for EID registration.
func processInlineSegments(c *content.Content, segments []fb2.InlineSegment, nw *normalizingWriter, styles *StyleRegistry, inlineCtx StyleContext) TextOnlyResult {
	// Use the shared implementation with nil imageResources (images become alt text)
	result := processMixedInlineSegments(segments, styles, c, inlineCtx, nil)

	// Concatenate all text items into the provided normalizing writer
	for _, item := range result.Items {
		if !item.IsImage {
			nw.WriteString(item.Text)
		}
	}

	return TextOnlyResult{
		Events:         result.Events,
		BacklinkRefIDs: result.BacklinkRefIDs,
	}
}
