package pdf

import (
	"strings"
	"testing"
	"time"

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
	appendImageBlockWithOptions(&blocks, pdfImageBlockOptions{Image: &fb2.Image{Href: "#block.png", Alt: "Block"}, FallbackID: "anchor"})
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

	appendPoemBlocks(&blocks, nil, poem, 1, nil, "", false)

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
			if !blockHasStyleClass(block, pdfStyleStanza) {
				t.Fatalf("block %q classes = %q, want stanza wrapper class", block.Text, block.StyleClasses)
			}
			if block.Text == "Stanza title" {
				if block.Kind != pdfBlockParagraph {
					t.Fatalf("stanza title kind = %v, want paragraph", block.Kind)
				}
				if block.StyleClasses != pdfStyleStanzaTitle+" "+pdfStyleStanzaTitle+"-first "+pdfStyleStanza {
					t.Fatalf("stanza title classes = %q, want stanza-title first variant plus stanza wrapper", block.StyleClasses)
				}
			}
		}
	}
	if !poemSeen {
		t.Fatalf("poem title not found in %#v", blocks)
	}
	for _, block := range blocks {
		if block.Kind == pdfBlockEmptyLine {
			t.Fatalf("blocks = %#v, want no trailing empty line after final stanza", blocks)
		}
	}
}

func TestAppendPoemBlocksAddsEmptyLineOnlyBetweenStanzas(t *testing.T) {
	poem := &fb2.Poem{Stanzas: []fb2.Stanza{
		{Verses: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "First stanza"}}}}},
		{Verses: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Text: "Second stanza"}}}}},
	}}
	var blocks []pdfTextBlock

	appendPoemBlocks(&blocks, nil, poem, 1, nil, "", false)

	emptyLines := 0
	for _, block := range blocks {
		if block.Kind == pdfBlockEmptyLine {
			emptyLines++
		}
	}
	if emptyLines != 1 {
		t.Fatalf("blocks = %#v, want one empty line between two stanzas", blocks)
	}
}

func TestAppendPoemBlocksEmitsDateWithPoemContext(t *testing.T) {
	poem := &fb2.Poem{Date: &fb2.Date{Display: "December 2025"}}
	var blocks []pdfTextBlock

	appendPoemBlocks(&blocks, nil, poem, 1, nil, "", false)

	if len(blocks) != 1 {
		t.Fatalf("blocks = %#v, want one poem date block", blocks)
	}
	if got := blocks[0]; got.Kind != pdfBlockParagraph || got.Text != "December 2025" || got.StyleName != pdfStyleParagraph || got.StyleClasses != pdfStyleDate || got.ContextClasses != pdfStylePoem {
		t.Fatalf("poem date block = %#v, want date paragraph in poem context", got)
	}
}

func TestAppendPoemBlocksEmitsValueOnlyDate(t *testing.T) {
	poem := &fb2.Poem{Date: &fb2.Date{Value: time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)}}
	var blocks []pdfTextBlock

	appendPoemBlocks(&blocks, nil, poem, 1, nil, "", false)

	if len(blocks) != 1 || blocks[0].Text != "2025-12-31" || blocks[0].StyleClasses != pdfStyleDate {
		t.Fatalf("poem value date block = %#v, want ISO date paragraph", blocks)
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

func appendTitleBlocks(blocks *[]pdfTextBlock, title *fb2.Title, depth int) {
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{Title: title, Depth: depth, HeaderStyleName: pdfHeadingStyleName(depth)})
}

func appendTitleBlocksWithIDAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, styleClasses string) {
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{
		Title:           title,
		Depth:           depth,
		ID:              id,
		HeaderStyleName: pdfHeadingStyleName(depth),
		StyleClasses:    styleClasses,
		ContextClasses:  strings.TrimSpace(styleClasses),
	})
}

func appendTitleBlocksWithIDHeaderAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string) {
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{
		Title:           title,
		Depth:           depth,
		ID:              id,
		HeaderStyleName: headerStyleName,
		StyleClasses:    styleClasses,
		ContextClasses:  strings.TrimSpace(styleClasses),
	})
}

func appendParagraphBlockWithClasses(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string) {
	appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Kind: kind, Paragraph: paragraph, Depth: depth, StyleClasses: styleClasses, ContextClasses: strings.TrimSpace(styleClasses)})
}

func appendEpigraphBlocks(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph) {
	appendEpigraphBlocksFull(blocks, nil, epigraph, "", false)
}
