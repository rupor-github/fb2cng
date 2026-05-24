package pdf

import (
	"fbc/content"
	"fbc/convert/pdf/structure"
	"fbc/fb2"
)

type pdfDocumentSpec struct {
	PageWidth                      float64
	PageHeight                     float64
	ScreenWidthPx                  int
	ScreenHeightPx                 int
	ScreenDPI                      int
	Title                          string
	Author                         string
	Subject                        string
	Keywords                       string
	Blocks                         []pdfTextBlock
	TOC                            []*structure.TOCEntry
	PrintedFootnotes               map[string]pdfPrintedFootnote
	PageBottomReserves             []float64
	DynamicPrintedFootnoteReserves bool
	DebugPlan                      pdfDebugStructurePlan
	Content                        *content.Content
	Styles                         *pdfStyleResolver
	Images                         fb2.BookImages
	CoverID                        string
	Hyphenator                     paragraphHyphenator
	Fonts                          *pdfFontRegistry
	Debug                          bool
	WorkDir                        string
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

type pdfContentPlan struct {
	Blocks           []pdfTextBlock
	TOC              []*structure.TOCEntry
	PrintedFootnotes map[string]pdfPrintedFootnote
	DebugPlan        pdfDebugStructurePlan
}

type pdfPrintedFootnote struct {
	ID                      string
	TitleBlocks             []pdfTextBlock
	BodyBlocks              []pdfTextBlock
	ContinuationTitleBlocks []pdfTextBlock
	Blocks                  []pdfTextBlock
	ContinuationBlocks      []pdfTextBlock
}

type pdfTextBlock struct {
	Kind                       pdfBlockKind
	ID                         string
	Text                       string
	Runs                       []pdfInlineRun
	Depth                      int
	StyleName                  string
	StyleClasses               string
	ContextClasses             string
	StripRootHorizontalMargins bool
	ImageID                    string
	Links                      []pdfTextLink
	Table                      *fb2.Table
	TableCellRuns              map[pdfTableCellKey][]pdfInlineRun
	BacklinkRefIDs             []string
}

type pdfTableCellKey [2]int

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

type pdfTextLink struct {
	Start int
	End   int
	Href  string
}

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
