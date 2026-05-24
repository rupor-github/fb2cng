package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/content"
)

func TestLayoutPDFPagesAppliesPageLocalPrintedFootnoteReferenceLabels(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}

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
	}, face)
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
	prefix, err := shapeText(face, "A ")
	if err != nil {
		t.Fatalf("shape prefix: %v", err)
	}
	suffix, err := shapeText(face, " B")
	if err != nil {
		t.Fatalf("shape suffix: %v", err)
	}
	ref, err := shapeText(face, "10")
	if err != nil {
		t.Fatalf("shape ref: %v", err)
	}
	pages := []pdfPage{{Lines: []pdfPageLine{{Fragments: []pdfPageLineFragment{
		{Text: prefix, Width: shapedWidthPoints(prefix, pdfBaseFontSize), FontSize: pdfBaseFontSize, FontKey: face.Key},
		{Text: ref, Width: 40, FontSize: pdfBaseFontSize, FontKey: face.Key, FootnoteID: "n1", ImageID: "word.png", ImageHeight: 8},
		{Text: suffix, Width: shapedWidthPoints(suffix, pdfBaseFontSize), FontSize: pdfBaseFontSize, FontKey: face.Key},
	}}}}}
	used := make(map[pdfFontKey]map[uint16]shapedGlyph)

	if err := applyPDFPageLocalFootnoteReferenceLabels(pages, nil, used); err != nil {
		t.Fatalf("applyPDFPageLocalFootnoteReferenceLabels() error = %v", err)
	}
	imageFragment := pages[0].Lines[0].Fragments[1]
	if imageFragment.Width != 40 || shapedRunes(imageFragment.Text) != "10" {
		t.Fatalf("image footnote fragment = width %v text %q, want original image width/text preserved", imageFragment.Width, shapedRunes(imageFragment.Text))
	}
}

func TestLayoutPDFPagesKeepsFloatPrintedFootnoteReferenceLabels(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}

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
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if got := pageText(pages[0]); !strings.Contains(got, "A 1.17") {
		t.Fatalf("page text = %q, want original float label preserved", got)
	}
}

func TestLayoutPDFPagesFloatRenumberedUsesRawLabelsOverDecoratedText(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 160,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n17": {ID: "n17"},
		},
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "A [1.17]",
			Runs: []pdfInlineRun{{Text: "A "}, {Text: "[1.17]", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n17", Superscript: true}},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if got := pageText(pages[0]); !strings.Contains(got, "A 1") || strings.Contains(got, "[") || strings.Contains(got, "]") {
		t.Fatalf("page text = %q, want raw floatRenumbered label without pseudo decoration", got)
	}
}
