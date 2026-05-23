package structure

import "fbc/fb2"

// UnitKind identifies a top-level structural unit in the book plan.
type UnitKind int

const (
	UnitCover UnitKind = iota
	UnitBodyImage
	UnitBodyIntro
	UnitSection
	UnitFootnotesBody
	UnitAnnotation
	UnitTOC
)

// Unit describes one structural rendering unit. Units are intentionally coarse:
// they preserve chapter/page-start semantics but not renderer-specific layout.
type Unit struct {
	Kind         UnitKind
	ID           string
	Title        string
	Depth        int
	TitleDepth   int
	ForceNewPage bool

	Body       *fb2.Body
	Section    *fb2.Section
	IsTopLevel bool
}

// TOCEntry represents a table-of-contents node in a renderer-neutral form.
type TOCEntry struct {
	ID           string
	Title        string
	IncludeInTOC bool
	Children     []*TOCEntry
}

// LandmarkInfo captures notable navigation points for later renderers.
type LandmarkInfo struct {
	CoverID  string
	StartID  string
	TOCID    string
	TOCLabel string
}

// Plan is the renderer-neutral structural representation of the book.
type Plan struct {
	Units     []Unit
	TOC       []*TOCEntry
	Landmarks LandmarkInfo
}
