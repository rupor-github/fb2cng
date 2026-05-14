package pdf

import (
	"testing"

	"fbc/fb2"
)

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

func TestAppendImageBlockAddsImageBlockClass(t *testing.T) {
	var blocks []pdfTextBlock
	appendImageBlock(&blocks, &fb2.Image{Href: "#block.png", Alt: "Block"}, "anchor")
	if len(blocks) != 1 || blocks[0].Kind != pdfBlockImage {
		t.Fatalf("blocks = %#v, want one image block", blocks)
	}
	if blocks[0].StyleClasses != "image-block" {
		t.Fatalf("image block classes = %q, want %q", blocks[0].StyleClasses, "image-block")
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

func TestAppendEpigraphBlocksPropagatesEpigraphContextClasses(t *testing.T) {
	epigraph := &fb2.Epigraph{
		Flow: fb2.Flow{Items: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Quote"}}},
		}}},
		TextAuthors: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Author"}}}},
	}
	var blocks []pdfTextBlock

	appendEpigraphBlocks(&blocks, epigraph)

	if len(blocks) != 2 {
		t.Fatalf("blocks = %#v, want epigraph paragraph and text-author", blocks)
	}
	for i, block := range blocks {
		if block.ContextClasses != pdfStyleEpigraph {
			t.Fatalf("block %d context = %q, want %q", i, block.ContextClasses, pdfStyleEpigraph)
		}
	}
}

func TestAppendPoemBlocksPropagatesStanzaContextClasses(t *testing.T) {
	poem := &fb2.Poem{
		Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Poem title"}}}}}},
		Stanzas: []fb2.Stanza{{
			Title:    &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Stanza title"}}}}}},
			Subtitle: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Stanza subtitle"}}},
			Verses:   []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Verse line"}}}},
		}},
	}
	var blocks []pdfTextBlock

	appendPoemBlocks(&blocks, poem, 1, nil, "", false)

	poemSeen := false
	wantStanzaContext := joinStyleClasses(pdfStylePoem, pdfStyleStanza)
	for _, block := range blocks {
		switch block.Text {
		case "Poem title":
			poemSeen = true
			if block.Kind != pdfBlockParagraph {
				t.Fatalf("poem title kind = %v, want paragraph", block.Kind)
			}
			if block.StyleClasses != pdfStylePoemTitle+" "+pdfStylePoemTitle+"-first" {
				t.Fatalf("poem title classes = %q, want poem-title first variant", block.StyleClasses)
			}
			if block.ContextClasses != pdfStylePoem {
				t.Fatalf("poem title context = %q, want %q", block.ContextClasses, pdfStylePoem)
			}
		case "Stanza title", "Stanza subtitle", "Verse line":
			if block.ContextClasses != wantStanzaContext {
				t.Fatalf("block %q context = %q, want %q", block.Text, block.ContextClasses, wantStanzaContext)
			}
			if block.Text == "Stanza title" {
				if block.Kind != pdfBlockParagraph {
					t.Fatalf("stanza title kind = %v, want paragraph", block.Kind)
				}
				if block.StyleClasses != pdfStyleStanzaTitle+" "+pdfStyleStanzaTitle+"-first" {
					t.Fatalf("stanza title classes = %q, want stanza-title first variant", block.StyleClasses)
				}
			}
		}
	}
	if !poemSeen {
		t.Fatalf("poem title not found in %#v", blocks)
	}
	if len(blocks) == 0 || blocks[len(blocks)-1].Kind != pdfBlockEmptyLine || blocks[len(blocks)-1].ContextClasses != wantStanzaContext {
		t.Fatalf("stanza empty line = %#v, want stanza context %q", blocks, wantStanzaContext)
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
