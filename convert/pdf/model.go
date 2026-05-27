package pdf

import (
	"fbc/content"
	"fbc/convert/pdf/structure"
	"fbc/fb2"
)

// pdfDocumentSpec is the handoff object between content collection, layout,
// resource preparation, and final PDF serialization. Keep it data-oriented:
// callers fill the book-level inputs, layout may add temporary pagination inputs
// such as PageBottomReserves, and buildPDFDocument owns object-number assignment.
type pdfDocumentSpec struct {
	PageWidth  float64
	PageHeight float64

	// Screen* values preserve the original configured raster target. They are used
	// for CSS absolute-unit conversion and for image fitting decisions that need to
	// reason in pixels rather than points.
	ScreenWidthPx  int
	ScreenHeightPx int
	ScreenDPI      int

	Title    string
	Author   string
	Subject  string
	Keywords string

	// Blocks is the flat, already-generated reading order. It includes source FB2
	// content and synthetic blocks such as page breaks, annotation pages, TOC pages,
	// backlinks, and printed-footnote continuations.
	Blocks           []pdfTextBlock
	TOC              []*structure.TOCEntry
	PrintedFootnotes map[string]pdfPrintedFootnote

	// PageBottomReserves is indexed by source page and tells the normal text pass
	// how much vertical space is reserved for printed footnotes on that page.
	PageBottomReserves             []float64
	DynamicPrintedFootnoteReserves bool

	DebugPlan pdfDebugStructurePlan
	Content   *content.Content
	Styles    *pdfStyleResolver
	Images    fb2.BookImages
	CoverID   string

	Hyphenator  paragraphHyphenator
	Fonts       *pdfFontRegistry
	TextShapers *pdfTextShaperCache

	Debug   bool
	WorkDir string
}

type pdfBlockKind int

func (k pdfBlockKind) String() string {
	switch k {
	case pdfBlockParagraph:
		return "paragraph"
	case pdfBlockHeading:
		return "heading"
	case pdfBlockSubtitle:
		return "subtitle"
	case pdfBlockPoem:
		return "poem"
	case pdfBlockTextAuthor:
		return "text-author"
	case pdfBlockEmptyLine:
		return "empty-line"
	case pdfBlockImage:
		return "image"
	case pdfBlockTOCEntry:
		return "toc-entry"
	case pdfBlockPageBreak:
		return "page-break"
	case pdfBlockTable:
		return "table"
	case pdfBlockTableCell:
		return "table-cell"
	default:
		return "unknown"
	}
}

const (
	pdfBlockParagraph pdfBlockKind = iota
	pdfBlockHeading
	pdfBlockSubtitle
	pdfBlockPoem
	pdfBlockTextAuthor
	pdfBlockEmptyLine
	pdfBlockImage
	pdfBlockTOCEntry
	pdfBlockPageBreak
	pdfBlockTable
	pdfBlockTableCell
)

// pdfContentPlan is the content-stage result before physical page layout. It is
// intentionally independent from PDF object numbers and resource names.
type pdfContentPlan struct {
	Blocks           []pdfTextBlock
	TOC              []*structure.TOCEntry
	PrintedFootnotes map[string]pdfPrintedFootnote
	DebugPlan        pdfDebugStructurePlan
}

// pdfPrintedFootnote stores both the logical source note and prebuilt block
// variants used by the float-footnote paginator. Continuation* blocks let later
// chunks render a suitable repeated title without mutating the source blocks.
type pdfPrintedFootnote struct {
	ID                      string
	LabelText               string
	TitleBlocks             []pdfTextBlock
	BodyBlocks              []pdfTextBlock
	ContinuationTitleBlocks []pdfTextBlock
	Blocks                  []pdfTextBlock
	ContinuationBlocks      []pdfTextBlock
}

// pdfTextBlock is the renderer's block-level intermediate representation. Text
// and Runs describe the same content: Text is convenient for searching and plain
// paragraph shaping, while Runs carry inline styling, links, anchors, images, and
// footnote metadata.
type pdfTextBlock struct {
	Kind  pdfBlockKind
	ID    string
	Text  string
	Runs  []pdfInlineRun
	Depth int

	// StyleName selects the element/default style; StyleClasses are explicit
	// classes on this block; ContextClasses are ancestor/container classes used by
	// descendant CSS selectors and inherited margins.
	StyleName      string
	StyleClasses   string
	ContextClasses string

	// StripRootHorizontalMargins lets generated title/vignette/image blocks ignore
	// page-level horizontal body margins without changing vertical content margins.
	StripRootHorizontalMargins bool

	ImageID       string
	Links         []pdfTextLink
	Table         *fb2.Table
	TableCellRuns map[pdfTableCellKey][]pdfInlineRun

	// BacklinkRefIDs is kept so backlink text can be regenerated after pagination
	// once the referenced source page numbers are known.
	BacklinkRefIDs []string
}

type pdfTableCellKey [2]int

// pdfInlineRun preserves inline semantics inside a block. Runs are later mapped
// to paragraph fragments, where font fallback, shaping, link rectangles, and
// baseline shifts are resolved.
type pdfInlineRun struct {
	Text           string
	StyleClasses   string
	ContextClasses string
	LinkHref       string
	AnchorID       string
	FootnoteID     string
	ImageID        string
	Bold           bool
	Italic         bool
	Underline      bool
	Strikethrough  bool
	Subscript      bool
	Superscript    bool
	Code           bool
}

// pdfTextLink is a plain-text rune range used for links that are easier to
// describe before inline runs are split, such as generated TOC entries.
type pdfTextLink struct {
	Start int
	End   int
	Href  string
}

// pdfPageLine is a positioned baseline in the logical page display list. Simple
// lines use Text and the line-level font fields; mixed inline styling uses
// Fragments with per-fragment font and link metadata.
type pdfPageLine struct {
	X                float64
	Y                float64
	FontSize         float64
	LetterSpacing    float64
	FontKey          pdfFontKey
	FontName         string
	Color            pdfColor
	Underline        bool
	Strikethrough    bool
	Text             shapedText
	Fragments        []pdfPageLineFragment
	ExtraWordSpacing float64
	ExtraCharSpacing float64
	BreakStats       paragraphLineBreakStats
}

type pdfPageLineFragment struct {
	Text          shapedText
	Width         float64
	FontSize      float64
	LetterSpacing float64
	FontKey       pdfFontKey
	FontName      string
	Color         pdfColor
	Underline     bool
	Strikethrough bool
	BaselineShift float64
	StyleClasses  string
	LinkHref      string
	AnchorID      string
	FootnoteID    string
	ImageID       string
	ImageHeight   float64
}

// pdfPage is a logical display list produced by pagination. ObjectID and
// ContentID are filled only after all pages, resources, outlines, and annotations
// are known.
type pdfPage struct {
	ObjectID    int
	ContentID   int
	Backgrounds []pdfPageRect
	Borders     []pdfPageBorder
	Strokes     []pdfPageStroke
	Lines       []pdfPageLine
	Images      []pdfPageImage
	Anchors     []string
	Annotations []pdfLinkAnnotation
}

type pdfPageRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Color  pdfColor
}

type pdfPageBorder struct {
	X         float64
	Y         float64
	Width     float64
	Height    float64
	LineWidth float64
	Color     pdfColor
}

type pdfPageStroke struct {
	X1        float64
	Y1        float64
	X2        float64
	Y2        float64
	LineWidth float64
	Color     pdfColor
}

type pdfPageImage struct {
	ImageID string
	Name    string
	X       float64
	Y       float64
	Width   float64
	Height  float64
}

type pdfLinkAnnotation struct {
	ObjectID int
	Rect     pdfRect
	Href     string
}

type pdfRect struct {
	X1 float64
	Y1 float64
	X2 float64
	Y2 float64
}
