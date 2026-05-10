package pdf

import (
	"testing"

	"fbc/common"
	"fbc/config"
	"fbc/content"
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
	if got := plan.Blocks[2]; got.Kind != pdfBlockParagraph || got.Text != "Book annotation." {
		t.Fatalf("annotation paragraph = %#v", got)
	}
	if len(plan.TOC) == 0 || plan.TOC[0].ID != "annotation-page" || plan.TOC[0].Title != "About" {
		t.Fatalf("TOC = %#v, want annotation entry first", plan.TOC)
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

func textBlocksOnly(blocks []pdfTextBlock) []pdfTextBlock {
	out := make([]pdfTextBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind != pdfBlockPageBreak {
			out = append(out, block)
		}
	}
	return out
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
