package kfx

import (
	"strings"

	"fbc/fb2"
)

// resolveInlineStyle resolves an inline style spec to a generated style name.
// It does NOT mark the style as used - the caller must call MarkUsage for styles
// that actually survive processing (e.g., after segmentation deduplication).
func resolveInlineStyle(styles *StyleRegistry, ancestorTag string, styleSpec string) string {
	if styles == nil || styleSpec == "" {
		return styleSpec
	}
	parts := strings.Fields(styleSpec)
	if len(parts) == 1 {
		name := parts[0]
		if ancestorTag != "" {
			descendant := ancestorTag + "--" + name
			if _, ok := styles.Get(descendant); ok {
				return styles.EnsureStyleNoMark(descendant)
			}
		}
		return styles.EnsureStyleNoMark(name)
	}
	// For multi-part style specs, apply descendant lookup to each part
	// so that e.g. "emphasis sub" in h1 context uses "h1--sub" (which has no font-size)
	// instead of "sub" (which has font-size: 0.75rem).
	if ancestorTag != "" {
		resolvedParts := make([]string, len(parts))
		for i, name := range parts {
			descendant := ancestorTag + "--" + name
			if _, ok := styles.Get(descendant); ok {
				resolvedParts[i] = descendant
			} else {
				resolvedParts[i] = name
			}
		}
		return styles.ResolveStyleNoMark(strings.Join(resolvedParts, " "))
	}
	return styles.ResolveStyleNoMark(styleSpec)
}

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
	Items  []InlineContentItem // Content items (text chunks and inline images)
	Events []StyleEventRef     // Style events for inline formatting
}

// processMixedInlineSegments processes inline segments that may contain mixed content
// (text interleaved with images). This is used for both paragraphs and table cells.
//
// Parameters:
//   - segments: the inline segments to process
//   - styles: style registry for resolving and registering styles
//   - footnotesIndex: map of footnote IDs for detecting footnote links
//   - ancestorTag: the ancestor HTML tag for style resolution (e.g., "p", "td", "th")
//   - imageResources: image resource info for resolving inline images
//
// Returns MixedContentResult with items and style events.
func processMixedInlineSegments(
	segments []fb2.InlineSegment,
	styles *StyleRegistry,
	footnotesIndex fb2.FootnoteRefs,
	ancestorTag string,
	imageResources imageResourceInfoByID,
) MixedContentResult {
	var (
		items  []InlineContentItem
		events []StyleEventRef
		nw     = newNormalizingWriter()
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
			imgStyle := styles.ResolveInlineImageStyle(imgInfo.Width, imgInfo.Height)
			items = append(items, InlineContentItem{
				IsImage:      true,
				ResourceName: imgInfo.ResourceName,
				Style:        imgStyle,
				AltText:      seg.Image.Alt,
			})
			// Mark that we're continuing after an image, so leading spaces
			// in subsequent text are preserved as word separators
			nw.SetPendingSpace()
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
				if _, isNote := footnotesIndex[linkTo]; isNote {
					segStyle = "link-footnote"
					isFootnoteLink = true
				} else {
					segStyle = "link-internal"
				}
			} else {
				segStyle = "link-external"
				linkTo = styles.RegisterExternalLink(seg.Href)
			}
		}

		// Track position for style event using rune count
		start := nw.RuneCount()

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
			for _, ctx := range styleContext {
				styleNames = append(styleNames, ctx.Style)
			}
			styleNames = append(styleNames, segStyle)

			var mergedStyle string
			if len(styleNames) > 1 {
				mergedSpec := strings.Join(styleNames, " ")
				mergedStyle = resolveInlineStyle(styles, ancestorTag, mergedSpec)
			} else {
				mergedStyle = resolveInlineStyle(styles, ancestorTag, segStyle)
			}

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
		Items:  items,
		Events: events,
	}
}

// processInlineSegments processes inline segments for text-only content (no inline images).
// This is a convenience wrapper used when inline images should be converted to alt text.
// For mixed content with actual inline images, use processMixedInlineSegments instead.
//
// Parameters:
//   - segments: the inline segments to process
//   - nw: normalizing writer for text accumulation
//   - styles: style registry for resolving and registering styles
//   - footnotesIndex: map of footnote IDs for detecting footnote links
//   - ancestorTag: the ancestor HTML tag for style resolution (e.g., "p", "td")
//
// Returns the accumulated style events.
func processInlineSegments(segments []fb2.InlineSegment, nw *normalizingWriter, styles *StyleRegistry, footnotesIndex fb2.FootnoteRefs, ancestorTag string) []StyleEventRef {
	// Use the shared implementation with nil imageResources (images become alt text)
	result := processMixedInlineSegments(segments, styles, footnotesIndex, ancestorTag, nil)

	// Concatenate all text items into the provided normalizing writer
	for _, item := range result.Items {
		if !item.IsImage {
			nw.WriteString(item.Text)
		}
	}

	return result.Events
}
