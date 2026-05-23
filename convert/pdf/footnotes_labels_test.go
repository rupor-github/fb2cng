package pdf

import (
	"strings"
	"testing"
)

func TestLayoutPDFPagesAppliesPageLocalPrintedFootnoteReferenceLabels(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}

	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 160,
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

func TestPDFPageLocalFootnoteReferenceTextPreservesPseudoPunctuation(t *testing.T) {
	if got := pdfPageLocalFootnoteReferenceText("[1.17]", "2"); got != "[2]" {
		t.Fatalf("decorated label = %q, want [2]", got)
	}
	if got := pdfPageLocalFootnoteReferenceText("  (1.17)  ", "3"); got != "(3)" {
		t.Fatalf("decorated label = %q, want (3)", got)
	}
}
