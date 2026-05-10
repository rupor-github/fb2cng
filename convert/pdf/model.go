package pdf

import (
	"fbc/convert/structure"
	"fbc/fb2"
)

type skeletonDocument struct {
	PageWidth      float64
	PageHeight     float64
	ScreenWidthPx  int
	ScreenHeightPx int
	Title          string
	Author         string
	Subject        string
	Keywords       string
	Blocks         []pdfTextBlock
	TOC            []*structure.TOCEntry
	DebugPlan      pdfDebugStructurePlan
	Styles         *pdfStyleResolver
	Images         fb2.BookImages
	CoverID        string
	Hyphenator     paragraphHyphenator
	Fonts          *pdfFontRegistry
	Debug          bool
	WorkDir        string
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
)

type pdfContentPlan struct {
	Blocks    []pdfTextBlock
	TOC       []*structure.TOCEntry
	DebugPlan pdfDebugStructurePlan
}

type pdfTextBlock struct {
	Kind         pdfBlockKind
	ID           string
	Text         string
	Depth        int
	StyleName    string
	StyleClasses string
	ImageID      string
	Links        []pdfTextLink
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
	FontKey          pdfFontKey
	FontName         string
	Text             shapedText
	ExtraWordSpacing float64
}

type pdfPage struct {
	ObjectID    int
	ContentID   int
	Lines       []pdfPageLine
	Images      []pdfPageImage
	Anchors     []string
	Annotations []pdfLinkAnnotation
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
