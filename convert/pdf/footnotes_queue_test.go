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
		{FootnoteID: "n1", Text: testShapedTextRunes("[17]")},
		{FootnoteID: "n1", Text: testShapedTextRunes("[duplicate]")},
		{FootnoteID: "n2", LinkHref: "#n2", Text: testShapedTextRunes("[clickable]")},
		{FootnoteID: "missing", Text: testShapedTextRunes("[missing]")},
	}}}}

	refs := pdfPrintedFootnotePageRefs(doc, page)
	want := []pdfPrintedFootnoteRef{{ID: "n1", Label: "[17]"}}
	if !reflect.DeepEqual(refs, want) {
		t.Fatalf("refs = %#v, want %#v", refs, want)
	}
}

func testShapedTextRunes(text string) shapedText {
	shaped := shapedText{Glyphs: make([]shapedGlyph, 0, len([]rune(text)))}
	for _, r := range text {
		shaped.Glyphs = append(shaped.Glyphs, shapedGlyph{Rune: r})
	}
	return shaped
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

	queue := buildPDFPrintedFootnoteQueue(doc, []pdfPrintedFootnoteRef{{ID: "n1", Label: "1"}})
	want := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}, {ID: "n2", PageLabel: "2", Nested: true}}
	if !reflect.DeepEqual(queue, want) {
		t.Fatalf("queue = %#v, want %#v", queue, want)
	}

	mainBlocks := pdfPrintedFootnoteBlocksForQueueEntry(c, footnotes["n1"], queue[0], false, nil)
	if len(mainBlocks) == 0 || mainBlocks[0].Text != "1\u00A0Main actual title" {
		t.Fatalf("main queued blocks = %#v, want page-local label plus actual title", mainBlocks)
	}
	queueBlocks := pdfPrintedFootnoteQueueBlocks(doc, queue)
	if got := plainPDFBlockTexts(queueBlocks); !reflect.DeepEqual(got, []string{"1\u00A0Main actual title", "See 2", "2\u00A0Nested actual title", "Nested body."}) {
		t.Fatalf("queue block texts = %#v, want relabeled non-clickable nested ref and nested body", got)
	}
}

func plainPDFBlockTexts(blocks []pdfTextBlock) []string {
	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		texts = append(texts, block.Text)
	}
	return texts
}

func TestBuildPDFPrintedFootnoteQueueFloatRenumberedUsesRawNestedReferenceLabels(t *testing.T) {
	c := testPDFPrintedFootnoteContent(
		fb2.Section{
			ID: "n1",
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Text: "See "},
					{Kind: fb2.InlineLink, Href: "#n2", Children: []fb2.InlineSegment{{Text: "[23]"}}},
				}},
			}},
		},
		fb2.Section{ID: "n2", Title: pdfTitleFromStrings("Nested actual title")},
	)
	c.FootnotesMode = common.FootnotesModeFloatRenumbered
	footnotes := buildPDFPrintedFootnoteBlocks(c)
	doc := pdfDocumentSpec{Content: c, PrintedFootnotes: footnotes}

	queue := buildPDFPrintedFootnoteQueue(doc, []pdfPrintedFootnoteRef{{ID: "n1", Label: "[1]"}})
	want := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}, {ID: "n2", PageLabel: "2", Nested: true}}
	if !reflect.DeepEqual(queue, want) {
		t.Fatalf("queue = %#v, want raw renumbered labels %#v", queue, want)
	}
	blocks := pdfPrintedFootnoteQueueBlocks(doc, queue)
	if got := plainPDFBlockTexts(blocks); !reflect.DeepEqual(got, []string{"1", "See 2", "2\u00A0Nested actual title"}) {
		t.Fatalf("queue block texts = %#v, want raw nested reference and title", got)
	}
}

func TestBuildPDFPrintedFootnoteQueueFloatUsesVisibleReferenceLabels(t *testing.T) {
	c := testPDFPrintedFootnoteContent(
		fb2.Section{
			ID:    "n1",
			Title: pdfTitleFromStrings("Main actual title"),
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Text: "See "},
					{Kind: fb2.InlineLink, Href: "#n2", Children: []fb2.InlineSegment{{Text: "[23]"}}},
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
	footnotes := buildPDFPrintedFootnoteBlocks(c)
	doc := pdfDocumentSpec{Content: c, PrintedFootnotes: footnotes}

	queue := buildPDFPrintedFootnoteQueue(doc, []pdfPrintedFootnoteRef{{ID: "n1", Label: "[17]"}})
	want := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "[17]"}, {ID: "n2", PageLabel: "[23]", Nested: true}}
	if !reflect.DeepEqual(queue, want) {
		t.Fatalf("queue = %#v, want %#v", queue, want)
	}
	blocks := pdfPrintedFootnoteQueueBlocks(doc, queue)
	if got := plainPDFBlockTexts(blocks); !reflect.DeepEqual(got, []string{"[17]\u00A0Main actual title", "See [23]", "[23]\u00A0Nested actual title", "Nested body."}) {
		t.Fatalf("queue block texts = %#v, want visible float reference labels", got)
	}
	for _, block := range blocks {
		for _, run := range block.Runs {
			if run.FootnoteID != "" && run.LinkHref != "" {
				t.Fatalf("queue run = %#v, want nested printed footnote refs non-clickable", run)
			}
		}
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

	queue := buildPDFPrintedFootnoteQueue(doc, []pdfPrintedFootnoteRef{{ID: "n1", Label: "1"}, {ID: "n2", Label: "2"}})
	want := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}, {ID: "n2", PageLabel: "2"}}
	if !reflect.DeepEqual(queue, want) {
		t.Fatalf("queue = %#v, want %#v", queue, want)
	}
}
