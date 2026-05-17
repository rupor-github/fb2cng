package pdf

import (
	"math"
	"testing"

	"fbc/fb2"
)

func TestLayoutPDFPagesKeepsGapBetweenTitleVignetteAndHeadingImage(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	vignette := &fb2.BookImage{}
	vignette.Dim.Width = 120
	vignette.Dim.Height = 10
	heading := &fb2.BookImage{}
	heading.Dim.Width = 380
	heading.Dim.Height = 30

	pages, _, err := layoutPDFPages(skeletonDocument{
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
	}, face)
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

func TestLayoutPDFPagesCentersTitleContentBetweenVignettes(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	vignette := &fb2.BookImage{}
	vignette.Dim.Width = 120
	vignette.Dim.Height = 10

	pages, _, err := layoutPDFPages(skeletonDocument{
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
	}, face)
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

func TestLayoutPDFPagesTitleImageCanStripRootHorizontalMargins(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	heading := &fb2.BookImage{}
	heading.Dim.Width = 380
	heading.Dim.Height = 30
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `html { margin: 0 -10pt 0 -10pt; }`,
	}}}, nil)

	pages, _, err := layoutPDFPages(skeletonDocument{
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
	}, face)
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
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	vignette := &fb2.BookImage{}
	vignette.Dim.Width = 120
	vignette.Dim.Height = 10
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `html { margin: 0 -10pt 0 -10pt; }`,
	}}}, nil)

	pages, _, err := layoutPDFPages(skeletonDocument{
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
	}, face)
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
