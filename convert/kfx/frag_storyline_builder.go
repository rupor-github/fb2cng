package kfx

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"fbc/fb2"
)

// BuildStoryline creates a $259 storyline fragment.
// Based on reference KFX, storyline has:
// - Named FID (like "l1", "l2", etc.) - simple decimal format for readability
// - $176 (story_name) as symbol reference
// - $146 (content_list) with content entries
//
// Naming pattern: "l{N}" where N is sequential (e.g., l1, l2, l3).
// Uses simple format instead of base36 for better human readability during debugging.
func BuildStoryline(storyName string, contentEntries []any) *Fragment {
	storyline := NewStruct().
		Set(SymStoryName, SymbolByName(storyName)) // $176 = story_name as symbol

	if len(contentEntries) > 0 {
		storyline.SetList(SymContentList, contentEntries) // $146 = content_list
	}

	return &Fragment{
		FType:   SymStoryline,
		FIDName: storyName,
		Value:   storyline,
	}
}

// BuildSection creates a $260 section fragment.
// Based on reference KFX, section has:
// - Named FID (like "c0", "c1", etc.) - simple decimal format for readability
// - $174 (section_name) as symbol reference
// - $141 (page_templates) with layout entries for storylines
//
// Naming pattern: "c{N}" where N is sequential starting from 0 (e.g., c0, c1, c2).
// Uses simple format instead of base36 for better human readability during debugging.
func BuildSection(sectionName string, pageTemplates []any) *Fragment {
	section := NewStruct().
		Set(SymSectionName, SymbolByName(sectionName)) // $174 = section_name as symbol

	if len(pageTemplates) > 0 {
		section.SetList(SymPageTemplates, pageTemplates) // $141 = page_templates
	}

	return &Fragment{
		FType:   SymSection,
		FIDName: sectionName,
		Value:   section,
	}
}

// NewPageTemplateEntry creates a page template entry for section's $141.
// Based on KPV reference: {$155: eid, $159: $269, $176: storyline_name}
// The page template simply references the storyline by name and uses text type.
func NewPageTemplateEntry(eid int, storylineName string) StructValue {
	return NewStruct().
		SetInt(SymUniqueID, int64(eid)).               // $155 = id
		SetSymbol(SymType, SymText).                   // $159 = type = $269 (text)
		Set(SymStoryName, SymbolByName(storylineName)) // $176 = story_name ref
}

// NewCoverPageTemplateEntry creates a page template entry for a cover section.
// Based on reference KFX cover section: {$140: $320, $155: eid, $156: $326, $159: $270, $176: storyline, $66: width, $67: height}
// The layout mode ($156) uses scale_fit ($326) which preserves aspect ratio.
// Note: KFX doesn't have a direct equivalent to EPUB's "stretch" mode, so scale_fit is used for all modes.
func NewCoverPageTemplateEntry(eid int, storylineName string, width, height int) StructValue {
	return NewStruct().
		SetSymbol(SymFloat, SymCenter).                 // $140 = center ($320)
		SetInt(SymUniqueID, int64(eid)).                // $155 = id
		SetSymbol(SymLayout, SymScaleFit).              // $156 = scale_fit ($326)
		SetSymbol(SymType, SymContainer).               // $159 = container ($270)
		Set(SymStoryName, SymbolByName(storylineName)). // $176 = story_name ref
		SetInt(SymContainerWidth, int64(width)).        // $66 = container width
		SetInt(SymContainerHeight, int64(height))       // $67 = container height
}

// ContentRef represents a reference to content within a storyline.
type ContentRef struct {
	EID           int             // Element ID ($155)
	Type          KFXSymbol       // Content type symbol ($269=text, $270=container, $271=image, etc.)
	ContentName   string          // Name of the content fragment
	ContentOffset int             // Offset within content fragment ($403)
	ResourceName  string          // For images: external_resource fragment id/name ($175)
	AltText       string          // For images: alt text ($584)
	RenderInline  bool            // For inline images: set render ($601) = inline ($283)
	Style         string          // Resolved style name (set by StyleSpec resolution or directly)
	StyleSpec     string          // Raw style specification for deferred resolution
	StyleEvents   []StyleEventRef // Optional inline style events ($142)
	Children      []any           // Optional nested content for containers
	HeadingLevel  int             // For headings: 1-6 for h1-h6 ($790), 0 means not a heading
	RawEntry      StructValue     // Pre-built entry (for complex structures like tables)
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

// SegmentStyleEvents takes inline style events and a base style, and returns
// non-overlapping events. The base style fills gaps between inline events.
// This matches KP3 behavior which throws "Cannot create Overlapping Style Events"
// if overlaps are detected.
//
// For example, if text is "Hello World Link Here" (21 chars) with:
//   - inline event at offset=12, len=4 (link style)
//   - base style covering the whole text
//
// Instead of overlapping events, we produce:
//   - offset=0, len=12, base style (before link)
//   - offset=12, len=4, link style
//   - offset=16, len=5, base style (after link)
//
// Events are returned sorted by offset ascending, then length ascending.
func SegmentStyleEvents(inlineEvents []StyleEventRef, baseStyle string, totalLength int) []StyleEventRef {
	if totalLength <= 0 {
		return nil
	}

	// If no inline events, just return base style covering everything
	if len(inlineEvents) == 0 {
		if baseStyle == "" {
			return nil
		}
		return []StyleEventRef{{Offset: 0, Length: totalLength, Style: baseStyle}}
	}

	// Sort inline events by offset, then by length (shorter first at same offset)
	sorted := make([]StyleEventRef, len(inlineEvents))
	copy(sorted, inlineEvents)
	slices.SortFunc(sorted, func(a, b StyleEventRef) int {
		if c := cmp.Compare(a.Offset, b.Offset); c != 0 {
			return c
		}
		return cmp.Compare(a.Length, b.Length)
	})

	// Build segmented events list
	var result []StyleEventRef
	pos := 0

	for _, ev := range sorted {
		// Skip events that start before current position (shouldn't happen with proper input)
		if ev.Offset < pos {
			continue
		}

		// Fill gap before this event with base style
		if baseStyle != "" && ev.Offset > pos {
			result = append(result, StyleEventRef{
				Offset: pos,
				Length: ev.Offset - pos,
				Style:  baseStyle,
			})
		}

		// Add the inline event
		result = append(result, ev)
		pos = ev.Offset + ev.Length
	}

	// Fill remaining gap after last event with base style
	if baseStyle != "" && pos < totalLength {
		result = append(result, StyleEventRef{
			Offset: pos,
			Length: totalLength - pos,
			Style:  baseStyle,
		})
	}

	return result
}

// SegmentNestedStyleEvents takes a list of possibly overlapping style events
// (as produced by recursive inline walks) and returns non-overlapping events.
//
// The algorithm:
// 1. Deduplicate events at same offset+length (keep most specific)
// 2. For each position, determine which event should be active (shortest/most-specific wins)
// 3. Generate non-overlapping segments based on these active events
//
// This handles complex cases like <code>text <a>link</a> more</code> where:
//   - code event covers [0-14]
//   - link event covers [5-9]
//
// Result (non-overlapping):
//   - [0-5] code style
//   - [5-9] link style (already has code properties merged)
//   - [9-14] code style
//
// For events with identical offset and length (e.g., from <a><sup>text</sup></a>),
// only the event with the longest style name (most merged styles) or LinkTo is kept.
//
// Events are returned sorted by offset ascending.
func SegmentNestedStyleEvents(events []StyleEventRef) []StyleEventRef {
	if len(events) == 0 {
		return nil
	}
	if len(events) == 1 {
		return events
	}

	// First, deduplicate events with identical offset+length by keeping the one
	// with the most specific (longest) style name or with LinkTo.
	type posKey struct {
		offset, length int
	}
	bestByPos := make(map[posKey]StyleEventRef)
	for _, ev := range events {
		key := posKey{ev.Offset, ev.Length}
		if existing, ok := bestByPos[key]; ok {
			// Keep the one with LinkTo or longer style name (more merged styles)
			keepNew := false
			if ev.LinkTo != "" && existing.LinkTo == "" {
				keepNew = true
			} else if ev.LinkTo == existing.LinkTo && len(ev.Style) > len(existing.Style) {
				keepNew = true
			}
			if keepNew {
				bestByPos[key] = ev
			}
		} else {
			bestByPos[key] = ev
		}
	}

	// Convert back to slice
	deduped := make([]StyleEventRef, 0, len(bestByPos))
	for _, ev := range bestByPos {
		deduped = append(deduped, ev)
	}

	if len(deduped) == 1 {
		return deduped
	}

	// Collect all unique boundary points (starts and ends of events)
	pointSet := make(map[int]struct{})
	for _, ev := range deduped {
		pointSet[ev.Offset] = struct{}{}
		pointSet[ev.Offset+ev.Length] = struct{}{}
	}
	points := make([]int, 0, len(pointSet))
	for p := range pointSet {
		points = append(points, p)
	}
	slices.Sort(points)

	// For each segment between consecutive points, find the most specific
	// (shortest) event that covers it. Shorter events are more specific because
	// they represent inner/nested elements with merged styles.
	var result []StyleEventRef
	for i := 0; i < len(points)-1; i++ {
		segStart := points[i]
		segEnd := points[i+1]
		if segEnd <= segStart {
			continue
		}

		// Find the shortest event that fully covers this segment
		var bestEvent *StyleEventRef
		bestLength := int(^uint(0) >> 1) // max int

		for j := range deduped {
			ev := &deduped[j]
			evEnd := ev.Offset + ev.Length
			// Event covers segment if ev.Offset <= segStart && evEnd >= segEnd
			if ev.Offset <= segStart && evEnd >= segEnd {
				if ev.Length < bestLength {
					bestLength = ev.Length
					bestEvent = ev
				} else if ev.Length == bestLength {
					// Tie-breaker: prefer event with LinkTo or longer style name
					if ev.LinkTo != "" && bestEvent.LinkTo == "" {
						bestEvent = ev
					} else if len(ev.Style) > len(bestEvent.Style) {
						bestEvent = ev
					}
				}
			}
		}

		if bestEvent != nil {
			// Create or extend a segment with this style
			seg := StyleEventRef{
				Offset:         segStart,
				Length:         segEnd - segStart,
				Style:          bestEvent.Style,
				LinkTo:         bestEvent.LinkTo,
				IsFootnoteLink: bestEvent.IsFootnoteLink,
			}

			// Try to merge with previous segment if same style and adjacent
			if len(result) > 0 {
				prev := &result[len(result)-1]
				if prev.Style == seg.Style &&
					prev.LinkTo == seg.LinkTo &&
					prev.IsFootnoteLink == seg.IsFootnoteLink &&
					prev.Offset+prev.Length == seg.Offset {
					// Extend previous segment
					prev.Length += seg.Length
					continue
				}
			}
			result = append(result, seg)
		}
	}

	return result
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
		if ref.AltText != "" {
			entry.SetString(SymAltText, ref.AltText) // $584 = alt_text (only if non-empty)
		}
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

	return entry
}

// StorylineBuilder helps build storyline content incrementally.
type StorylineBuilder struct {
	name            string // Storyline name (e.g., "l1")
	sectionName     string // Associated section name (e.g., "c0")
	styles          *StyleRegistry
	contentEntries  []ContentRef
	eidCounter      int
	pageTemplateEID int // Separate EID for page template container

	// Block wrapper support - stack allows nested wrappers.
	blockStack []*BlockBuilder
}

// BlockBuilder collects content entries for a wrapper/container element.
// It mirrors how EPUB generates <div class="..."> wrappers.
//
// Style resolution is deferred until EndBlock() to enable position-aware resolution.
// This allows KP3-compatible margin filtering where first elements lose margin-top
// and non-first elements lose margin-bottom (CSS margin collapsing).
type BlockBuilder struct {
	styleSpec        string          // Raw style specification (e.g., "poem", "cite") - resolved in EndBlock
	styles           *StyleRegistry  // Style registry for deferred resolution
	eid              int             // EID for the wrapper container
	children         []ContentRef    // Nested content entries (styles resolved in EndBlock)
	position         ElementPosition // This block's position within its parent container
	childrenPosAware bool            // If true, resolve child styles with position filtering
}

// AllEIDs returns all EIDs used by this section (page template + content entries).
// For wrapper containers (entries with Children), wrapper EID comes first in DFS order,
// followed by all child EIDs - this matches how position_id_map is validated.
func (sb *StorylineBuilder) AllEIDs() []int {
	eids := make([]int, 0, len(sb.contentEntries)+1)
	eids = append(eids, sb.pageTemplateEID)
	for _, ref := range sb.contentEntries {
		if ref.RawEntry != nil {
			// Pre-built entry (e.g., table): recursively collect all EIDs
			eids = append(eids, collectStructEIDs(ref.RawEntry)...)
		} else if len(ref.Children) > 0 {
			// Wrapper container: include wrapper EID first, then child EIDs
			eids = append(eids, ref.EID)
			eids = append(eids, collectChildEIDs(ref.Children)...)
		} else {
			// Regular content: include the entry's EID
			eids = append(eids, ref.EID)
		}
	}
	return eids
}

// collectStructEIDs recursively extracts EIDs from a StructValue and its nested content.
// For mixed content (text type with content_list containing raw strings), we include both
// the parent EID and all inline image EIDs, since KP3 includes them in position_map.
// For wrapper-style content_list (all items are StructValue), we recurse into children.
func collectStructEIDs(sv StructValue) []int {
	var eids []int

	// Get this struct's EID
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
					if typeSym, ok := childSV[SymType].(SymbolValue); ok && KFXSymbol(typeSym) == SymImage {
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

// NewStorylineBuilder creates a new storyline builder.
// Allocates the first EID for the page template container.
func NewStorylineBuilder(storyName, sectionName string, startEID int, styles *StyleRegistry) *StorylineBuilder {
	return &StorylineBuilder{
		name:            storyName,
		sectionName:     sectionName,
		styles:          styles,
		pageTemplateEID: startEID,     // First EID goes to page template
		eidCounter:      startEID + 1, // Content EIDs start after page template
	}
}

// StartBlock begins a new wrapper/container block with position-aware styling.
// The position parameter specifies where this block sits within its parent container,
// which affects style resolution (e.g., first elements lose margin-top, non-first lose margin-bottom).
// All content added until EndBlock is called will be nested inside this wrapper.
// The styleSpec is the raw style name (e.g., "poem", "cite") - resolution is deferred
// until EndBlock to avoid registering styles for empty wrappers.
// Returns the EID of the wrapper for reference.
func (sb *StorylineBuilder) StartBlock(styleSpec string, styles *StyleRegistry, pos ElementPosition) int {
	return sb.startBlockInternal(styleSpec, styles, pos, false)
}

// StartBlockWithChildPositions begins a wrapper block where child elements also get position-aware styling.
// This is used for containers where children should have margin filtering based on their position
// within this block (not just the block itself).
func (sb *StorylineBuilder) StartBlockWithChildPositions(styleSpec string, styles *StyleRegistry, pos ElementPosition) int {
	return sb.startBlockInternal(styleSpec, styles, pos, true)
}

// startBlockInternal is the internal implementation for starting blocks.
func (sb *StorylineBuilder) startBlockInternal(styleSpec string, styles *StyleRegistry, pos ElementPosition, childrenPosAware bool) int {
	eid := sb.eidCounter
	sb.eidCounter++

	sb.blockStack = append(sb.blockStack, &BlockBuilder{
		styleSpec:        styleSpec,
		styles:           styles,
		eid:              eid,
		children:         make([]ContentRef, 0),
		position:         pos,
		childrenPosAware: childrenPosAware,
	})

	return eid
}

// EndBlock closes the current wrapper block and adds it to the storyline.
// The wrapper becomes a container entry with nested children.
// Empty wrappers (with no children) are discarded to avoid position_map validation errors.
// Style resolution is deferred until here to prevent registering styles for discarded wrappers.
//
// The wrapper's style is resolved with its position context (set in StartBlock), so different
// positions produce different styles (e.g., first section keeps margin-bottom, others don't).
// If childrenPosAware was set, child styles are also resolved with position filtering.
func (sb *StorylineBuilder) EndBlock() {
	if len(sb.blockStack) == 0 {
		return
	}

	block := sb.blockStack[len(sb.blockStack)-1]
	sb.blockStack = sb.blockStack[:len(sb.blockStack)-1]

	// Skip empty wrappers - they have no content and cause position_map validation errors
	if len(block.children) == 0 {
		return
	}

	// Resolve wrapper style with position context
	// This produces different styles for first vs non-first elements (CSS margin collapsing)
	resolvedStyle := ""
	if block.styles != nil && block.styleSpec != "" {
		resolvedStyle = block.styles.ResolveStyleWithPosition(block.styleSpec, block.position)
		block.styles.MarkUsage(resolvedStyle, styleUsageWrapper)
	}

	// Resolve child styles with position awareness if enabled
	if block.childrenPosAware && block.styles != nil {
		block.resolveChildStylesWithPosition()
	}

	// Convert children to content entries for the $146 list
	children := make([]any, 0, len(block.children))
	for _, child := range block.children {
		children = append(children, NewContentEntry(child))
	}

	if resolvedStyle != "" {
		block.styles.tracer.TraceAssign("wrapper", fmt.Sprintf("%d", block.eid), resolvedStyle, sb.sectionName+"/"+sb.name)
	}

	wrapperRef := ContentRef{
		EID:      block.eid,
		Type:     SymText, // Container wrappers use $269 (text) type in KFX
		Style:    resolvedStyle,
		Children: children,
	}
	if len(sb.blockStack) > 0 {
		sb.blockStack[len(sb.blockStack)-1].children = append(sb.blockStack[len(sb.blockStack)-1].children, wrapperRef)
	} else {
		sb.contentEntries = append(sb.contentEntries, wrapperRef)
	}
}

// resolveChildStylesWithPosition resolves StyleSpec fields to Style with position filtering.
// This implements KP3's position-aware CSS processing where first/last elements in a block
// have margins/padding/break properties filtered out.
func (bb *BlockBuilder) resolveChildStylesWithPosition() {
	count := len(bb.children)
	for i := range bb.children {
		child := &bb.children[i]

		// Only resolve if StyleSpec is set and Style is not already resolved
		if child.StyleSpec == "" || child.Style != "" {
			continue
		}

		// Calculate position for this child
		pos := PositionFromIndex(i, count)

		// Resolve style with position filtering
		child.Style = bb.styles.ResolveStyleWithPosition(child.StyleSpec, pos)

		// Mark usage based on content type
		usage := styleUsageText
		if child.Type == SymImage {
			usage = styleUsageImage
		}
		bb.styles.MarkUsage(child.Style, usage)
	}
}

// addEntry is the internal method that routes content to the appropriate destination.
func (sb *StorylineBuilder) addEntry(ref ContentRef) int {
	if len(sb.blockStack) > 0 {
		// Add to current block's children
		sb.blockStack[len(sb.blockStack)-1].children = append(sb.blockStack[len(sb.blockStack)-1].children, ref)
	} else {
		// Add directly to storyline
		sb.contentEntries = append(sb.contentEntries, ref)
	}
	return ref.EID
}

// AddContent adds a content reference to the storyline (or current block).
func (sb *StorylineBuilder) AddContent(contentType KFXSymbol, contentName string, contentOffset int, style string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(contentType), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name)
		sb.styles.MarkUsage(style, styleUsageText)
	}

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		Style:         style,
	})
}

// AddContentAndEvents adds content with style events (to storyline or current block).
func (sb *StorylineBuilder) AddContentAndEvents(contentType KFXSymbol, contentName string, contentOffset int, style string, events []StyleEventRef) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(contentType), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name)
		sb.styles.MarkUsage(style, styleUsageText)
	}

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		Style:         style,
		StyleEvents:   events,
	})
}

// AddContentWithHeading adds content with style events and heading level (to storyline or current block).
func (sb *StorylineBuilder) AddContentWithHeading(contentType KFXSymbol, contentName string, contentOffset int, style string, events []StyleEventRef, headingLevel int) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(contentType), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name)
		sb.styles.MarkUsage(style, styleUsageText)
	}

	return sb.addEntry(ContentRef{
		EID:           eid,
		Type:          contentType,
		ContentName:   contentName,
		ContentOffset: contentOffset,
		Style:         style,
		StyleEvents:   events,
		HeadingLevel:  headingLevel,
	})
}

// AddImage adds an image reference (to storyline or current block).
func (sb *StorylineBuilder) AddImage(resourceName, style, altText string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(SymImage), fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name)
		sb.styles.MarkUsage(style, styleUsageImage)
	}

	return sb.addEntry(ContentRef{
		EID:          eid,
		Type:         SymImage,
		ResourceName: resourceName,
		Style:        style,
		AltText:      altText,
	})
}

// AddInlineImage adds an inline image reference (embedded within text).
// Unlike block images, inline images have render: inline and use em-based dimensions
// with baseline-style: center for vertical alignment within text.
func (sb *StorylineBuilder) AddInlineImage(resourceName, style, altText string) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(SymImage)+" (inline)", fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name)
		sb.styles.MarkUsage(style, styleUsageImage)
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
func (sb *StorylineBuilder) AddMixedContent(style string, items []InlineContentItem, events []StyleEventRef, headingLevel int) int {
	eid := sb.eidCounter
	sb.eidCounter++

	if style != "" && sb.styles != nil {
		sb.styles.tracer.TraceAssign(traceSymbolName(SymText)+" (mixed)", fmt.Sprintf("%d", eid), style, sb.sectionName+"/"+sb.name)
		sb.styles.MarkUsage(style, styleUsageText)
	}

	// Build content_list array with text strings and image entries
	contentList := make([]any, 0, len(items))
	for _, item := range items {
		if item.IsImage {
			// Create inline image entry
			imgEid := sb.eidCounter
			sb.eidCounter++

			if item.Style != "" && sb.styles != nil {
				sb.styles.tracer.TraceAssign(traceSymbolName(SymImage)+" (inline/mixed)", fmt.Sprintf("%d", imgEid), item.Style, sb.sectionName+"/"+sb.name)
				sb.styles.MarkUsage(item.Style, styleUsageImage)
			}

			imgEntry := NewStruct().
				SetInt(SymUniqueID, int64(imgEid)).
				SetSymbol(SymType, SymImage).
				Set(SymResourceName, SymbolByName(item.ResourceName)).
				SetSymbol(SymRender, SymInline) // render: inline

			if item.Style != "" {
				imgEntry.Set(SymStyle, SymbolByName(item.Style))
			}
			if item.AltText != "" {
				imgEntry.SetString(SymAltText, item.AltText)
			}

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

	// Add as raw entry
	return sb.addEntry(ContentRef{
		EID:      eid,
		RawEntry: entry,
	})
}

// ============================================================================
// Deferred Style Resolution Methods
// ============================================================================
// These methods store StyleSpec instead of Style, allowing position-aware
// resolution in EndBlock(). Use these when adding content to position-aware blocks.

// AddContentDeferred adds content with deferred style resolution.
// The styleSpec will be resolved with position filtering in EndBlock().
// Use this when adding content to blocks started with StartBlockWithPosition().
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

// AddTable adds a table with proper KFX structure.
// Structure: table($278) -> body($454) -> rows($279) -> cells($270) -> text($269)
func (sb *StorylineBuilder) AddTable(table *fb2.Table, styles *StyleRegistry, ca *ContentAccumulator) int {
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

			// Get cell text content
			var cellText strings.Builder
			for _, seg := range cell.Content {
				cellText.WriteString(seg.AsText())
			}
			text := cellText.String()

			// Add text to content accumulator
			contentName, offset := ca.Add(text)

			// Determine cell style based on header/alignment
			var cellStyle string
			if cell.Header {
				cellStyle = styles.ResolveStyle("th")
			} else {
				cellStyle = styles.ResolveStyle("td")
			}
			if styles != nil {
				styles.MarkUsage(cellStyle, styleUsageText)
			}

			// Create text entry inside cell
			textEID := sb.eidCounter
			sb.eidCounter++
			textEntry := NewStruct().
				SetInt(SymUniqueID, int64(textEID)).
				SetSymbol(SymType, SymText).
				Set(SymStyle, SymbolByName(cellStyle))

			// Add content reference
			contentRef := map[string]any{
				"name": SymbolByName(contentName),
				"$403": offset,
			}
			textEntry.Set(SymContent, contentRef)

			// Create cell container with nested text
			cellEntry := NewStruct().
				SetInt(SymUniqueID, int64(cellEID)).
				SetSymbol(SymType, SymContainer).         // $270
				SetSymbol(SymLayout, SymVertical).        // $156 = $323 (vertical)
				SetList(SymContentList, []any{textEntry}) // Nested text content

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
	tableStyle := styles.ResolveStyle("table")
	styles.tracer.TraceAssign(traceSymbolName(SymTable), fmt.Sprintf("%d", tableEID), tableStyle, sb.sectionName+"/"+sb.name)
	styles.MarkUsage(tableStyle, styleUsageWrapper)
	tableEntry := NewStruct().
		SetInt(SymUniqueID, int64(tableEID)).
		SetSymbol(SymType, SymTable). // $278
		Set(SymStyle, SymbolByName(tableStyle)).
		SetBool(SymTableBorderCollapse, true). // $150 = true
		SetList(SymContentList, []any{bodyEntry})

	// Add to storyline
	if len(sb.blockStack) > 0 {
		sb.blockStack[len(sb.blockStack)-1].children = append(sb.blockStack[len(sb.blockStack)-1].children, ContentRef{
			EID:      tableEID,
			Type:     SymTable,
			RawEntry: tableEntry,
		})
	} else {
		sb.contentEntries = append(sb.contentEntries, ContentRef{
			EID:      tableEID,
			Type:     SymTable,
			RawEntry: tableEntry,
		})
	}

	return tableEID
}

// FirstEID returns the first EID used by this storyline content.
func (sb *StorylineBuilder) FirstEID() int {
	if len(sb.contentEntries) > 0 {
		return sb.contentEntries[0].EID
	}
	return sb.eidCounter
}

// NextEID returns the next EID that will be assigned.
func (sb *StorylineBuilder) NextEID() int {
	return sb.eidCounter
}

// SetNextEID updates the EID counter (used when building complex structures externally).
func (sb *StorylineBuilder) SetNextEID(eid int) {
	sb.eidCounter = eid
}

// AddRawEntry adds a pre-built StructValue entry to the storyline.
// This is used for complex structures like lists that are built externally.
func (sb *StorylineBuilder) AddRawEntry(entry StructValue) {
	ref := ContentRef{
		RawEntry: entry,
	}
	if len(sb.blockStack) > 0 {
		sb.blockStack[len(sb.blockStack)-1].children = append(sb.blockStack[len(sb.blockStack)-1].children, ref)
	} else {
		sb.contentEntries = append(sb.contentEntries, ref)
	}
}

// PageTemplateEID returns the EID allocated for the page template container.
func (sb *StorylineBuilder) PageTemplateEID() int {
	return sb.pageTemplateEID
}

// Build creates the storyline and section fragments.
// Returns storyline fragment, section fragment.
func (sb *StorylineBuilder) Build() (*Fragment, *Fragment) {
	// Build content entries for storyline
	entries := make([]any, 0, len(sb.contentEntries))
	for _, ref := range sb.contentEntries {
		entries = append(entries, NewContentEntry(ref))
	}

	// Create storyline fragment
	storylineFrag := BuildStoryline(sb.name, entries)

	// Create page template entry for section - uses dedicated EID
	pageTemplates := []any{
		NewPageTemplateEntry(sb.pageTemplateEID, sb.name),
	}

	// Create section fragment
	sectionFrag := BuildSection(sb.sectionName, pageTemplates)

	return storylineFrag, sectionFrag
}

// BuildStorylineOnly creates only the storyline fragment without the section.
// Used for cover sections where the section uses container type instead of text type.
func (sb *StorylineBuilder) BuildStorylineOnly() *Fragment {
	entries := make([]any, 0, len(sb.contentEntries))
	for _, ref := range sb.contentEntries {
		entries = append(entries, NewContentEntry(ref))
	}
	return BuildStoryline(sb.name, entries)
}
