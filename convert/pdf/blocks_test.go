package pdf

import (
	"strings"
	"testing"

	"fbc/common"
	"fbc/config"
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

func TestAppendParagraphBlockWithClassesConvertsImageOnlyParagraphsToContextStyledImageBlocks(t *testing.T) {
	paragraph := &fb2.Paragraph{ID: "p", Text: []fb2.InlineSegment{{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#block.png", Alt: "Block"}}}}
	var blocks []pdfTextBlock

	appendParagraphBlockWithClasses(&blocks, pdfBlockParagraph, paragraph, 1, pdfStyleCite)

	if len(blocks) != 1 || blocks[0].Kind != pdfBlockImage || blocks[0].ImageID != "block.png" || blocks[0].ID != "p" || blocks[0].Text != "Block" {
		t.Fatalf("blocks = %#v, want image-only paragraph block", blocks)
	}
	if blocks[0].StyleName != pdfStyleParagraph || blocks[0].StyleClasses != pdfStyleCite {
		t.Fatalf("image-only paragraph image style = %q / %q, want paragraph style name with cite context", blocks[0].StyleName, blocks[0].StyleClasses)
	}
}

func TestAppendParagraphBlockWithClassesUsesContextSpecificSubtitleStyleName(t *testing.T) {
	paragraph := &fb2.Paragraph{ID: "s", Text: []fb2.InlineSegment{{Text: "Quote subtitle"}}}
	var blocks []pdfTextBlock

	appendParagraphBlockWithClasses(&blocks, pdfBlockSubtitle, paragraph, 1, pdfStyleCiteSubtitle)

	if len(blocks) != 1 || blocks[0].Kind != pdfBlockSubtitle || blocks[0].StyleName != pdfStyleCiteSubtitle || blocks[0].StyleClasses != pdfStyleCiteSubtitle {
		t.Fatalf("blocks = %#v, want cite-subtitle style name without section-subtitle inheritance", blocks)
	}
}

func TestAppendParagraphBlockWithClassesConvertsImageOnlySubtitlesToImageBlocks(t *testing.T) {
	paragraph := &fb2.Paragraph{ID: "h", Text: []fb2.InlineSegment{{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#heading.png", Alt: "Heading"}}}}
	var blocks []pdfTextBlock

	appendParagraphBlockWithClasses(&blocks, pdfBlockSubtitle, paragraph, 1, "custom")

	if len(blocks) != 1 || blocks[0].Kind != pdfBlockImage || blocks[0].ImageID != "heading.png" || blocks[0].ID != "h" || blocks[0].Text != "Heading" || blocks[0].StyleClasses != pdfStyleSubtitle+" custom" {
		t.Fatalf("blocks = %#v, want image-only subtitle block without force-upscale class", blocks)
	}
}

func TestAppendTitleBlocksWithIDAndClassesConvertsImageOnlyTitlesToStyledImageBlocks(t *testing.T) {
	title := &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#title.png", Alt: "Title"}}}}}}}
	var blocks []pdfTextBlock

	appendTitleBlocksWithIDAndClasses(&blocks, title, 1, "title-id", pdfStyleChapterTitle)

	if len(blocks) != 1 || blocks[0].Kind != pdfBlockImage || blocks[0].ImageID != "title.png" || blocks[0].ID != "title-id" || blocks[0].Text != "Title" || blocks[0].StyleClasses != pdfStyleChapterTitleHeader+" "+pdfStyleChapterTitle+" "+pdfStyleChapterTitleHeader+"-first "+pdfStyleHeadingImage {
		t.Fatalf("blocks = %#v, want styled image-only title block", blocks)
	}
	if !blocks[0].StripRootHorizontalMargins {
		t.Fatalf("image-only title block should strip root horizontal margins")
	}
}

func TestAppendTitleBlocksWithInlineImagesStripRootHorizontalMargins(t *testing.T) {
	title := &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
		{Text: "Before "},
		{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#title.png", Alt: "Title"}},
		{Text: " After"},
	}}}}}
	var blocks []pdfTextBlock

	appendTitleBlocksWithIDHeaderAndClasses(&blocks, title, 1, "title-id", pdfStyleChapterTitleHeader, pdfStyleChapterTitle)

	if len(blocks) != 1 || blocks[0].Kind != pdfBlockHeading {
		t.Fatalf("blocks = %#v, want one heading block", blocks)
	}
	if !blocks[0].StripRootHorizontalMargins {
		t.Fatalf("title block with inline image should strip root horizontal margins")
	}
}

func TestAppendTitleBlocksWithIDHeaderAndClassesUsesBodyTitleHeaderAndPositionClasses(t *testing.T) {
	title := &fb2.Title{Items: []fb2.TitleItem{
		{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Line 1"}}}},
		{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Line 2"}}}},
	}}
	var blocks []pdfTextBlock

	appendTitleBlocksWithIDHeaderAndClasses(&blocks, title, 1, "body-title", pdfStyleBodyTitleHeader, pdfStyleBodyTitle)

	if len(blocks) != 2 {
		t.Fatalf("blocks = %#v, want 2 title blocks", blocks)
	}
	if blocks[0].StyleName != pdfStyleBodyTitleHeader || blocks[0].StyleClasses != pdfStyleBodyTitle+" "+pdfStyleBodyTitleHeader+"-first" {
		t.Fatalf("first title block = %#v, want body-title-header first", blocks[0])
	}
	if !blocks[0].StripRootHorizontalMargins {
		t.Fatalf("first body-title block should strip root horizontal margins")
	}
	if blocks[1].StyleName != pdfStyleBodyTitleHeader || blocks[1].StyleClasses != pdfStyleBodyTitle+" "+pdfStyleBodyTitleHeader+"-next" {
		t.Fatalf("second title block = %#v, want body-title-header next", blocks[1])
	}
	if !blocks[1].StripRootHorizontalMargins {
		t.Fatalf("second body-title block should strip root horizontal margins")
	}
}

func TestAppendTitleBlocksWithoutWrapperClassesDoNotStripRootHorizontalMargins(t *testing.T) {
	title := &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Line 1"}}}}}}
	var blocks []pdfTextBlock

	appendTitleBlocks(&blocks, title, 1)

	if len(blocks) != 1 || blocks[0].Kind != pdfBlockHeading {
		t.Fatalf("blocks = %#v, want one heading block", blocks)
	}
	if blocks[0].StripRootHorizontalMargins {
		t.Fatalf("plain title block should not strip root horizontal margins")
	}
}

func TestAppendTitleBlocksWithIDHeaderAndClassesMarksTextAfterImage(t *testing.T) {
	title := &fb2.Title{Items: []fb2.TitleItem{
		{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineImageSegment, Image: &fb2.InlineImage{Href: "#title.png", Alt: "Title"}}}}},
		{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Caption"}}}},
	}}
	var blocks []pdfTextBlock

	appendTitleBlocksWithIDHeaderAndClasses(&blocks, title, 1, "title-id", pdfStyleChapterTitleHeader, pdfStyleChapterTitle)

	if len(blocks) != 2 {
		t.Fatalf("blocks = %#v, want image plus following text", blocks)
	}
	if blocks[1].StyleClasses != pdfStyleChapterTitle+" "+pdfStyleChapterTitleHeader+"-next "+pdfStyleTitleAfterImage {
		t.Fatalf("text-after-image classes = %q, want title-after-image marker", blocks[1].StyleClasses)
	}
	if !blocks[1].StripRootHorizontalMargins {
		t.Fatalf("title text after image should strip root horizontal margins as wrapper child")
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

func TestCollectPDFContentAddsAnnotationPage(t *testing.T) {
	book := &fb2.FictionBook{
		Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
				Kind:      fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Book annotation."}}},
			}}},
		}},
		Bodies: []fb2.Body{{
			Kind: fb2.BodyMain,
			Sections: []fb2.Section{{
				ID:    "chapter-1",
				Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			}},
		}},
	}
	plan, err := collectPDFContent(&content.Content{Book: book}, &config.DocumentConfig{
		Annotation: config.AnnotationConfig{Enable: true, Title: "About", InTOC: true},
	})
	if err != nil {
		t.Fatalf("collectPDFContent() error = %v", err)
	}
	if len(plan.Blocks) < 4 {
		t.Fatalf("blocks = %#v, want annotation and chapter blocks", plan.Blocks)
	}
	if got := plan.Blocks[0]; got.Kind != pdfBlockPageBreak || got.ID != "annotation-page" {
		t.Fatalf("first block = %#v, want annotation page break", got)
	}
	if got := plan.Blocks[1]; got.Kind != pdfBlockHeading || got.Text != "About" {
		t.Fatalf("second block = %#v, want annotation heading", got)
	}
	if got := plan.Blocks[2]; got.Kind != pdfBlockParagraph || got.Text != "Book annotation." || got.StyleClasses != pdfStyleAnnotation {
		t.Fatalf("annotation paragraph = %#v", got)
	}
	if plan.Blocks[2].StripRootHorizontalMargins {
		t.Fatalf("generated annotation page paragraph should keep normal root margins")
	}
	if len(plan.TOC) == 0 || plan.TOC[0].ID != "annotation-page" || plan.TOC[0].Title != "About" {
		t.Fatalf("TOC = %#v, want annotation entry first", plan.TOC)
	}
}

func TestCollectTextBlocksMarksMultiBlockSectionAnnotationForRootHorizontalStripping(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "First annotation."}}}},
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Second annotation."}}}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	var annotationBlocks []pdfTextBlock
	for _, block := range blocks {
		if block.Kind == pdfBlockParagraph && block.StyleClasses == pdfStyleAnnotation {
			annotationBlocks = append(annotationBlocks, block)
		}
	}
	if len(annotationBlocks) != 2 {
		t.Fatalf("annotation blocks = %#v, want 2 section annotation paragraphs", annotationBlocks)
	}
	for i, block := range annotationBlocks {
		if !block.StripRootHorizontalMargins {
			t.Fatalf("annotation block %d should strip root horizontal margins: %#v", i, block)
		}
	}
}

func TestCollectTextBlocksKeepsSingleBlockSectionAnnotationOnNormalRootMargins(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
				Kind:      fb2.FlowParagraph,
				Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Only annotation."}}},
			}}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	for _, block := range blocks {
		if block.Kind == pdfBlockParagraph && block.Text == "Only annotation." {
			if block.StripRootHorizontalMargins {
				t.Fatalf("single-block section annotation should keep normal root margins: %#v", block)
			}
			return
		}
	}
	t.Fatalf("single-block annotation paragraph not found in %#v", blocks)
}

func TestCollectTextBlocksPropagatesWrappedAnnotationRootHorizontalStrippingIntoCite(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Lead annotation."}}}},
				{Kind: fb2.FlowCite, Cite: &fb2.Cite{Items: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested cite."}}},
				}}}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	for _, block := range blocks {
		if block.Kind == pdfBlockParagraph && block.Text == "Nested cite." {
			if block.StyleClasses != pdfStyleCite {
				t.Fatalf("nested cite block classes = %q, want %q", block.StyleClasses, pdfStyleCite)
			}
			if !block.StripRootHorizontalMargins {
				t.Fatalf("nested cite block should inherit wrapped-annotation root stripping: %#v", block)
			}
			return
		}
	}
	t.Fatalf("nested cite block not found in %#v", blocks)
}

func TestCollectTextBlocksPropagatesWrappedAnnotationRootHorizontalStrippingIntoPoem(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Lead annotation."}}}},
				{Kind: fb2.FlowPoem, Poem: &fb2.Poem{
					Title:   &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Poem title"}}}}}},
					Stanzas: []fb2.Stanza{{Verses: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Verse line."}}}}}},
				}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	seenTitle := false
	seenVerse := false
	for _, block := range blocks {
		switch {
		case block.Kind == pdfBlockHeading && block.Text == "Poem title":
			seenTitle = true
			if !block.StripRootHorizontalMargins {
				t.Fatalf("poem title should inherit wrapped-annotation root stripping: %#v", block)
			}
		case block.Kind == pdfBlockPoem && block.Text == "Verse line.":
			seenVerse = true
			if block.StyleClasses != pdfStylePoem {
				t.Fatalf("verse block classes = %q, want %q", block.StyleClasses, pdfStylePoem)
			}
			if !block.StripRootHorizontalMargins {
				t.Fatalf("verse block should inherit wrapped-annotation root stripping: %#v", block)
			}
		}
	}
	if !seenTitle || !seenVerse {
		t.Fatalf("expected stripped poem title and verse, got %#v", blocks)
	}
}

func TestCollectTextBlocksPropagatesWrappedAnnotationRootHorizontalStrippingIntoNestedSection(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{
				{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Lead annotation."}}}},
				{Kind: fb2.FlowSection, Section: &fb2.Section{
					Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested section note."}}},
					}}},
					Content: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested section body."}}},
					}},
				}},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	seenBody := false
	seenAnnotation := false
	for _, block := range blocks {
		switch {
		case block.Kind == pdfBlockParagraph && block.Text == "Nested section body.":
			seenBody = true
			if !block.StripRootHorizontalMargins {
				t.Fatalf("nested section body should inherit wrapped-annotation root stripping: %#v", block)
			}
		case block.Kind == pdfBlockParagraph && block.Text == "Nested section note.":
			seenAnnotation = true
			if block.StyleClasses != pdfStyleAnnotation {
				t.Fatalf("nested section annotation classes = %q, want %q", block.StyleClasses, pdfStyleAnnotation)
			}
			if !block.StripRootHorizontalMargins {
				t.Fatalf("nested section annotation should inherit wrapped-annotation root stripping: %#v", block)
			}
		}
	}
	if !seenBody || !seenAnnotation {
		t.Fatalf("expected stripped nested section body and annotation, got %#v", blocks)
	}
}

func TestCollectPDFContentAddsTOCPageBeforeContent(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter-1",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
		}},
	}}}
	plan, err := collectPDFContent(&content.Content{Book: book}, &config.DocumentConfig{
		TOCPage: config.TOCPageConfig{Placement: common.TOCPagePlacementBefore},
	})
	if err != nil {
		t.Fatalf("collectPDFContent() error = %v", err)
	}
	if len(plan.Blocks) < 4 {
		t.Fatalf("blocks = %#v, want TOC and chapter blocks", plan.Blocks)
	}
	if got := plan.Blocks[0]; got.Kind != pdfBlockPageBreak || got.ID != "toc-page" {
		t.Fatalf("first block = %#v, want TOC page break", got)
	}
	if got := plan.Blocks[1]; got.Kind != pdfBlockHeading || got.Text != "Contents" {
		t.Fatalf("second block = %#v, want TOC heading", got)
	}
	if got := plan.Blocks[2]; got.Kind != pdfBlockTOCEntry || got.Text != "Chapter 1" || len(got.Links) != 1 || got.Links[0].Href != "#chapter-1" {
		t.Fatalf("TOC entry block = %#v", got)
	}
}

func TestCollectTextBlocksUsesStructuralPageBreaks(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			ID:    "chapter",
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}}}},
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowSection,
				Section: &fb2.Section{
					ID:    "nested",
					Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested"}}}}}},
					Content: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Nested body."}}},
					}},
				},
			}},
		}},
	}}}
	book.SetSectionPageBreaks(map[int]bool{2: true})

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	var pageBreaks int
	var texts []string
	for _, block := range blocks {
		if block.Kind == pdfBlockPageBreak {
			pageBreaks++
			continue
		}
		texts = append(texts, block.Text)
	}
	if pageBreaks != 2 {
		t.Fatalf("page breaks = %d, want 2", pageBreaks)
	}
	wantTexts := []string{"Chapter", "Nested", "Nested body."}
	if len(texts) != len(wantTexts) {
		t.Fatalf("texts = %#v, want %#v", texts, wantTexts)
	}
	for i := range wantTexts {
		if texts[i] != wantTexts[i] {
			t.Fatalf("texts = %#v, want %#v", texts, wantTexts)
		}
	}
}

func TestCollectTextBlocksPreservesContainerStyleClasses(t *testing.T) {
	book := &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{
			Epigraphs: []fb2.Epigraph{{
				Flow: fb2.Flow{Items: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Epigraph text."}}},
				}, {
					Kind:     fb2.FlowSubtitle,
					Subtitle: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Epigraph subtitle."}}},
				}}},
			}},
			Content: []fb2.FlowItem{{
				Kind: fb2.FlowCite,
				Cite: &fb2.Cite{Items: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Style: "source-class", Text: []fb2.InlineSegment{{Text: "Cite text."}}},
				}, {
					Kind:     fb2.FlowSubtitle,
					Subtitle: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Cite subtitle."}}},
				}}},
			}, {
				Kind: fb2.FlowPoem,
				Poem: &fb2.Poem{
					Subtitles: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Poem subtitle."}}}},
					Stanzas: []fb2.Stanza{{
						Subtitle: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Stanza subtitle."}}},
						Verses:   []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Verse line."}}}},
					}},
				},
			}},
		}},
	}}}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}

	wantClasses := map[string]string{
		"Epigraph text.":     pdfStyleEpigraph,
		"Epigraph subtitle.": pdfStyleEpigraphSubtitle,
		"Cite text.":         "source-class " + pdfStyleCite,
		"Cite subtitle.":     pdfStyleCiteSubtitle,
		"Poem subtitle.":     pdfStylePoemSubtitle,
		"Stanza subtitle.":   pdfStyleStanzaSubtitle,
		"Verse line.":        pdfStylePoem,
	}
	for text, want := range wantClasses {
		found := false
		for _, block := range blocks {
			if block.Text != text {
				continue
			}
			found = true
			if block.StyleClasses != want {
				t.Fatalf("block %q classes = %q, want %q", text, block.StyleClasses, want)
			}
		}
		if !found {
			t.Fatalf("missing block %q in %#v", text, blocks)
		}
	}
}

func textBlocksOnly(blocks []pdfTextBlock) []pdfTextBlock {
	out := make([]pdfTextBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind != pdfBlockPageBreak {
			out = append(out, block)
		}
	}
	return out
}

func TestCollectTextBlocksIncludesVignettes(t *testing.T) {
	book := &fb2.FictionBook{
		VignetteIDs: map[common.VignettePos]string{
			common.VignettePosBookTitleTop:       "book-top",
			common.VignettePosBookTitleBottom:    "book-bottom",
			common.VignettePosChapterTitleTop:    "chapter-top",
			common.VignettePosChapterTitleBottom: "chapter-bottom",
			common.VignettePosChapterEnd:         "chapter-end",
		},
		Bodies: []fb2.Body{{
			Kind:  fb2.BodyMain,
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Book title"}}}}}},
			Sections: []fb2.Section{{
				Title:   &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}}}},
				Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Body."}}}}},
			}},
		}},
	}

	blocks, err := collectTextBlocks(&content.Content{Book: book})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	var got []string
	for _, block := range textBlocksOnly(blocks) {
		if block.Kind == pdfBlockImage && strings.Contains(block.StyleClasses, "vignette") {
			flag := "keep"
			if block.StripRootHorizontalMargins {
				flag = "strip"
			}
			got = append(got, flag+":"+block.ImageID+":"+block.StyleClasses)
		}
	}
	want := []string{
		"strip:book-top:vignette vignette-book-title-top body-title",
		"strip:book-bottom:vignette vignette-book-title-bottom body-title",
		"strip:chapter-top:vignette vignette-chapter-title-top chapter-title",
		"strip:chapter-bottom:vignette vignette-chapter-title-bottom chapter-title",
		"keep:chapter-end:vignette vignette-chapter-end",
	}
	if len(got) != len(want) {
		t.Fatalf("vignette blocks = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("vignette block %d = %q, want %q (all: %#v)", i, got[i], want[i], got)
		}
	}
}

func TestCollectTextBlocksIncludesBlockImages(t *testing.T) {
	blocks, err := collectTextBlocks(&content.Content{
		Book: &fb2.FictionBook{Bodies: []fb2.Body{{
			Kind:  fb2.BodyMain,
			Image: &fb2.Image{Href: "#body-image", Alt: "Body image"},
			Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Body title"}}}}}},
			Sections: []fb2.Section{{Content: []fb2.FlowItem{{
				Kind:  fb2.FlowImage,
				Image: &fb2.Image{Href: "#flow-image", ID: "image-anchor", Alt: "Flow image"},
			}}}},
		}}},
	})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	blocks = textBlocksOnly(blocks)
	imageBlocks := make([]pdfTextBlock, 0, 2)
	for _, block := range blocks {
		if block.Kind == pdfBlockImage {
			imageBlocks = append(imageBlocks, block)
		}
	}
	if len(imageBlocks) != 2 {
		t.Fatalf("image blocks = %d, want 2: %#v", len(imageBlocks), blocks)
	}
	if got := imageBlocks[0]; got.ImageID != "body-image" || got.Text != "Body image" {
		t.Fatalf("body image block = %#v", got)
	}
	if got := imageBlocks[1]; got.ImageID != "flow-image" || got.ID != "image-anchor" {
		t.Fatalf("flow image block = %#v", got)
	}
}
