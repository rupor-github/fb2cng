package pdf

import (
	"testing"

	"fbc/common"
	"fbc/content"
	"fbc/convert/pdf/structure"
	"fbc/fb2"
)

func testContentWithFootnotes(ids ...string) *content.Content {
	refs := make(fb2.FootnoteRefs, len(ids))
	for i, id := range ids {
		refs[id] = fb2.FootnoteRef{BodyIdx: 1, SectionIdx: i}
	}
	return &content.Content{FootnotesIndex: refs}
}

func TestCollectTextBlocksIncludesLinkChildren(t *testing.T) {
	c := &content.Content{Book: &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{Content: []fb2.FlowItem{{
			Kind: fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{
				Text: "See ",
			}, {
				Kind:     fb2.InlineLink,
				Href:     "#target",
				Children: []fb2.InlineSegment{{Text: "linked text"}},
			}}},
		}}}},
	}}}}

	blocks, err := collectTextBlocks(c)
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	blocks = textBlocksOnly(blocks)
	if len(blocks) != 1 {
		t.Fatalf("collectTextBlocks() produced %d text blocks, want 1", len(blocks))
	}
	if got := blocks[0].Text; got != "See linked text" {
		t.Fatalf("block text = %q, want %q", got, "See linked text")
	}
}

func TestInlineSegmentsTextAndLinksNormalizesWhitespaceAndLinkRanges(t *testing.T) {
	text, links := inlineSegmentsTextAndLinks([]fb2.InlineSegment{
		{Text: "\n  "},
		{Kind: fb2.InlineEmphasis, Children: []fb2.InlineSegment{
			{Kind: fb2.InlineLink, Href: "#one", Children: []fb2.InlineSegment{{Text: "One"}}},
			{Text: "\n   |\n   "},
			{Kind: fb2.InlineLink, Href: "#two", Children: []fb2.InlineSegment{{Text: "Two"}}},
		}},
		{Text: "\n"},
	})

	if text != "One | Two" {
		t.Fatalf("text = %q, want normalized text", text)
	}
	want := []pdfTextLink{{Start: 0, End: 3, Href: "#one"}, {Start: 6, End: 9, Href: "#two"}}
	if len(links) != len(want) {
		t.Fatalf("links = %#v, want %#v", links, want)
	}
	for i := range want {
		if links[i] != want[i] {
			t.Fatalf("links[%d] = %#v, want %#v", i, links[i], want[i])
		}
	}
}

func TestTitleBlocksPreserveInlineLinkFormatting(t *testing.T) {
	var blocks []pdfTextBlock
	c := testContentWithFootnotes("note")
	appendTitleBlocksFull(&blocks, c, &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "Heading"},
		{Kind: fb2.InlineLink, Href: "#note", Children: []fb2.InlineSegment{{Text: "1.1"}}},
	}}}}}, 1, "heading-id", pdfHeadingStyleName(1), "", "", false)

	if len(blocks) != 1 {
		t.Fatalf("title blocks = %#v, want one heading", blocks)
	}
	if blocks[0].StyleName != pdfStyleChapterTitleHeader || blocks[0].StyleClasses != pdfStyleChapterTitleHeader+"-first" {
		t.Fatalf("title block style = %q / %q, want chapter title first-line styling", blocks[0].StyleName, blocks[0].StyleClasses)
	}
	if len(blocks[0].Links) != 1 || blocks[0].Links[0].Href != "#note" {
		t.Fatalf("title block links = %#v, want note link", blocks[0].Links)
	}
	if len(blocks[0].Runs) != 2 || blocks[0].Runs[1].Text != "1.1" || blocks[0].Runs[1].StyleClasses != pdfStyleLinkFootnote {
		t.Fatalf("title block runs = %#v, want footnote link class", blocks[0].Runs)
	}
}

func TestParagraphInlineRunsClassifyLinks(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "See "},
		{Kind: fb2.InlineLink, Href: "https://example.com", Children: []fb2.InlineSegment{{Text: "external"}}},
		{Text: " and note"},
		{Kind: fb2.InlineLink, Href: "#note", Children: []fb2.InlineSegment{{Text: "1.1"}}},
	}}

	runs := paragraphInlineRuns(paragraph, testContentWithFootnotes("note"))
	if len(runs) != 4 {
		t.Fatalf("inline runs = %#v, want 4 runs", runs)
	}
	if runs[1].Text != "external" || runs[1].StyleClasses != pdfStyleLinkExternal {
		t.Fatalf("external link run = %#v, want external link class", runs[1])
	}
	if runs[3].Text != "1.1" || runs[3].StyleClasses != pdfStyleLinkFootnote || runs[3].LinkHref != "#note" {
		t.Fatalf("note link run = %#v, want footnote link class", runs[3])
	}
}

func TestPDFPrintedFootnoteReferencesAreStyledButNotClickable(t *testing.T) {
	c := &content.Content{
		OutputFormat:  common.OutputFmtPdf,
		FootnotesMode: common.FootnotesModeFloat,
		FootnotesIndex: fb2.FootnoteRefs{
			"n1": {BodyIdx: 1, SectionIdx: 0},
		},
		Book: &fb2.FictionBook{Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Text: "Body "},
					{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
				}},
			}}}},
		}}},
	}

	blocks, err := collectTextBlocks(c)
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	blocks = textBlocksOnly(blocks)
	if len(blocks) != 1 {
		t.Fatalf("blocks = %#v, want one main-text paragraph", blocks)
	}
	block := blocks[0]
	if len(block.Links) != 0 {
		t.Fatalf("printed footnote main-text Links = %#v, want none", block.Links)
	}
	if len(block.Runs) != 2 || block.Runs[1].StyleClasses != pdfStyleLinkFootnote || block.Runs[1].LinkHref != "" || block.Runs[1].FootnoteID != "n1" {
		t.Fatalf("printed footnote run = %#v, want styled non-clickable ref with metadata", block.Runs)
	}
	if len(c.BackLinkIndex) != 0 {
		t.Fatalf("printed footnotes should not register backlinks, got %#v", c.BackLinkIndex)
	}
}

func TestPDFFootnoteReferenceDropsFormattingWhitespaceInsideLink(t *testing.T) {
	c := testContentWithFootnotes("n1")
	c.OutputFormat = common.OutputFmtPdf
	c.FootnotesMode = common.FootnotesModeFloatRenumbered
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "Body"},
		{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{
			{Text: "\n          "},
			{Kind: fb2.InlineSup, Children: []fb2.InlineSegment{{Text: "{II}"}}},
			{Text: "\n        "},
		}},
		{Text: ", tail"},
	}}

	runs := paragraphInlineRuns(paragraph, c)
	if len(runs) != 3 {
		t.Fatalf("runs = %#v, want body, one footnote ref, tail", runs)
	}
	if runs[1].Text != "{II}" || runs[1].FootnoteID != "n1" || !runs[1].Superscript {
		t.Fatalf("footnote run = %#v, want only visible superscript child with footnote metadata", runs[1])
	}
}

func TestPDFPrintedFootnotePseudoContentDecoratesNonClickableRefs(t *testing.T) {
	resolver := &pdfStyleResolver{pseudoContent: map[string]pdfPseudoElementContent{
		pdfStyleLinkFootnote: {Before: "[", After: "]"},
	}}
	blocks := applyPDFPseudoContentToBlocks([]pdfTextBlock{{
		Kind: pdfBlockParagraph,
		Text: "Body 1",
		Runs: []pdfInlineRun{
			{Text: "Body "},
			{Text: "1", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n1"},
		},
	}}, resolver)

	if got := blocks[0].Runs[1]; got.Text != "[1]" || got.LinkHref != "" || got.FootnoteID != "n1" {
		t.Fatalf("decorated run = %#v, want non-clickable footnote metadata preserved", got)
	}
}

func TestPDFDefaultModeFootnoteBacklinks(t *testing.T) {
	c := &content.Content{
		OutputFormat:     common.OutputFmtPdf,
		FootnotesMode:    common.FootnotesModeDefault,
		BacklinkTemplate: "↩",
		FootnotesIndex: fb2.FootnoteRefs{
			"n1": {BodyIdx: 1, SectionIdx: 0},
		},
		Book: &fb2.FictionBook{Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Text: "Body "},
					{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
				}},
			}}}},
		}, {
			Kind: fb2.BodyFootnotes,
			Sections: []fb2.Section{{
				ID:    "n1",
				Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "1"}}}}}},
				Content: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote."}}},
				}},
			}},
		}}},
	}

	blocks, err := collectTextBlocks(c)
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	if refs := c.BackLinkIndex["n1"]; len(refs) != 1 || refs[0].RefID != "ref-n1-1" {
		t.Fatalf("BackLinkIndex[n1] = %#v, want ref-n1-1", refs)
	}

	var referenceRun *pdfInlineRun
	var backlinkBlock *pdfTextBlock
	for i := range blocks {
		block := &blocks[i]
		for j := range block.Runs {
			run := &block.Runs[j]
			if run.StyleClasses == pdfStyleLinkFootnote && run.LinkHref == "#n1" {
				referenceRun = run
			}
			if run.StyleClasses == pdfStyleLinkBacklink && run.LinkHref == "#ref-n1-1" {
				backlinkBlock = block
			}
		}
	}
	if referenceRun == nil || referenceRun.AnchorID != "ref-n1-1" {
		t.Fatalf("footnote reference run = %#v, want backlink anchor ref-n1-1", referenceRun)
	}
	if backlinkBlock == nil {
		t.Fatalf("blocks = %#v, want backlink paragraph to #ref-n1-1", blocks)
	}
	if hasPDFStyleClass(backlinkBlock.StyleClasses, pdfStyleFootnote) {
		t.Fatalf("backlink block classes = %q, want no footnote paragraph class", backlinkBlock.StyleClasses)
	}
	if !hasPDFStyleClass(backlinkBlock.ContextClasses, pdfStyleFootnote) {
		t.Fatalf("backlink block context = %q, want footnote context", backlinkBlock.ContextClasses)
	}
}

func TestPDFDefaultModeFootnoteBacklinksFromFootnoteReferences(t *testing.T) {
	c := &content.Content{
		OutputFormat:     common.OutputFmtPdf,
		FootnotesMode:    common.FootnotesModeDefault,
		BacklinkTemplate: "↩",
		FootnotesIndex: fb2.FootnoteRefs{
			"n1": {BodyIdx: 1, SectionIdx: 0},
			"n2": {BodyIdx: 1, SectionIdx: 1},
		},
		Book: &fb2.FictionBook{Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{Content: []fb2.FlowItem{{
				Kind: fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
					{Text: "Body "},
					{Kind: fb2.InlineLink, Href: "#n2", Children: []fb2.InlineSegment{{Text: "2"}}},
				}},
			}}}},
		}, {
			Kind: fb2.BodyFootnotes,
			Sections: []fb2.Section{{
				ID: "n1",
				Content: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "First footnote."}}},
				}},
			}, {
				ID: "n2",
				Content: []fb2.FlowItem{{
					Kind: fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
						{Text: "Second footnote refers to "},
						{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
					}},
				}},
			}},
		}}},
	}

	blocks, err := collectTextBlocks(c)
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	if refs := c.BackLinkIndex["n1"]; len(refs) != 1 || refs[0].RefID != "ref-n1-1" {
		t.Fatalf("BackLinkIndex[n1] = %#v, want ref-n1-1 from nested footnote reference", refs)
	}
	if refs := c.BackLinkIndex["n2"]; len(refs) != 1 || refs[0].RefID != "ref-n2-1" {
		t.Fatalf("BackLinkIndex[n2] = %#v, want ref-n2-1 from main text reference", refs)
	}

	var nestedReferenceRun *pdfInlineRun
	backlinks := map[string]bool{}
	for i := range blocks {
		block := &blocks[i]
		for j := range block.Runs {
			run := &block.Runs[j]
			if run.StyleClasses == pdfStyleLinkFootnote && run.LinkHref == "#n1" {
				nestedReferenceRun = run
			}
			if run.StyleClasses == pdfStyleLinkBacklink {
				backlinks[run.LinkHref] = true
			}
		}
	}
	if nestedReferenceRun == nil || nestedReferenceRun.AnchorID != "ref-n1-1" {
		t.Fatalf("nested footnote reference run = %#v, want backlink anchor ref-n1-1", nestedReferenceRun)
	}
	if !backlinks["#ref-n1-1"] {
		t.Fatalf("blocks = %#v, want backlink paragraph to nested footnote reference #ref-n1-1", blocks)
	}
	if !backlinks["#ref-n2-1"] {
		t.Fatalf("blocks = %#v, want backlink paragraph to main text reference #ref-n2-1", blocks)
	}
}

func TestPDFDefaultModeFootnoteBacklinksAfterTableKeepsTableMargin(t *testing.T) {
	c := &content.Content{
		OutputFormat:     common.OutputFmtPdf,
		FootnotesMode:    common.FootnotesModeDefault,
		BacklinkTemplate: "↩",
		FootnotesIndex: fb2.FootnoteRefs{
			"n1": {BodyIdx: 1, SectionIdx: 0},
		},
		Book: &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: "table { margin: 1em auto; } p { margin: 0; }"}}, Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{Content: []fb2.FlowItem{{
				Kind: fb2.FlowTable,
				Table: &fb2.Table{Rows: []fb2.TableRow{{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{
					{Text: "Cell "},
					{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
				}}}}}},
			}}}},
		}, {
			Kind: fb2.BodyFootnotes,
			Sections: []fb2.Section{{
				ID: "n1",
				Content: []fb2.FlowItem{{
					Kind: fb2.FlowTable,
					Table: &fb2.Table{Rows: []fb2.TableRow{{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{
						{Text: "Footnote table"},
					}}}}}},
				}},
			}},
		}}},
	}

	blocks, err := collectTextBlocks(c)
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	resolver := newPDFStyleResolver(c.Book, nil, nil)
	styles := resolver.collapsedBlockStyles(blocks)
	for i := 1; i < len(blocks); i++ {
		if blocks[i-1].Kind == pdfBlockTable && blocks[i].Kind == pdfBlockParagraph {
			if styles[i-1].SpaceAfter <= 0 {
				t.Fatalf("table before backlink SpaceAfter = %v, want positive table margin; block=%#v style=%#v", styles[i-1].SpaceAfter, blocks[i-1], styles[i-1])
			}
			return
		}
	}
	t.Fatalf("blocks = %#v, want table followed by backlink paragraph", blocks)
}

func TestPDFDefaultModeFootnoteBacklinksFromTableCells(t *testing.T) {
	c := &content.Content{
		OutputFormat:     common.OutputFmtPdf,
		FootnotesMode:    common.FootnotesModeDefault,
		BacklinkTemplate: "↩",
		FootnotesIndex: fb2.FootnoteRefs{
			"n1": {BodyIdx: 1, SectionIdx: 0},
		},
		Book: &fb2.FictionBook{Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{Content: []fb2.FlowItem{{
				Kind: fb2.FlowTable,
				Table: &fb2.Table{Rows: []fb2.TableRow{{Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{
					{Text: "Cell "},
					{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
				}}}}}},
			}}}},
		}, {
			Kind: fb2.BodyFootnotes,
			Sections: []fb2.Section{{
				ID:    "n1",
				Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "1"}}}}}},
				Content: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Footnote."}}},
				}},
			}},
		}}},
	}

	blocks, err := collectTextBlocks(c)
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	if refs := c.BackLinkIndex["n1"]; len(refs) != 1 || refs[0].RefID != "ref-n1-1" {
		t.Fatalf("BackLinkIndex[n1] = %#v, want ref-n1-1", refs)
	}

	var tableBlock *pdfTextBlock
	var hasBacklink bool
	for i := range blocks {
		block := &blocks[i]
		if block.Kind == pdfBlockTable {
			tableBlock = block
		}
		for _, run := range block.Runs {
			if run.StyleClasses == pdfStyleLinkBacklink && run.LinkHref == "#ref-n1-1" {
				hasBacklink = true
			}
		}
	}
	if tableBlock == nil {
		t.Fatalf("blocks = %#v, want table block", blocks)
	}
	cellRuns := tableBlock.TableCellRuns[pdfTableCellKey{0, 0}]
	if len(cellRuns) != 2 || cellRuns[1].AnchorID != "ref-n1-1" {
		t.Fatalf("table cell runs = %#v, want backlink anchor ref-n1-1", cellRuns)
	}
	if !hasBacklink {
		t.Fatalf("blocks = %#v, want backlink paragraph to #ref-n1-1", blocks)
	}
}

func TestParagraphInlineRunsClassifyFootnotesByContent(t *testing.T) {
	c := testContentWithFootnotes("n1")
	tests := []struct {
		name      string
		segments  []fb2.InlineSegment
		wantText  string
		wantClass string
		wantSup   bool
		wantCode  bool
	}{
		{
			name:      "ordinary direct note ref",
			segments:  []fb2.InlineSegment{{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}}},
			wantText:  "1",
			wantClass: pdfStyleLinkFootnote,
		},
		{
			name: "ordinary sup note ref",
			segments: []fb2.InlineSegment{{Kind: fb2.InlineSup, Children: []fb2.InlineSegment{
				{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
			}}},
			wantText:  "1",
			wantClass: pdfStyleLinkFootnote,
			wantSup:   true,
		},
		{
			name: "code note ref",
			segments: []fb2.InlineSegment{{Kind: fb2.InlineCode, Children: []fb2.InlineSegment{
				{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
			}}},
			wantText:  "1",
			wantClass: joinStyleClasses(pdfStyleCode, pdfStyleLinkFootnote),
			wantCode:  true,
		},
		{
			name:      "note link type false positive",
			segments:  []fb2.InlineSegment{{Kind: fb2.InlineLink, Href: "#ordinary", LinkType: "note", Children: []fb2.InlineSegment{{Text: "ordinary"}}}},
			wantText:  "ordinary",
			wantClass: pdfStyleLinkInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := paragraphInlineRuns(&fb2.Paragraph{Text: tt.segments}, c)
			if len(runs) != 1 {
				t.Fatalf("inline runs = %#v, want one run", runs)
			}
			run := runs[0]
			if run.Text != tt.wantText || run.StyleClasses != tt.wantClass || run.LinkHref == "" || run.Superscript != tt.wantSup || run.Code != tt.wantCode {
				t.Fatalf("run = %#v, want text %q classes %q href superscript:%t code:%t", run, tt.wantText, tt.wantClass, tt.wantSup, tt.wantCode)
			}
		})
	}
}

func TestTitleBlocksClassifyFootnotesByContent(t *testing.T) {
	c := testContentWithFootnotes("n1")
	for _, tt := range []struct {
		name     string
		depth    int
		segments []fb2.InlineSegment
		wantSup  bool
	}{
		{
			name:     "h1 direct note ref",
			depth:    1,
			segments: []fb2.InlineSegment{{Text: "Heading"}, {Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}}},
		},
		{
			name:  "h1 sup note ref",
			depth: 1,
			segments: []fb2.InlineSegment{{Text: "Heading"}, {Kind: fb2.InlineSup, Children: []fb2.InlineSegment{
				{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
			}}},
			wantSup: true,
		},
		{
			name:     "h2 direct note ref",
			depth:    2,
			segments: []fb2.InlineSegment{{Text: "Heading"}, {Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var blocks []pdfTextBlock
			appendTitleBlocksFull(&blocks, c, &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: tt.segments}}}}, tt.depth, "", pdfHeadingStyleName(tt.depth), "", "", false)
			if len(blocks) != 1 || len(blocks[0].Runs) != 2 {
				t.Fatalf("title blocks = %#v, want one heading with two runs", blocks)
			}
			run := blocks[0].Runs[1]
			if run.Text != "1" || run.StyleClasses != pdfStyleLinkFootnote || run.LinkHref != "#n1" || run.Superscript != tt.wantSup {
				t.Fatalf("title note run = %#v, want footnote link superscript:%t", run, tt.wantSup)
			}
		})
	}
}

func TestFootnoteBodyBlocksClassifyNoteRefsByContent(t *testing.T) {
	c := testContentWithFootnotes("n1")
	body := &fb2.Body{Sections: []fb2.Section{{Content: []fb2.FlowItem{{
		Kind: fb2.FlowParagraph,
		Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
			{Text: "See "},
			{Kind: fb2.InlineLink, Href: "#n1", Children: []fb2.InlineSegment{{Text: "1"}}},
		}},
	}}}}}

	var blocks []pdfTextBlock
	appendFootnoteBodyBlocks(&blocks, c, body, nil)
	if len(blocks) != 1 {
		t.Fatalf("footnote body blocks = %#v, want one paragraph", blocks)
	}
	block := blocks[0]
	if block.StyleClasses != pdfStyleFootnote || block.ContextClasses != pdfStyleFootnote {
		t.Fatalf("footnote block classes = %q / %q, want footnote context", block.StyleClasses, block.ContextClasses)
	}
	if len(block.Runs) != 2 || block.Runs[1].StyleClasses != pdfStyleLinkFootnote || block.Runs[1].LinkHref != "#n1" {
		t.Fatalf("footnote body runs = %#v, want note ref as footnote link", block.Runs)
	}
}

func TestBuildTOCPageBlocksClassifiesEntryLinks(t *testing.T) {
	blocks := buildTOCPageBlocks([]*structure.TOCEntry{{ID: "chapter", Title: "Chapter", IncludeInTOC: true}}, true, common.TOCTypeFlat)
	if len(blocks) != 3 {
		t.Fatalf("toc blocks = %#v, want title and one entry", blocks)
	}
	entry := blocks[2]
	if len(entry.Runs) != 1 || entry.Runs[0].Text != "Chapter" || entry.Runs[0].StyleClasses != pdfStyleLinkTOC {
		t.Fatalf("toc entry runs = %#v, want TOC link class", entry.Runs)
	}
}

func TestParagraphInlineRunsPreserveInlineImages(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "before "},
		{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#inline.png", Alt: "ALT"}},
		{Text: " after"},
	}}

	text, links := paragraphTextAndLinks(paragraph)
	if text != "before after" || len(links) != 0 {
		t.Fatalf("paragraph text/links = %q %#v, want text without image alt and no links", text, links)
	}
	runs := paragraphInlineRuns(paragraph, nil)
	if len(runs) != 3 {
		t.Fatalf("inline runs = %#v, want text/image/text", runs)
	}
	if runs[1].ImageID != "inline.png" || runs[1].Text != "" {
		t.Fatalf("image run = %#v, want inline image without alt text", runs[1])
	}
}

func TestParagraphInlineRunsPreserveLinkedInlineImages(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "before "},
		{Kind: fb2.InlineLink, Href: "#target", Children: []fb2.InlineSegment{{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#inline.png"}}}},
		{Text: " after"},
	}}

	runs := paragraphInlineRuns(paragraph, nil)
	if len(runs) != 3 {
		t.Fatalf("inline runs = %#v, want text/image/text", runs)
	}
	if runs[1].ImageID != "inline.png" || runs[1].LinkHref != "#target" || runs[1].StyleClasses != pdfStyleLinkInternal {
		t.Fatalf("linked image run = %#v, want image carrying internal link style", runs[1])
	}
}

func TestParagraphInlineRunsPreserveImageOnlyParagraphs(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#heading.png"}}}}

	runs := paragraphInlineRuns(paragraph, nil)
	if len(runs) != 1 || runs[0].ImageID != "heading.png" {
		t.Fatalf("inline runs = %#v, want image-only run", runs)
	}
}

func TestParagraphInlineRunsTrimScriptWhitespace(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "word"},
		{Kind: fb2.InlineSup, Text: "\n  1.1\n  "},
		{Text: " next"},
	}}

	runs := paragraphInlineRuns(paragraph, nil)
	if len(runs) != 3 {
		t.Fatalf("inline runs = %#v, want 3 runs", runs)
	}
	if runs[1].Text != "1.1" || !runs[1].Superscript {
		t.Fatalf("superscript run = %#v, want trimmed superscript label", runs[1])
	}
}

func TestParagraphInlineRunsPreserveCodeBlockWhitespace(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineCode, Text: "\n  alpha\n    beta\n  "}}}

	runs := paragraphInlineRuns(paragraph, nil)
	if len(runs) != 1 {
		t.Fatalf("inline runs = %#v, want one code run", runs)
	}
	if runs[0].Text != "\n  alpha\n    beta\n  " || !runs[0].Code || runs[0].StyleClasses != pdfStyleCode {
		t.Fatalf("code run = %#v, want raw preformatted whitespace preserved", runs[0])
	}
}

func TestParagraphInlineRunsPreserveFB2InlineStyles(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "plain "},
		{Kind: fb2.InlineStrong, Children: []fb2.InlineSegment{{Kind: fb2.InlineEmphasis, Text: "bold italic"}}},
		{Text: " "},
		{Kind: fb2.InlineStrikethrough, Children: []fb2.InlineSegment{{Text: "strike"}}},
		{Text: " "},
		{Kind: fb2.InlineSub, Children: []fb2.InlineSegment{{Text: "sub"}}},
		{Text: " "},
		{Kind: fb2.InlineSup, Children: []fb2.InlineSegment{{Text: "sup"}}},
		{Text: " "},
		{Kind: fb2.InlineCode, Children: []fb2.InlineSegment{{Text: "code"}}},
		{Text: " "},
		{Kind: fb2.InlineNamedStyle, Style: "accent", Children: []fb2.InlineSegment{{Text: "styled"}}},
	}}

	runs := paragraphInlineRuns(paragraph, nil)
	if len(runs) != 12 {
		t.Fatalf("inline runs = %#v, want 12 style-preserving runs", runs)
	}
	assertRun := func(index int, text string, check func(pdfInlineRun) bool) {
		t.Helper()
		if runs[index].Text != text || !check(runs[index]) {
			t.Fatalf("run[%d] = %#v", index, runs[index])
		}
	}
	assertRun(1, "bold italic", func(run pdfInlineRun) bool { return run.Bold && run.Italic })
	assertRun(3, "strike", func(run pdfInlineRun) bool { return run.Strikethrough })
	assertRun(5, "sub", func(run pdfInlineRun) bool { return run.Subscript && !run.Superscript })
	assertRun(7, "sup", func(run pdfInlineRun) bool { return run.Superscript && !run.Subscript })
	assertRun(9, "code", func(run pdfInlineRun) bool { return run.Code && run.StyleClasses == pdfStyleCode })
	assertRun(11, "styled", func(run pdfInlineRun) bool { return run.StyleClasses == "accent" })
}
