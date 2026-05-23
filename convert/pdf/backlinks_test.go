package pdf

import (
	"testing"

	"fbc/common"
	"fbc/content"
)

func TestResolvePDFBacklinkPagesSkipsSamePageBacklink(t *testing.T) {
	c := &content.Content{
		OutputFormat:     common.OutputFmtPdf,
		BacklinkTemplate: "↩",
		BackLinkIndex: map[string][]content.BackLinkRef{
			"n1": {{RefID: "ref-n1-1", TargetID: "n1", RefNumber: 1, Format: "pdf"}},
		},
		FootnotesMode: common.FootnotesModeDefault,
	}
	doc := pdfDocumentSpec{
		Content: c,
		Blocks: []pdfTextBlock{{
			Kind:           pdfBlockParagraph,
			Text:           "↩",
			Runs:           []pdfInlineRun{{Text: "↩", StyleClasses: pdfStyleLinkBacklink, LinkHref: "#ref-n1-1"}},
			BacklinkRefIDs: []string{"ref-n1-1"},
		}},
	}
	changed := resolvePDFBacklinkPagesAndText(&doc, []pdfPage{{
		Anchors:     []string{"ref-n1-1"},
		Annotations: []pdfLinkAnnotation{{Href: "#ref-n1-1"}},
	}})
	if !changed {
		t.Fatal("resolvePDFBacklinkPagesAndText() changed = false, want true")
	}
	if got := doc.Blocks[0].Text; got != "" {
		t.Fatalf("same-page backlink text = %q, want empty", got)
	}
	if len(doc.Blocks[0].Runs) != 0 {
		t.Fatalf("same-page backlink runs = %#v, want none", doc.Blocks[0].Runs)
	}
	if len(doc.Blocks[0].BacklinkRefIDs) != 0 {
		t.Fatalf("same-page BacklinkRefIDs = %#v, want none", doc.Blocks[0].BacklinkRefIDs)
	}
}

func TestResolvePDFBacklinkPagesKeepsDifferentPageBacklink(t *testing.T) {
	const backlinkTemplate = `[{{- if .PageNumber -}}page {{ .PageNumber }}{{- else -}}<{{- end -}}]`
	c := &content.Content{
		OutputFormat:     common.OutputFmtPdf,
		BacklinkTemplate: backlinkTemplate,
		BackLinkIndex: map[string][]content.BackLinkRef{
			"n1": {{RefID: "ref-n1-1", TargetID: "n1", RefNumber: 1, Format: "pdf"}},
		},
		FootnotesMode: common.FootnotesModeDefault,
	}
	doc := pdfDocumentSpec{
		Content: c,
		Blocks: []pdfTextBlock{{
			Kind:           pdfBlockParagraph,
			Text:           "[<]",
			Runs:           []pdfInlineRun{{Text: "[<]", StyleClasses: pdfStyleLinkBacklink, LinkHref: "#ref-n1-1"}},
			BacklinkRefIDs: []string{"ref-n1-1"},
		}},
	}
	changed := resolvePDFBacklinkPagesAndText(&doc, []pdfPage{
		{Anchors: []string{"ref-n1-1"}},
		{Annotations: []pdfLinkAnnotation{{Href: "#ref-n1-1"}}},
	})
	if !changed {
		t.Fatal("resolvePDFBacklinkPagesAndText() changed = false, want true")
	}
	if got := doc.Blocks[0].Text; got != "[page 1]" {
		t.Fatalf("different-page backlink text = %q, want [page 1]", got)
	}
	if len(doc.Blocks[0].BacklinkRefIDs) != 1 || doc.Blocks[0].BacklinkRefIDs[0] != "ref-n1-1" {
		t.Fatalf("different-page BacklinkRefIDs = %#v, want ref-n1-1", doc.Blocks[0].BacklinkRefIDs)
	}
}

func TestResolvePDFBacklinkPagesRendersTemplate(t *testing.T) {
	const backlinkTemplate = `[{{- if .PageNumber -}}page {{ .PageNumber }}{{- else -}}<{{- end -}}]`
	c := &content.Content{
		OutputFormat:        common.OutputFmtPdf,
		BacklinkTemplate:    backlinkTemplate,
		BackLinkIndex:       map[string][]content.BackLinkRef{"n1": {{RefID: "ref-n1-1", TargetID: "n1", RefNumber: 1, Format: "pdf"}}},
		FootnotesMode:       common.FootnotesModeDefault,
		FootnotesIndex:      nil,
		CurrentChapterTitle: "Chapter",
	}
	text, runs := pdfBacklinkBlockContent(c, []string{"ref-n1-1"})
	doc := pdfDocumentSpec{
		Content: c,
		Blocks: []pdfTextBlock{{
			Kind:           pdfBlockParagraph,
			Text:           text,
			Runs:           runs,
			BacklinkRefIDs: []string{"ref-n1-1"},
		}},
	}
	changed := resolvePDFBacklinkPagesAndText(&doc, []pdfPage{{Anchors: []string{"ref-n1-1"}}})
	if !changed {
		t.Fatal("resolvePDFBacklinkPagesAndText() changed = false, want true")
	}
	if got := doc.Blocks[0].Text; got != "[page 1]" {
		t.Fatalf("backlink text = %q, want [page 1]", got)
	}
	if len(doc.Blocks[0].Runs) != 1 || doc.Blocks[0].Runs[0].Text != "[page 1]" {
		t.Fatalf("backlink runs = %#v, want one [page 1] run", doc.Blocks[0].Runs)
	}
}
