package pdf

import (
	"testing"

	"fbc/common"
	"fbc/content"
)

func TestPDFPrintedFootnotePlanReservesOnlyPagesWithFootnotes(t *testing.T) {
	doc := pdfDocumentSpec{PageWidth: 260, PageHeight: 180}
	plans := []pdfPrintedFootnotePagePlan{
		{PageIndex: 1, QueuePages: []pdfPage{{Lines: []pdfPageLine{{}}}}},
		{PageIndex: 3, QueuePages: []pdfPage{{Lines: []pdfPageLine{{}}}}},
	}

	reserves := pdfPrintedFootnotePlanReserves(doc, plans, 4, 50)
	if len(reserves) != 4 {
		t.Fatalf("reserves = %#v, want 4 page slots", reserves)
	}
	if reserves[0] != 0 || reserves[2] != 0 {
		t.Fatalf("reserves = %#v, want zero on pages without footnote plans", reserves)
	}
	if reserves[1] <= 0 || reserves[3] != reserves[1] {
		t.Fatalf("reserves = %#v, want positive equal reserves on planned pages", reserves)
	}
}

func TestPDFPrintedFootnotePlanReservesClampToLeaveTextArea(t *testing.T) {
	doc := pdfDocumentSpec{PageWidth: 260, PageHeight: 110}
	plans := []pdfPrintedFootnotePagePlan{{PageIndex: 0, QueuePages: []pdfPage{{Lines: []pdfPageLine{{}}}}}}

	reserves := pdfPrintedFootnotePlanReserves(doc, plans, 1, 10_000)
	if len(reserves) != 1 || reserves[0] <= 0 {
		t.Fatalf("reserves = %#v, want one positive clamped reserve", reserves)
	}
	reservedBottom := pdfReservedContentBottom(24, 86, reserves[0])
	if reservedBottom >= 86 {
		t.Fatalf("reserved bottom = %v, want below top", reservedBottom)
	}
}

func TestPDFPrintedFootnoteTextAreaHeightUsesContentFraction(t *testing.T) {
	doc := pdfDocumentSpec{PageWidth: 260, PageHeight: 200}
	height := pdfPrintedFootnoteTextAreaHeight(doc, nil)
	if height <= 0 {
		t.Fatalf("text area height = %v, want positive", height)
	}
	contentHeight := 200.0 - 48.0
	if height >= contentHeight {
		t.Fatalf("text area height = %v, want less than content height %v", height, contentHeight)
	}
}

func TestBuildPDFPrintedFootnotePagePlansAndReserves(t *testing.T) {
	doc := pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 180,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {ID: "n1", BodyBlocks: []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Footnote body.", Runs: []pdfInlineRun{{Text: "Footnote body."}}, StyleClasses: pdfStyleFootnote, ContextClasses: pdfStyleFootnote}}},
		},
	}
	pages := []pdfPage{{Lines: []pdfPageLine{{Fragments: []pdfPageLineFragment{{FootnoteID: "n1"}}}}}, {Lines: []pdfPageLine{{}}}}

	plans, reserves, err := buildPDFPrintedFootnotePagePlansAndReserves(doc, pages, 70)
	if err != nil {
		t.Fatalf("buildPDFPrintedFootnotePagePlansAndReserves() error = %v", err)
	}
	if len(plans) != 1 || plans[0].PageIndex != 0 {
		t.Fatalf("plans = %#v, want one plan for first page", plans)
	}
	if len(reserves) != 2 || reserves[0] <= 0 || reserves[1] != 0 {
		t.Fatalf("reserves = %#v, want reserve only on first page", reserves)
	}
}
