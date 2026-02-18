package kfx

import (
	"fmt"
	"maps"
)

// AddContent adds a content reference to the storyline (or current block).
// The styleSpec is the original style specification (e.g., "p section") used for
// position-based re-resolution at build time. The style is the pre-resolved style name.
func (sb *StorylineBuilder) AddContent(contentType KFXSymbol, contentName string, contentOffset int, styleSpec, style string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(contentType), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name, styleSpec)
		// Only mark usage now if no styleSpec (immediate style, won't be re-resolved)
		// Deferred styles (with styleSpec) are marked after position filtering in Build()
		if styleSpec == "" {
			sb.styles.ResolveStyle(style, styleUsageText)
		}
	}

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		StyleSpec:     styleSpec,
		Style:         style,
	})
}

// AddContentAndEvents adds content with style events (to storyline or current block).
// The styleSpec is the original style specification used for position-based re-resolution.
func (sb *StorylineBuilder) AddContentAndEvents(contentType KFXSymbol, contentName string, contentOffset int, styleSpec, style string, events []StyleEventRef) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(contentType), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name, styleSpec)
		// Only mark usage now if no styleSpec (immediate style, won't be re-resolved)
		// Deferred styles (with styleSpec) are marked after position filtering in Build()
		if styleSpec == "" {
			sb.styles.ResolveStyle(style, styleUsageText)
		}
	}

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		StyleSpec:     styleSpec,
		Style:         style,
		StyleEvents:   events,
	})
}

// AddFootnoteContentAndEvents adds footnote content with style events and position/classification markers.
// This is used for the first paragraph of footnote content (with "more" indicator if present).
// It adds position:footer ($183=$455) and yj.classification:footnote ($615=$281) markers
// that identify the content as footnote body text for Kindle's footnote rendering.
func (sb *StorylineBuilder) AddFootnoteContentAndEvents(contentType KFXSymbol, contentName string, contentOffset int, styleSpec, style string, events []StyleEventRef) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(contentType)+" (footnote)", fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name, styleSpec)
		// Only mark usage now if no styleSpec (immediate style, won't be re-resolved)
		// Deferred styles (with styleSpec) are marked after position filtering in Build()
		if styleSpec == "" {
			sb.styles.ResolveStyle(style, styleUsageText)
		}
	}

	return sb.addEntry(ContentRef{
		EID:             eid,
		Type:            contentType,
		ContentName:     contentName,
		ContentOffset:   contentOffset,
		StyleSpec:       styleSpec,
		Style:           style,
		StyleEvents:     events,
		FootnoteContent: true,
	})
}

// AddContentWithHeading adds content with style events and heading level (to storyline or current block).
func (sb *StorylineBuilder) AddContentWithHeading(contentType KFXSymbol, contentName string, contentOffset int, styleSpec, style string, events []StyleEventRef, headingLevel int) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(contentType), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name, styleSpec)
		// Only mark usage now if no styleSpec (immediate style, won't be re-resolved)
		// Deferred styles (with styleSpec) are marked after position filtering in Build()
		if styleSpec == "" {
			sb.styles.ResolveStyle(style, styleUsageText)
		}
	}

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		StyleSpec:     styleSpec,
		Style:         style,
		StyleEvents:   events,
		HeadingLevel:  headingLevel,
	})
}

// AddImage adds an image reference (to storyline or current block).
//
// isFloatImage marks images that should act as a barrier for margin post-processing.
// At the moment we don't classify any images as float images (always false), but the
// flag is kept to preserve structure for future KP3-parity experiments.
func (sb *StorylineBuilder) AddImage(resourceName, style, altText string, isFloatImage bool) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(SymImage), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name, "")
		sb.styles.ResolveStyle(style, styleUsageImage)
	}

	return sb.addEntry(ContentRef{
		EID:          eid,
		Type:         SymImage,
		ResourceName: resourceName,
		Style:        style,
		AltText:      altText,
		IsFloatImage: isFloatImage,
	})
}

// AddInlineImage adds an inline image reference (embedded within text).
// Unlike block images, inline images have render: inline and use em-based dimensions
// with baseline-style: center for vertical alignment within text.
func (sb *StorylineBuilder) AddInlineImage(resourceName, style, altText string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(SymImage)+" (inline)", fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name, "")
		sb.styles.ResolveStyle(style, styleUsageImage)
	}

	return sb.addEntry(ContentRef{
		EID:          eid,
		Type:         SymImage,
		ResourceName: resourceName,
		Style:        style,
		AltText:      altText,
		RenderInline: true, // Sets render: inline ($601 = $283)
	})
}

// AddMixedContent adds a text entry with interleaved inline images using content_list.
// This creates the KP3-compatible structure where text and images are mixed in a single entry:
//
//	{id, style, type: text, content_list: ["text", {image}, "more text", {image}, ...], style_events: [...]}
//
// The items slice contains InlineContentItem elements which are either text strings or inline images.
// Style events apply to the text portions only (offsets are relative to concatenated text).
// If headingLevel > 0, the entry includes yj.semantics.heading_level for TOC navigation.
// The styleSpec parameter enables deferred style resolution with position filtering in Build().
// If styleSpec is non-empty, style resolution is deferred; otherwise style is used directly.
func (sb *StorylineBuilder) AddMixedContent(styleSpec, style string, items []InlineContentItem, events []StyleEventRef, headingLevel int) int {
	eid := sb.eidCounter
	sb.eidCounter++

	// For immediate styles (no styleSpec), mark usage now
	if style != "" && styleSpec == "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(SymText)+" (mixed)", fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name, "")
		sb.styles.ResolveStyle(style, styleUsageText)
	}

	// Build content_list array with text strings and image entries
	contentList := make([]any, 0, len(items))
	for _, item := range items {
		if item.IsImage {
			// Create inline image entry
			imgEid := sb.eidCounter
			sb.eidCounter++

			if item.Style != "" && sb.styles != nil {
				sb.styles.tracer.TraceAssign(traceSymbolName(SymImage)+" (inline/mixed)", fmt.Sprintf("%d", imgEid), item.Style, sb.sectionName+"/"+sb.name, "")
				sb.styles.ResolveStyle(item.Style, styleUsageImage)
			}

			imgEntry := NewStruct().
				SetInt(SymUniqueID, int64(imgEid)).
				SetSymbol(SymType, SymImage).
				Set(SymResourceName, SymbolByName(item.ResourceName)).
				SetSymbol(SymRender, SymInline) // render: inline

			if item.Style != "" {
				imgEntry.Set(SymStyle, SymbolByName(item.Style))
			}
			imgEntry.SetString(SymAltText, item.AltText) // Always include, even if empty

			contentList = append(contentList, imgEntry)
		} else {
			// Raw text string
			contentList = append(contentList, item.Text)
		}
	}

	// Build the entry with content_list instead of content
	entry := NewStruct().
		SetInt(SymUniqueID, int64(eid)).
		SetSymbol(SymType, SymText).
		SetList(SymContentList, contentList)

	if style != "" {
		entry.Set(SymStyle, SymbolByName(style))
	}

	// Add style events if present
	if len(events) > 0 {
		eventList := make([]any, 0, len(events))
		for _, se := range events {
			ev := NewStruct().
				SetInt(SymOffset, int64(se.Offset)).
				SetInt(SymLength, int64(se.Length))
			if se.Style != "" {
				ev.Set(SymStyle, SymbolByName(se.Style))
			}
			if se.LinkTo != "" {
				ev.Set(SymLinkTo, SymbolByName(se.LinkTo))
			}
			if se.IsFootnoteLink {
				ev.SetSymbol(SymYjDisplay, SymYjNote)
			}
			eventList = append(eventList, ev)
		}
		entry.SetList(SymStyleEvents, eventList)
	}

	// Add heading level if present (for TOC navigation)
	if headingLevel > 0 {
		entry.SetInt(SymYjHeadingLevel, int64(headingLevel))
	}

	// Add as raw entry with optional styleSpec for deferred resolution.
	// Set Style on ContentRef so captureMargins can extract margins for collapsing.
	// The style is also on RawEntry for serialization.
	return sb.addEntry(ContentRef{
		EID:       eid,
		StyleSpec: styleSpec,
		Style:     style,
		Type:      SymText,
		RawEntry:  entry,
	})
}

// ============================================================================
// Deferred Style Resolution Methods
// ============================================================================
// These methods store StyleSpec instead of Style, allowing position-aware
// resolution in EndBlock(). Use these when adding content to position-aware blocks.

// AddContentDeferred adds content with deferred style resolution.
// The styleSpec will be resolved with position filtering in EndBlock().
// Use this when adding content to blocks started with StartBlock.
func (sb *StorylineBuilder) AddContentDeferred(contentType KFXSymbol, contentName string, contentOffset int, styleSpec string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		StyleSpec:     styleSpec,
	})
}

// AddContentAndEventsDeferred adds content with style events and deferred style resolution.
// The styleSpec will be resolved with position filtering in EndBlock().
func (sb *StorylineBuilder) AddContentAndEventsDeferred(contentType KFXSymbol, contentName string, contentOffset int, styleSpec string, events []StyleEventRef) int {
	eid := sb.eidCounter
	sb.eidCounter++

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		StyleSpec:     styleSpec,
		StyleEvents:   events,
	})
}

// AddContentWithHeadingDeferred adds content with heading level and deferred style resolution.
// The styleSpec will be resolved with position filtering in EndBlock().
func (sb *StorylineBuilder) AddContentWithHeadingDeferred(contentType KFXSymbol, contentName string, contentOffset int, styleSpec string, events []StyleEventRef, headingLevel int) int {
	eid := sb.eidCounter
	sb.eidCounter++

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		StyleSpec:     styleSpec,
		StyleEvents:   events,
		HeadingLevel:  headingLevel,
	})
}

// AddImageDeferred adds an image with deferred style resolution.
// The styleSpec will be resolved with position filtering in EndBlock().
func (sb *StorylineBuilder) AddImageDeferred(resourceName, styleSpec, altText string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	return sb.addEntry(ContentRef{
		EID:          eid,
		Type:         SymImage,
		ResourceName: resourceName,
		StyleSpec:    styleSpec,
		AltText:      altText,
	})
}

// AddRawEntry adds a pre-built StructValue entry to the storyline.
// This is used for complex structures like lists that are built externally.
func (sb *StorylineBuilder) AddRawEntry(entry StructValue) {
	ref := ContentRef{
		RawEntry: entry,
	}
	// Route through addEntry so container tracking and pending empty-line
	// margin consumption stays consistent.
	sb.addEntry(ref)
}

// AddEmptyLineSpacer inserts a KP3-like empty-line spacer between block elements.
// KP3 represents <empty-line/> between two block images as a container($270)
// with layout: vertical and margin-top set to the empty-line margin.
func (sb *StorylineBuilder) AddEmptyLineSpacer(marginTopLh float64, styles *StyleRegistry) int {
	eid := sb.eidCounter
	sb.eidCounter++

	styleName := "emptyline-spacer"
	resolved := styleName
	if styles != nil {
		styles.EnsureBaseStyle(styleName)
		resolved = styles.ResolveStyle(styleName, styleUsageWrapper)
	}

	entry := NewStruct().
		SetInt(SymUniqueID, int64(eid)).
		SetSymbol(SymType, SymContainer).
		SetSymbol(SymLayout, SymVertical).
		Set(SymStyle, SymbolByName(resolved))

	// Override to the exact margin for this spacer.
	// We keep this in the style (not EmptyLineMarginTop) because KP3 stores it
	// as a real style margin-top on the spacer container.
	if styles != nil && resolved != "" {
		if def, ok := styles.Get(resolved); ok {
			props := make(map[KFXSymbol]any, len(def.Properties)+1)
			maps.Copy(props, def.Properties)
			props[SymMarginTop] = DimensionValue(marginTopLh, SymUnitLh)
			resolved = styles.RegisterResolved(props, styles.GetUsage(resolved), true)
			entry.Set(SymStyle, SymbolByName(resolved))
		}
	}

	return sb.addEntry(ContentRef{
		EID:      eid,
		Type:     SymContainer,
		Style:    resolved,
		RawEntry: entry,
	})
}
