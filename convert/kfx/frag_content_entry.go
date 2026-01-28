package kfx

// ContentRef represents a reference to content within a storyline.
type ContentRef struct {
	EID             int             // Element ID ($155)
	Type            KFXSymbol       // Content type symbol ($269=text, $270=container, $271=image, etc.)
	ContentName     string          // Name of the content fragment
	ContentOffset   int             // Offset within content fragment ($403)
	ResourceName    string          // For images: external_resource fragment id/name ($175)
	AltText         string          // For images: alt text ($584)
	RenderInline    bool            // For inline images: set render ($601) = inline ($283)
	Style           string          // Resolved style name (set by StyleSpec resolution or directly)
	StyleSpec       string          // Raw style specification for deferred resolution
	StyleEvents     []StyleEventRef // Optional inline style events ($142)
	Children        []any           // Optional nested content for containers (already converted)
	childRefs       []ContentRef    // Original child refs for deferred resolution (internal use)
	styleCtx        *StyleContext   // Style context for child resolution (set by EndBlock)
	HeadingLevel    int             // For headings: 1-6 for h1-h6 ($790), 0 means not a heading
	RawEntry        StructValue     // Pre-built entry (for complex structures like tables)
	FootnoteContent bool            // If true, adds position:footer and yj.classification:footnote markers

	// Container tracking for margin collapsing (post-processing).
	// These fields are set during content generation and used by CollapseMargins().
	ContainerID    int            // Unique container ID (0 = top-level/root)
	ParentID       int            // Parent container ID (0 = root)
	ContainerKind  ContainerKind  // Type of container (Section, Poem, Stanza, etc.)
	ContainerFlags ContainerFlags // Flags controlling collapse behavior (TitleBlock, HasBorder, etc.)
	EntryOrder     int            // Order in which this entry was added (for sibling ordering in tree)

	// Margin values for post-processing margin collapsing.
	// These are captured from the resolved style during content generation,
	// then modified by CollapseMargins() in post-processing.
	// Values are in lh units.
	//   nil = no margin (will be removed from final style)
	//   0.0 = explicit zero margin (also removed - KP3 doesn't output zero margins)
	MarginTop    *float64
	MarginBottom *float64

	// HasBreakAfterAvoid is true if the element has page-break-after: avoid (yj-break-after: avoid).
	// Elements with this property keep their margin-bottom and don't collapse with next sibling.
	HasBreakAfterAvoid bool

	// StripMarginBottom is true if this element's margin-bottom should be stripped.
	// This is set when an empty-line follows this element, matching KP3 behavior where
	// the preceding element loses its mb and the empty-line's margin goes to the next element's mt.
	StripMarginBottom bool

	// EmptyLineMarginBottom stores the empty-line margin to apply as this element's margin-bottom.
	// This is set when an empty-line is followed by an image - KP3 puts the empty-line margin
	// on the PREVIOUS element (as mb) rather than the image (as mt).
	// Applied during post-processing in applyEmptyLineMargins.
	EmptyLineMarginBottom *float64

	// EmptyLineMarginTop stores the empty-line margin to apply as this element's margin-top.
	// This is set when an empty-line precedes this element (text or other non-image content).
	// KP3 behavior: the empty-line margin goes to the next element's margin-top, but should
	// NOT be scaled by font-size (unlike CSS margins). This is applied during post-processing
	// in applyEmptyLineMargins, after captureMargins captures the style-based margins.
	EmptyLineMarginTop *float64

	// IsFloatImage marks full-width standalone block images (≥512px).
	// Float images have fixed 2.6lh margins that do NOT participate in sibling margin collapsing.
	// They act as barriers between elements - their margins stay on the image itself.
	IsFloatImage bool
}

// InlineContentItem represents either a text string or an inline image in mixed content.
// For text: Text is set, IsImage is false
// For image: IsImage is true, ResourceName/Style/AltText are set
type InlineContentItem struct {
	Text         string // Raw text string (for text items)
	IsImage      bool   // True if this is an inline image
	ResourceName string // For images: external resource name
	Style        string // For images: resolved style name
	AltText      string // For images: alt text
}

// StyleEventRef represents a style event for inline formatting ($142).
type StyleEventRef struct {
	Offset         int    // $143 - start offset
	Length         int    // $144 - length
	Style          string // $157 - style name
	LinkTo         string // $179 - link target (internal anchor ID or external link anchor ID)
	IsFootnoteLink bool   // If true, adds $616: $617 (yj.display: yj.note)
}

// NewContentEntry creates a content entry for storyline's $146.
// Based on reference: {$155: eid, $157: style, $159: type, $145: {name: content_X, $403: offset}}
func NewContentEntry(ref ContentRef) StructValue {
	// If a pre-built entry is provided, use it directly
	if ref.RawEntry != nil {
		return ref.RawEntry
	}

	entry := NewStruct().
		SetInt(SymUniqueID, int64(ref.EID)). // $155 = id
		SetSymbol(SymType, ref.Type)         // $159 = type

	if ref.Style != "" {
		entry.Set(SymStyle, SymbolByName(ref.Style)) // $157 = style as symbol
	}

	// Heading level for h1-h6 elements
	if ref.HeadingLevel > 0 {
		entry.SetInt(SymYjHeadingLevel, int64(ref.HeadingLevel)) // $790 = yj.semantics.heading_level
	}

	// Style events for inline formatting
	if len(ref.StyleEvents) > 0 {
		events := make([]any, 0, len(ref.StyleEvents))
		for _, se := range ref.StyleEvents {
			ev := NewStruct().
				SetInt(SymOffset, int64(se.Offset)). // $143 = offset
				SetInt(SymLength, int64(se.Length))  // $144 = length
			if se.Style != "" {
				ev.Set(SymStyle, SymbolByName(se.Style)) // $157 = style
			}
			if se.LinkTo != "" {
				ev.Set(SymLinkTo, SymbolByName(se.LinkTo)) // $179 = link_to
			}
			if se.IsFootnoteLink {
				ev.SetSymbol(SymYjDisplay, SymYjNote) // $616 = $617 (yj.display = yj.note)
			}
			events = append(events, ev)
		}
		entry.SetList(SymStyleEvents, events) // $142 = style_events
	}

	if ref.Type == SymImage {
		entry.Set(SymResourceName, SymbolByName(ref.ResourceName)) // $175 = resource_name (symbol reference)
		entry.SetString(SymAltText, ref.AltText)                   // $584 = alt_text (always include, even if empty)
		if ref.RenderInline {
			entry.SetSymbol(SymRender, SymInline) // $601 = inline ($283) for inline images
		}
	} else if ref.ContentName != "" {
		// Content reference - nested struct with name and offset
		// Only add if we have a content name (containers with children don't have content)
		contentRef := map[string]any{
			"name": SymbolByName(ref.ContentName),
			"$403": ref.ContentOffset,
		}
		entry.Set(SymContent, contentRef) // $145 = content
	}

	// Nested children for containers
	if len(ref.Children) > 0 {
		entry.SetList(SymContentList, ref.Children) // $146 = content_list
	}

	// Footnote content markers: position: footer, yj.classification: footnote
	// These identify the first paragraph of footnote content for Kindle's footnote rendering
	if ref.FootnoteContent {
		entry.SetSymbol(SymPosition, SymFooter)           // $183 = $455 (position = footer)
		entry.SetSymbol(SymYjClassification, SymFootnote) // $615 = $281 (yj.classification = footnote)
	}

	return entry
}

// collectStructEIDs recursively extracts EIDs from a StructValue and its nested content.
// For mixed content (text type with content_list containing raw strings), we include both
// the parent EID and all inline image EIDs, since KP3 includes them in position_map.
// For wrapper-style content_list (all items are StructValue), we recurse into children.
// Table structural types and cell containers are INCLUDED in position_map with length=1.
func collectStructEIDs(sv StructValue) []int {
	var eids []int

	// Get this struct's EID (include all types, including table structure)
	if eid, exists := sv[SymUniqueID]; exists {
		if eidInt, ok := eid.(int64); ok {
			eids = append(eids, int(eidInt))
		}
	}

	// Check for content_list
	contentList, hasContentList := sv[SymContentList].([]any)
	if !hasContentList || len(contentList) == 0 {
		return eids
	}

	// Determine if this is mixed content (has raw strings) or wrapper-style (all StructValue)
	// Mixed content: content_list has strings interleaved with inline image entries
	// Wrapper-style: content_list has only StructValue children (like title blocks)
	hasMixedContent := false
	for _, child := range contentList {
		if _, isString := child.(string); isString {
			hasMixedContent = true
			break
		}
	}

	if hasMixedContent {
		// Mixed content: include inline image EIDs for position_map
		// KP3 includes these EIDs and generates granular position entries for them
		for _, child := range contentList {
			if childSV, ok := child.(StructValue); ok {
				// Check if this is an inline image
				if childEID, ok := childSV[SymUniqueID].(int64); ok {
					if childType, ok := childSV[SymType].(SymbolValue); ok && KFXSymbol(childType) == SymImage {
						eids = append(eids, int(childEID))
					}
				}
			}
		}
		return eids
	}

	// Wrapper-style: recurse into all StructValue children
	for _, child := range contentList {
		if childSV, ok := child.(StructValue); ok {
			eids = append(eids, collectStructEIDs(childSV)...)
		}
	}

	return eids
}

// collectChildEIDs extracts EIDs from nested content entries.
func collectChildEIDs(children []any) []int {
	if len(children) == 0 {
		return nil
	}
	eids := make([]int, 0, len(children))
	for _, child := range children {
		if sv, ok := child.(StructValue); ok {
			eids = append(eids, collectStructEIDs(sv)...)
		}
	}
	return eids
}
