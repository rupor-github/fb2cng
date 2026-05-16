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
		Images:         fb2.BookImages{"vignette": vignette, "heading": heading},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses("vignette", "vignette-chapter-title-top", pdfStyleChapterTitle), ImageID: "vignette"},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses(pdfStyleChapterTitleHeader, pdfStyleChapterTitle, pdfStyleHeadingImage), ImageID: "heading"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 2 {
		t.Fatalf("layout pages images = %#v, want title vignette plus heading image", pages)
	}
	gap := pages[1].Images[0].Y - (pages[1].Images[1].Y + pages[1].Images[1].Height)
	if math.Abs(gap-pdfHeadingSpaceBefore) > 0.001 {
		t.Fatalf("title image gap = %v, want %v", gap, pdfHeadingSpaceBefore)
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
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one title image", pages)
	}
	image := pages[1].Images[0]
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
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one vignette image", pages)
	}
	image := pages[1].Images[0]
	if math.Abs(image.X-24) > 0.001 {
		t.Fatalf("title vignette x = %v, want base-margin x 24", image.X)
	}
	if math.Abs(image.Width-(520.0-48.0)) > 0.001 {
		t.Fatalf("title vignette width = %v, want rootless content width %v", image.Width, 520.0-48.0)
	}
}
