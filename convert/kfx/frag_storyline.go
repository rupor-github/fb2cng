package kfx

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"fbc/common"
	"fbc/fb2"
)

// normalizingWriter accumulates text with whitespace normalization.
// It collapses consecutive whitespace to single spaces while tracking
// the rune count for style event offsets. Leading and trailing whitespace
// is automatically trimmed using the pendingSpace approach.
type normalizingWriter struct {
	buf          strings.Builder
	runeCount    int
	pendingSpace bool // Deferred space - only written if followed by non-space
	preserveWS   bool // When true, write text as-is (for code blocks)
}

// newNormalizingWriter creates a new normalizing writer.
func newNormalizingWriter() *normalizingWriter {
	return &normalizingWriter{}
}

// WriteString writes text, normalizing whitespace unless preserveWS is set.
// Returns the rune count of what was actually written.
func (nw *normalizingWriter) WriteString(s string) int {
	if s == "" {
		return 0
	}

	if nw.preserveWS {
		// In preserve mode, write any pending space first
		if nw.pendingSpace {
			nw.buf.WriteRune(' ')
			nw.runeCount++
			nw.pendingSpace = false
		}
		nw.buf.WriteString(s)
		count := utf8.RuneCountInString(s)
		nw.runeCount += count
		return count
	}

	written := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			// Only mark pending space if we've written content
			if nw.buf.Len() > 0 || nw.pendingSpace {
				nw.pendingSpace = true
			}
		} else {
			// Write pending space before this non-space character
			if nw.pendingSpace {
				nw.buf.WriteRune(' ')
				nw.runeCount++
				written++
				nw.pendingSpace = false
			}
			nw.buf.WriteRune(r)
			nw.runeCount++
			written++
		}
	}
	return written
}

// SetPreserveWhitespace sets whether to preserve whitespace (for code blocks).
func (nw *normalizingWriter) SetPreserveWhitespace(preserve bool) {
	nw.preserveWS = preserve
}

// WriteRaw writes a string directly without normalization.
// Used for structural characters like newlines between title paragraphs.
// Discards any pending space and resets trailing state.
func (nw *normalizingWriter) WriteRaw(s string) {
	nw.pendingSpace = false // Discard pending space before structural break
	nw.buf.WriteString(s)
	nw.runeCount += utf8.RuneCountInString(s)
}

// String returns the accumulated text. No trimming needed since pending space
// approach ensures no leading/trailing whitespace is written.
func (nw *normalizingWriter) String() string {
	return nw.buf.String()
}

// Len returns the byte length of accumulated text.
func (nw *normalizingWriter) Len() int {
	return nw.buf.Len()
}

// RuneCount returns the current rune count (matches string length in runes).
func (nw *normalizingWriter) RuneCount() int {
	return nw.runeCount
}

// Reset clears the writer for reuse.
func (nw *normalizingWriter) Reset() {
	nw.buf.Reset()
	nw.runeCount = 0
	nw.pendingSpace = false
	nw.preserveWS = false
}

// StyleScope represents a single level in the element hierarchy.
// It captures both the element tag and its classes at that level.
type StyleScope struct {
	Tag     string   // HTML element tag: "div", "p", "h1", "span", etc.
	Classes []string // CSS classes applied to this element
}

// StyleContext accumulates inherited CSS properties as we descend the element hierarchy.
// This mimics how browsers propagate inherited properties from parent to child.
//
// In CSS, some properties (font-*, color, text-align, line-height, etc.) automatically
// inherit from parent to child elements. Other properties (margin, padding, border, etc.)
// do NOT inherit - they apply only to the element where they're defined.
//
// When resolving a style for an element:
// 1. Inherited properties come from the accumulated context (ancestors)
// 2. Non-inherited properties come only from the element's own tag/classes
type StyleContext struct {
	// Inherited properties accumulated from ancestors.
	// Only CSS-inherited properties are stored here.
	inherited map[KFXSymbol]any

	// Full scope chain from root to current level (for debugging/future use)
	scopes []StyleScope
}

// NewStyleContext creates an empty context (root level).
func NewStyleContext() StyleContext {
	return StyleContext{
		inherited: make(map[KFXSymbol]any),
		scopes:    nil,
	}
}

// Push enters a new element scope and returns a new context with that element's
// inherited properties added. Non-inherited properties are ignored for inheritance.
//
// tag: HTML element type ("div", "p", "h1", etc.)
// classes: space-separated CSS classes ("section poem" or "" for none)
// registry: style registry to look up property definitions
func (sc StyleContext) Push(tag, classes string, registry *StyleRegistry) StyleContext {
	// Copy existing inherited properties
	newInherited := make(map[KFXSymbol]any, len(sc.inherited))
	for k, v := range sc.inherited {
		newInherited[k] = v
	}

	// Add inherited properties from tag defaults
	if tag != "" {
		if def, ok := registry.Get(tag); ok {
			resolved := registry.resolveInheritance(def)
			for sym, val := range resolved.Properties {
				if isInheritedProperty(sym) {
					newInherited[sym] = val
				}
			}
		}
	}

	// Parse and add inherited properties from each class
	var classList []string
	if classes != "" {
		classList = strings.Fields(classes)
		for _, class := range classList {
			if def, ok := registry.Get(class); ok {
				resolved := registry.resolveInheritance(def)
				for sym, val := range resolved.Properties {
					if isInheritedProperty(sym) {
						newInherited[sym] = val
					}
				}
			}
		}
	}

	// Append to scope chain
	newScopes := append(sc.scopes, StyleScope{Tag: tag, Classes: classList})

	return StyleContext{
		inherited: newInherited,
		scopes:    newScopes,
	}
}

// Resolve creates the final style for an element within this context.
// Since KFX flattens styles (no nested containers), we apply ALL properties
// from the scope chain, not just inherited ones. This ensures wrapper margins
// propagate to content elements.
//
// Order of application (later overrides earlier):
// 1. Inherited properties accumulated through Push calls
// 2. All properties from scope chain classes (wrappers like body-title, section, etc.)
// 3. Element tag defaults (all properties)
// 4. Element's classes (all properties, in order)
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// registry: style registry for lookups and registration
// Returns the registered style name.
func (sc StyleContext) Resolve(tag, classes string, registry *StyleRegistry) string {
	merged := make(map[KFXSymbol]any)

	// 1. Start with inherited properties from context
	for k, v := range sc.inherited {
		merged[k] = v
	}

	// 2. Apply ALL properties from scope chain classes (not just inherited)
	// This ensures wrapper margins (body-title, section, etc.) propagate to content
	for _, scope := range sc.scopes {
		for _, class := range scope.Classes {
			if def, ok := registry.Get(class); ok {
				resolved := registry.resolveInheritance(def)
				for k, v := range resolved.Properties {
					merged[k] = v
				}
			}
		}
	}

	// 3. Apply element tag defaults (all properties)
	if tag != "" {
		if def, ok := registry.Get(tag); ok {
			resolved := registry.resolveInheritance(def)
			for k, v := range resolved.Properties {
				merged[k] = v
			}
		}
	}

	// 4. Apply element's classes (all properties, in order)
	if classes != "" {
		for _, class := range strings.Fields(classes) {
			if def, ok := registry.Get(class); ok {
				resolved := registry.resolveInheritance(def)
				for k, v := range resolved.Properties {
					merged[k] = v
				}
			}
		}
	}

	// 5. Register and return
	return registry.RegisterResolved(merged)
}

// isInheritedProperty returns true for CSS properties that inherit by default.
// Reference: https://developer.mozilla.org/en-US/docs/Web/CSS/inheritance
func isInheritedProperty(sym KFXSymbol) bool {
	switch sym {
	// Font properties
	case SymFontFamily, SymFontSize, SymFontWeight, SymFontStyle:
		return true
	// Text properties
	case SymTextAlignment, SymTextIndent, SymLineHeight:
		return true
	// Color
	case SymTextColor:
		return true
	// Spacing that inherits
	case SymLetterspacing:
		return true
	default:
		return false
	}
}

// TOCEntry represents a table of contents entry with hierarchical structure.
// This mirrors the chapterData structure in epub for consistent TOC generation.
type TOCEntry struct {
	ID           string      // Unique ID for this entry
	Title        string      // Display title for TOC
	SectionName  string      // KFX section name (e.g., "c0")
	StoryName    string      // KFX storyline name (e.g., "l1")
	FirstEID     int         // First content EID for navigation target
	IncludeInTOC bool        // Whether to include in TOC
	Children     []*TOCEntry // Nested TOC entries for subsections
}

// MaxContentFragmentSize is the maximum size in bytes for a content fragment's content_list.
// KFXInput validates that content fragments don't exceed 8192 bytes.
// This is separate from the container's ChunkSize ($412) which is used for streaming/compression.
const MaxContentFragmentSize = 8192

// ContentAccumulator manages content fragments with automatic chunking.
// Each paragraph/text entry is a separate item in content_list.
// When accumulated size exceeds MaxContentFragmentSize, a new fragment is created.
// ContentAccumulator accumulates paragraph content into named content fragments,
// automatically splitting into chunks when size limits are exceeded.
//
// Fragment naming pattern: "content_{N}" where N is sequential (e.g., content_1, content_2, content_3).
// This human-readable format is used instead of base36 for better debuggability.
type ContentAccumulator struct {
	counter     int                 // Global counter for sequential naming
	currentName string              // Current content fragment name
	currentList []string            // Current content list (each entry is one paragraph)
	currentSize int                 // Current accumulated size in bytes
	fragments   map[string][]string // All completed content fragments
}

// NewContentAccumulator creates a new content accumulator.
// Fragment names follow pattern "content_{N}" with sequential numbering.
func NewContentAccumulator(startCounter int) *ContentAccumulator {
	name := fmt.Sprintf("content_%d", startCounter)
	return &ContentAccumulator{
		counter:     startCounter,
		currentName: name,
		currentList: make([]string, 0),
		currentSize: 0,
		fragments:   make(map[string][]string),
	}
}

// Add adds a paragraph/text entry to the accumulator.
// Each call adds one entry to content_list. Creates new chunk if size limit exceeded.
// Returns the content name and offset for the added text.
func (ca *ContentAccumulator) Add(text string) (name string, offset int) {
	textSize := len(text)

	// Check if we need to start a new chunk
	// Start new chunk if current is non-empty and adding this would exceed limit
	if ca.currentSize > 0 && ca.currentSize+textSize > MaxContentFragmentSize {
		ca.finishCurrentChunk()
	}

	name = ca.currentName
	offset = len(ca.currentList)
	ca.currentList = append(ca.currentList, text)
	ca.currentSize += textSize

	return name, offset
}

// finishCurrentChunk saves the current chunk and starts a new one with sequential naming.
func (ca *ContentAccumulator) finishCurrentChunk() {
	if len(ca.currentList) > 0 {
		ca.fragments[ca.currentName] = ca.currentList
	}

	ca.counter++
	ca.currentName = fmt.Sprintf("content_%d", ca.counter)
	ca.currentList = make([]string, 0)
	ca.currentSize = 0
}

// Finish completes accumulation and returns all content fragments.
func (ca *ContentAccumulator) Finish() map[string][]string {
	// Save current chunk if it has content
	if len(ca.currentList) > 0 {
		ca.fragments[ca.currentName] = ca.currentList
	}
	return ca.fragments
}

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
	Style         string          // Optional style name
	StyleEvents   []StyleEventRef // Optional inline style events ($142)
	Children      []any           // Optional nested content for containers
	HeadingLevel  int             // For headings: 1-6 for h1-h6 ($790), 0 means not a heading
	RawEntry      StructValue     // Pre-built entry (for complex structures like tables)
}

// StyleEventRef represents a style event for inline formatting ($142).
type StyleEventRef struct {
	Offset         int    // $143 - start offset
	Length         int    // $144 - length
	Style          string // $157 - style name
	LinkTo         string // $179 - link target (optional)
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
		if ref.AltText != "" {
			entry.SetString(SymAltText, ref.AltText) // $584 = alt_text (only if non-empty)
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

	// Block wrapper support - when activeBlock is non-nil, content is added to it
	activeBlock *BlockBuilder
}

// BlockBuilder collects content entries for a wrapper/container element.
// It mirrors how EPUB generates <div class="..."> wrappers.
type BlockBuilder struct {
	styleSpec string         // Raw style specification (e.g., "poem", "cite") - resolved in EndBlock
	styles    *StyleRegistry // Style registry for deferred resolution
	eid       int            // EID for the wrapper container
	children  []ContentRef   // Nested content entries
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
func collectStructEIDs(sv StructValue) []int {
	var eids []int

	// Get this struct's EID
	if eid, exists := sv[SymUniqueID]; exists {
		if eidInt, ok := eid.(int64); ok {
			eids = append(eids, int(eidInt))
		}
	}

	// Recursively collect from content_list ($146)
	if contentList, exists := sv[SymContentList]; exists {
		if children, ok := contentList.([]any); ok {
			for _, child := range children {
				if childSV, ok := child.(StructValue); ok {
					eids = append(eids, collectStructEIDs(childSV)...)
				}
			}
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

// StartBlock begins a new wrapper/container block.
// All content added until EndBlock is called will be nested inside this wrapper.
// The styleSpec is the raw style name (e.g., "poem", "cite") - resolution is deferred
// until EndBlock to avoid registering styles for empty wrappers.
// Returns the EID of the wrapper for reference.
func (sb *StorylineBuilder) StartBlock(styleSpec string, styles *StyleRegistry) int {
	if sb.activeBlock != nil {
		// Nested blocks not supported - end current block first
		sb.EndBlock()
	}

	eid := sb.eidCounter
	sb.eidCounter++

	sb.activeBlock = &BlockBuilder{
		styleSpec: styleSpec,
		styles:    styles,
		eid:       eid,
		children:  make([]ContentRef, 0),
	}

	return eid
}

// EndBlock closes the current wrapper block and adds it to the storyline.
// The wrapper becomes a container entry with nested children.
// Empty wrappers (with no children) are discarded to avoid position_map validation errors.
// Style resolution is deferred until here to prevent registering styles for discarded wrappers.
func (sb *StorylineBuilder) EndBlock() {
	if sb.activeBlock == nil {
		return
	}

	// Skip empty wrappers - they have no content and cause position_map validation errors
	if len(sb.activeBlock.children) == 0 {
		sb.activeBlock = nil
		return
	}

	// Resolve style only now that we know the wrapper will be used
	resolvedStyle := ""
	if sb.activeBlock.styles != nil && sb.activeBlock.styleSpec != "" {
		resolvedStyle = sb.activeBlock.styles.ResolveStyle(sb.activeBlock.styleSpec)
	}

	// Convert children to content entries for the $146 list
	children := make([]any, 0, len(sb.activeBlock.children))
	for _, child := range sb.activeBlock.children {
		children = append(children, NewContentEntry(child))
	}

	if resolvedStyle != "" {
		sb.activeBlock.styles.tracer.TraceAssign("wrapper", fmt.Sprintf("%d", sb.activeBlock.eid), resolvedStyle, sb.sectionName+"/"+sb.name)
	}

	// Add the wrapper as a container entry
	sb.contentEntries = append(sb.contentEntries, ContentRef{
		EID:      sb.activeBlock.eid,
		Type:     SymText, // Container wrappers use $269 (text) type in KFX
		Style:    resolvedStyle,
		Children: children,
	})

	sb.activeBlock = nil
}

// addEntry is the internal method that routes content to the appropriate destination.
func (sb *StorylineBuilder) addEntry(ref ContentRef) int {
	if sb.activeBlock != nil {
		// Add to current block's children
		sb.activeBlock.children = append(sb.activeBlock.children, ref)
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
	}

	return sb.addEntry(ContentRef{
		EID:          eid,
		Type:         SymImage,
		ResourceName: resourceName,
		Style:        style,
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
	tableEntry := NewStruct().
		SetInt(SymUniqueID, int64(tableEID)).
		SetSymbol(SymType, SymTable). // $278
		Set(SymStyle, SymbolByName(tableStyle)).
		SetBool(SymTableBorderCollapse, true). // $150 = true
		SetList(SymContentList, []any{bodyEntry})

	// Add to storyline
	if sb.activeBlock != nil {
		sb.activeBlock.children = append(sb.activeBlock.children, ContentRef{
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
	sb.contentEntries = append(sb.contentEntries, ContentRef{
		RawEntry: entry,
	})
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

// generateStoryline creates storyline and section fragments from an FB2 book.
// It uses the provided StyleRegistry to reference styles by name.
// Returns fragments, next EID, section names for document_data, TOC entries, per-section EID sets,
// and mapping of original FB2 IDs to EIDs (for $266 anchors).
func generateStoryline(book *fb2.FictionBook, styles *StyleRegistry,
	imageResources imageResourceInfoByID, startEID int, footnotesIndex fb2.FootnoteRefs,
) (*FragmentList, int, sectionNameList, []*TOCEntry, sectionEIDsBySectionName, eidByFB2ID, LandmarkInfo, error) {
	fragments := NewFragmentList()
	eidCounter := startEID
	sectionNames := make(sectionNameList, 0)
	tocEntries := make([]*TOCEntry, 0)
	sectionEIDs := make(sectionEIDsBySectionName)
	idToEID := make(eidByFB2ID)
	landmarks := LandmarkInfo{}

	// Single shared content accumulator for the entire book.
	// KPV consolidates content across all storylines into fewer, larger fragments.
	ca := NewContentAccumulator(1)

	// Default screen width for image style calculations
	defaultWidth := 600

	sectionCount := 0

	// Collect footnote bodies for processing at the end
	var footnoteBodies []*fb2.Body

	// Create separate cover section at the very beginning (like reference KFX)
	// Cover is a container-type section with just the image, not embedded in body intro
	if len(book.Description.TitleInfo.Coverpage) > 0 {
		cover := &book.Description.TitleInfo.Coverpage[0]
		coverID := strings.TrimPrefix(cover.Href, "#")
		if imgInfo, ok := imageResources[coverID]; ok {
			sectionCount++
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline with just the cover image
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)
			// Use minimal cover style - no width constraints since page template defines dimensions
			resolved := styles.ResolveCoverImageStyle()
			sb.AddImage(imgInfo.ResourceName, resolved, cover.Alt)

			sectionEIDs[sectionName] = sb.AllEIDs()
			eidCounter = sb.NextEID()

			// Build cover storyline fragment
			storylineFrag := sb.BuildStorylineOnly()
			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}

			// Build cover section with container type and image dimensions
			// Reference KFX: {$140: $320, $155: eid, $156: $326, $159: $270, $176: storyline, $66: width, $67: height}
			// The page template EID should be unique (not the same as the image content EID)
			pageTemplateEID := sb.PageTemplateEID()
			pageTemplate := NewCoverPageTemplateEntry(pageTemplateEID, storyName, imgInfo.Width, imgInfo.Height)
			sectionFrag := BuildSection(sectionName, []any{pageTemplate})
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}

			// Track cover EID for landmarks
			landmarks.CoverEID = pageTemplateEID
		}
	}

	// Process each body
	for i := range book.Bodies {
		body := &book.Bodies[i]

		// Collect footnote bodies for later processing (at the end, like EPUB does)
		if body.Footnotes() {
			footnoteBodies = append(footnoteBodies, body)
			continue
		}

		// Process body intro content (title, epigraphs, image) as separate storyline
		// This mirrors epub's bodyIntroToXHTML which creates a separate chapter for body intro
		if body.Title != nil {
			sectionCount++
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline builder for body intro
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

			if err := processBodyIntroContent(book, body, sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}

			sectionEIDs[sectionName] = sb.AllEIDs()

			// Create TOC entry for body intro
			// Use "a-" prefix for anchor ID to avoid collision with section fragment ID
			title := body.Title.AsTOCText("Untitled")
			anchorID := "a-" + sectionName
			tocEntry := &TOCEntry{
				ID:           anchorID,
				Title:        title,
				SectionName:  sectionName,
				StoryName:    storyName,
				FirstEID:     sb.FirstEID(),
				IncludeInTOC: true,
			}
			tocEntries = append(tocEntries, tocEntry)
			idToEID[anchorID] = sb.FirstEID()

			// Track start reading location (first body intro is the "Start" landmark)
			if landmarks.StartEID == 0 {
				landmarks.StartEID = sb.FirstEID()
			}

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
		}

		// Process top-level sections as chapters (like epub does)
		for j := range body.Sections {
			section := &body.Sections[j]
			sectionCount++

			// Generate names using base36: "l1", "l2", ... "lA", "lB", ... for storylines
			storyName := "l" + toBase36(sectionCount)
			sectionName := "c" + toBase36(sectionCount-1)
			sectionNames = append(sectionNames, sectionName)

			// Create storyline builder
			sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

			// Track nested section info for TOC hierarchy
			var nestedTOCEntries []*TOCEntry

			if err := processStorylineContent(book, section, sb, styles, imageResources, ca, 1, &nestedTOCEntries, idToEID, defaultWidth, footnotesIndex); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}

			sectionEIDs[sectionName] = sb.AllEIDs()

			// Create TOC entry for this section
			title := section.AsTitleText("")
			tocEntry := &TOCEntry{
				ID:           section.ID,
				Title:        title,
				SectionName:  sectionName,
				StoryName:    storyName,
				FirstEID:     sb.FirstEID(),
				IncludeInTOC: title != "",
				Children:     nestedTOCEntries,
			}
			tocEntries = append(tocEntries, tocEntry)

			// Update EID counter
			eidCounter = sb.NextEID()

			// Build storyline and section fragments
			storylineFrag, sectionFrag := sb.Build()

			if err := fragments.Add(storylineFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
			if err := fragments.Add(sectionFrag); err != nil {
				return nil, 0, nil, nil, nil, nil, landmarks, err
			}
		}
	}

	// Process footnote bodies at the end - each footnote section becomes part of a single storyline
	// This ensures footnote IDs (n_1, n_2, etc.) are registered in idToEID for anchor generation
	if len(footnoteBodies) > 0 {
		sectionCount++
		storyName := "l" + toBase36(sectionCount)
		sectionName := "c" + toBase36(sectionCount-1)
		sectionNames = append(sectionNames, sectionName)

		sb := NewStorylineBuilder(storyName, sectionName, eidCounter, styles)

		// Process all footnote bodies into a single storyline
		for _, body := range footnoteBodies {
			// Process body title if present (not wrapped, same as EPUB)
			if body.Title != nil {
				for _, item := range body.Title.Items {
					if item.Paragraph != nil {
						addParagraphWithImages(item.Paragraph, "footnote-title", sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
					}
				}
			}

			// Process each section in the footnote body
			// Each section gets wrapped in a container (mirrors EPUB's <div class="footnote">)
			for j := range body.Sections {
				section := &body.Sections[j]

				// Start wrapper block for this footnote section
				sb.StartBlock("footnote", styles)

				// Register the section ID for anchor generation
				if section.ID != "" {
					if _, exists := idToEID[section.ID]; !exists {
						idToEID[section.ID] = sb.NextEID()
					}
				}

				// Process section title
				if section.Title != nil {
					for _, item := range section.Title.Items {
						if item.Paragraph != nil {
							addParagraphWithImages(item.Paragraph, "footnote-title", sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
						}
					}
				}

				// Process section content (paragraphs, poems, etc.)
				footnoteCtx := NewStyleContext().Push("div", "footnote", styles)
				for k := range section.Content {
					processFlowItem(&section.Content[k], footnoteCtx, "footnote", sb, styles, imageResources, ca, idToEID, defaultWidth, footnotesIndex)
				}

				// End wrapper block for this footnote section
				sb.EndBlock()
			}
		}

		sectionEIDs[sectionName] = sb.AllEIDs()

		// Create TOC entry for footnotes (not included in main TOC)
		// Use "a-" prefix for anchor ID to avoid collision with section fragment ID
		anchorID := "a-" + sectionName
		tocEntry := &TOCEntry{
			ID:           anchorID,
			Title:        "Notes",
			SectionName:  sectionName,
			StoryName:    storyName,
			FirstEID:     sb.FirstEID(),
			IncludeInTOC: false, // Don't include footnotes in TOC
		}
		tocEntries = append(tocEntries, tocEntry)
		idToEID[anchorID] = sb.FirstEID()

		eidCounter = sb.NextEID()
		storylineFrag, sectionFrag := sb.Build()

		if err := fragments.Add(storylineFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, err
		}
		if err := fragments.Add(sectionFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, err
		}
	}

	// Create content fragments from accumulated content (single shared accumulator)
	for name, contentList := range ca.Finish() {
		contentFrag := buildContentFragmentByName(name, contentList)
		if err := fragments.Add(contentFrag); err != nil {
			return nil, 0, nil, nil, nil, nil, landmarks, err
		}
	}

	return fragments, eidCounter, sectionNames, tocEntries, sectionEIDs, idToEID, landmarks, nil
}

// processBodyIntroContent processes body intro content using ContentAccumulator.
func addVignetteImage(book *fb2.FictionBook, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, pos common.VignettePos, screenWidth int) {
	if book == nil || !book.IsVignetteEnabled(pos) {
		return
	}
	vigID := book.VignetteIDs[pos]
	if vigID == "" {
		return
	}
	imgInfo, ok := imageResources[vigID]
	if !ok {
		return
	}
	resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
	sb.AddImage(imgInfo.ResourceName, resolved, "") // Vignettes are decorative, no alt text
}

func processBodyIntroContent(book *fb2.FictionBook, body *fb2.Body, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) error {
	if body.Image != nil {
		imgID := strings.TrimPrefix(body.Image.Href, "#")
		if imgInfo, ok := imageResources[imgID]; ok {
			resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
			eid := sb.AddImage(imgInfo.ResourceName, resolved, body.Image.Alt)
			if body.Image.ID != "" {
				if _, exists := idToEID[body.Image.ID]; !exists {
					idToEID[body.Image.ID] = eid
				}
			}
		}
	}

	// Process body title with wrapper (mirrors EPUB's <div class="body-title">)
	if body.Title != nil {
		// Start wrapper block - this is the KFX equivalent of <div class="body-title">
		sb.StartBlock("body-title", styles)

		if body.Main() {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosBookTitleTop, screenWidth)
		}
		// Add title as single combined heading entry (matches KPV behavior)
		// Uses body-title-header as base for -first/-next styles, heading level 1
		// Context includes wrapper class for margin inheritance
		titleCtx := NewStyleContext().Push("div", "body-title", styles)
		addTitleAsHeading(body.Title, titleCtx, "body-title-header", 1, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		if body.Main() {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosBookTitleBottom, screenWidth)
		}

		// End wrapper block
		sb.EndBlock()
	}

	// Process body epigraphs - each wrapped in <div class="epigraph">
	epigraphCtx := NewStyleContext().Push("div", "epigraph", styles)
	for _, epigraph := range body.Epigraphs {
		// Start wrapper block - mirrors EPUB's <div class="epigraph">
		wrapperEID := sb.StartBlock("epigraph", styles)
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = wrapperEID
			}
		}
		for i := range epigraph.Flow.Items {
			processFlowItem(&epigraph.Flow.Items[i], epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		for i := range epigraph.TextAuthors {
			styleName := epigraphCtx.Resolve("p", "text-author", styles)
			addParagraphWithImages(&epigraph.TextAuthors[i], styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		sb.EndBlock()
	}

	return nil
}

// processStorylineContent processes FB2 section content using ContentAccumulator.
func processStorylineContent(book *fb2.FictionBook, section *fb2.Section, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, depth int, nestedTOC *[]*TOCEntry, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) error {
	if section != nil && section.ID != "" {
		if _, exists := idToEID[section.ID]; !exists {
			idToEID[section.ID] = sb.NextEID()
		}
	}

	if section.Image != nil {
		imgID := strings.TrimPrefix(section.Image.Href, "#")
		if imgInfo, ok := imageResources[imgID]; ok {
			resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
			eid := sb.AddImage(imgInfo.ResourceName, resolved, section.Image.Alt)
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
		if depth == 1 {
			wrapperClass = "chapter-title"
			headerClassBase = "chapter-title-header"
			headingLevel = 1
		} else {
			wrapperClass = "section-title"
			headerClassBase = "section-title-header"
			// Map depth to heading level: 2->h2, 3->h3, 4+->h4
			headingLevel = depth
			if headingLevel > 4 {
				headingLevel = 4
			}
		}

		// Start wrapper block - this is the KFX equivalent of <div class="chapter-title"> or <div class="section-title">
		sb.StartBlock(wrapperClass, styles)

		// Add top vignette
		if depth == 1 {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosChapterTitleTop, screenWidth)
		} else {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosSectionTitleTop, screenWidth)
		}

		// Add title as single combined heading entry (matches KPV behavior)
		// Context includes wrapper class for margin inheritance
		titleCtx := NewStyleContext().Push("div", wrapperClass, styles)
		addTitleAsHeading(section.Title, titleCtx, headerClassBase, headingLevel, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)

		// Add bottom vignette
		if depth == 1 {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosChapterTitleBottom, screenWidth)
		} else {
			addVignetteImage(book, sb, styles, imageResources, common.VignettePosSectionTitleBottom, screenWidth)
		}

		// End wrapper block
		sb.EndBlock()
	}

	// Process annotation - wrapped in <div class="annotation">
	if section.Annotation != nil {
		// Start wrapper block - mirrors EPUB's <div class="annotation">
		wrapperEID := sb.StartBlock("annotation", styles)
		if section.Annotation.ID != "" {
			if _, exists := idToEID[section.Annotation.ID]; !exists {
				idToEID[section.Annotation.ID] = wrapperEID
			}
		}
		annotationCtx := NewStyleContext().Push("div", "annotation", styles)
		for i := range section.Annotation.Items {
			processFlowItem(&section.Annotation.Items[i], annotationCtx, "annotation", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		sb.EndBlock()
	}

	// Process epigraphs - each wrapped in <div class="epigraph">
	epigraphCtx := NewStyleContext().Push("div", "epigraph", styles)
	for _, epigraph := range section.Epigraphs {
		// Start wrapper block - mirrors EPUB's <div class="epigraph">
		wrapperEID := sb.StartBlock("epigraph", styles)
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = wrapperEID
			}
		}
		for i := range epigraph.Flow.Items {
			processFlowItem(&epigraph.Flow.Items[i], epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		for i := range epigraph.TextAuthors {
			styleName := epigraphCtx.Resolve("p", "text-author", styles)
			addParagraphWithImages(&epigraph.TextAuthors[i], styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		sb.EndBlock()
	}

	// Process content items
	sectionCtx := NewStyleContext().Push("div", "section", styles)
	var lastTitledEntry *TOCEntry
	for i := range section.Content {
		item := &section.Content[i]
		if item.Kind == fb2.FlowSection && item.Section != nil {
			// Nested section - track for TOC hierarchy
			nestedSection := item.Section

			// Track the EID where this nested section starts
			firstEID := sb.NextEID()

			// Process nested section content recursively
			var childTOC []*TOCEntry
			if err := processStorylineContent(book, nestedSection, sb, styles, imageResources, ca, depth+1, &childTOC, idToEID, screenWidth, footnotesIndex); err != nil {
				return err
			}

			// Create TOC entry for nested section only if it has a title
			if nestedSection.HasTitle() {
				titleText := nestedSection.AsTitleText("")
				tocEntry := &TOCEntry{
					ID:           nestedSection.ID,
					Title:        titleText,
					FirstEID:     firstEID,
					IncludeInTOC: true,
					Children:     childTOC,
				}
				*nestedTOC = append(*nestedTOC, tocEntry)
				lastTitledEntry = tocEntry
			} else if len(childTOC) > 0 {
				// Section without title - nest children inside the last titled sibling if one exists
				if lastTitledEntry != nil {
					lastTitledEntry.Children = append(lastTitledEntry.Children, childTOC...)
				} else {
					// No preceding titled sibling, promote children to parent level
					*nestedTOC = append(*nestedTOC, childTOC...)
				}
			}
		} else {
			processFlowItem(item, sectionCtx, "section", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
	}

	if depth == 1 {
		addVignetteImage(book, sb, styles, imageResources, common.VignettePosChapterEnd, screenWidth)
	} else {
		addVignetteImage(book, sb, styles, imageResources, common.VignettePosSectionEnd, screenWidth)
	}

	return nil
}

// processFlowItem processes a flow item using ContentAccumulator.
// ctx tracks the full ancestor context chain for CSS cascade emulation.
// contextName is the immediate context name (e.g., "section", "cite") used for subtitle naming.
func processFlowItem(item *fb2.FlowItem, ctx StyleContext, contextName string, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	switch item.Kind {
	case fb2.FlowParagraph:
		if item.Paragraph != nil {
			// Use full context chain for CSS cascade emulation.
			// Base "p" + ancestor contexts + optional custom class from FB2.
			styleName := ctx.Resolve("p", "", styles)
			if item.Paragraph.Style != "" {
				styleName = styleName + " " + item.Paragraph.Style
			}
			addParagraphWithImages(item.Paragraph, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowSubtitle:
		if item.Subtitle != nil {
			// Subtitles use <p> in EPUB, so use p as base here too
			// Context-specific subtitle style adds alignment, margins
			styleName := ctx.Resolve("p", contextName+"-subtitle", styles)
			if item.Subtitle.Style != "" {
				styleName = styleName + " " + item.Subtitle.Style
			}
			addParagraphWithImages(item.Subtitle, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

	case fb2.FlowEmptyLine:
		// Empty lines in KFX are handled via block margins on surrounding content,
		// not via explicit newline content entries. Reference KFX files from
		// Kindle Previewer don't have standalone "\n" content entries.
		// The emptyline style should set appropriate margin-top/margin-bottom
		// on adjacent elements instead.
		return

	case fb2.FlowPoem:
		if item.Poem != nil {
			// Start wrapper block - mirrors EPUB's <div class="poem">
			wrapperEID := sb.StartBlock("poem", styles)
			if item.Poem.ID != "" {
				if _, exists := idToEID[item.Poem.ID]; !exists {
					idToEID[item.Poem.ID] = wrapperEID
				}
			}
			processPoem(item.Poem, ctx.Push("div", "poem", styles), sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
			sb.EndBlock()
		}

	case fb2.FlowCite:
		if item.Cite != nil {
			// Start wrapper block - mirrors EPUB's <blockquote class="cite">
			wrapperEID := sb.StartBlock("cite", styles)
			if item.Cite.ID != "" {
				if _, exists := idToEID[item.Cite.ID]; !exists {
					idToEID[item.Cite.ID] = wrapperEID
				}
			}
			processCite(item.Cite, ctx.Push("blockquote", "cite", styles), sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
			sb.EndBlock()
		}

	case fb2.FlowTable:
		if item.Table != nil {
			eid := sb.AddTable(item.Table, styles, ca)
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
		resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
		eid := sb.AddImage(imgInfo.ResourceName, resolved, item.Image.Alt)
		if item.Image.ID != "" {
			if _, exists := idToEID[item.Image.ID]; !exists {
				idToEID[item.Image.ID] = eid
			}
		}

	case fb2.FlowSection:
		// Nested sections handled in processStorylineContent
	}
}

// processPoem processes poem content using ContentAccumulator.
// Matches EPUB's appendPoemElement handling: title, epigraphs, subtitles, stanzas, text-authors, date.
// ctx contains the ancestor context chain (e.g., may already include "cite" if poem is inside cite).
func processPoem(poem *fb2.Poem, ctx StyleContext, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	// Process poem title - uses current context + poem-title
	if poem.Title != nil {
		for _, item := range poem.Title.Items {
			if item.Paragraph != nil {
				styleName := ctx.Resolve("p", "poem-title", styles)
				addParagraphWithImages(item.Paragraph, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
			}
		}
	}

	// Process poem epigraphs - each wrapped in <div class="epigraph">
	epigraphCtx := ctx.Push("div", "epigraph", styles)
	for _, epigraph := range poem.Epigraphs {
		// Start wrapper block - mirrors EPUB's <div class="epigraph">
		wrapperEID := sb.StartBlock("epigraph", styles)
		if epigraph.Flow.ID != "" {
			if _, exists := idToEID[epigraph.Flow.ID]; !exists {
				idToEID[epigraph.Flow.ID] = wrapperEID
			}
		}
		for i := range epigraph.Flow.Items {
			processFlowItem(&epigraph.Flow.Items[i], epigraphCtx, "epigraph", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		for i := range epigraph.TextAuthors {
			styleName := epigraphCtx.Resolve("p", "text-author", styles)
			addParagraphWithImages(&epigraph.TextAuthors[i], styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		sb.EndBlock()
	}

	// Process poem subtitles (matches EPUB's poem.Subtitles handling)
	for i := range poem.Subtitles {
		styleName := ctx.Resolve("p", "poem-subtitle", styles)
		addParagraphWithImages(&poem.Subtitles[i], styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}

	// Process stanzas - each wrapped in <div class="stanza">
	stanzaCtx := ctx.Push("div", "stanza", styles)
	for _, stanza := range poem.Stanzas {
		// Start wrapper block - mirrors EPUB's <div class="stanza">
		sb.StartBlock("stanza", styles)

		// Stanza title (matches EPUB's "stanza-title" class)
		if stanza.Title != nil {
			for _, item := range stanza.Title.Items {
				if item.Paragraph != nil {
					styleName := stanzaCtx.Resolve("p", "stanza-title", styles)
					addParagraphWithImages(item.Paragraph, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
				}
			}
		}
		// Stanza subtitle (matches EPUB's "stanza-subtitle" class)
		if stanza.Subtitle != nil {
			styleName := stanzaCtx.Resolve("p", "stanza-subtitle", styles)
			addParagraphWithImages(stanza.Subtitle, styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		// Verses - use stanza context
		for i := range stanza.Verses {
			styleName := stanzaCtx.Resolve("p", "verse", styles)
			addParagraphWithImages(&stanza.Verses[i], styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}

		sb.EndBlock()
	}

	// Process text authors
	for i := range poem.TextAuthors {
		styleName := ctx.Resolve("p", "text-author", styles)
		addParagraphWithImages(&poem.TextAuthors[i], styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
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
			styleName := ctx.Resolve("p", "date", styles)
			resolved := styles.ResolveStyle(styleName)
			contentName, offset := ca.Add(dateText)
			sb.AddContent(SymText, contentName, offset, resolved)
		}
	}
}

// processCite processes cite content using ContentAccumulator.
// Matches EPUB's appendCiteElement handling: processes all flow items with "cite" context,
// followed by text-authors.
// ctx contains the ancestor context chain (already includes "cite" pushed by caller).
func processCite(cite *fb2.Cite, ctx StyleContext, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	// Process all cite flow items with full context chain
	for i := range cite.Items {
		processFlowItem(&cite.Items[i], ctx, "cite", sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}

	// Process text authors
	for i := range cite.TextAuthors {
		styleName := ctx.Resolve("p", "text-author", styles)
		addParagraphWithImages(&cite.TextAuthors[i], styleName, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
	}
}

func addParagraphWithImages(para *fb2.Paragraph, styleName string, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	var (
		nw     = newNormalizingWriter() // Normalizes whitespace and tracks rune count
		events []StyleEventRef
	)

	// Determine heading level from style name
	headingLevel := styleToHeadingLevel(styleName)

	flush := func() {
		if nw.Len() == 0 {
			return
		}
		resolved := styles.ResolveStyle(styleName)
		contentName, offset := ca.Add(nw.String())
		var eid int
		if headingLevel > 0 {
			eid = sb.AddContentWithHeading(SymText, contentName, offset, resolved, events, headingLevel)
		} else {
			eid = sb.AddContentAndEvents(SymText, contentName, offset, resolved, events)
		}
		if para.ID != "" {
			if _, exists := idToEID[para.ID]; !exists {
				idToEID[para.ID] = eid
			}
		}
		nw.Reset()
		events = nil
	}

	var walk func(seg *fb2.InlineSegment, inlineStyle string)
	walk = func(seg *fb2.InlineSegment, inlineStyle string) {
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
			resolved := styles.ResolveImageStyle(imgInfo.Width, screenWidth)
			sb.AddImage(imgInfo.ResourceName, resolved, seg.Image.Alt)
			return
		}

		// Determine style for this segment based on its kind
		var segStyle string
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
			// Links use link-footnote or link-internal style
			if strings.HasPrefix(seg.Href, "#") {
				segStyle = "link-footnote"
			} else {
				segStyle = "link-external"
			}
		}

		// Track position for style event using rune count (KFX uses character offsets)
		start := nw.RuneCount()

		// Add text content (normalizingWriter handles whitespace and rune counting)
		nw.WriteString(seg.Text)

		// Process children with current style context
		for i := range seg.Children {
			walk(&seg.Children[i], segStyle)
		}

		// Restore whitespace handling after code block
		if seg.Kind == fb2.InlineCode {
			nw.SetPreserveWhitespace(false)
		}

		end := nw.RuneCount()

		// Create style event if we have styled content
		if segStyle != "" && end > start {
			resolved := styles.ResolveStyle(segStyle)
			event := StyleEventRef{
				Offset: start,
				Length: end - start,
				Style:  resolved,
			}
			// Add link target for links and detect footnote links
			if seg.Kind == fb2.InlineLink {
				linkTo := strings.TrimPrefix(seg.Href, "#")
				if linkTo != "" && linkTo != seg.Href {
					event.LinkTo = linkTo
					// Check if this is a footnote link using FootnotesIndex (like EPUB does)
					if _, isFootnote := footnotesIndex[linkTo]; isFootnote {
						event.IsFootnoteLink = true
					}
				}
			}
			events = append(events, event)
		}
	}

	for i := range para.Text {
		walk(&para.Text[i], "")
	}
	flush()
}

// addTitleAsHeading creates a single combined content entry for multi-paragraph titles.
// This matches KPV behavior where all title lines are combined with newlines and
// style events are used for -first/-next styling within the combined entry.
// The heading level ($790) is applied only to this combined entry.
// ctx provides the style context (wrapper class like "body-title") for proper margin inheritance.
func addTitleAsHeading(title *fb2.Title, ctx StyleContext, headerStyleBase string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	if title == nil || len(title.Items) == 0 {
		return
	}

	// Check if title contains inline images - if so, fall back to separate paragraphs
	// since KFX can't mix text and images in a single content entry
	if titleHasInlineImages(title) {
		addTitleAsSeparateParagraphs(title, ctx, headerStyleBase, headingLevel, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		return
	}

	var (
		nw               = newNormalizingWriter() // Normalizes whitespace and tracks rune count
		events           []StyleEventRef
		firstParagraph   = true
		prevWasEmptyLine = false
		firstParaID      string // Store ID of first paragraph for EID mapping
	)

	// Process inline segment and accumulate style events
	var processSegment func(seg *fb2.InlineSegment, inlineStyle string)
	processSegment = func(seg *fb2.InlineSegment, inlineStyle string) {
		// Inline images should have been filtered out by titleHasInlineImages check
		if seg.Kind == fb2.InlineImageSegment {
			return
		}

		// Determine style for this segment based on its kind
		var segStyle string
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
			if strings.HasPrefix(seg.Href, "#") {
				segStyle = "link-footnote"
			} else {
				segStyle = "link-external"
			}
		}

		// Track position for style event using rune count (KFX uses character offsets)
		start := nw.RuneCount()

		// Add text content (normalizingWriter handles whitespace and rune counting)
		nw.WriteString(seg.Text)

		// Process children with current style context
		for i := range seg.Children {
			processSegment(&seg.Children[i], segStyle)
		}

		// Restore whitespace handling after code block
		if seg.Kind == fb2.InlineCode {
			nw.SetPreserveWhitespace(false)
		}

		end := nw.RuneCount()

		// Create style event if we have styled content
		if segStyle != "" && end > start {
			resolved := styles.ResolveStyle(segStyle)
			event := StyleEventRef{
				Offset: start,
				Length: end - start,
				Style:  resolved,
			}
			// Add link target for links and detect footnote links
			if seg.Kind == fb2.InlineLink {
				linkTo := strings.TrimPrefix(seg.Href, "#")
				if linkTo != "" && linkTo != seg.Href {
					event.LinkTo = linkTo
					if _, isFootnote := footnotesIndex[linkTo]; isFootnote {
						event.IsFootnoteLink = true
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
				breakStyle := headerStyleBase + "-break"
				resolved := styles.ResolveStyle(breakStyle)
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
				processSegment(&item.Paragraph.Text[i], "")
			}

			paraEnd := nw.RuneCount()

			// Add paragraph-level style event (like EPUB's span class)
			if paraEnd > paraStart {
				resolved := styles.ResolveStyle(paraStyle)
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
			emptylineStyle := headerStyleBase + "-emptyline"
			resolved := styles.ResolveStyle(emptylineStyle)
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

	// Combine heading element style (h1-h6) with context and header class style
	// ctx contains the wrapper class (e.g., "body-title") which provides margins
	// This produces "h1 body-title body-title-header" so margins are inherited
	headingElementStyle := fmt.Sprintf("h%d", headingLevel)
	combinedStyleSpec := ctx.Resolve(headingElementStyle, headerStyleBase, styles)
	resolved := styles.ResolveStyle(combinedStyleSpec)
	contentName, offset := ca.Add(nw.String())
	eid := sb.AddContentWithHeading(SymText, contentName, offset, resolved, events, headingLevel)

	// Map first paragraph ID to the combined entry's EID
	if firstParaID != "" {
		if _, exists := idToEID[firstParaID]; !exists {
			idToEID[firstParaID] = eid
		}
	}
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

// addTitleAsSeparateParagraphs adds title paragraphs as separate entries (fallback for titles with images).
// This is the original behavior before combined heading support was added.
// Note: EmptyLine items are ignored as spacing is handled via block margins.
// ctx provides the style context (wrapper class like "body-title") for proper margin inheritance.
func addTitleAsSeparateParagraphs(title *fb2.Title, ctx StyleContext, headerStyleBase string, headingLevel int, sb *StorylineBuilder, styles *StyleRegistry, imageResources imageResourceInfoByID, ca *ContentAccumulator, idToEID eidByFB2ID, screenWidth int, footnotesIndex fb2.FootnoteRefs) {
	firstParagraph := true
	for _, item := range title.Items {
		if item.Paragraph != nil {
			// Determine style for this paragraph (-first or -next)
			var paraStyle string
			if firstParagraph {
				paraStyle = headerStyleBase + "-first"
				firstParagraph = false
			} else {
				paraStyle = headerStyleBase + "-next"
			}
			// Combine heading level indicator with context and paragraph style
			headingElementStyle := fmt.Sprintf("h%d", headingLevel)
			fullStyle := ctx.Resolve(headingElementStyle, paraStyle, styles)
			addParagraphWithImages(item.Paragraph, fullStyle, sb, styles, imageResources, ca, idToEID, screenWidth, footnotesIndex)
		}
		// EmptyLine items are ignored - spacing is handled via block margins like regular flow content
	}
}

// buildContentFragmentByName creates a content ($145) fragment with string name.
// The name parameter comes from ContentAccumulator and follows the pattern "content_{N}"
// with sequential numbering. This human-readable naming convention
// is maintained throughout the conversion for easier debugging and inspection.
func buildContentFragmentByName(name string, contentList []string) *Fragment {
	// Use string-keyed map for content with local symbol names
	// The "name" field value should be a symbol, not a string
	content := map[string]any{
		"$146": anySlice(contentList), // content_list
		"name": SymbolByName(name),    // name as symbol value
	}

	return &Fragment{
		FType:   SymContent,
		FIDName: name,
		Value:   content,
	}
}

// anySlice converts []string to []any
func anySlice(s []string) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

// BuildNavigation creates a $389 book_navigation fragment from TOC entries.
// This creates a hierarchical TOC structure similar to epub's NCX/nav.
// The $389 fragment value is a list of reading order navigation entries.
// If posItems and pageSize are provided (pageSize > 0), an APPROXIMATE_PAGE_LIST
// is also included in the navigation.
// If landmarks has non-zero EIDs, a landmarks container is added.
func BuildNavigation(tocEntries []*TOCEntry, startEID int, posItems []PositionItem, pageSize int, landmarks LandmarkInfo) *Fragment {
	// Build TOC entries recursively
	entries := buildNavEntries(tocEntries, startEID)

	// Create TOC navigation container
	tocContainer := NewTOCContainer(entries)

	// Create nav_containers list starting with TOC
	navContainers := []any{tocContainer}

	// Add landmarks container if we have any landmark positions
	if landmarksContainer := buildLandmarksContainer(landmarks); landmarksContainer != nil {
		navContainers = append(navContainers, landmarksContainer)
	}

	// Add APPROXIMATE_PAGE_LIST if page mapping is enabled
	if pageSize > 0 && len(posItems) > 0 {
		pages := CalculateApproximatePages(posItems, pageSize)
		if len(pages) > 0 {
			pageEntries := buildPageListEntries(pages)
			pageListContainer := NewApproximatePageListContainer(pageEntries)
			navContainers = append(navContainers, pageListContainer)
		}
	}

	// Build reading order navigation entry
	// Structure: {$178: $351 (default), $392: [nav_containers]}
	readingOrderNav := NewStruct().
		SetSymbol(SymReadOrderName, SymDefault). // $178 = default reading order
		SetList(SymNavContainers, navContainers) // $392 = nav_containers

	// $389 is a list of reading order navigation entries
	bookNavList := []any{readingOrderNav}

	return &Fragment{
		FType: SymBookNavigation,
		FID:   SymBookNavigation, // Root fragment - FID == FType
		Value: bookNavList,
	}
}

// buildLandmarksContainer creates a landmarks navigation container from LandmarkInfo.
// Returns nil if no landmarks are configured.
func buildLandmarksContainer(landmarks LandmarkInfo) StructValue {
	landmarkEntries := make([]any, 0, 3)

	// Add cover landmark (if cover exists)
	if landmarks.CoverEID > 0 {
		landmarkEntries = append(landmarkEntries,
			NewLandmarkEntry(SymCoverPage, "cover-nav-unit", landmarks.CoverEID))
	}

	// Add TOC landmark (if TOC page exists)
	if landmarks.TOCEID > 0 {
		label := landmarks.TOCLabel
		if label == "" {
			label = "Table of Contents"
		}
		landmarkEntries = append(landmarkEntries,
			NewLandmarkEntry(SymTOC, label, landmarks.TOCEID))
	}

	// Add start reading location (if configured)
	if landmarks.StartEID > 0 {
		landmarkEntries = append(landmarkEntries,
			NewLandmarkEntry(SymSRL, "Start", landmarks.StartEID))
	}

	if len(landmarkEntries) == 0 {
		return nil
	}

	return NewLandmarksContainer(landmarkEntries)
}

// buildNavEntries recursively builds navigation unit entries from TOC entries.
func buildNavEntries(tocEntries []*TOCEntry, startEID int) []any {
	entries := make([]any, 0, len(tocEntries))

	for _, entry := range tocEntries {
		if !entry.IncludeInTOC {
			continue
		}

		// Create target position pointing to the first EID of the content
		targetPos := NewStruct().SetInt(SymUniqueID, int64(entry.FirstEID)) // $155 = id

		// Create nav unit with label and target
		navUnit := NewNavUnit(entry.Title, targetPos)

		// Add nested entries for children (hierarchical TOC)
		if len(entry.Children) > 0 {
			childEntries := buildNavEntries(entry.Children, startEID)
			if len(childEntries) > 0 {
				navUnit.SetList(SymEntries, childEntries) // $247 = nested entries
			}
		}

		entries = append(entries, navUnit)
	}

	return entries
}

// PageEntry represents an approximate page position in the book.
type PageEntry struct {
	PageNumber int   // 1-based page number
	EID        int   // Element ID where this page starts
	Offset     int64 // Offset within the element's text (in runes)
}

// CalculateApproximatePages computes page positions from position items.
// Each page is approximately pageSize runes. Returns page entries with
// EID and offset for navigation.
func CalculateApproximatePages(posItems []PositionItem, pageSize int) []PageEntry {
	if len(posItems) == 0 || pageSize <= 0 {
		return nil
	}

	var pages []PageEntry
	pageNumber := 1
	runesInPage := 0

	for _, item := range posItems {
		itemLen := item.Length
		if itemLen <= 0 {
			itemLen = 1
		}

		// Calculate how many runes we've consumed within this item
		runesConsumed := int64(0)

		for runesConsumed < int64(itemLen) {
			runesRemaining := int64(itemLen) - runesConsumed
			runesNeeded := int64(pageSize - runesInPage)

			if runesInPage == 0 {
				// Start of a new page - record position
				pages = append(pages, PageEntry{
					PageNumber: pageNumber,
					EID:        item.EID,
					Offset:     runesConsumed,
				})
			}

			if runesRemaining >= runesNeeded {
				// This item fills the page
				runesConsumed += runesNeeded
				runesInPage = 0
				pageNumber++
			} else {
				// This item doesn't fill the page
				runesConsumed += runesRemaining
				runesInPage += int(runesRemaining)
			}
		}
	}

	return pages
}

// buildPageListEntries creates navigation entries for APPROXIMATE_PAGE_LIST.
func buildPageListEntries(pages []PageEntry) []any {
	entries := make([]any, 0, len(pages))

	for _, page := range pages {
		// Create target position: {$143: offset, $155: eid}
		targetPos := NewStruct().
			SetInt(SymOffset, page.Offset).      // $143 = offset
			SetInt(SymUniqueID, int64(page.EID)) // $155 = id (EID as int)

		// Create nav unit with page number as label
		navUnit := NewNavUnit(fmt.Sprintf("%d", page.PageNumber), targetPos)

		entries = append(entries, navUnit)
	}

	return entries
}
