package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/content"
)

func TestLayoutPDFPagesAppliesPageLocalPrintedFootnoteReferenceLabels(t *testing.T) {

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 160,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n17": {ID: "n17"},
			"n23": {ID: "n23"},
		},
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "A 1.17 B 2.3 C 1.17",
			Runs: []pdfInlineRun{
				{Text: "A "},
				{Text: "1.17", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n17", Superscript: true},
				{Text: " B "},
				{Text: "2.3", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n23", Superscript: true},
				{Text: " C "},
				{Text: "1.17", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n17", Superscript: true},
			},
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	if got := pageText(pages[0]); !strings.Contains(got, "A 1 B 2 C 1") {
		t.Fatalf("page text = %q, want page-local labels in reference order", got)
	}
}

func TestApplyPDFPageLocalFootnoteReferenceLabelsKeepsInlineImageFootnoteWidth(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	prefix, err := shapeTextWithCache(nil, face, "A ")
	if err != nil {
		t.Fatalf("shape prefix: %v", err)
	}
	suffix, err := shapeTextWithCache(nil, face, " B")
	if err != nil {
		t.Fatalf("shape suffix: %v", err)
	}
	ref, err := shapeTextWithCache(nil, face, "10")
	if err != nil {
		t.Fatalf("shape ref: %v", err)
	}
	pages := []pdfPage{{Lines: []pdfPageLine{{Fragments: []pdfPageLineFragment{
		{Text: prefix, Width: shapedWidthPoints(prefix, pdfBaseFontSize, 0), FontSize: pdfBaseFontSize, FontKey: face.Key},
		{Text: ref, Width: 40, FontSize: pdfBaseFontSize, FontKey: face.Key, FootnoteID: "n1", ImageID: "word.png", ImageHeight: 8},
		{Text: suffix, Width: shapedWidthPoints(suffix, pdfBaseFontSize, 0), FontSize: pdfBaseFontSize, FontKey: face.Key},
	}}}}}
	used := make(map[pdfFontKey]map[uint16]shapedGlyph)

	if err := applyPDFPageLocalFootnoteReferenceLabels(pages, nil, used, nil, nil); err != nil {
		t.Fatalf("applyPDFPageLocalFootnoteReferenceLabels() error = %v", err)
	}
	imageFragment := pages[0].Lines[0].Fragments[1]
	if imageFragment.Width != 40 || shapedRunes(imageFragment.Text) != "10" {
		t.Fatalf(
			"image footnote fragment = width %v text %q, want original image width/text preserved",
			imageFragment.Width,
			shapedRunes(imageFragment.Text),
		)
	}
}

func TestApplyPDFPageLocalFootnoteReferenceLabelsRejustifiesChangedLine(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	prefix, err := shapeTextWithCache(nil, face, "Alpha ")
	if err != nil {
		t.Fatalf("shape prefix: %v", err)
	}
	ref, err := shapeTextWithCache(nil, face, "17")
	if err != nil {
		t.Fatalf("shape ref: %v", err)
	}
	suffix, err := shapeTextWithCache(nil, face, " beta")
	if err != nil {
		t.Fatalf("shape suffix: %v", err)
	}
	newRef, err := shapeTextWithCache(nil, face, "1")
	if err != nil {
		t.Fatalf("shape new ref: %v", err)
	}
	fragments := []pdfPageLineFragment{
		{Text: prefix, Width: shapedWidthPoints(prefix, pdfBaseFontSize, 0), FontSize: pdfBaseFontSize, FontKey: face.Key},
		{Text: ref, Width: shapedWidthPoints(ref, pdfBaseFontSize, 0), FontSize: pdfBaseFontSize, FontKey: face.Key, FootnoteID: "n1"},
		{Text: suffix, Width: shapedWidthPoints(suffix, pdfBaseFontSize, 0), FontSize: pdfBaseFontSize, FontKey: face.Key},
	}
	line := pdfPageLine{
		FontSize:         pdfBaseFontSize,
		Fragments:        fragments,
		Text:             shapedTextFromPageLineFragments(fragments),
		ExtraWordSpacing: 0.5,
	}
	newNaturalWidth := shapedWidthPoints(
		prefix,
		pdfBaseFontSize,
		0,
	) + shapedWidthPoints(
		newRef,
		pdfBaseFontSize,
		0,
	) + shapedWidthPoints(
		suffix,
		pdfBaseFontSize,
		0,
	)
	line.BreakStats.AvailableWidth = newNaturalWidth + 1
	pages := []pdfPage{{Lines: []pdfPageLine{line}}}
	used := make(map[pdfFontKey]map[uint16]shapedGlyph)

	if err := applyPDFPageLocalFootnoteReferenceLabels(pages, nil, used, nil, nil); err != nil {
		t.Fatalf("applyPDFPageLocalFootnoteReferenceLabels() error = %v", err)
	}
	updated := pages[0].Lines[0]
	if got := shapedRunes(updated.Text); got != "Alpha 1 beta" {
		t.Fatalf("line text = %q, want relabeled footnote", got)
	}
	if got, want := pdfPageLineDrawnWidth(updated), updated.BreakStats.AvailableWidth; got < want-pdfLineWidthTolerance ||
		got > want+pdfLineWidthTolerance {
		t.Fatalf("drawn width = %v, available = %v, want relabeled line rejustified", got, want)
	}
}

func TestLayoutPDFPagesKeepsFloatPrintedFootnoteReferenceLabels(t *testing.T) {

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 160,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloat},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n17": {ID: "n17"},
		},
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "A 1.17",
			Runs: []pdfInlineRun{{Text: "A "}, {Text: "1.17", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n17", Superscript: true}},
		}},
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if got := pageText(pages[0]); !strings.Contains(got, "A 1.17") {
		t.Fatalf("page text = %q, want original float label preserved", got)
	}
}

func TestLayoutPDFPagesFloatRenumberedDecoratesPageLocalReferenceLabels(t *testing.T) {

	styles := newPDFStyleResolverWithCSS(t, `
		.link-footnote::before { content: "["; }
		.link-footnote::after { content: "]"; }
	`)
	blocks := applyPDFPseudoContentToBlocks([]pdfTextBlock{{
		Kind: pdfBlockParagraph,
		Text: "A 1.17",
		Runs: []pdfInlineRun{{Text: "A "}, {Text: "1.17", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n17", Superscript: true}},
	}}, styles)

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 160,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		Styles:     styles,
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n17": {ID: "n17"},
		},
		Blocks: blocks,
	})
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if got := pageText(pages[0]); !strings.Contains(got, "A [1]") || strings.Contains(got, "1.17") {
		t.Fatalf("page text = %q, want decorated page-local label", got)
	}
}
