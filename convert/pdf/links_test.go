package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

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

func TestAddLinkAnnotationsUsesInlineFragments(t *testing.T) {
	page := &pdfPage{}
	line := paragraphLine{Fragments: []paragraphLineFragment{
		{Text: shapedText{Glyphs: []shapedGlyph{{GlyphID: 1, Rune: 'A', Width: 500}}}, Width: 20, FontSize: 10},
		{Text: shapedText{Glyphs: []shapedGlyph{{GlyphID: 2, Rune: '1', Width: 300}, {GlyphID: 3, Rune: '.', Width: 150}, {GlyphID: 4, Rune: '1', Width: 300}}}, Width: 12, FontSize: 7.5, BaselineShift: 3.4, LinkHref: "#note"},
	}}

	addLinkAnnotations(page, pdfTextBlock{}, line, 0, 100, 200, 10)

	if len(page.Annotations) != 1 {
		t.Fatalf("annotations = %#v, want one fragment annotation", page.Annotations)
	}
	annotation := page.Annotations[0]
	if annotation.Href != "#note" {
		t.Fatalf("annotation href = %q, want #note", annotation.Href)
	}
	if annotation.Rect.X1 != 120 || annotation.Rect.X2 != 132 || annotation.Rect.Y1 != 201.9 || annotation.Rect.Y2 != 210.9 {
		t.Fatalf("annotation rect = %#v, want fragment visual bounds", annotation.Rect)
	}
}

func TestInlineRunAnchorCreatesPDFNamedDestination(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := &pdfStyleResolver{styles: defaultPDFStyles()}
	pages, _, err := layoutPDFPages(pdfDocumentSpec{
		PageWidth:  240,
		PageHeight: 160,
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:      pdfBlockParagraph,
			Text:      "Body 1",
			StyleName: pdfStyleParagraph,
			Runs: []pdfInlineRun{
				{Text: "Body "},
				{Text: "1", StyleClasses: pdfStyleLinkFootnote, LinkHref: "#n1", AnchorID: "ref-n1-1"},
			},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	if len(pages[0].Anchors) != 1 || pages[0].Anchors[0] != "ref-n1-1" {
		t.Fatalf("anchors = %#v, want ref-n1-1", pages[0].Anchors)
	}
}

func TestNamedDestinations(t *testing.T) {
	got := docwriter.Format(namedDestinations([]pdfPage{
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

func TestGenerateAnnotationPage(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{
		Annotation: config.AnnotationConfig{Enable: true, Title: "About", InTOC: true},
		Images: config.ImagesConfig{
			Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300},
		},
	}
	c := &content.Content{
		SrcName: "book.fb2",
		Book: &fb2.FictionBook{
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Annotation Book"},
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
		"/Outlines",
		"/Names [<616E6E6F746174696F6E2D70616765>",
		"/Fit]",
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain %q", want)
		}
	}
}

func TestGenerateTOCPageLinkAnnotations(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{
		TOCPage: config.TOCPageConfig{Placement: common.TOCPagePlacementBefore},
		Images: config.ImagesConfig{
			Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300},
		},
	}
	c := &content.Content{
		SrcName: "book.fb2",
		Book: &fb2.FictionBook{
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "TOC Book"}}},
			Bodies: []fb2.Body{{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{
					ID:    "chapter-1",
					Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter 1"}}}}}},
					Content: []fb2.FlowItem{{
						Kind:      fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Body text."}}},
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
		"/Subtype /Link",
		"/Dest <636861707465722D31>",
		"/Names [<636861707465722D31>",
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain %q", want)
		}
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
