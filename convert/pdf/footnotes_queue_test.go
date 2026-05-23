package pdf

import (
	"reflect"
	"testing"

	"fbc/common"
	"fbc/fb2"
)

func TestPDFPrintedFootnotePageRefsDedupesMainRefsAndSkipsNestedLinks(t *testing.T) {
	doc := pdfDocumentSpec{PrintedFootnotes: map[string]pdfPrintedFootnote{
		"n1": {ID: "n1"},
		"n2": {ID: "n2"},
	}}
	page := pdfPage{Lines: []pdfPageLine{{Fragments: []pdfPageLineFragment{
		{FootnoteID: "n1"},
		{FootnoteID: "n1"},
		{FootnoteID: "n2", LinkHref: "#n2"},
		{FootnoteID: "missing"},
	}}}}

	refs := pdfPrintedFootnotePageRefs(doc, page)
	if !reflect.DeepEqual(refs, []string{"n1"}) {
		t.Fatalf("refs = %#v, want only deduped non-clickable main ref n1", refs)
	}
}

func TestBuildPDFPrintedFootnoteQueueAddsNestedRefsAfterMainRefs(t *testing.T) {
	c := testPDFPrintedFootnoteContent(
		fb2.Section{
			ID:    "n1",
			Title: pdfTitleFromStrings("Main actual title"),
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Text: "See "},
					{Kind: fb2.InlineLink, Href: "#n2", Children: []fb2.InlineSegment{{Text: "nested"}}},
				}},
			}},
		},
		fb2.Section{
			ID:    "n2",
			Title: pdfTitleFromStrings("Nested actual title"),
			Content: []fb2.FlowItem{{
				Kind:      fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested body."}}},
			}},
		},
	)
	c.FootnotesMode = common.FootnotesModeFloatRenumbered
	footnotes := buildPDFPrintedFootnoteBlocks(c)
	doc := pdfDocumentSpec{Content: c, PrintedFootnotes: footnotes}

	queue := buildPDFPrintedFootnoteQueue(doc, []string{"n1"})
	want := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}, {ID: "n2", Nested: true}}
	if !reflect.DeepEqual(queue, want) {
		t.Fatalf("queue = %#v, want %#v", queue, want)
	}

	mainBlocks := pdfPrintedFootnoteBlocksForQueueEntry(c, footnotes["n1"], queue[0], false)
	if len(mainBlocks) == 0 || mainBlocks[0].Text != "1" {
		t.Fatalf("main queued blocks = %#v, want floatRenumbered page-local label only", mainBlocks)
	}
	nestedBlocks := pdfPrintedFootnoteBlocksForQueueEntry(c, footnotes["n2"], queue[1], false)
	if len(nestedBlocks) < 2 || nestedBlocks[0].Text != "Nested actual title" || nestedBlocks[1].Text != "Nested body." {
		t.Fatalf("nested queued blocks = %#v, want actual section title and body", nestedBlocks)
	}
}

func TestBuildPDFPrintedFootnoteQueueDedupesNestedRefsAlreadyOnPage(t *testing.T) {
	c := testPDFPrintedFootnoteContent(
		fb2.Section{
			ID: "n1",
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Kind: fb2.InlineLink, Href: "#n2", Children: []fb2.InlineSegment{{Text: "nested"}}},
				}},
			}},
		},
		fb2.Section{ID: "n2", Content: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Second body."}}},
		}}},
	)
	footnotes := buildPDFPrintedFootnoteBlocks(c)
	doc := pdfDocumentSpec{Content: c, PrintedFootnotes: footnotes}

	queue := buildPDFPrintedFootnoteQueue(doc, []string{"n1", "n2"})
	want := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}, {ID: "n2", PageLabel: "2"}}
	if !reflect.DeepEqual(queue, want) {
		t.Fatalf("queue = %#v, want %#v", queue, want)
	}
}
