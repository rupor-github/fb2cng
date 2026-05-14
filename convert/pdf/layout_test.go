package pdf

import (
	"math"
	"strings"
	"testing"

	"fbc/fb2"
)

func TestLayoutPDFPagesKeepsHeadingWithNextParagraph(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	contentWidth := 220.0 - 48.0
	filler := textWithParagraphLineCount(t, face, pdfStyleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph, contentWidth, 2, "filler")

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 120,
		Title:      "Title",
		Author:     "Author",
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: filler},
			{Kind: pdfBlockHeading, Text: "Heading", Depth: 1},
			{Kind: pdfBlockParagraph, Text: "Body text after heading."},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 3", len(pages))
	}
	if got := pageText(pages[1]); strings.Contains(got, "Heading") {
		t.Fatalf("heading stranded on previous page: %q", got)
	}
	if got := pageText(pages[2]); !strings.Contains(got, "Heading") || !strings.Contains(got, "Body text") {
		t.Fatalf("heading page text = %q, want heading with following paragraph", got)
	}
}

func TestLayoutPDFPagesAvoidsParagraphWidowOrphanSplit(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	contentWidth := 220.0 - 48.0
	style := pdfStyleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	filler := textWithParagraphLineCount(t, face, style, contentWidth, 2, "filler")
	target := textWithParagraphLineCount(t, face, style, contentWidth, 3, "target")

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 98,
		Title:      "Title",
		Author:     "Author",
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: filler},
			{Kind: pdfBlockParagraph, Text: target},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 3", len(pages))
	}
	targetFirstWord := strings.Fields(target)[0]
	if got := pageText(pages[1]); strings.Contains(got, targetFirstWord) {
		t.Fatalf("paragraph orphan left on previous page: %q", got)
	}
	if got := pageText(pages[2]); !strings.Contains(got, targetFirstWord) {
		t.Fatalf("paragraph did not move to next page: %q", got)
	}
}

func TestLayoutPDFPagesHonorsCSSPageBreakAndHiddenStyles(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	before := resolver.styles[pdfStyleParagraph]
	before.PageBreakBefore = true
	resolver.styles["forced-before"] = before
	after := resolver.styles[pdfStyleParagraph]
	after.PageBreakAfter = true
	resolver.styles["forced-after"] = after
	hidden := resolver.styles[pdfStyleParagraph]
	hidden.Hidden = true
	resolver.styles["hidden"] = hidden

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "first paragraph"},
			{Kind: pdfBlockParagraph, Text: "hidden paragraph", StyleClasses: "hidden"},
			{Kind: pdfBlockParagraph, Text: "second paragraph", StyleClasses: "forced-before"},
			{Kind: pdfBlockParagraph, Text: "third paragraph", StyleClasses: "forced-after"},
			{Kind: pdfBlockParagraph, Text: "fourth paragraph"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 4 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 4", len(pages))
	}
	if got := pageText(pages[1]); !strings.Contains(got, "first paragraph") || strings.Contains(got, "hidden paragraph") || strings.Contains(got, "second paragraph") {
		t.Fatalf("first body page text = %q, want first visible paragraph only", got)
	}
	if got := pageText(pages[2]); !strings.Contains(got, "second paragraph") || !strings.Contains(got, "third paragraph") || strings.Contains(got, "fourth paragraph") {
		t.Fatalf("second body page text = %q, want second and third paragraphs before forced break-after", got)
	}
	if got := pageText(pages[3]); !strings.Contains(got, "fourth paragraph") {
		t.Fatalf("third body page text = %q, want fourth paragraph", got)
	}
}

func TestLayoutPDFPagesAppliesRootPageMargins(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	resolver.styles[pdfStylePage] = pdfBlockResolvedStyle{MarginLeft: -6, MarginRight: -4, SpaceBefore: 5, SpaceAfter: -3}
	paragraph := resolver.styles[pdfStyleParagraph]
	paragraph.Paragraph.FirstLineIndent = 0
	resolver.styles[pdfStyleParagraph] = paragraph

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "root margins"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line", pages)
	}
	line := pages[1].Lines[0]
	if line.X != 18 || line.Y != 140.5 {
		t.Fatalf("line position = %v/%v, want 18/140.5", line.X, line.Y)
	}
}

func TestLayoutPDFPagesAppliesFirstBlockTopMargin(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	topGap := resolver.styles[pdfStyleParagraph]
	topGap.Paragraph.FirstLineIndent = 0
	topGap.SpaceBefore = 7
	topGap.SpaceAfter = 0
	resolver.styles["top-gap"] = topGap

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockParagraph,
			Text:         "top margin",
			StyleClasses: "top-gap",
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line", pages)
	}
	line := pages[1].Lines[0]
	if line.X != 24 || line.Y != 138.5 {
		t.Fatalf("line position = %v/%v, want 24/138.5", line.X, line.Y)
	}
}

func TestLayoutPDFPagesAppliesPadding(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	padded := resolver.styles[pdfStyleParagraph]
	padded.Paragraph.FirstLineIndent = 0
	padded.SpaceBefore = 0
	padded.SpaceAfter = 0
	padded.PaddingTop = 5
	padded.PaddingLeft = 7
	padded.BackgroundColor = pdfColor{B: 1}
	padded.HasBackground = true
	padded.BorderWidth = 1
	padded.BorderColor = pdfColor{R: 1}
	padded.HasBorder = true
	resolver.styles["padded"] = padded

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "padded text", StyleClasses: "padded"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 || len(pages[1].Backgrounds) != 1 || len(pages[1].Borders) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line and decorations", pages)
	}
	line := pages[1].Lines[0]
	if line.X != 31 { // 24pt page margin + 7pt left padding.
		t.Fatalf("line X = %v, want 31", line.X)
	}
	if line.Y != 140.5 { // 180pt page height - 24pt page margin - 5pt top padding - 10.5pt font size.
		t.Fatalf("line Y = %v, want 140.5", line.Y)
	}
	background := pages[1].Backgrounds[0]
	if background.X != 24 || background.Y != 127.1 || background.Width != 172 || background.Height < 28.899 || background.Height > 28.901 || background.Color.String() != "#0000ff" {
		t.Fatalf("background = %#v, want padded block background", background)
	}
	border := pages[1].Borders[0]
	if border.X != background.X || border.Y != background.Y || border.Width != background.Width || border.Height != background.Height || border.LineWidth != 1 || border.Color.String() != "#ff0000" {
		t.Fatalf("border = %#v, want matching padded block border", border)
	}
}

func TestLayoutPDFPagesAppliesInlineStyles(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	pages, used, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "plain bold strike sub sup code",
			Runs: []pdfInlineRun{
				{Text: "plain "},
				{Text: "bold", Bold: true, Italic: true},
				{Text: " "},
				{Text: "strike", Strikethrough: true},
				{Text: " "},
				{Text: "sub", Subscript: true},
				{Text: " "},
				{Text: "sup", Superscript: true},
				{Text: " "},
				{Text: "code", Code: true},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one styled body line", pages)
	}
	line := pages[1].Lines[0]
	if got := pdfPageLineText(line); got != "plain bold strike sub sup code" {
		t.Fatalf("line text = %q", got)
	}
	var sawBoldItalic, sawStrike, sawSub, sawSup, sawCode bool
	for _, fragment := range line.Fragments {
		switch shapedRunes(fragment.Text) {
		case "bold":
			sawBoldItalic = fragment.FontKey.Bold && fragment.FontKey.Italic
		case "strike":
			sawStrike = fragment.Strikethrough
		case "sub":
			sawSub = fragment.BaselineShift < 0 && fragment.FontSize < line.FontSize
		case "sup":
			sawSup = fragment.BaselineShift > 0 && fragment.FontSize < line.FontSize
		case "code":
			sawCode = fragment.FontKey.Family == "monospace" && fragment.FontSize < line.FontSize
		}
	}
	if !sawBoldItalic || !sawStrike || !sawSub || !sawSup || !sawCode {
		t.Fatalf("inline fragments = %#v", line.Fragments)
	}
	for _, key := range []pdfFontKey{{Family: "serif"}, {Family: "serif", Bold: true, Italic: true}, {Family: "monospace"}} {
		if len(used[key]) == 0 {
			t.Fatalf("used glyphs for font key %#v missing in %#v", key, used)
		}
	}
}

func TestLayoutPDFPagesPreservesCodeBlockWhitespace(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  260,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Blocks: []pdfTextBlock{{
			Kind:         pdfBlockParagraph,
			Text:         "alpha beta",
			StyleClasses: pdfStyleCode,
			Runs: []pdfInlineRun{{
				Text:         "\n  alpha\n    beta\n  ",
				StyleClasses: pdfStyleCode,
				Code:         true,
			}},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 2 {
		t.Fatalf("layoutPDFPages() pages = %#v, want two preformatted code lines", pages)
	}
	if got := pdfPageLineText(pages[1].Lines[0]); got != "  alpha" {
		t.Fatalf("first code line = %q, want preserved indentation", got)
	}
	if got := pdfPageLineText(pages[1].Lines[1]); got != "    beta" {
		t.Fatalf("second code line = %q, want preserved indentation", got)
	}
	for _, line := range pages[1].Lines {
		if line.X != 24 || line.Fragments[0].FontKey.Family != "monospace" || line.Fragments[0].FontSize >= pdfBaseFontSize {
			t.Fatalf("code line = %#v, want left-aligned smaller monospace", line)
		}
	}
}

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

func TestLayoutPDFPagesRendersInlineImages(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	img := &fb2.BookImage{}
	img.Dim.Width = 120
	img.Dim.Height = 60

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:      520,
		PageHeight:     180,
		ScreenWidthPx:  1200,
		ScreenHeightPx: 1600,
		Title:          "Title",
		Author:         "Author",
		Images:         fb2.BookImages{"inline": img},
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "before after",
			Runs: []pdfInlineRun{
				{Text: "before "},
				{ImageID: "inline", StyleClasses: pdfStyleLinkInternal, LinkHref: "#target"},
				{Text: " after"},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Images) != 1 {
		t.Fatalf("layout pages images = %#v, want one inline image on body page", pages)
	}
	image := pages[1].Images[0]
	if image.ImageID != "inline" || image.Width <= 0 || image.Height <= 0 {
		t.Fatalf("inline image = %#v, want placed image", image)
	}
	if len(pages[1].Lines) != 1 {
		t.Fatalf("lines = %#v, want one body line", pages[1].Lines)
	}
	line := pages[1].Lines[0]
	var imageFragment *pdfPageLineFragment
	for i := range line.Fragments {
		fragment := &line.Fragments[i]
		if fragment.ImageID == "inline" {
			imageFragment = fragment
			break
		}
	}
	if imageFragment == nil {
		t.Fatalf("line fragments = %#v, want inline image fragment", line.Fragments)
	}
	if !imageFragment.Underline {
		t.Fatalf("inline image fragment = %#v, want link underline decoration", *imageFragment)
	}
	if math.Abs(image.Height-pdfBaseLineHeight) > 0.001 || math.Abs(imageFragment.ImageHeight-pdfBaseLineHeight) > 0.001 {
		t.Fatalf("inline image height = %v / fragment %v, want current line height %v", image.Height, imageFragment.ImageHeight, pdfBaseLineHeight)
	}
	if math.Abs(image.Y-(line.Y+imageFragment.BaselineShift)) > 0.001 || image.Y >= line.Y || image.Y+image.Height <= line.Y {
		t.Fatalf("inline image Y = %v, line Y = %v, fragment baseline shift = %v", image.Y, line.Y, imageFragment.BaselineShift)
	}
	if len(pages[1].Annotations) != 1 {
		t.Fatalf("annotations = %#v, want inline image link annotation", pages[1].Annotations)
	}
	annotation := pages[1].Annotations[0]
	if annotation.Href != "#target" || math.Abs(annotation.Rect.X1-image.X) > 0.001 || math.Abs(annotation.Rect.Y1-image.Y) > 0.001 || math.Abs(annotation.Rect.X2-(image.X+image.Width)) > 0.001 || math.Abs(annotation.Rect.Y2-(image.Y+image.Height)) > 0.001 {
		t.Fatalf("annotation = %#v, image = %#v, want image rectangle link", annotation, image)
	}
}

func TestLayoutPDFPagesAnnotationWrapperParagraphCanStripRootHorizontalMargins(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			html { margin: 0 -20pt 0 -20pt; }
			p { margin: 0; text-indent: 0; }
			.annotation { margin-left: 12pt; margin-right: 12pt; }
		`,
	}}}, nil)

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:                       pdfBlockParagraph,
			Text:                       "Wrapped annotation.",
			StyleClasses:               pdfStyleAnnotation,
			StripRootHorizontalMargins: true,
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one annotation line", pages)
	}
	if got := pages[1].Lines[0].X; math.Abs(got-36) > 0.001 {
		t.Fatalf("annotation line X = %v, want 36 (24 base margin + 12 annotation margin)", got)
	}
}

func TestLayoutPDFPagesAppliesInlineNamedStyleClasses(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	accent := resolver.styles[pdfStyleParagraph]
	accent.Paragraph.FontFamily = "sans-serif"
	accent.Paragraph.Bold = true
	accent.Paragraph.Italic = true
	accent.Paragraph.Color = pdfColor{R: 1}
	accent.Paragraph.Underline = true
	accent.Paragraph.VerticalAlign = textVerticalAlignSuper
	resolver.styles["accent"] = accent

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  520,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "plain styled",
			Runs: []pdfInlineRun{
				{Text: "plain "},
				{Text: "styled", StyleClasses: "accent"},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one styled body line", pages)
	}
	var styled *pdfPageLineFragment
	for i := range pages[1].Lines[0].Fragments {
		fragment := &pages[1].Lines[0].Fragments[i]
		if shapedRunes(fragment.Text) == "styled" {
			styled = fragment
			break
		}
	}
	if styled == nil {
		t.Fatalf("styled fragment missing: %#v", pages[1].Lines[0].Fragments)
	}
	if styled.FontKey.Family != "sans-serif" || !styled.FontKey.Bold || !styled.FontKey.Italic || !styled.Underline || styled.Color.String() != "#ff0000" || styled.BaselineShift <= 0 || styled.FontSize >= pages[1].Lines[0].FontSize {
		t.Fatalf("styled fragment = %#v, want accent class styling", *styled)
	}
}

func TestLayoutPDFPagesAppliesBlockWidth(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	fixed := resolver.styles[pdfStyleParagraph]
	fixed.Paragraph.FirstLineIndent = 0
	fixed.SpaceBefore = 0
	fixed.SpaceAfter = 0
	fixed.PaddingLeft = 5
	fixed.PaddingRight = 7
	fixed.Width = pdfBlockLength{Value: 60}
	fixed.HasWidth = true
	fixed.BackgroundColor = pdfColor{G: 1}
	fixed.HasBackground = true
	resolver.styles["fixed-width"] = fixed

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "fixed", StyleClasses: "fixed-width"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 || len(pages[1].Backgrounds) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line and background", pages)
	}
	line := pages[1].Lines[0]
	if line.X != 29 { // 24pt page margin + 5pt left padding.
		t.Fatalf("line X = %v, want 29", line.X)
	}
	background := pages[1].Backgrounds[0]
	if background.X != 24 || background.Width != 72 { // 60pt content width + 12pt horizontal padding.
		t.Fatalf("background = %#v, want 72pt fixed-width block", background)
	}
}

func textWithParagraphLineCount(t *testing.T, face *builtinFontFace, style paragraphStyle, width float64, wantLines int, word string) string {
	t.Helper()
	for words := 1; words <= 80; words++ {
		parts := make([]string, words)
		for i := range parts {
			parts[i] = word
		}
		text := strings.Join(parts, " ")
		lines, err := layoutParagraph(face, text, style, width)
		if err != nil {
			t.Fatalf("layoutParagraph() error = %v", err)
		}
		if len(lines) == wantLines {
			return text
		}
	}
	t.Fatalf("could not build paragraph with %d lines", wantLines)
	return ""
}

func pageText(page pdfPage) string {
	parts := make([]string, 0, len(page.Lines))
	for _, line := range page.Lines {
		parts = append(parts, shapedRunes(line.Text))
	}
	return strings.Join(parts, "\n")
}
