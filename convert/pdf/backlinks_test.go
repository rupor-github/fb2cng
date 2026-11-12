package pdf

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestPDFBacklinkBlockContent(t *testing.T) {
	for _, tt := range []struct {
		name, template, want string
		wantHrefs            []string
	}{
		{name: "empty", template: `{{- "" -}}`},
		{name: "whitespace", template: "{{ ` \n\t ` }}"},
		{name: "nonempty", template: " return ", want: "return\u00a0return", wantHrefs: []string{"#ref-n1-1", "#ref-n1-2"}},
		{
			name: "selective", template: `{{- if eq .RefNumber 2 -}}second{{- end -}}`,
			want: "second", wantHrefs: []string{"#ref-n1-2"},
		},
		{
			name: "selective first", template: `{{- if eq .RefNumber 1 -}}first{{- end -}}`,
			want: "first", wantHrefs: []string{"#ref-n1-1"},
		},
		{
			name: "missing template", want: "[<]\u00a0[<]",
			wantHrefs: []string{"#ref-n1-1", "#ref-n1-2"},
		},
		{
			name: "invalid template", template: "{{", want: "[<]\u00a0[<]",
			wantHrefs: []string{"#ref-n1-1", "#ref-n1-2"},
		},
		{
			name: "execution failure", template: "{{.MissingField}}", want: "[<]\u00a0[<]",
			wantHrefs: []string{"#ref-n1-1", "#ref-n1-2"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &content.Content{OutputFormat: common.OutputFmtPdf, BacklinkTemplate: tt.template}
			first := c.AddFootnoteBackLinkRef("n1")
			second := c.AddFootnoteBackLinkRef("n1")
			text, runs := pdfBacklinkBlockContent(c, []string{first.RefID, second.RefID})
			if text != tt.want {
				t.Fatalf("backlink text = %q, want %q", text, tt.want)
			}
			var hrefs []string
			for _, run := range runs {
				if run.LinkHref != "" {
					hrefs = append(hrefs, run.LinkHref)
				}
			}
			if !slices.Equal(hrefs, tt.wantHrefs) {
				t.Fatalf("backlink hrefs = %v, want %v", hrefs, tt.wantHrefs)
			}
			if tt.want == "" && len(runs) != 0 {
				t.Fatalf("empty backlinks have runs: %#v", runs)
			}
		})
	}
}

func TestPDFBacklinkTemplateModes(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []common.FootnotesMode{
		common.FootnotesModeDefault, common.FootnotesModeFloat, common.FootnotesModeFloatRenumbered,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			var printedPages []pdfPage
			for _, tt := range []struct {
				name, template string
				enabled        bool
			}{
				{name: "default template", template: cfg.Document.Footnotes.BacklinkTemplate, enabled: true},
				{name: "custom template", template: "RETURN", enabled: true},
				{name: "page-dependent template", template: `{{- if .PageNumber -}}RETURN{{- end -}}`, enabled: true},
				{name: "empty", template: `{{- "" -}}`},
				{name: "whitespace", template: "{{ ` \n\t ` }}"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					c := testContentWithFootnotes("n1")
					c.OutputFormat = common.OutputFmtPdf
					c.FootnotesMode = mode
					c.BacklinkTemplate = tt.template
					c.Book = &fb2.FictionBook{Bodies: []fb2.Body{{
						Kind: fb2.BodyMain,
						Sections: []fb2.Section{{Content: []fb2.FlowItem{{
							Kind: fb2.FlowParagraph,
							Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
								{Text: "Body "},
								{Kind: fb2.InlineLink, Href: "#n1", Text: "1"},
								{Text: " repeated "},
								{Kind: fb2.InlineLink, Href: "#n1", Text: "1"},
							}},
						}}}},
					}, {
						Kind: fb2.BodyFootnotes,
						Sections: []fb2.Section{{ID: "n1", Content: []fb2.FlowItem{{
							Kind:      fb2.FlowParagraph,
							Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote body."}}},
						}}}},
					}}}
					plan, err := collectPDFContent(c, &cfg.Document)
					if err != nil {
						t.Fatal(err)
					}
					doc := pdfDocumentSpec{
						PageWidth: 240, PageHeight: 320, Content: c, Blocks: plan.Blocks, PrintedFootnotes: plan.PrintedFootnotes,
					}
					pages, _, _, err := layoutPDFDocumentPages(doc)
					if err != nil {
						t.Fatal(err)
					}
					for range 3 {
						if !resolvePDFBacklinkPagesAndText(&doc, pages) {
							break
						}
						pages, _, _, err = layoutPDFDocumentPages(doc)
						if err != nil {
							t.Fatal(err)
						}
					}
					var text strings.Builder
					forward, backward := 0, 0
					for _, page := range pages {
						text.WriteString(pageText(page))
						for _, annotation := range page.Annotations {
							if annotation.Href == "#n1" {
								forward++
							} else if strings.HasPrefix(annotation.Href, "#ref-n1-") {
								backward++
							}
						}
					}
					if !strings.Contains(text.String(), "Body") || !strings.Contains(text.String(), "Footnote body.") {
						t.Fatalf("lost main text or note: %q", text.String())
					}
					wantForward, wantBackward := 0, 0
					if mode == common.FootnotesModeDefault {
						wantForward = 2
						if tt.enabled {
							wantBackward = 2
							if tt.name == "default template" && !strings.Contains(text.String(), "[page\u00a01]") {
								t.Fatalf("default template lost page-number label: %q", text.String())
							}
						}
					} else if printedPages == nil {
						printedPages = pages
					} else if !reflect.DeepEqual(pages, printedPages) {
						t.Fatal("backlink template changed printed-footnote layout")
					}
					if forward != wantForward || backward != wantBackward {
						t.Fatalf("links forward/backward = %d/%d, want %d/%d", forward, backward, wantForward, wantBackward)
					}
					if !tt.enabled && strings.Contains(text.String(), "[<]") {
						t.Fatalf("empty result became fallback: %q", text.String())
					}
				})
			}
		})
	}
}

func TestPDFBacklinkInitiallyEmptyPageTemplate(t *testing.T) {
	c := &content.Content{
		OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeDefault,
		BacklinkTemplate: `{{- if gt .PageNumber 1 -}}page {{ .PageNumber }}{{- end -}}`,
	}
	ref := c.AddFootnoteBackLinkRef("n1")
	var blocks []pdfTextBlock
	appendPDFDefaultFootnoteBacklinkBlock(&blocks, c, "n1")
	if len(blocks) != 1 || !pdfEmptyBacklinkBlock(blocks[0]) {
		t.Fatalf("blocks = %#v, want empty candidate retained for pagination", blocks)
	}
	doc := pdfDocumentSpec{Content: c, Blocks: blocks}
	for _, tt := range []struct {
		name string
		page int
		want string
	}{
		{name: "appears after pagination", page: 2, want: "page 2"},
		{name: "disappears after page change", page: 1},
		{name: "reappears after page change", page: 3, want: "page 3"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pages := make([]pdfPage, tt.page)
			pages[tt.page-1].Anchors = []string{ref.RefID}
			if !resolvePDFBacklinkPagesAndText(&doc, pages) {
				t.Fatal("expected page/text update")
			}
			if got := doc.Blocks[0].Text; got != tt.want {
				t.Fatalf("text = %q, want %q", got, tt.want)
			}
			if len(doc.Blocks[0].BacklinkRefIDs) != 1 {
				t.Fatal("lost initially empty backlink candidate")
			}
			if tt.want == "" && len(doc.Blocks[0].Runs) != 0 {
				t.Fatal("suppressed backlink retained link runs")
			}
		})
	}
}

func TestPDFEmptyBacklinkDoesNotAffectLayout(t *testing.T) {
	doc := pdfDocumentSpec{
		PageWidth: 240, PageHeight: 180,
		Styles: newPDFStyleResolverWithCSS(t, `p { margin: 12pt 0; } .footnote p { page-break-before: always; margin: 40pt; }`),
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: "Before"},
			{Kind: pdfBlockParagraph, ContextClasses: pdfStyleFootnote, BacklinkRefIDs: []string{"ref-n1-1"}},
			{Kind: pdfBlockParagraph, Text: "After"},
		},
	}
	pages, _, err := layoutPDFPages(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 3 || len(doc.Blocks[1].BacklinkRefIDs) != 1 {
		t.Fatal("layout mutated source candidates")
	}
	doc.Blocks = []pdfTextBlock{doc.Blocks[0], doc.Blocks[2]}
	wantPages, _, err := layoutPDFPages(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pages, wantPages) {
		t.Fatal("empty backlink introduced spacing, page breaks, or annotations")
	}
}

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
