package pdf

import (
	"testing"

	"fbc/common"
	"fbc/content"
	"fbc/convert/structure"
	"fbc/fb2"
)

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
	appendTitleBlocksWithID(&blocks, &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "Heading"},
		{Kind: fb2.InlineLink, Href: "#note", LinkType: "note", Children: []fb2.InlineSegment{{Text: "1.1"}}},
	}}}}}, 1, "heading-id")

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
		{Kind: fb2.InlineLink, Href: "#note", LinkType: "note", Children: []fb2.InlineSegment{{Text: "1.1"}}},
	}}

	runs := paragraphInlineRuns(paragraph)
	if len(runs) != 4 {
		t.Fatalf("inline runs = %#v, want 4 runs", runs)
	}
	if runs[1].Text != "external" || runs[1].StyleClasses != pdfStyleLinkExternal {
		t.Fatalf("external link run = %#v, want external link class", runs[1])
	}
	if runs[3].Text != "1.1" || runs[3].StyleClasses != pdfStyleLinkFootnote {
		t.Fatalf("note link run = %#v, want footnote link class", runs[3])
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
	runs := paragraphInlineRuns(paragraph)
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

	runs := paragraphInlineRuns(paragraph)
	if len(runs) != 3 {
		t.Fatalf("inline runs = %#v, want text/image/text", runs)
	}
	if runs[1].ImageID != "inline.png" || runs[1].LinkHref != "#target" || runs[1].StyleClasses != pdfStyleLinkInternal {
		t.Fatalf("linked image run = %#v, want image carrying internal link style", runs[1])
	}
}

func TestParagraphInlineRunsPreserveImageOnlyParagraphs(t *testing.T) {
	paragraph := &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#heading.png"}}}}

	runs := paragraphInlineRuns(paragraph)
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

	runs := paragraphInlineRuns(paragraph)
	if len(runs) != 3 {
		t.Fatalf("inline runs = %#v, want 3 runs", runs)
	}
	if runs[1].Text != "1.1" || !runs[1].Superscript {
		t.Fatalf("superscript run = %#v, want trimmed superscript label", runs[1])
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

	runs := paragraphInlineRuns(paragraph)
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
