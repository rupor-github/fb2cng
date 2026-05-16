package pdf

import (
	"strings"
	"testing"
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
	if background.X != 24 || background.Y != 127.9 || background.Width != 172 || background.Height < 28.099 || background.Height > 28.101 || background.Color.String() != "#0000ff" {
		t.Fatalf("background = %#v, want padded block background", background)
	}
	border := pages[1].Borders[0]
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
