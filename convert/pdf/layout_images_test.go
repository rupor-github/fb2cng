package pdf

import (
	"math"
	"testing"

	"fbc/fb2"
)

func TestLayoutPDFPagesUpscalesVignettesToContentWidth(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 120
	img.Dim.Height = 10

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one vignette image", pages)
	}
	if got, want := pages[0].Images[0].Width, 520.0-48.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("vignette width = %v, want content width %v", got, want)
	}
}

func TestLayoutPDFPagesDoesNotApplyTextIndentToImageOnlyParagraphs(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 100
	img.Dim.Height = 80
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{
		Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, HasFirstLineIndent: true, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
	}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  520,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:      pdfBlockImage,
			StyleName: pdfStyleParagraph,
			ImageID:   "block",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one image-only paragraph", pages)
	}
	if got, want := pages[0].Images[0].X, 24.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("image-only paragraph x = %v, want left edge %v without text indent", got, want)
	}
}

func TestLayoutPDFPagesLeftAlignsImageOnlyParagraphsInsideCite(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 100
	img.Dim.Height = 80
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{
		Paragraph: paragraphStyle{FontFamily: "serif", FontSize: pdfBaseFontSize, LineHeight: pdfBaseLineHeight, FirstLineIndent: pdfBodyIndent, Align: textAlignJustify, Hyphenation: paragraphHyphenationAuto},
	}
	resolver.styles[pdfStyleCite] = pdfBlockResolvedStyle{MarginLeft: 21, MarginRight: 21}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one cite image", pages)
	}
	if got, want := pages[0].Images[0].X, 24.0+21.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("cite image x = %v, want left-aligned at %v", got, want)
	}
}

func TestLayoutPDFPagesSizesBlockImagesLikeKP3(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 100
	img.Dim.Height = 50

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     220,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Images:         fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:    pdfBlockImage,
			ImageID: "block",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one block image", pages)
	}
	contentWidth := 520.0 - 48.0
	wantWidth := contentWidth * 100.0 / pdfKP3ContentWidthPx
	if got := pages[0].Images[0].Width; math.Abs(got-wantWidth) > 0.001 {
		t.Fatalf("block image width = %v, want KP3-style width %v", got, wantWidth)
	}
	wantHeight := wantWidth / 2
	if got := pages[0].Images[0].Height; math.Abs(got-wantHeight) > 0.001 {
		t.Fatalf("block image height = %v, want aspect-preserving height %v", got, wantHeight)
	}
}

func TestLayoutPDFPagesSizesPartialWidthBlockImagesAgainstRootlessWidth(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 200
	img.Dim.Height = 200
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `html { margin: 0 -12pt 0 -12pt; }`,
	}}}, nil)

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     300,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         resolver,
		Images:         fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:    pdfBlockImage,
			ImageID: "block",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one block image", pages)
	}
	rootlessContentWidth := 520.0 - 48.0
	wantWidth := rootlessContentWidth * 200.0 / pdfKP3ContentWidthPx
	if got := pages[0].Images[0].Width; math.Abs(got-wantWidth) > 0.001 {
		t.Fatalf("partial block image width = %v, want KP3 rootless width %v", got, wantWidth)
	}
	contentLeft := 12.0
	contentWidth := rootlessContentWidth + 24.0
	wantX := contentLeft + max((contentWidth-wantWidth)/2, 0)
	if got := pages[0].Images[0].X; math.Abs(got-wantX) > 0.001 {
		t.Fatalf("partial block image x = %v, want centered in expanded content at %v", got, wantX)
	}
}

func TestLayoutPDFPagesCapsLargeGIFBlockImagesByConfiguredScreen(t *testing.T) {
	img := &fb2.BookImage{MimeType: "image/gif"}
	img.Dim.Width = 600
	img.Dim.Height = 300

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     220,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Images:         fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:    pdfBlockImage,
			ImageID: "block",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one block image", pages)
	}
	wantWidth := 520.0 * 600.0 / 1200.0
	if got := pages[0].Images[0].Width; math.Abs(got-wantWidth) > 0.001 {
		t.Fatalf("large block image width = %v, want screen-sized width %v", got, wantWidth)
	}
	wantHeight := wantWidth / 2
	if got := pages[0].Images[0].Height; math.Abs(got-wantHeight) > 0.001 {
		t.Fatalf("large GIF block image height = %v, want aspect-preserving height %v", got, wantHeight)
	}
}

func TestLayoutPDFPagesClampsLargePNGBlockImagesLikeKP3(t *testing.T) {
	img := &fb2.BookImage{MimeType: "image/png"}
	img.Dim.Width = 600
	img.Dim.Height = 300

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     400,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Images:         fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:    pdfBlockImage,
			ImageID: "block",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one block image", pages)
	}
	wantWidth := 520.0 - 48.0
	if got := pages[0].Images[0].Width; math.Abs(got-wantWidth) > 0.001 {
		t.Fatalf("large PNG block image width = %v, want KP3 clamped width %v", got, wantWidth)
	}
}

func TestLayoutPDFPagesSizesFullWidthBlockImagesAgainstRootlessWidth(t *testing.T) {
	img := &fb2.BookImage{MimeType: "image/png"}
	img.Dim.Width = 580
	img.Dim.Height = 458
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `html { margin: 0 -12pt 0 -12pt; }`,
	}}}, nil)

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:      520,
		PageHeight:     500,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Styles:         resolver,
		Images:         fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{{
			Kind:    pdfBlockImage,
			ImageID: "block",
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one block image", pages)
	}
	rootlessContentWidth := 520.0 - 48.0
	if got := pages[0].Images[0].Width; math.Abs(got-rootlessContentWidth) > 0.001 {
		t.Fatalf("full-width block image width = %v, want rootless KP3 width %v", got, rootlessContentWidth)
	}
	contentLeft := 12.0
	contentWidth := rootlessContentWidth + 24.0
	wantX := contentLeft + max((contentWidth-rootlessContentWidth)/2, 0)
	if got := pages[0].Images[0].X; math.Abs(got-wantX) > 0.001 {
		t.Fatalf("full-width block image x = %v, want centered in expanded content at %v", got, wantX)
	}
}

func TestLayoutPDFPagesKeepsNearFittingImageWithSmallBottomOverflow(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 511
	img.Dim.Height = 423
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignLeft}}
	resolver.styles[pdfStyleImage] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignCenter}, KeepTogether: true}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "one"},
			{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "two"},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, ImageID: "block"},
		},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages = %#v, want text and near-fitting image on one page", pages)
	}
	if overflow := 24.0 - pages[0].Images[0].Y; overflow <= 0 || overflow > pdfBlockImageBottomFitOverflow {
		t.Fatalf("image bottom overflow = %v, want small allowed overflow <= %v", overflow, pdfBlockImageBottomFitOverflow)
	}
}

func TestLayoutPDFPagesBreaksImagePastBottomOverflowTolerance(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 511
	img.Dim.Height = 440
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignLeft}}
	resolver.styles[pdfStyleImage] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignCenter}, KeepTogether: true}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "one"},
			{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "two"},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, ImageID: "block"},
		},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[0].Images) != 0 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages = %#v, want image moved when overflow exceeds tolerance", pages)
	}
}

func TestLayoutPDFPagesDoesNotKeepImageWithNextWhenAvoidIsAbsent(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 512
	img.Dim.Height = 216
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignLeft}}
	resolver.styles[pdfStyleImage] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignCenter}, SpaceBefore: 10, KeepTogether: true}
	resolver.styles["after"] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignLeft}, SpaceBefore: 30}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  520,
		PageHeight: 300,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"block": img},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "before"},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, ImageID: "block"},
			{Kind: pdfBlockParagraph, StyleName: "after", Text: "after"},
		},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("layout pages = %#v, want image kept on first page and following paragraph on second", pages)
	}
	if len(pages[0].Images) != 1 || len(pages[0].Lines) != 1 {
		t.Fatalf("first page = %#v, want before text plus image", pages[0])
	}
	if len(pages[1].Images) != 0 || len(pages[1].Lines) != 1 {
		t.Fatalf("second page = %#v, want following paragraph only", pages[1])
	}
}

func TestLayoutPDFPagesDoesNotCarryPreviousEmptyLineMarginToImagePage(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 511
	img.Dim.Height = 350
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStyleParagraph] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignLeft}}
	resolver.styles[pdfStyleEmptyLine] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 12}, SpaceBefore: 10}
	resolver.styles[pdfStyleImage] = pdfBlockResolvedStyle{Paragraph: paragraphStyle{FontSize: 10, LineHeight: 10, Align: textAlignCenter}, KeepTogether: true}
	blocks := []pdfTextBlock{
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "one"},
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "two"},
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "three"},
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "four"},
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "five"},
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "six"},
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "seven"},
		{Kind: pdfBlockParagraph, StyleName: pdfStyleParagraph, Text: "eight"},
		{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine},
		{Kind: pdfBlockImage, StyleName: pdfStyleImage, ImageID: "block"},
	}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"block": img},
		Blocks:     blocks,
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages = %#v, want text page plus image page", pages)
	}
	wantHeight := (220.0 - 48.0) * 350.0 / pdfKP3ContentWidthPx
	wantY := 220.0 - 24.0 - wantHeight
	if got := pages[1].Images[0].Y; math.Abs(got-wantY) > 0.001 {
		t.Fatalf("image y on fresh page = %v, want top without carried empty-line margin %v", got, wantY)
	}
}

func TestLayoutPDFPagesAvoidsBlankPageBeforeTallImageAfterEmptyLine(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 600
	img.Dim.Height = 800

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("layout pages = %#v, want text page + image page", pages)
	}
	if len(pages[0].Lines) == 0 || len(pages[0].Images) != 0 {
		t.Fatalf("text page = %#v, want only intro text", pages[0])
	}
	if len(pages[1].Images) != 1 || len(pages[1].Lines) != 0 {
		t.Fatalf("image page = %#v, want only image", pages[1])
	}
	if len(pages[1].Anchors) != 1 || pages[1].Anchors[0] != "img" {
		t.Fatalf("image anchors = %#v, want image anchor on rendered image page", pages[1].Anchors)
	}
}

func TestLayoutPDFPagesRendersImageOnlyHeadings(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 380
	img.Dim.Height = 30

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want image-only heading image", pages)
	}
	if len(pages[0].Lines) != 0 {
		t.Fatalf("heading lines = %#v, want image-only heading rendered as block image", pages[0].Lines)
	}
	if got, want := pages[0].Images[0].Width, 520.0-48.0; math.Abs(got-want) > 0.001 {
		t.Fatalf("heading image width = %v, want content width %v", got, want)
	}
}

func TestLayoutPDFPagesKeepsGapAfterImageOnlySubtitle(t *testing.T) {
	img := &fb2.BookImage{}
	img.Dim.Width = 380
	img.Dim.Height = 30

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 || len(pages[0].Images) != 1 || len(pages[0].Lines) == 0 {
		t.Fatalf("layout pages = %#v, want image plus following text", pages)
	}
	wantWidth := (520.0 - 48.0) * 380.0 / pdfKP3ContentWidthPx
	if got := pages[0].Images[0].Width; math.Abs(got-wantWidth) > 0.001 {
		t.Fatalf("subtitle image width = %v, want KP3-style width %v", got, wantWidth)
	}
	gap := pages[0].Images[0].Y - pages[0].Lines[0].Y
	if math.Abs(gap-(pdfSubtitleSpaceAfter+pdfBaseFontSize)) > 0.001 {
		t.Fatalf("subtitle image gap = %v, want %v", gap, pdfSubtitleSpaceAfter+pdfBaseFontSize)
	}
}
