package pdf

import (
	"reflect"
	"strings"
	"testing"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

func TestBuildPDFPrintedFootnotePagePlansDetectsContinuationPages(t *testing.T) {
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
	pages := []pdfPage{{Lines: []pdfPageLine{{Fragments: []pdfPageLineFragment{{FootnoteID: "n1"}}}}}}

	plans, err := buildPDFPrintedFootnotePagePlans(doc, pages, 80)
	if err != nil {
		t.Fatalf("buildPDFPrintedFootnotePagePlans() error = %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %#v, want one page plan", plans)
	}
	if plans[0].ContinuationPages == 0 || len(plans[0].QueuePages) < 2 {
		t.Fatalf("plan = %#v, want continuation queue pages for long footnote", plans[0])
	}
	var text strings.Builder
	for _, page := range plans[0].QueuePages {
		text.WriteString(pageText(page))
		text.WriteByte('\n')
	}
	if got := text.String(); !strings.Contains(got, "ENDMARK") {
		t.Fatalf("queue pages text = %q, want final marker preserved", got)
	}
}

func TestBuildPDFPrintedFootnotePagePlansQueuesNestedFootnotesWithActualTitle(t *testing.T) {
	c := testPDFPrintedFootnoteContent(
		fb2.Section{
			ID: "n1",
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
	doc := pdfDocumentSpec{
		PageWidth:        260,
		PageHeight:       180,
		Content:          c,
		PrintedFootnotes: buildPDFPrintedFootnoteBlocks(c),
	}
	pages := []pdfPage{{Lines: []pdfPageLine{{Fragments: []pdfPageLineFragment{{FootnoteID: "n1"}}}}}}

	plans, err := buildPDFPrintedFootnotePagePlans(doc, pages, 120)
	if err != nil {
		t.Fatalf("buildPDFPrintedFootnotePagePlans() error = %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("plans = %#v, want one page plan", plans)
	}
	wantQueue := []pdfPrintedFootnoteQueueEntry{{ID: "n1", PageLabel: "1"}, {ID: "n2", PageLabel: "2", Nested: true}}
	if !reflect.DeepEqual(plans[0].Queue, wantQueue) {
		t.Fatalf("queue = %#v, want %#v", plans[0].Queue, wantQueue)
	}
	var text strings.Builder
	for _, page := range plans[0].QueuePages {
		text.WriteString(pageText(page))
		text.WriteByte('\n')
	}
	got := text.String()
	if !strings.Contains(got, "1") || !strings.Contains(got, "2 Nested actual title") || !strings.Contains(got, "Nested body.") {
		t.Fatalf("queue pages text = %q, want main label plus nested label/title/body", got)
	}
}

func TestBuildPDFPrintedFootnotePagePlansSkipsPagesWithoutPrintedRefs(t *testing.T) {
	doc := pdfDocumentSpec{PrintedFootnotes: map[string]pdfPrintedFootnote{"n1": {ID: "n1"}}}
	pages := []pdfPage{{Lines: []pdfPageLine{{Fragments: []pdfPageLineFragment{{FootnoteID: "n1", LinkHref: "#n1"}}}}}}

	plans, err := buildPDFPrintedFootnotePagePlans(doc, pages, 80)
	if err != nil {
		t.Fatalf("buildPDFPrintedFootnotePagePlans() error = %v", err)
	}
	if len(plans) != 0 {
		t.Fatalf("plans = %#v, want none for clickable nested-only refs", plans)
	}
}
