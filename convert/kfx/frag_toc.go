package kfx

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
