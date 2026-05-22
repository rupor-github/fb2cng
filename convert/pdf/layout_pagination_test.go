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

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	if len(pages) != 2 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 2", len(pages))
	}
	if got := pageText(pages[0]); strings.Contains(got, "Heading") {
		t.Fatalf("heading stranded on previous page: %q", got)
	}
	if got := pageText(pages[1]); !strings.Contains(got, "Heading") || !strings.Contains(got, "Body text") {
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

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	if len(pages) != 2 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 2", len(pages))
	}
	targetFirstWord := strings.Fields(target)[0]
	if got := pageText(pages[0]); strings.Contains(got, targetFirstWord) {
		t.Fatalf("paragraph orphan left on previous page: %q", got)
	}
	if got := pageText(pages[1]); !strings.Contains(got, targetFirstWord) {
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

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	if len(pages) != 3 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 3", len(pages))
	}
	if got := pageText(pages[0]); !strings.Contains(got, "first paragraph") || strings.Contains(got, "hidden paragraph") || strings.Contains(got, "second paragraph") {
		t.Fatalf("first body page text = %q, want first visible paragraph only", got)
	}
	if got := pageText(pages[1]); !strings.Contains(got, "second paragraph") || !strings.Contains(got, "third paragraph") || strings.Contains(got, "fourth paragraph") {
		t.Fatalf("second body page text = %q, want second and third paragraphs before forced break-after", got)
	}
	if got := pageText(pages[2]); !strings.Contains(got, "fourth paragraph") {
		t.Fatalf("third body page text = %q, want fourth paragraph", got)
	}
}

func TestLayoutPDFPagesHonorsPageBreakBeforeAvoidFromDefaultCSS(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolverWithDefaultCSS(t, `
		p { margin: 0; }
		.text-author { margin: 0; }
	`)
	contentWidth := 220.0 - 48.0
	style := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	filler := textWithParagraphLineCount(t, face, style, contentWidth, 2, "filler")

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 100,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: filler},
			{Kind: pdfBlockParagraph, Text: "kept paragraph"},
			{Kind: pdfBlockParagraph, Text: "Author", StyleClasses: pdfStyleTextAuthor},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 2", len(pages))
	}
	if got := pageText(pages[0]); strings.Contains(got, "kept paragraph") || strings.Contains(got, "Author") {
		t.Fatalf("page-break-before:avoid did not move previous paragraph: %q", got)
	}
	if got := pageText(pages[1]); !strings.Contains(got, "kept paragraph") || !strings.Contains(got, "Author") {
		t.Fatalf("second page text = %q, want kept paragraph with author", got)
	}
}

func TestLayoutPDFPagesHonorsPageBreakBeforeAvoidForDefaultVignettes(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolverWithDefaultCSS(t, `
		p { margin: 0; }
		.vignette-section-end { margin: 0; }
	`)
	contentWidth := 220.0 - 48.0
	style := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	filler := textWithParagraphLineCount(t, face, style, contentWidth, 6, "filler")
	img := &fb2.BookImage{}
	img.Dim.Width = 100
	img.Dim.Height = 20

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 155,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"vignette": img},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: filler},
			{Kind: pdfBlockParagraph, Text: "before vignette"},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: joinStyleClasses(pdfStyleImageVignette, pdfStyleVignette, pdfStyleVignetteSectionEnd), ImageID: "vignette"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 2", len(pages))
	}
	if got := pageText(pages[0]); strings.Contains(got, "before vignette") || len(pages[0].Images) != 0 {
		t.Fatalf("page-break-before:avoid vignette did not move previous paragraph: images=%d text=%q", len(pages[0].Images), got)
	}
	if got := pageText(pages[1]); !strings.Contains(got, "before vignette") || len(pages[1].Images) != 1 {
		t.Fatalf("second page images/text = %d/%q, want paragraph with vignette", len(pages[1].Images), got)
	}
}

func TestLayoutPDFPagesHonorsPageBreakAfterAvoidForImages(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolverWithCSS(t, `
		p { margin: 0; }
		.keep-image { page-break-after: avoid; margin: 0; }
	`)
	contentWidth := 220.0 - 48.0
	style := resolver.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	filler := textWithParagraphLineCount(t, face, style, contentWidth, 6, "filler")
	img := &fb2.BookImage{}
	img.Dim.Width = 100
	img.Dim.Height = 30

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 155,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Images:     fb2.BookImages{"img": img},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: filler},
			{Kind: pdfBlockImage, StyleName: pdfStyleImage, StyleClasses: "keep-image", ImageID: "img"},
			{Kind: pdfBlockParagraph, Text: "after image"},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 2", len(pages))
	}
	if len(pages[0].Images) != 0 || strings.Contains(pageText(pages[0]), "after image") {
		t.Fatalf("image was stranded before page-break-after:avoid target: images=%d text=%q", len(pages[0].Images), pageText(pages[0]))
	}
	if len(pages[1].Images) != 1 || !strings.Contains(pageText(pages[1]), "after image") {
		t.Fatalf("second page images/text = %d/%q, want image with following text", len(pages[1].Images), pageText(pages[1]))
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
	paragraph.SpaceBefore = 0
	paragraph.SpaceAfter = 0
	resolver.styles[pdfStyleParagraph] = paragraph

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	if len(pages) != 1 || len(pages[0].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line", pages)
	}
	line := pages[0].Lines[0]
	if line.X != 18 || math.Abs(line.Y-142.6) > 0.001 {
		t.Fatalf("line position = %v/%v, want 18/142.6", line.X, line.Y)
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

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	if len(pages) != 1 || len(pages[0].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line", pages)
	}
	line := pages[0].Lines[0]
	if line.X != 24 || math.Abs(line.Y-140.6) > 0.001 {
		t.Fatalf("line position = %v/%v, want 24/140.6", line.X, line.Y)
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

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	if len(pages) != 1 || len(pages[0].Lines) != 1 || len(pages[0].Backgrounds) != 1 || len(pages[0].Borders) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line and decorations", pages)
	}
	line := pages[0].Lines[0]
	if line.X != 31 { // 24pt page margin + 7pt left padding.
		t.Fatalf("line X = %v, want 31", line.X)
	}
	if math.Abs(line.Y-142.6) > 0.001 { // 180pt page height - 24pt page margin - 5pt top padding - 8.4pt font size.
		t.Fatalf("line Y = %v, want 142.6", line.Y)
	}
	background := pages[0].Backgrounds[0]
	if background.X != 24 || math.Abs(background.Y-128.32) > 0.001 || background.Width != 172 || background.Height < 27.679 || background.Height > 27.681 || background.Color.String() != "#0000ff" {
		t.Fatalf("background = %#v, want padded block background", background)
	}
	border := pages[0].Borders[0]
	if border.X != background.X || border.Y != background.Y || border.Width != background.Width || border.Height != background.Height || border.LineWidth != 1 || border.Color.String() != "#ff0000" {
		t.Fatalf("border = %#v, want matching padded block border", border)
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

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
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
	if len(pages) != 1 || len(pages[0].Lines) != 1 || len(pages[0].Backgrounds) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one body line and background", pages)
	}
	line := pages[0].Lines[0]
	if line.X != 29 { // 24pt page margin + 5pt left padding.
		t.Fatalf("line X = %v, want 29", line.X)
	}
	background := pages[0].Backgrounds[0]
	if background.X != 24 || background.Width != 72 { // 60pt content width + 12pt horizontal padding.
		t.Fatalf("background = %#v, want 72pt fixed-width block", background)
	}
}
