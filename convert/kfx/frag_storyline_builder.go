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

// Build creates the storyline and section fragments.
// Returns storyline fragment, section fragment.
//
// Before building, resolves deferred styles for all content entries.
// Top-level entries are resolved WITHOUT position filtering (they keep all margins)
// because they are not fragmented. Children within wrapper blocks get position-based
// margin filtering.
func (sb *StorylineBuilder) Build() (*Fragment, *Fragment) {
	// Apply position-based style filtering to top-level entries
	sb.applyStorylinePositionFiltering()

	// Mark usage for all deferred styles (those with StyleSpec) after position filtering
	// This ensures we only mark the final resolved styles, not pre-filtered versions
	if sb.styles != nil {
		for _, ref := range sb.contentEntries {
			if ref.StyleSpec != "" && ref.Style != "" {
				sb.styles.MarkUsage(ref.Style, styleUsageText)
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
