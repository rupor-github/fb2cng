package pdf

import (
	"math"
	"testing"

	"fbc/fb2"
)

func TestEffectiveParagraphLineHeightClampsToFontSize(t *testing.T) {
	style := paragraphStyle{FontSize: 24, LineHeight: 10}
	if got := pdfEffectiveParagraphLineHeight(style); got != 24 {
		t.Fatalf("effective line height = %v, want font size 24", got)
	}
	style.LineHeight = 30
	if got := pdfEffectiveParagraphLineHeight(style); got != 30 {
		t.Fatalf("effective line height = %v, want explicit line height 30", got)
	}
}

func TestLayoutPDFPagesKeepsGapBetweenTitleVignetteAndHeadingImage(t *testing.T) {
	vignette := &fb2.BookImage{}
	vignette.Dim.Width = 120
	vignette.Dim.Height = 10
	heading := &fb2.BookImage{}
	heading.Dim.Width = 380
	heading.Dim.Height = 30

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     220,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         newPDFStyleResolverWithDefaultCSS(t),
		Images:         fb2.BookImages{"vignette": vignette, "heading": heading},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("vignette", "vignette-chapter-title-top", pdfStyleChapterTitle), ImageID: "vignette"},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses(pdfStyleChapterTitleHeader, pdfStyleChapterTitle, pdfStyleHeadingImage), ImageID: "heading"},
		},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 2 {
		t.Fatalf("layout pages images = %#v, want title vignette plus heading image", pages)
	}
	gap := pages[0].Images[0].Y - (pages[0].Images[1].Y + pages[0].Images[1].Height)
	wantGap := headingPDFStyle(1).SpaceBefore
	if math.Abs(gap-wantGap) > 0.001 {
		t.Fatalf("title image gap = %v, want %v", gap, wantGap)
	}
}

func TestLayoutPDFPagesUsesTightTitleHeaderLineFlow(t *testing.T) {

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     260,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         newPDFStyleResolverWithDefaultCSS(t),
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockHeading, Text: "One", Depth: 1, StyleName: pdfStyleChapterTitleHeader, StyleClasses: joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-first")},
			{Kind: pdfBlockHeading, Text: "Two", Depth: 1, StyleName: pdfStyleChapterTitleHeader, StyleClasses: joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-next")},
			{Kind: pdfBlockHeading, Text: "Three", Depth: 1, StyleName: pdfStyleChapterTitleHeader, StyleClasses: joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-next")},
		},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Lines) != 3 {
		t.Fatalf("layout pages = %#v, want three title lines", pages)
	}
	for i := 1; i < len(pages[0].Lines); i++ {
		gap := pages[0].Lines[i-1].Y - pages[0].Lines[i].Y
		if math.Abs(gap-pdfAdjustedLineHeight) > 0.001 {
			t.Fatalf("title baseline gap %d = %v, want one KP3 title line-height %v", i, gap, pdfAdjustedLineHeight)
		}
	}
}

func TestLayoutPDFPagesCentersTitleContentBetweenVignettes(t *testing.T) {
	vignette := &fb2.BookImage{}
	vignette.Dim.Width = 120
	vignette.Dim.Height = 10

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     420,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         newPDFStyleResolverWithDefaultCSS(t),
		Images:         fb2.BookImages{"top": vignette, "bottom": vignette},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("vignette", "vignette-book-title-top", pdfStyleBodyTitle), ImageID: "top"},
			{Kind: pdfBlockHeading, Text: "Author", Depth: 1, StyleName: pdfStyleBodyTitleHeader, StyleClasses: joinStyleClasses(pdfStyleBodyTitle, pdfStyleBodyTitleHeader+"-first")},
			{Kind: pdfBlockHeading, Text: "Book", Depth: 1, StyleName: pdfStyleBodyTitleHeader, StyleClasses: joinStyleClasses(pdfStyleBodyTitle, pdfStyleBodyTitleHeader+"-next")},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("vignette", "vignette-book-title-bottom", pdfStyleBodyTitle), ImageID: "bottom"},
		},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 2 || len(pages[0].Lines) != 2 {
		t.Fatalf("layout pages = %#v, want two title lines between two vignettes", pages)
	}
	visualTop, visualBottom, ok := pdfPageTitleContentVisualBounds(&pages[0], 0, len(pages[0].Lines), 1, 1, nil)
	if !ok {
		t.Fatalf("title content has no visual bounds: %#v", pages[0])
	}
	gotCenter := (visualTop + visualBottom) / 2
	wantCenter := (pages[0].Images[0].Y + pages[0].Images[1].Y + pages[0].Images[1].Height) / 2
	if math.Abs(gotCenter-wantCenter) > 0.001 {
		t.Fatalf("title visual center = %v, want center between vignettes %v (visual bounds %v/%v)", gotCenter, wantCenter, visualTop, visualBottom)
	}
}

func TestLayoutPDFPagesDoesNotMoveBottomTitleVignetteForTooTallFollowingImage(t *testing.T) {
	vignette := &fb2.BookImage{}
	vignette.Dim.Width = 120
	vignette.Dim.Height = 10
	following := &fb2.BookImage{}
	following.Dim.Width = 200
	following.Dim.Height = 260

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     300,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         newPDFStyleResolverWithDefaultCSS(t),
		Images:         fb2.BookImages{"top": vignette, "bottom": vignette, "following": following},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("vignette", "vignette-chapter-title-top", pdfStyleChapterTitle), ImageID: "top"},
			{Kind: pdfBlockHeading, Text: "Part One", Depth: 1, StyleName: pdfStyleChapterTitleHeader, StyleClasses: joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-first")},
			{Kind: pdfBlockHeading, Text: "In the Rear", Depth: 1, StyleName: pdfStyleChapterTitleHeader, StyleClasses: joinStyleClasses(pdfStyleChapterTitle, pdfStyleChapterTitleHeader+"-next")},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("vignette", "vignette-chapter-title-bottom", pdfStyleChapterTitle), ImageID: "bottom"},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: "image-block", ImageID: "following"},
		},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("layout page count = %d, want title page plus following image page: %#v", len(pages), pages)
	}
	if len(pages[0].Images) != 2 || pages[0].Images[1].ImageID != "bottom" {
		t.Fatalf("first page images = %#v, want top and bottom title vignettes together", pages[0].Images)
	}
	if len(pages[1].Images) != 1 || pages[1].Images[0].ImageID != "following" {
		t.Fatalf("second page images = %#v, want following oversized image", pages[1].Images)
	}
}

func TestLayoutPDFPagesTitleImageCanStripRootHorizontalMargins(t *testing.T) {
	heading := &fb2.BookImage{}
	heading.Dim.Width = 380
	heading.Dim.Height = 30
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `html { margin: 0 -10pt 0 -10pt; }`,
	}}}, nil)

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     220,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         resolver,
		Images:         fb2.BookImages{"heading": heading},
		Blocks: []pdfTextBlock{{
			Kind:                       pdfBlockImage,
			StyleName:                  pdfStyleImage,
			StyleClasses:               joinStyleClasses(pdfStyleChapterTitleHeader, pdfStyleChapterTitle, pdfStyleHeadingImage),
			StripRootHorizontalMargins: true,
			ImageID:                    "heading",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one title image", pages)
	}
	image := pages[0].Images[0]
	if math.Abs(image.X-24) > 0.001 {
		t.Fatalf("title image x = %v, want base-margin x 24", image.X)
	}
	if math.Abs(image.Width-(520.0-48.0)) > 0.001 {
		t.Fatalf("title image width = %v, want rootless content width %v", image.Width, 520.0-48.0)
	}
}

func TestLayoutPDFPagesTitleVignetteCanStripRootHorizontalMargins(t *testing.T) {
	vignette := &fb2.BookImage{}
	vignette.Dim.Width = 120
	vignette.Dim.Height = 10
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `html { margin: 0 -10pt 0 -10pt; }`,
	}}}, nil)

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     220,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         resolver,
		Images:         fb2.BookImages{"vignette": vignette},
		Blocks: []pdfTextBlock{{
			Kind:                       pdfBlockImage,
			StyleName:                  pdfStyleImage,
			StyleClasses:               joinStyleClasses("vignette", "vignette-chapter-title-top", pdfStyleChapterTitle),
			StripRootHorizontalMargins: true,
			ImageID:                    "vignette",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one vignette image", pages)
	}
	image := pages[0].Images[0]
	if math.Abs(image.X-24) > 0.001 {
		t.Fatalf("title vignette x = %v, want base-margin x 24", image.X)
	}
	if math.Abs(image.Width-(520.0-48.0)) > 0.001 {
		t.Fatalf("title vignette width = %v, want rootless content width %v", image.Width, 520.0-48.0)
	}
}
