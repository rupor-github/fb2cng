package pdf

import (
	"math"
	"testing"

	"fbc/content"
	"fbc/fb2"
)

func TestLayoutPDFPagesAnnotationWrapperParagraphCanStripRootHorizontalMargins(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			html { margin: 0 -20pt 0 -20pt; }
			p { margin: 0; text-indent: 0; }
			.annotation { margin-left: 12pt; margin-right: 12pt; }
		`,
	}}}, nil)

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:                       pdfBlockParagraph,
			Text:                       "Wrapped annotation.",
			StyleClasses:               pdfStyleAnnotation,
			StripRootHorizontalMargins: true,
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one annotation line", pages)
	}
	if got := pages[1].Lines[0].X; math.Abs(got-36) > 0.001 {
		t.Fatalf("annotation line X = %v, want 36 (24 base margin + 12 annotation margin)", got)
	}
}

func TestLayoutPDFPagesAnnotationWrapperNestedCiteCanStripRootHorizontalMargins(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			html { margin: 0 -20pt 0 -20pt; }
			p { margin: 0; text-indent: 0; }
			.cite { margin-left: 21pt; margin-right: 21pt; }
		`,
	}}}, nil)

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:                       pdfBlockParagraph,
			Text:                       "Nested cite.",
			StyleClasses:               pdfStyleCite,
			StripRootHorizontalMargins: true,
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one cite line", pages)
	}
	if got := pages[1].Lines[0].X; math.Abs(got-45) > 0.001 {
		t.Fatalf("cite line X = %v, want 45 (24 base margin + 21 cite margin)", got)
	}
}

func TestLayoutPDFPagesAnnotationWrapperNestedPoemCanStripRootHorizontalMargins(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	resolver := newPDFStyleResolver(&fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			html { margin: 0 -20pt 0 -20pt; }
			p { margin: 0; text-indent: 0; }
			.verse { margin-left: 15pt; margin-right: 15pt; }
		`,
	}}}, nil)

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 180,
		Title:      "Title",
		Author:     "Author",
		Styles:     resolver,
		Blocks: []pdfTextBlock{{
			Kind:                       pdfBlockPoem,
			Text:                       "Verse line.",
			StyleClasses:               pdfStylePoem,
			ContextClasses:             pdfStylePoem,
			StripRootHorizontalMargins: true,
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 || len(pages[1].Lines) != 1 {
		t.Fatalf("layoutPDFPages() pages = %#v, want one verse line", pages)
	}
	wantX := 24 + pdfPoemMarginLeft + 15
	if got := pages[1].Lines[0].X; math.Abs(got-wantX) > 0.001 {
		t.Fatalf("verse line X = %v, want %v (24 base margin + poem margin + 15 verse margin)", got, wantX)
	}
}

func TestLayoutPDFPagesAnnotationNestedSectionPreservesRootHorizontalMargins(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			html { margin: 0 -20pt 0 -20pt; }
			p { margin: 0; text-indent: 0; }
			.annotation { margin-left: 12pt; margin-right: 12pt; }
		`,
	}}, Bodies: []fb2.Body{{
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

	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:  220,
		PageHeight: 220,
		Title:      "Title",
		Author:     "Author",
		Styles:     newPDFStyleResolver(book, nil),
		Blocks:     blocks,
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}

	var bodyX, noteX float64
	foundBody := false
	foundNote := false
	for _, page := range pages {
		for _, line := range page.Lines {
			switch shapedRunes(line.Text) {
			case "Nested section body.":
				bodyX = line.X
				foundBody = true
			case "Nested section note.":
				noteX = line.X
				foundNote = true
			}
		}
	}
	if !foundBody || !foundNote {
		t.Fatalf("expected nested section body and note lines, got %#v", pages)
	}
	if math.Abs(bodyX-4) > 0.001 {
		t.Fatalf("nested section body X = %v, want 4 (24 base margin - 20 root inset)", bodyX)
	}
	if math.Abs(noteX-16) > 0.001 {
		t.Fatalf("nested section note X = %v, want 16 (24 base margin - 20 root inset + 12 annotation margin)", noteX)
	}
}
