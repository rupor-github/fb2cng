package kfx

import (
	"strings"
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
// Based on KP3 reference: {$155: eid, $159: $269, $176: storyline_name}
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

	// Container tracking for margin collapsing.
	// containerIDCounter generates unique IDs for each container.
	// marginContainerStack tracks the current container hierarchy.
	// containerMarginsMap stores container margins by ID (persists after ExitContainer).
	containerIDCounter   int
	marginContainerStack []marginContainerInfo
	containerMarginsMap  map[int]containerMargins
	// containerHierarchy stores parent, kind, and flags for each container.
	// This persists after ExitContainer so tree building can reconstruct the hierarchy.
	containerHierarchy map[int]containerHierarchyInfo
	// entryOrderCounter tracks the order of content entries and container entries.
	// This is used to correctly order siblings in the margin collapsing tree.
	entryOrderCounter int
}

// containerMargins stores the CSS margins for a container.
type containerMargins struct {
	marginTop    float64
	marginBottom float64
}

// containerHierarchyInfo stores the hierarchy information for a container.
type containerHierarchyInfo struct {
	parentID   int
	kind       ContainerKind
	flags      ContainerFlags
	entryOrder int // Order in which this container was entered (for sibling ordering)
}

// marginContainerInfo holds information about a container in the margin collapsing stack.
type marginContainerInfo struct {
	id       int            // Unique container ID
	kind     ContainerKind  // Type of container
	flags    ContainerFlags // Flags controlling collapse behavior
	marginMT float64        // Container's margin-top (in lh units)
	marginMB float64        // Container's margin-bottom (in lh units)
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

// EnterContainer pushes a new container onto the margin collapsing stack.
// Returns the unique container ID that should be set on content entries
// created within this container.
//
// Parameters:
//   - kind: The type of container (ContainerSection, ContainerPoem, etc.)
//   - flags: Flags controlling collapse behavior (FlagTitleBlockMode, etc.)
//
// Call ExitContainer() when done adding content to this container.
func (sb *StorylineBuilder) EnterContainer(kind ContainerKind, flags ContainerFlags) int {
	sb.containerIDCounter++
	id := sb.containerIDCounter

	// Get parent ID from the current top of stack (0 if root)
	parentID := 0
	if len(sb.marginContainerStack) > 0 {
		parentID = sb.marginContainerStack[len(sb.marginContainerStack)-1].id
	}

	sb.marginContainerStack = append(sb.marginContainerStack, marginContainerInfo{
		id:    id,
		kind:  kind,
		flags: flags,
	})

	// Increment entry order counter - this tracks the order of containers and content
	sb.entryOrderCounter++

	// Store hierarchy info for tree building (persists after ExitContainer)
	if sb.containerHierarchy == nil {
		sb.containerHierarchy = make(map[int]containerHierarchyInfo)
	}
	sb.containerHierarchy[id] = containerHierarchyInfo{
		parentID:   parentID,
		kind:       kind,
		flags:      flags,
		entryOrder: sb.entryOrderCounter,
	}

	return id
}

// ExitContainer pops the current container from the margin collapsing stack.
// Must be called after all content for the container has been added.
func (sb *StorylineBuilder) ExitContainer() {
	if len(sb.marginContainerStack) > 0 {
		sb.marginContainerStack = sb.marginContainerStack[:len(sb.marginContainerStack)-1]
	}
}

// SetContainerMargins sets the margin-top and margin-bottom for the current container.
// This should be called after EnterContainer() once the container's CSS margins are known
// (typically after PushBlock() which resolves the container's CSS).
//
// The margins are stored and will be used by post-processing margin collapsing to
// distribute container margins to first/last children according to CSS rules.
//
// mt, mb: margin values in lh units (0 means no margin)
func (sb *StorylineBuilder) SetContainerMargins(mt, mb float64) {
	if len(sb.marginContainerStack) == 0 {
		return
	}
	idx := len(sb.marginContainerStack) - 1
	containerID := sb.marginContainerStack[idx].id
	sb.marginContainerStack[idx].marginMT = mt
	sb.marginContainerStack[idx].marginMB = mb

	// Also store in the persistent map for use during tree building
	if sb.containerMarginsMap == nil {
		sb.containerMarginsMap = make(map[int]containerMargins)
	}
	sb.containerMarginsMap[containerID] = containerMargins{
		marginTop:    mt,
		marginBottom: mb,
	}
}

// GetContainerMargins returns the margins for a container by ID.
// Returns (0, 0) if the container is not found or has no margins set.
func (sb *StorylineBuilder) GetContainerMargins(containerID int) (mt, mb float64) {
	if sb.containerMarginsMap != nil {
		if m, ok := sb.containerMarginsMap[containerID]; ok {
			return m.marginTop, m.marginBottom
		}
	}
	return 0, 0
}

// CurrentContainerID returns the ID of the current container (top of stack).
// Returns 0 if no container is active (root level).
func (sb *StorylineBuilder) CurrentContainerID() int {
	if len(sb.marginContainerStack) == 0 {
		return 0 // Root level
	}
	return sb.marginContainerStack[len(sb.marginContainerStack)-1].id
}

// CurrentContainerKind returns the kind of the current container.
// Returns ContainerRoot if no container is active.
func (sb *StorylineBuilder) CurrentContainerKind() ContainerKind {
	if len(sb.marginContainerStack) == 0 {
		return ContainerRoot
	}
	return sb.marginContainerStack[len(sb.marginContainerStack)-1].kind
}

// CurrentContainerFlags returns the flags of the current container.
// Returns 0 if no container is active.
func (sb *StorylineBuilder) CurrentContainerFlags() ContainerFlags {
	if len(sb.marginContainerStack) == 0 {
		return 0
	}
	return sb.marginContainerStack[len(sb.marginContainerStack)-1].flags
}

// ParentContainerID returns the ID of the parent container.
// Returns 0 if the current container is at root level or there's only one container.
func (sb *StorylineBuilder) ParentContainerID() int {
	if len(sb.marginContainerStack) <= 1 {
		return 0 // Root or only one container
	}
	return sb.marginContainerStack[len(sb.marginContainerStack)-2].id
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

// PageTemplateEID returns the EID allocated for the page template container.
func (sb *StorylineBuilder) PageTemplateEID() int {
	return sb.pageTemplateEID
}

// MarkPreviousEntryStripMB marks the previous content entry to have its margin-bottom stripped.
// This is called when an empty-line is encountered, matching KP3 behavior where the preceding
// element loses its mb and the empty-line's margin goes to the next element's mt.
// Does nothing if there are no previous entries.
func (sb *StorylineBuilder) MarkPreviousEntryStripMB() {
	if len(sb.contentEntries) == 0 {
		return
	}
	prevIdx := len(sb.contentEntries) - 1
	sb.contentEntries[prevIdx].StripMarginBottom = true
}

// Build creates the storyline and section fragments.
// Returns storyline fragment, section fragment.
//
// Before building, resolves deferred styles for all content entries.
// Top-level entries are resolved WITHOUT position filtering (they keep all margins)
// because they are not fragmented. Children within wrapper blocks get position-based
// margin filtering.
//
// After style resolution, applies CSS margin collapsing as a post-processing step
// to match KP3 behavior.
func (sb *StorylineBuilder) Build() (*Fragment, *Fragment) {
	// Apply position-based style filtering to top-level entries
	sb.applyStorylinePositionFiltering()

	// Capture margin values from resolved styles for post-processing.
	// This must be done after style resolution but before margin collapsing.
	sb.captureMargins()

	// Apply CSS margin collapsing as post-processing (matches KP3 behavior).
	// This builds a tree from the flat content entries, collapses margins
	// according to CSS rules, and creates new style variants with the collapsed values.
	tree := sb.CollapseMargins()
	sb.applyCollapsedMargins(tree)

	// Rebuild Children for wrapper entries after margin collapsing.
	// The Children array was built before margin collapsing (in EndBlock or resolveChildStyles),
	// so it has old styles. The childRefs have been updated by applyCollapsedMargins,
	// so we need to rebuild Children from childRefs to get the new styles.
	sb.rebuildWrapperChildren()

	// Mark usage for all deferred styles (those with StyleSpec) after position filtering
	// AND after margin collapsing (which may have created new style variants)
	if sb.styles != nil {
		for _, ref := range sb.contentEntries {
			if ref.StyleSpec != "" && ref.Style != "" {
				sb.styles.ResolveStyle(ref.Style, styleUsageText)
			}
		}
	}

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

// parseStyleSpec parses a style specification into tag and classes.
// StyleSpec format: "tag class1 class2" where tag is an HTML element (h1-h6, p, etc.)
// or just "class1 class2" for non-element styles.
// Returns (tag, "class1 class2") or ("", "class1 class2") if no tag.
func parseStyleSpec(spec string) (tag, classes string) {
	parts := strings.Fields(spec)
	if len(parts) == 0 {
		return "", ""
	}

	// Check if first part is an HTML element tag
	first := parts[0]
	if isHTMLTag(first) {
		tag = first
		if len(parts) > 1 {
			classes = strings.Join(parts[1:], " ")
		}
		return tag, classes
	}

	// No tag, all parts are classes
	return "", spec
}

// isHTMLTag returns true if the string is a known HTML element tag.
func isHTMLTag(s string) bool {
	switch s {
	case "h1", "h2", "h3", "h4", "h5", "h6", "p", "div", "span",
		"pre", "code", "blockquote", "ol", "ul", "li",
		"table", "tr", "td", "th", "img",
		"strong", "b", "em", "i", "u", "s", "sub", "sup":
		return true
	}
	return false
}
