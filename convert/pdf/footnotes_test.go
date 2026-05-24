package pdf

import (
	"testing"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

func testPDFPrintedFootnoteContent(sections ...fb2.Section) *content.Content {
	refs := make(fb2.FootnoteRefs, len(sections))
	for i := range sections {
		refs[sections[i].ID] = fb2.FootnoteRef{BodyIdx: 0, SectionIdx: i, NoteNum: i + 1}
	}
	return &content.Content{
		OutputFormat:   common.OutputFmtPdf,
		FootnotesMode:  common.FootnotesModeFloat,
		FootnotesIndex: refs,
		Book: &fb2.FictionBook{Bodies: []fb2.Body{{
			Kind:     fb2.BodyFootnotes,
			Name:     "notes",
			Sections: sections,
		}}},
	}
}

func TestBuildPDFPrintedFootnoteBlocksSeparatesExistingTitleAndBody(t *testing.T) {
	c := testPDFPrintedFootnoteContent(fb2.Section{
		ID:    "n1",
		Title: pdfTitleFromStrings("Translator note"),
		Content: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
		}},
	})
	c.FootnotesIndex["n1"] = fb2.FootnoteRef{BodyIdx: 0, SectionIdx: 0, NoteNum: 1, DisplayText: "1"}

	footnotes := buildPDFPrintedFootnoteBlocks(c)
	note := footnotes["n1"]
	if len(note.TitleBlocks) != 1 || note.TitleBlocks[0].Text != "Translator note" || note.TitleBlocks[0].ID != "n1" {
		t.Fatalf("title blocks = %#v, want existing section title anchored to n1", note.TitleBlocks)
	}
	if note.TitleBlocks[0].Text == c.FootnotesIndex["n1"].DisplayText {
		t.Fatalf("existing title was replaced with display label: %#v", note.TitleBlocks[0])
	}
	if len(note.BodyBlocks) != 1 || note.BodyBlocks[0].Text != "Footnote body." {
		t.Fatalf("body blocks = %#v, want footnote content without title", note.BodyBlocks)
	}
}

func TestPDFPrintedFootnotePageBlocksFloatPrependPageLocalLabelToActualTitle(t *testing.T) {
	c := testPDFPrintedFootnoteContent(fb2.Section{
		ID:    "n1",
		Title: pdfTitleFromStrings("Translator note"),
		Content: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
		}},
	})

	blocks := pdfPrintedFootnotePageBlocks(c, buildPDFPrintedFootnoteBlocks(c)["n1"], "2", false)
	if len(blocks) < 2 || blocks[0].Text != "2 Translator note" || blocks[1].Text != "Footnote body." {
		t.Fatalf("page footnote blocks = %#v, want page label, title, and body", blocks)
	}
}

func TestPDFPrintedFootnotePageBlocksFloatRenumberedPrependsPageLocalLabelToActualTitle(t *testing.T) {
	c := testPDFPrintedFootnoteContent(fb2.Section{
		ID:    "n1",
		Title: pdfTitleFromStrings("Примечание 17"),
		Content: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
		}},
	})
	c.FootnotesMode = common.FootnotesModeFloatRenumbered
	c.FootnotesIndex["n1"] = fb2.FootnoteRef{BodyIdx: 0, SectionIdx: 0, NoteNum: 1, DisplayText: "1"}

	blocks := pdfPrintedFootnotePageBlocks(c, buildPDFPrintedFootnoteBlocks(c)["n1"], "3", false)
	if len(blocks) < 2 || blocks[0].Text != "3 Примечание 17" || blocks[1].Text != "Footnote body." {
		t.Fatalf("page footnote blocks = %#v, want page-local label, title, and body", blocks)
	}
}

func TestPDFPrintedFootnotePageBlocksMissingTitleUsesOnlyPageLocalLabel(t *testing.T) {
	c := testPDFPrintedFootnoteContent(fb2.Section{
		ID: "n1",
		Content: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
		}},
	})
	c.FootnotesIndex["n1"] = fb2.FootnoteRef{BodyIdx: 0, SectionIdx: 0, NoteNum: 1, DisplayText: "7"}

	blocks := pdfPrintedFootnotePageBlocks(c, buildPDFPrintedFootnoteBlocks(c)["n1"], "4", false)
	if len(blocks) < 2 || blocks[0].Text != "4" || blocks[0].ID != "n1" {
		t.Fatalf("page title block = %#v, want page-local label 4 anchored to n1", blocks)
	}
	if !hasPDFStyleClass(blocks[0].StyleClasses, pdfStyleFootnoteTitle) {
		t.Fatalf("page title classes = %q, want %q", blocks[0].StyleClasses, pdfStyleFootnoteTitle)
	}
}

func TestBuildPDFPrintedFootnoteBlocksContinuationTitleDoesNotAppendMarker(t *testing.T) {
	c := testPDFPrintedFootnoteContent(fb2.Section{
		ID: "n1",
		Title: &fb2.Title{Items: []fb2.TitleItem{
			{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "First"}}}},
			{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Second"}}}},
		}},
		Content: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
		}},
	})
	footnotes := buildPDFPrintedFootnoteBlocks(c)
	note := footnotes["n1"]
	if len(note.ContinuationTitleBlocks) != 2 {
		t.Fatalf("continuation title blocks = %#v, want two title paragraphs", note.ContinuationTitleBlocks)
	}
	if note.ContinuationTitleBlocks[0].Text != "First" || note.ContinuationTitleBlocks[1].Text != "Second" {
		t.Fatalf("continuation title blocks = %#v, want unmarked title paragraphs", note.ContinuationTitleBlocks)
	}
	pageBlocks := pdfPrintedFootnotePageBlocks(c, note, "1", true)
	if pageBlocks[0].Text != "1 First" || pageBlocks[1].Text != "Second" {
		t.Fatalf("continuation page title blocks = %#v, want page label plus unmarked title", pageBlocks[:2])
	}
}

func TestBuildPDFPrintedFootnoteBlocksAppliesFootnoteContextToAllBodyBlocks(t *testing.T) {
	c := testPDFPrintedFootnoteContent(fb2.Section{
		ID: "n1",
		Epigraphs: []fb2.Epigraph{{Flow: fb2.Flow{Items: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Epigraph."}}},
		}}}}},
		Content: []fb2.FlowItem{{
			Kind: fb2.FlowCite,
			Cite: &fb2.Cite{Items: []fb2.FlowItem{{
				Kind:      fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Quote."}}},
			}}},
		}},
	})

	for _, block := range buildPDFPrintedFootnoteBlocks(c)["n1"].BodyBlocks {
		if !hasPDFStyleClass(block.ContextClasses, pdfStyleFootnote) {
			t.Fatalf("body block context = %q for %#v, want footnote context", block.ContextClasses, block)
		}
	}
}

func TestBuildPDFPrintedFootnoteBlocksKeepsNestedFootnoteRefsNonClickable(t *testing.T) {
	c := testPDFPrintedFootnoteContent(
		fb2.Section{
			ID:    "n1",
			Title: pdfTitleFromStrings("1"),
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Text: "See "},
					{Kind: fb2.InlineLink, Href: "#n2", Children: []fb2.InlineSegment{{Text: "2"}}},
				}},
			}},
		},
		fb2.Section{ID: "n2", Title: pdfTitleFromStrings("2")},
	)

	footnotes := buildPDFPrintedFootnoteBlocks(c)
	var body *pdfTextBlock
	for i := range footnotes["n1"].BodyBlocks {
		block := &footnotes["n1"].BodyBlocks[i]
		if block.Text == "See 2" {
			body = block
			break
		}
	}
	if body == nil {
		t.Fatalf("nested-ref body block not found: %#v", footnotes["n1"].BodyBlocks)
	}
	if len(body.Runs) != 2 || body.Runs[1].LinkHref != "" || body.Runs[1].FootnoteID != "n2" {
		t.Fatalf("nested footnote ref run = %#v, want non-clickable printed target n2", body.Runs)
	}
	if len(body.Links) != 0 {
		t.Fatalf("nested footnote block links = %#v, want footnote link span removed", body.Links)
	}
}
