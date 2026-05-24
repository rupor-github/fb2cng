package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/content"
)

func TestLayoutPDFPagesWithPrintedFootnoteReservesPushesMainText(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	doc := pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 130,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {ID: "n1", BodyBlocks: []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Footnote body.", Runs: []pdfInlineRun{{Text: "Footnote body."}}, StyleClasses: pdfStyleFootnote, ContextClasses: pdfStyleFootnote}}},
		},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "One 17", Runs: []pdfInlineRun{{Text: "One "}, {Text: "17", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n1", Superscript: true}}},
			{Kind: pdfBlockParagraph, Text: "Two"},
			{Kind: pdfBlockParagraph, Text: "Three"},
		},
	}
	unreserved, _, err := layoutPDFPages(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(unreserved) != 1 || !strings.Contains(pageText(unreserved[0]), "Three") {
		t.Fatalf("unreserved pages = %#v, want all main text on first page", unreserved)
	}

	reserved, err := layoutPDFPagesWithPrintedFootnoteReserves(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFPagesWithPrintedFootnoteReserves() error = %v", err)
	}
	if len(reserved.Plans) != 1 || reserved.Plans[0].PageIndex != 0 {
		t.Fatalf("plans = %#v, want one first-page footnote plan", reserved.Plans)
	}
	if len(reserved.PageBottomReserves) != 2 || reserved.PageBottomReserves[0] <= 0 || reserved.PageBottomReserves[1] != 0 {
		t.Fatalf("reserves = %#v, want first page reserve only", reserved.PageBottomReserves)
	}
	if len(reserved.Pages) != 2 {
		t.Fatalf("reserved pages = %d, want 2", len(reserved.Pages))
	}
	if got := pageText(reserved.Pages[0]); !strings.Contains(got, "One 1") || strings.Contains(got, "Three") {
		t.Fatalf("reserved first page text = %q, want footnote ref and no trailing Three", got)
	}
	if got := pageText(reserved.Pages[1]); !strings.Contains(got, "Two") || !strings.Contains(got, "Three") {
		t.Fatalf("reserved second page text = %q, want pushed body text", got)
	}
}

func TestLayoutPDFPagesWithPrintedFootnoteReservesPacksTextAboveShortFootnote(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	doc := pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 260,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {
				ID: "n1",
				BodyBlocks: []pdfTextBlock{{
					Kind:           pdfBlockParagraph,
					Text:           "Short note.",
					Runs:           []pdfInlineRun{{Text: "Short note."}},
					StyleClasses:   pdfStyleFootnote,
					ContextClasses: pdfStyleFootnote,
				}},
			},
		},
		Blocks: []pdfTextBlock{
			{
				Kind: pdfBlockParagraph,
				Text: "One 17",
				Runs: []pdfInlineRun{
					{Text: "One "},
					{Text: "17", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n1", Superscript: true},
				},
			},
			{Kind: pdfBlockParagraph, Text: "Two"},
			{Kind: pdfBlockParagraph, Text: "Three"},
			{Kind: pdfBlockParagraph, Text: "Four"},
			{Kind: pdfBlockParagraph, Text: "Five"},
		},
	}

	reserved, err := layoutPDFPagesWithPrintedFootnoteReserves(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFPagesWithPrintedFootnoteReserves() error = %v", err)
	}
	if len(reserved.Plans) != 1 || reserved.Plans[0].ContinuationPages != 0 {
		t.Fatalf("plans = %#v, want one fully fitting short footnote plan", reserved.Plans)
	}
	if len(reserved.Pages) == 0 {
		t.Fatalf("reserved pages = %#v, want at least one page", reserved.Pages)
	}
	if got := pageText(reserved.Pages[0]); !strings.Contains(got, "Four") {
		t.Fatalf("first page text = %q, want main text packed above short printed footnote", got)
	}
}

func TestLayoutPDFPagesWithPrintedFootnoteReservesNoFootnotesMatchesNormalLayout(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	doc := pdfDocumentSpec{PageWidth: 220, PageHeight: 130, Blocks: []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Only body."}}}
	normal, _, err := layoutPDFPages(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	reserved, err := layoutPDFPagesWithPrintedFootnoteReserves(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFPagesWithPrintedFootnoteReserves() error = %v", err)
	}
	if len(reserved.Plans) != 0 || len(reserved.PageBottomReserves) != 0 {
		t.Fatalf("reserved layout = %#v, want no plans/reserves", reserved)
	}
	if len(reserved.Pages) != len(normal) || pageText(reserved.Pages[0]) != pageText(normal[0]) {
		t.Fatalf("reserved pages = %#v, normal = %#v", reserved.Pages, normal)
	}
}
