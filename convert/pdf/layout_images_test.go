package pdf

import (
	"math"
	"testing"

	"fbc/fb2"
)

func TestLayoutPDFPagesUpscalesVignettesToContentWidth(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 120
	img.Dim.Height = 10

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:      520,
		PageHeight:     220,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Images:         fb2.BookImages{"vignette": img},
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockImage,
			StyleName:    pdfStyleImage,
			StyleClasses: "vignette vignette-chapter-title-top",
			ImageID:      "vignette",
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one vignette image", pages)
	}
	if got, want := pages[1].Images[0].Width, 520.0-48.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("vignette width = %v, want content width %v", got, want)
	}
}

func TestLayoutPDFPagesLeftAlignsImageOnlyParagraphsInsideCite(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 100
	img.Dim.Height = 80
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{
		Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
	}
	resolver.styles[pdfStyleCite] = pdfBlockResolvedStyle{MarginLeft: 21, MarginRight: 21}

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockImage,
			StyleName:    pdfStyleParagraph,
			StyleClasses: pdfStyleCite,
			ImageID:      "block",
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one cite image", pages)
	}
	if got, want := pages[1].Images[0].X, 24.0+21.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("cite image x = %v, want left-aligned at %v", got, want)
	}
}

func TestLayoutPDFPagesSizesBlockImagesLikeKP3(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 100
	img.Dim.Height = 50

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Images:     fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:    pdfBlockImage,
			ImageID: "block",
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one block image", pages)
	}
	contentWidth := 520.0 - 48.0
	wantWidth := contentWidth * 100.0 / pdfKP3ContentWidthPx
	if got := pages[1].Images[0].Width; math.Abs(got-wantWidth) > 0.001 {
		t.Fatalf("block image width = %v, want KP3-style width %v", got, wantWidth)
	}
	wantHeight := wantWidth / 2
	if got := pages[1].Images[0].Height; math.Abs(got-wantHeight) > 0.001 {
		t.Fatalf("block image height = %v, want aspect-preserving height %v", got, wantHeight)
	}
}

func TestLayoutPDFPagesAvoidsBlankPageBeforeTallImageAfterEmptyLine(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 600
	img.Dim.Height = 800

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Images:     fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "Intro paragraph."},
			{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine},
			{Kind: pdfBlockImage, ID: "img", ImageID: "block"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("layout pages = %#v, want title page + text page + image page", pages)
	}
	if len(pages[1].Lines) == 0 || len(pages[1].Images) != 0 {
		t.Fatalf("text page = %#v, want only intro text", pages[1])
	}
	if len(pages[2].Images) != 1 || len(pages[2].Lines) != 0 {
		t.Fatalf("image page = %#v, want only image", pages[2])
	}
	if len(pages[2].Anchors) != 1 || pages[2].Anchors[0] != "img" {
		t.Fatalf("image anchors = %#v, want image anchor on rendered image page", pages[2].Anchors)
	}
}

func TestLayoutPDFPagesRendersImageOnlyHeadings(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 380
	img.Dim.Height = 30

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:      520,
		PageHeight:     180,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Images:         fb2.BookImages{"heading": img},
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockImage,
			StyleName:    pdfStyleImage,
			StyleClasses: pdfStyleHeadingImage,
			ImageID:      "heading",
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want image-only heading image", pages)
	}
	if len(pages[1].Lines) != 0 {
		t.Fatalf("heading lines = %#v, want image-only heading rendered as block image", pages[1].Lines)
	}
	if got, want := pages[1].Images[0].Width, 520.0-48.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("heading image width = %v, want content width %v", got, want)
	}
}

func TestLayoutPDFPagesKeepsGapAfterImageOnlySubtitle(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 380
	img.Dim.Height = 30

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:      520,
		PageHeight:     180,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Images:         fb2.BookImages{"subtitle": img},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: pdfStyleSubtitle, ImageID: "subtitle"},
			{Kind: pdfBlockParagraph, Text: "Body after subtitle image."},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 || len(pages[1].Lines) == 0 {
		t.Fatalf("layout pages = %#v, want image plus following text", pages)
	}
	wantWidth := (520.0 - 48.0) * 380.0 / pdfKP3ContentWidthPx
	if got := pages[1].Images[0].Width; math.Abs(got-wantWidth) > 0.001 {
		t.Fatalf("subtitle image width = %v, want KP3-style width %v", got, wantWidth)
	}
	gap := pages[1].Images[0].Y - pages[1].Lines[0].Y
	if math.Abs(gap-(pdfSubtitleSpaceAfter+pdfBaseFontSize)) > 0.001 {
		t.Fatalf("subtitle image gap = %v, want %v", gap, pdfSubtitleSpaceAfter+pdfBaseFontSize)
	}
}
