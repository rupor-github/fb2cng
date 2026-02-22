package kfx

import (
	"fmt"
	"strings"

	"fbc/content"
	"fbc/fb2"
)

// AddTable adds a table with proper KFX structure.
// Structure: table($278) -> body($454) -> rows($279) -> cells($270) -> text($269)
// Cell containers have border/padding/vertical-align styles.
// Text inside cells has text-align style and style_events for inline formatting.
// Image-only cells contain image elements directly.
// The idToEID map is used to register backlink RefIDs for footnote references in cells.
func (sb *StorylineBuilder) AddTable(c *content.Content, table *fb2.Table, styles *StyleRegistry, ca *ContentAccumulator, imageResources imageResourceInfoByID, idToEID eidByFB2ID) int {
	tableEID := sb.eidCounter
	sb.eidCounter++

	// Build rows
	var rowEntries []any
	for _, row := range table.Rows {
		rowEID := sb.eidCounter
		sb.eidCounter++

		// Build cells for this row
		var cellEntries []any
		for _, cell := range row.Cells {
			cellEID := sb.eidCounter
			sb.eidCounter++

			// Check if cell contains text and/or images
			hasText := cellContentHasText(cell.Content)
			cellImages := extractCellImages(cell.Content)
			hasImages := len(cellImages) > 0

			// Determine container style (border/padding/vertical-align)
			containerStyle := NewStyleContext(styles).Resolve("", "td-container")
			if cell.Header {
				containerStyle = NewStyleContext(styles).Resolve("", "th-container")
			}
			styles.ResolveStyle(containerStyle, styleUsageWrapper)

			// Determine ancestor tag and text style based on header flag
			ancestorTag := "td"
			textStyleBase := "td-text"
			if cell.Header {
				ancestorTag = "th"
				textStyleBase = "th-text"
			}

			// Determine text style with alignment
			textStyle := textStyleBase
			if cell.Align != "" {
				prefix := "td"
				if cell.Header {
					prefix = "th"
				}
				switch cell.Align {
				case "center":
					textStyle = prefix + "-text-center"
				case "right":
					textStyle = prefix + "-text-right"
				case "left":
					textStyle = prefix + "-text-left"
				case "justify":
					textStyle = prefix + "-text-justify"
				}
			}
			resolvedTextStyle := NewStyleContext(styles).Resolve("", textStyle)

			var contentList []any

			if hasImages && !hasText {
				// Image-only cell: create image entries directly
				contentList = sb.buildImageOnlyCellContent(cell, cellImages, imageResources, styles)
			} else if hasImages && hasText {
				// Mixed content cell: use content_list format with interleaved text and images
				contentList = sb.buildMixedCellContent(c, cell, imageResources, styles, ancestorTag, resolvedTextStyle, idToEID)
			} else {
				// Text-only cell: create text entry with content reference
				contentList = sb.buildTextOnlyCellContent(c, cell, ca, styles, ancestorTag, resolvedTextStyle, idToEID)
			}

			// Create cell container with nested content
			cellEntry := NewStruct().
				SetInt(SymUniqueID, int64(cellEID)).
				SetSymbol(SymType, SymContainer).            // $270
				SetSymbol(SymLayout, SymVertical).           // $156 = $323 (vertical)
				Set(SymStyle, SymbolByName(containerStyle)). // Container style with border/padding
				SetList(SymContentList, contentList)         // Nested content (text or images)

			// Add colspan/rowspan if specified
			if cell.ColSpan > 1 {
				cellEntry.SetInt(SymTableColSpan, int64(cell.ColSpan))
			}
			if cell.RowSpan > 1 {
				cellEntry.SetInt(SymTableRowSpan, int64(cell.RowSpan))
			}

			cellEntries = append(cellEntries, cellEntry)
		}

		// Create row entry
		rowEntry := NewStruct().
			SetInt(SymUniqueID, int64(rowEID)).
			SetSymbol(SymType, SymTableRow). // $279
			SetList(SymContentList, cellEntries)

		rowEntries = append(rowEntries, rowEntry)
	}

	// Create body wrapper
	bodyEID := sb.eidCounter
	sb.eidCounter++
	bodyEntry := NewStruct().
		SetInt(SymUniqueID, int64(bodyEID)).
		SetSymbol(SymType, SymTableBody). // $454
		SetList(SymContentList, rowEntries)

	// Create table entry with proper structure
	// Amazon reference: table has yj.table_features=[pan_zoom, scale_fit] which enables
	// table scaling to fit within page bounds instead of spanning multiple pages.
	tableStyle := NewStyleContext(styles).Resolve("", "table")
	styles.tracer.TraceAssign(traceSymbolName(SymTable), fmt.Sprintf("%d", tableEID), tableStyle, sb.sectionName+"/"+sb.name, "table")
	styles.ResolveStyle(tableStyle, styleUsageWrapper)

	// Get table element properties from CSS (KP3 moves these from style to element)
	tableProps := styles.GetTableElementProps()

	// Create table features list: [pan_zoom, scale_fit]
	// scale_fit ($326) enables table scaling to fit the page
	tableFeatures := []any{
		SymbolValue(SymPanZoom),  // $581
		SymbolValue(SymScaleFit), // $326
	}

	tableEntry := NewStruct().
		SetInt(SymUniqueID, int64(tableEID)).
		SetSymbol(SymType, SymTable). // $278
		Set(SymStyle, SymbolByName(tableStyle)).
		SetBool(SymTableBorderCollapse, tableProps.BorderCollapse). // $150
		Set(SymBorderSpacingVertical, tableProps.BorderSpacingV).   // $456
		Set(SymBorderSpacingHorizontal, tableProps.BorderSpacingH). // $457
		SetList(SymYjTableFeatures, tableFeatures).                 // $629 = [pan_zoom, scale_fit]
		SetSymbol(SymYjTableSelectionMode, SymYjRegional).          // $630 = yj.regional
		SetList(SymContentList, []any{bodyEntry})

	// Add to storyline via addEntry to properly handle:
	// - Container tracking for margin collapsing
	// - Consuming pending empty-line margin (so it goes to table, not next element)
	// - Entry ordering for correct sibling positioning
	sb.addEntry(ContentRef{
		EID:      tableEID,
		Type:     SymTable,
		Style:    tableStyle,
		RawEntry: tableEntry,
	})

	return tableEID
}

// buildImageOnlyCellContent creates content entries for a cell containing only images.
func (sb *StorylineBuilder) buildImageOnlyCellContent(cell fb2.TableCell, cellImages []string, imageResources imageResourceInfoByID, styles *StyleRegistry) []any {
	var contentList []any

	for _, imgID := range cellImages {
		imgInfo, ok := imageResources[imgID]
		if !ok {
			continue
		}

		imgEID := sb.eidCounter
		sb.eidCounter++

		// Determine image style based on header flag and alignment
		imgStyleBase := "td-image"
		if cell.Header {
			imgStyleBase = "th-image"
		}
		if cell.Align != "" {
			prefix := "td-image"
			if cell.Header {
				prefix = "th-image"
			}
			switch cell.Align {
			case "center":
				imgStyleBase = prefix + "-center"
			case "right":
				imgStyleBase = prefix + "-right"
			case "left":
				imgStyleBase = prefix + "-left"
			}
		}
		imgStyle := NewStyleContext(styles).ResolveImage(imgStyleBase)
		styles.ResolveStyle(imgStyle, styleUsageImage)

		// Get alt text from the original segment
		altText := ""
		for _, seg := range cell.Content {
			if seg.Kind == fb2.InlineImageSegment && seg.Image != nil {
				imgHref := strings.TrimPrefix(seg.Image.Href, "#")
				if imgHref == imgID {
					altText = seg.Image.Alt
					break
				}
			}
		}

		imgEntry := NewStruct().
			SetInt(SymUniqueID, int64(imgEID)).
			SetSymbol(SymType, SymImage).
			Set(SymStyle, SymbolByName(imgStyle)).
			Set(SymResourceName, SymbolByName(imgInfo.ResourceName)).
			SetString(SymAltText, altText)

		contentList = append(contentList, imgEntry)
	}

	return contentList
}

// buildTextOnlyCellContent creates a text entry for a cell containing only text.
func (sb *StorylineBuilder) buildTextOnlyCellContent(c *content.Content, cell fb2.TableCell, ca *ContentAccumulator, styles *StyleRegistry, ancestorTag, resolvedTextStyle string, idToEID eidByFB2ID) []any {
	// Create inline style context for table cell content.
	// This ensures inline styles inherit properties from the cell context.
	inlineCtx := NewStyleContext(styles).Push(ancestorTag, "")

	// Process cell content using shared inline segment processing
	nw := newNormalizingWriter()
	result := processInlineSegments(c, cell.Content, nw, styles, inlineCtx)

	text := nw.String()
	contentName, offset := ca.Add(text)

	styles.ResolveStyle(resolvedTextStyle, styleUsageText)

	// Segment and deduplicate style events
	segmentedEvents := SegmentNestedStyleEvents(result.Events)
	for _, ev := range segmentedEvents {
		styles.ResolveStyle(ev.Style, styleUsageText)
	}

	// Create text entry inside cell
	textEID := sb.eidCounter
	sb.eidCounter++

	// Register backlink RefIDs with this text EID so backlink paragraphs can link back
	for _, ref := range result.BacklinkRefIDs {
		if _, exists := idToEID[ref.RefID]; !exists {
			idToEID[ref.RefID] = anchorTarget{EID: textEID, Offset: ref.Offset}
		}
	}

	textEntry := NewStruct().
		SetInt(SymUniqueID, int64(textEID)).
		SetSymbol(SymType, SymText).
		Set(SymStyle, SymbolByName(resolvedTextStyle))

	// Add content reference
	contentRef := map[string]any{
		"name": SymbolByName(contentName),
		"$403": offset,
	}
	textEntry.Set(SymContent, contentRef)

	// Add style events for inline formatting (bold, italic, etc.)
	if len(segmentedEvents) > 0 {
		eventList := make([]any, 0, len(segmentedEvents))
		for _, se := range segmentedEvents {
			event := NewStruct().
				SetInt(SymOffset, int64(se.Offset)).
				SetInt(SymLength, int64(se.Length)).
				Set(SymStyle, SymbolByName(se.Style))
			if se.LinkTo != "" {
				event.Set(SymLinkTo, SymbolByName(se.LinkTo))
			}
			if se.IsFootnoteLink {
				event.SetSymbol(SymYjDisplay, SymYjNote)
			}
			eventList = append(eventList, event)
		}
		textEntry.SetList(SymStyleEvents, eventList)
	}

	return []any{textEntry}
}

// buildMixedCellContent creates a text entry with content_list for mixed content cells.
// This uses the same structure as AddMixedContent: interleaved text strings and inline images.
func (sb *StorylineBuilder) buildMixedCellContent(c *content.Content, cell fb2.TableCell, imageResources imageResourceInfoByID, styles *StyleRegistry, ancestorTag, resolvedTextStyle string, idToEID eidByFB2ID) []any {
	// Create inline style context for table cell content.
	// This ensures inline styles inherit properties from the cell context.
	inlineCtx := NewStyleContext(styles).Push(ancestorTag, "")

	// Process cell content using shared mixed content processing
	result := processMixedInlineSegments(cell.Content, styles, c, inlineCtx, imageResources)

	styles.ResolveStyle(resolvedTextStyle, styleUsageText)

	// Segment and deduplicate style events
	segmentedEvents := SegmentNestedStyleEvents(result.Events)
	for _, ev := range segmentedEvents {
		styles.ResolveStyle(ev.Style, styleUsageText)
	}

	// Create text entry with content_list (similar to AddMixedContent)
	textEID := sb.eidCounter
	sb.eidCounter++

	// Register backlink RefIDs with this text EID so backlink paragraphs can link back
	for _, ref := range result.BacklinkRefIDs {
		if _, exists := idToEID[ref.RefID]; !exists {
			idToEID[ref.RefID] = anchorTarget{EID: textEID, Offset: ref.Offset}
		}
	}

	// Build content_list array with text strings and image entries
	contentListItems := make([]any, 0, len(result.Items))
	for _, item := range result.Items {
		if item.IsImage {
			// Create inline image entry
			imgEid := sb.eidCounter
			sb.eidCounter++

			if item.Style != "" {
				styles.tracer.TraceAssign(traceSymbolName(SymImage)+" (inline/table)", fmt.Sprintf("%d", imgEid), item.Style, sb.sectionName+"/"+sb.name, "")
				styles.ResolveStyle(item.Style, styleUsageImage)
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

			contentListItems = append(contentListItems, imgEntry)
		} else {
			// Raw text string
			contentListItems = append(contentListItems, item.Text)
		}
	}

	// Build the entry with content_list
	textEntry := NewStruct().
		SetInt(SymUniqueID, int64(textEID)).
		SetSymbol(SymType, SymText).
		SetList(SymContentList, contentListItems).
		Set(SymStyle, SymbolByName(resolvedTextStyle))

	// Add style events if present
	if len(segmentedEvents) > 0 {
		eventList := make([]any, 0, len(segmentedEvents))
		for _, se := range segmentedEvents {
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
		textEntry.SetList(SymStyleEvents, eventList)
	}

	return []any{textEntry}
}
