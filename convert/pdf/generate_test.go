package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/internal/pdfdoc"
	"fbc/fb2"
)

func TestPageSizePoints(t *testing.T) {
	width, height, err := pageSizePoints(config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300})
	if err != nil {
		t.Fatalf("pageSizePoints() error = %v", err)
	}
	if width != 303.36 {
		t.Errorf("width = %v, want 303.36", width)
	}
	if height != 403.2 {
		t.Errorf("height = %v, want 403.2", height)
	}
}

func TestPageSizePoints_DefaultDPI(t *testing.T) {
	width, height, err := pageSizePoints(config.ScreenConfig{Width: 300, Height: 600})
	if err != nil {
		t.Fatalf("pageSizePoints() error = %v", err)
	}
	if width != 72 {
		t.Errorf("width = %v, want 72", width)
	}
	if height != 144 {
		t.Errorf("height = %v, want 144", height)
	}
}

func TestGenerateSkeletonPDF(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{
		Images: config.ImagesConfig{
			Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300},
		},
	}
	c := &content.Content{
		SrcName: "book.fb2",
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Authors: []fb2.Author{
						{FirstName: "First", LastName: "Author"},
						{Nickname: "Second"},
					},
				},
			},
		},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(outputName)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-1.4\n")) {
		t.Fatalf("PDF header = %q", data[:min(len(data), 16)])
	}
	pdfText := string(data)
	for _, want := range []string{
		"/Type /Catalog",
		"/Type /Pages",
		"/Type /Page",
		"/MediaBox [0 0 303.36 403.2]",
		"/Filter /FlateDecode",
		"/Subtype /Type0",
		"/Subtype /CIDFontType2",
		"/Encoding /Identity-H",
		"/CIDToGIDMap /Identity",
		"/FontFile2 9 0 R",
		"/ToUnicode 10 0 R",
		"/Root 1 0 R",
		"/Info 5 0 R",
		"xref\n0 11",
		"%%EOF",
	} {
		if !strings.Contains(pdfText, want) {
			t.Errorf("generated PDF does not contain %q", want)
		}
	}
	if !strings.Contains(pdfText, pdfdoc.Format(pdfdoc.UTF16TextString("Test Book"))) {
		t.Errorf("generated PDF does not contain UTF-16BE title metadata")
	}
	if !strings.Contains(pdfText, pdfdoc.Format(pdfdoc.UTF16TextString("First Author, Second"))) {
		t.Errorf("generated PDF does not contain UTF-16BE author metadata")
	}
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

func textBlocksOnly(blocks []pdfTextBlock) []pdfTextBlock {
	out := make([]pdfTextBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind != pdfBlockPageBreak {
			out = append(out, block)
		}
	}
	return out
}

func TestLayoutPDFPagesKeepsHeadingWithNextParagraph(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	contentWidth := 220.0 - 48.0
	filler := textWithParagraphLineCount(t, face, pdfStyleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph, contentWidth, 2, "filler")

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 110,
		Title:      "Title",
		Author:     "Author",
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: filler},
			{Kind: pdfBlockHeading, Text: "Heading", Depth: 1},
			{Kind: pdfBlockParagraph, Text: "Body text after heading."},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 3", len(pages))
	}
	if got := pageText(pages[1]); strings.Contains(got, "Heading") {
		t.Fatalf("heading stranded on previous page: %q", got)
	}
	if got := pageText(pages[2]); !strings.Contains(got, "Heading") || !strings.Contains(got, "Body text") {
		t.Fatalf("heading page text = %q, want heading with following paragraph", got)
	}
}

func TestLayoutPDFPagesAvoidsParagraphWidowOrphanSplit(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	contentWidth := 220.0 - 48.0
	style := pdfStyleForBlock(pdfTextBlock{Kind: pdfBlockParagraph}).Paragraph
	filler := textWithParagraphLineCount(t, face, style, contentWidth, 2, "filler")
	target := textWithParagraphLineCount(t, face, style, contentWidth, 3, "target")

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 98,
		Title:      "Title",
		Author:     "Author",
		Blocks: []pdfTextBlock{
			{Kind: pdfBlockParagraph, Text: filler},
			{Kind: pdfBlockParagraph, Text: target},
		},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 3 {
		t.Fatalf("layoutPDFPages() produced %d pages, want 3", len(pages))
	}
	targetFirstWord := strings.Fields(target)[0]
	if got := pageText(pages[1]); strings.Contains(got, targetFirstWord) {
		t.Fatalf("paragraph orphan left on previous page: %q", got)
	}
	if got := pageText(pages[2]); !strings.Contains(got, targetFirstWord) {
		t.Fatalf("paragraph did not move to next page: %q", got)
	}
}

func textWithParagraphLineCount(t *testing.T, face *builtinFontFace, style paragraphStyle, width float64, wantLines int, word string) string {
	t.Helper()
	for words := 1; words <= 80; words++ {
		parts := make([]string, words)
		for i := range parts {
			parts[i] = word
		}
		text := strings.Join(parts, " ")
		lines, err := layoutParagraph(face, text, style, width)
		if err != nil {
			t.Fatalf("layoutParagraph() error = %v", err)
		}
		if len(lines) == wantLines {
			return text
		}
	}
	t.Fatalf("could not build paragraph with %d lines", wantLines)
	return ""
}

func pageText(page pdfPage) string {
	parts := make([]string, 0, len(page.Lines))
	for _, line := range page.Lines {
		parts = append(parts, shapedRunes(line.Text))
	}
	return strings.Join(parts, "\n")
}

func TestGenerateDebugDumps(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{
		Images: config.ImagesConfig{
			Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300},
		},
	}
	c := &content.Content{
		SrcName: "book.fb2",
		Debug:   true,
		WorkDir: tmpDir,
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Debug Book"}},
			},
			Bodies: []fb2.Body{{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{
					Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}}}},
					Content: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Debug body text."}}},
					}},
				}},
			}},
		},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	blockData, err := os.ReadFile(filepath.Join(tmpDir, "pdf-text-blocks.json"))
	if err != nil {
		t.Fatalf("read pdf-text-blocks.json: %v", err)
	}
	if !bytes.Contains(blockData, []byte(`"Chapter"`)) || !bytes.Contains(blockData, []byte(`"page-break"`)) {
		t.Fatalf("pdf-text-blocks.json missing expected content: %s", blockData)
	}

	pageData, err := os.ReadFile(filepath.Join(tmpDir, "pdf-layout-pages.json"))
	if err != nil {
		t.Fatalf("read pdf-layout-pages.json: %v", err)
	}
	var pages []pdfDebugPage
	if err := json.Unmarshal(pageData, &pages); err != nil {
		t.Fatalf("unmarshal pdf-layout-pages.json: %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("debug pages = %d, want at least 2", len(pages))
	}
	if got := pages[0].Lines[0].Text; got != "Debug Book" {
		t.Fatalf("first debug line = %q, want Debug Book", got)
	}
}

func TestCollectTextBlocksIncludesLinkSpans(t *testing.T) {
	blocks, err := collectTextBlocks(&content.Content{Book: &fb2.FictionBook{Bodies: []fb2.Body{{
		Kind: fb2.BodyMain,
		Sections: []fb2.Section{{Content: []fb2.FlowItem{{
			Kind: fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{
				Text: "See ",
			}, {
				Kind:     fb2.InlineLink,
				Href:     "https://example.com",
				Children: []fb2.InlineSegment{{Text: "example"}},
			}}},
		}}}},
	}}}})
	if err != nil {
		t.Fatalf("collectTextBlocks() error = %v", err)
	}
	blocks = textBlocksOnly(blocks)
	if len(blocks) != 1 {
		t.Fatalf("text blocks = %d, want 1", len(blocks))
	}
	if got := blocks[0].Links; len(got) != 1 || got[0].Start != 4 || got[0].End != 11 || got[0].Href != "https://example.com" {
		t.Fatalf("links = %#v, want example span", got)
	}
}

func TestNamedDestinations(t *testing.T) {
	got := pdfdoc.Format(namedDestinations([]pdfPage{
		{ObjectID: 4, Anchors: []string{"z", "a"}},
		{ObjectID: 8, Anchors: []string{"a", "m"}},
	}))
	for _, want := range []string{
		"/Dests << /Names [",
		"<61> [4 0 R /Fit]",
		"<6D> [8 0 R /Fit]",
		"<7A> [4 0 R /Fit]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("namedDestinations() = %q, missing %q", got, want)
		}
	}
	if strings.Contains(got, "[8 0 R /Fit] <6D>") {
		t.Fatalf("named destinations are not sorted by name: %q", got)
	}
}

func TestGenerateLinkAnnotations(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{
		Images: config.ImagesConfig{
			Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300},
		},
	}
	c := &content.Content{
		SrcName: "book.fb2",
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Link Book"}},
			},
			Bodies: []fb2.Body{{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{
					ID:    "target",
					Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Target"}}}}}},
					Content: []fb2.FlowItem{{
						Kind: fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{
							{Text: "Visit "},
							{Kind: fb2.InlineLink, Href: "https://example.com", Children: []fb2.InlineSegment{{Text: "example"}}},
							{Text: " and "},
							{Kind: fb2.InlineLink, Href: "#target", Children: []fb2.InlineSegment{{Text: "target"}}},
						}},
					}},
				}},
			}},
		},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	data, err := os.ReadFile(outputName)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	pdfText := string(data)
	for _, want := range []string{
		"/Annots [",
		"/Subtype /Link",
		"/URI <68747470733A2F2F6578616D706C652E636F6D>",
		"/Dest <746172676574>",
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain %q", want)
		}
	}
}

func TestGenerateTextBodyAddsPaginatedBodyPage(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{
		Images: config.ImagesConfig{
			Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300},
		},
	}
	c := &content.Content{
		SrcName: "book.fb2",
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Test Book"}},
			},
			Bodies: []fb2.Body{{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{
					ID:    "chapter-1",
					Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
					Content: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "This is the first native PDF body paragraph with selectable text."}}},
					}},
				}},
			}},
		},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(outputName)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	pdfText := string(data)
	for _, want := range []string{
		"/Count 2",
		"/Kids [3 0 R 11 0 R]",
		"/FontFile2 9 0 R",
		"/ToUnicode 10 0 R",
		"/Outlines 13 0 R",
		"/Type /Outlines",
		"/Dest [11 0 R /Fit]",
		"/Names [<636861707465722D31> [11 0 R /Fit]]",
		"xref\n0 15",
	} {
		if !strings.Contains(pdfText, want) {
			t.Errorf("generated PDF does not contain %q", want)
		}
	}
}
