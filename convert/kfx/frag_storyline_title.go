package kfx

import (
	"fmt"
	"strings"

	"fbc/fb2"
)

// addTitleAsHeading creates a single combined content entry for multi-paragraph titles.
// This matches KP3 behavior where all title lines are combined with newlines and
// style events are used for -first/-next styling within the combined entry.
// The heading level ($790) is applied only to this combined entry.
// ctx provides the style context (wrapper class like "body-title") for descendant rules and inheritance.
// elemPos provides the element's position within its container for position-aware style filtering.
func addTitleAsHeading(title *fb2.Title, ctx StyleContext, headerStyleBase string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs, elemPos ElementPosition) {
	if title == nil || len(title.Items) == 0 {
		return
	}

	wrapperClass := ""
	if len(ctx.scopes) > 0 {
		// Last scope class list is the current wrapper
		if len(ctx.scopes[len(ctx.scopes)-1].Classes) > 0 {
			wrapperClass = ctx.scopes[len(ctx.scopes)-1].Classes[0]
		}
	}
	descendantPrefix := headerStyleBase
	if wrapperClass != "" {
		descendantPrefix = wrapperClass + "--" + headerStyleBase
	}
	headingTag := fmt.Sprintf("h%d", headingLevel)

	// Check if title contains inline images - if so, fall back to separate paragraphs
	// since KFX can't mix text and images in a single content entry
	if titleHasInlineImages(title) {
		addTitleAsParagraphs(title, ctx, headerStyleBase, headingLevel, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		return
	}

	var (
		nw               = newNormalizingWriter() // Normalizes whitespace and tracks rune count
		events           []StyleEventRef
		firstParagraph   = true
		prevWasEmptyLine = false
		firstParaID      string // Store ID of first paragraph for EID mapping
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
				if _, isNote := footnotesIndex[linkTo]; isNote {
					segStyle = "link-footnote"
					isFootnoteLink = true
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
		for i := range seg.Children {
			processSegment(&seg.Children[i], childContext)
		}

		// Restore whitespace handling after code block
		if seg.Kind == fb2.InlineCode {
			nw.SetPreserveWhitespace(false)
		}

		end := nw.RuneCount()

		// Create style event if we have styled content
		if segStyle != "" && end > start {
			// Merge context styles with current style for nested inline elements.
			// E.g., link inside code gets "code link-footnote" merged.
			var styleNames []string
			for _, ctx := range styleContext {
				styleNames = append(styleNames, ctx.Style)
			}
			styleNames = append(styleNames, segStyle)

			var mergedStyle string
			if len(styleNames) > 1 {
				// Build space-separated style spec: "context1 context2 ... currentStyle"
				mergedSpec := strings.Join(styleNames, " ")
				mergedStyle = resolveInlineStyle(styles, headingTag, mergedSpec)
			} else {
				mergedStyle = resolveInlineStyle(styles, headingTag, segStyle)
			}
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

	// Process each title item
	for _, item := range title.Items {
		if item.Paragraph != nil {
			// Add newline between paragraphs with -break style (but not before first, not after empty line)
			if !firstParagraph && !prevWasEmptyLine {
				breakStart := nw.RuneCount()
				nw.WriteRaw("\n") // Use WriteRaw for structural newline

				// Add style event for the break newline (like EPUB's <br class="...-break">)
				breakStyle := descendantPrefix + "-break"
				resolved := breakStyle
				if styles != nil {
					resolved = styles.MarkUsage(breakStyle, styleUsageText)
				}
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

			// Add paragraph-level style event (like EPUB's span class)
			if paraEnd > paraStart {
				styleName := descendantPrefix + suffixFromParaStyle(paraStyle, headerStyleBase)
				resolved := styleName
				if styles != nil {
					resolved = styles.MarkUsage(styleName, styleUsageText)
				}
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
			emptylineStyle := descendantPrefix + "-emptyline"
			resolved := emptylineStyle
			if styles != nil {
				resolved = styles.MarkUsage(emptylineStyle, styleUsageText)
			}
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

	// Combine heading element style (h1-h6) with context and header class style.
	// Wrapper classes influence the heading through descendant selectors, while
	// wrapper margins stay on the container block.
	// Use position-aware resolution to apply KP3's margin filtering.
	// This single call builds merged properties and applies position filtering before
	// registration, avoiding the double-registration issue of separate Resolve + ResolveStyleWithPosition.
	resolved := ctx.ResolveWithPosition(headingTag, headerStyleBase, styles, elemPos)
	contentName, offset := ca.Add(nw.String())

	// Apply segmentation to eliminate overlapping style events (KP3 requirement)
	segmentedEvents := SegmentNestedStyleEvents(events)

	// Mark usage only for styles that survived segmentation
	if styles != nil {
		for _, ev := range segmentedEvents {
			styles.MarkUsage(ev.Style, styleUsageText)
		}
	}

	eid := sb.AddContentWithHeading(SymText, contentName, offset, resolved, segmentedEvents, headingLevel)

	// Map first paragraph ID to the combined entry's EID
	if firstParaID != "" {
		if _, exists := idToEID[firstParaID]; !exists {
			idToEID[firstParaID] = eid
		}
	}
}

// addSimpleTitleAsHeading creates a title entry for a simple string (used for generated section titles).
// This provides the same semantic styling as addTitleAsHeading but for cases where we have a plain
// string instead of an fb2.Title structure.
//
// Parameters:
//   - text: The title text
//   - styleName: The style name for the heading (e.g., "annotation-title", "toc-title")
//   - headingLevel: The semantic heading level (1-6)
//   - sb: StorylineBuilder to add the entry to
//   - styles: StyleRegistry for style resolution
//   - ca: ContentAccumulator for text content
//
// The function adds content with the specified style, layout-hints: [treat_as_title],
// and yj.semantics.heading_level for accessibility/navigation.
// Unlike addTitleAsHeading, this doesn't use a wrapper block - matching KP3 behavior
// for simple generated section titles.
func addSimpleTitleAsHeading(text, styleName string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, ca *ContentAccumulator) {
	if text == "" {
		return
	}

	// Ensure the style exists
	if styles != nil {
		styles.EnsureBaseStyle(styleName)
	}

	// Resolve the style (gets layout-hints via shouldHaveLayoutHintTitle for *-title patterns)
	resolved := styleName
	if styles != nil {
		resolved = styles.ResolveStyle(styleName)
		styles.MarkUsage(resolved, styleUsageText)
	}

	// Add content with heading level
	contentName, offset := ca.Add(text)
	sb.AddContentWithHeading(SymText, contentName, offset, resolved, nil, headingLevel)
}

func markTitleStylesUsed(wrapperClass, headerBase string, styles *StyleRegistry) {
	if styles == nil {
		return
	}

	styles.EnsureBaseStyle(headerBase)
	if wrapperClass != "" {
		styles.EnsureBaseStyle(wrapperClass)
		styles.EnsureBaseStyle(wrapperClass + "--" + headerBase)
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
//   - title: The FB2 title structure containing paragraphs and empty lines
//   - ctx: Style context for property inheritance
//   - styleBase: Base style name (e.g., "poem-title", "section-title-header") - suffixed with -first/-next
//   - headingLevel: Semantic heading level (1-6), or 0 for no heading semantics
//   - sb: StorylineBuilder to add entries to
//   - styles: StyleRegistry for style resolution
//   - imageResources: Image resource info for inline images
//   - ca: ContentAccumulator for text content
//   - idToEID: Map for ID to EID tracking
//   - screenWidth: Screen width for image sizing
//   - footnotesIndex: Footnote reference index
//
// Note: EmptyLine items are ignored as spacing is handled via block margins.
func addTitleAsParagraphs(title *fb2.Title, ctx StyleContext, styleBase string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	if title == nil || len(title.Items) == 0 {
		return
	}

	firstParagraph := true
	for _, item := range title.Items {
		if item.Paragraph != nil {
			// Determine style for this paragraph (-first or -next)
			var paraStyle string
			if firstParagraph {
				paraStyle = styleBase + "-first"
				firstParagraph = false
			} else {
				paraStyle = styleBase + "-next"
			}

			// Resolve style: use heading element tag (h1-h6) when headingLevel > 0
			var fullStyle string
			if headingLevel > 0 {
				headingElementStyle := fmt.Sprintf("h%d", headingLevel)
				fullStyle = ctx.Resolve(headingElementStyle, paraStyle, styles)
			} else {
				fullStyle = ctx.Resolve("p", paraStyle, styles)
			}

			addParagraphWithImages(item.Paragraph, fullStyle, headingLevel, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
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
