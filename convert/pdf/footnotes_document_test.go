package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/content"
)

func TestLayoutPDFDocumentPagesAppendsPrintedFootnotesAndReservesMainFlow(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	doc := pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 180,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {
				ID: "n1",
				BodyBlocks: []pdfTextBlock{{
					Kind:           pdfBlockParagraph,
					Text:           "Footnote body.",
					Runs:           []pdfInlineRun{{Text: "Footnote body."}},
					StyleClasses:   pdfStyleFootnote,
					ContextClasses: pdfStyleFootnote,
				}},
			},
		},
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "One 17", Runs: []pdfInlineRun{{Text: "One "}, {Text: "17", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n1", Superscript: true}}},
			{Kind: pdfBlockParagraph, Text: "Two"},
			{Kind: pdfBlockParagraph, Text: "Three"},
		},
	}

	pages, used, err := layoutPDFDocumentPages(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFDocumentPages() error = %v", err)
	}
	if len(pages) != 3 {
		texts := make([]string, len(pages))
		for i := range pages {
			texts[i] = pageText(pages[i])
		}
		t.Fatalf("pages = %d, want main page, footnote continuation, pushed main page; texts = %#v", len(pages), texts)
	}
	firstPageText := pageText(pages[0])
	if !strings.Contains(firstPageText, "One 1") {
		t.Fatalf("first page text = %q, want relabeled ref", firstPageText)
	}
	if strings.Contains(firstPageText, "Three") {
		t.Fatalf("first page text = %q, want trailing main text pushed away from footnote area", firstPageText)
	}
	if got := pageText(pages[1]); !strings.Contains(got, "Footnote body") || strings.Contains(got, "Three") {
		t.Fatalf("second page text = %q, want footnote continuation before pushed main text", got)
	}
	if got := pageText(pages[2]); !strings.Contains(got, "Three") {
		t.Fatalf("third page text = %q, want pushed trailing main text", got)
	}
	if len(pages[0].Backgrounds) == 0 {
		t.Fatalf("first page backgrounds=%#v, want separator geometry", pages[0].Backgrounds)
	}
	if len(pages[1].Backgrounds) != 0 {
		t.Fatalf("continuation backgrounds=%#v, want footnote continuation from top without bottom separator", pages[1].Backgrounds)
	}
	if usedGlyphCount(used) == 0 {
		t.Fatalf("used glyphs = %#v, want main and footnote glyph usage", used)
	}
}

func TestLayoutPDFDocumentPagesSkipsPrintedFootnotePathWhenModeDisabled(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	doc := pdfDocumentSpec{
		PageWidth:  220,
		PageHeight: 130,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeDefault},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {ID: "n1", BodyBlocks: []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Footnote body.", Runs: []pdfInlineRun{{Text: "Footnote body."}}}}},
		},
		Blocks: []pdfTextBlock{{Kind: pdfBlockParagraph, Text: "Body"}},
	}

	pages, _, err := layoutPDFDocumentPages(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFDocumentPages() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	if got := pageText(pages[0]); strings.Contains(got, "Footnote body") {
		t.Fatalf("page text = %q, want no printed-footnote append in default mode", got)
	}
	if len(pages[0].Backgrounds) != 0 {
		t.Fatalf("backgrounds = %#v, want no separator in default mode", pages[0].Backgrounds)
	}
}

func usedGlyphCount(used map[pdfFontKey]map[uint16]shapedGlyph) int {
	count := 0
	for _, glyphs := range used {
		count += len(glyphs)
	}
	return count
}
