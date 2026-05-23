package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/content"
)

func TestLayoutPDFPrintedFootnoteQueueDoesNotTruncateLongFootnote(t *testing.T) {
	longBody := strings.Repeat("long footnote text ", 80) + "ENDMARK"
	doc := pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 180,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {
				ID: "n1",
				BodyBlocks: []pdfTextBlock{{
					Kind:           pdfBlockParagraph,
					Text:           longBody,
					Runs:           []pdfInlineRun{{Text: longBody}},
					StyleClasses:   pdfStyleFootnote,
					ContextClasses: pdfStyleFootnote,
				}},
			},
		},
	}
	queue := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}}

	pages, _, err := layoutPDFPrintedFootnoteQueue(doc, queue, 80)
	if err != nil {
		t.Fatalf("layoutPDFPrintedFootnoteQueue() error = %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("queue pages = %d, want long footnote to continue instead of truncating", len(pages))
	}
	var allText strings.Builder
	for _, page := range pages {
		allText.WriteString(pageText(page))
		allText.WriteByte('\n')
	}
	if got := allText.String(); !strings.Contains(got, "1") || !strings.Contains(got, "ENDMARK") {
		t.Fatalf("queue text = %q, want page label and final marker preserved", got)
	}
}

func TestPDFPrintedFootnoteQueueBlocksUsesNestedActualTitle(t *testing.T) {
	doc := pdfDocumentSpec{
		Content: &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {
				ID:          "n1",
				TitleBlocks: []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Ordinary title", Runs: []pdfInlineRun{{Text: "Ordinary title"}}, StyleClasses: pdfStyleFootnoteTitle, ContextClasses: pdfStyleFootnoteTitle}},
				BodyBlocks:  []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Ordinary body", Runs: []pdfInlineRun{{Text: "Ordinary body"}}, StyleClasses: pdfStyleFootnote, ContextClasses: pdfStyleFootnote}},
			},
			"n2": {
				ID:     "n2",
				Blocks: []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Nested actual title", Runs: []pdfInlineRun{{Text: "Nested actual title"}}, StyleClasses: pdfStyleFootnoteTitle, ContextClasses: pdfStyleFootnoteTitle}},
			},
		},
	}
	queue := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}, {ID: "n2", Nested: true}}

	blocks := pdfPrintedFootnoteQueueBlocks(doc, queue)
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		texts = append(texts, block.Text)
	}
	want := []string{"1", "Ordinary body", "Nested actual title"}
	if strings.Join(texts, "|") != strings.Join(want, "|") {
		t.Fatalf("queue block texts = %#v, want %#v", texts, want)
	}
}
