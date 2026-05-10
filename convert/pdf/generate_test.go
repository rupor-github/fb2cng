package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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

func TestInfoDictionaryIncludesSubjectAndKeywords(t *testing.T) {
	info := docwriter.Format(infoDictionary(skeletonDocument{
		Title:    "Title",
		Author:   "Author",
		Subject:  "Annotation excerpt",
		Keywords: "one, two",
	}))
	for _, want := range []string{
		"/Title " + docwriter.Format(docwriter.UTF16TextString("Title")),
		"/Author " + docwriter.Format(docwriter.UTF16TextString("Author")),
		"/Subject " + docwriter.Format(docwriter.UTF16TextString("Annotation excerpt")),
		"/Keywords " + docwriter.Format(docwriter.UTF16TextString("one, two")),
	} {
		if !strings.Contains(info, want) {
			t.Fatalf("info dictionary = %q, missing %q", info, want)
		}
	}
}

func TestBookMetadataSubjectAndKeywords(t *testing.T) {
	longAnnotation := strings.Repeat("слово ", 120)
	c := &content.Content{Book: &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{
		Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
			Kind:      fb2.FlowParagraph,
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: longAnnotation}}},
		}}},
		Keywords: &fb2.TextField{Value: "one\n two\tthree"},
	}}}}
	if got := bookSubject(c); len([]rune(got)) != metadataExcerptMaxRunes {
		t.Fatalf("bookSubject length = %d, want %d", len([]rune(got)), metadataExcerptMaxRunes)
	}
	if strings.Contains(bookSubject(c), "\n") || strings.Contains(bookSubject(c), "\t") {
		t.Fatalf("bookSubject did not normalize whitespace: %q", bookSubject(c))
	}
	if got := bookKeywords(c); got != "one two three" {
		t.Fatalf("bookKeywords() = %q, want normalized keywords", got)
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
	if !strings.Contains(pdfText, docwriter.Format(docwriter.UTF16TextString("Test Book"))) {
		t.Errorf("generated PDF does not contain UTF-16BE title metadata")
	}
	if !strings.Contains(pdfText, docwriter.Format(docwriter.UTF16TextString("First Author, Second"))) {
		t.Errorf("generated PDF does not contain UTF-16BE author metadata")
	}
}

func TestGeneratePDFMetadataSubjectAndKeywords(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300}}}
	c := &content.Content{
		SrcName: "book.fb2",
		Book: &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{
			BookTitle:  fb2.TextField{Value: "Metadata Book"},
			Annotation: &fb2.Flow{Items: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Annotation text."}}}}}},
			Keywords:   &fb2.TextField{Value: "alpha, beta"},
		}}},
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
		docwriter.Format(docwriter.UTF16TextString("Annotation text.")),
		docwriter.Format(docwriter.UTF16TextString("alpha, beta")),
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain metadata %q", want)
		}
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

func TestPageContentDrawsImages(t *testing.T) {
	content := string(pageContent(pdfPage{Images: []pdfPageImage{{
		Name:   "Im1",
		X:      10,
		Y:      20,
		Width:  30,
		Height: 40,
	}}}))
	for _, want := range []string{"30 0 0 40 10 20 cm", "/Im1 Do"} {
		if !strings.Contains(content, want) {
			t.Fatalf("page content = %q, missing %q", content, want)
		}
	}
}

func TestLayoutPDFPagesAddsCoverImagePage(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	pages, _, err := layoutPDFPages(skeletonDocument{
		PageWidth:      100,
		PageHeight:     160,
		ScreenWidthPx:  100,
		ScreenHeightPx: 160,
		Title:          "Title",
		Author:         "Author",
		CoverID:        "cover",
		Images: fb2.BookImages{"cover": &fb2.BookImage{
			MimeType: "image/png",
			Dim: struct {
				Width  int
				Height int
			}{Width: 50, Height: 80},
		}},
	}, face)
	if err != nil {
		t.Fatalf("layoutPDFPages() error = %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("pages = %d, want cover plus title", len(pages))
	}
	if len(pages[0].Images) != 1 || pages[0].Images[0].ImageID != "cover" {
		t.Fatalf("cover page images = %#v", pages[0].Images)
	}
	if len(pages[0].Anchors) != 1 || pages[0].Anchors[0] != "cover" {
		t.Fatalf("cover page anchors = %#v", pages[0].Anchors)
	}
}

func TestMakePDFImageResourceEmbedsJPEGDirectly(t *testing.T) {
	resource, err := makePDFImageResource(&fb2.BookImage{
		MimeType: "image/jpeg",
		Data:     testJPEG(t, 2, 3),
	})
	if err != nil {
		t.Fatalf("makePDFImageResource() error = %v", err)
	}
	got := docwriter.Format(resource.Dict)
	for _, want := range []string{"/Filter /DCTDecode", "/Subtype /Image", "/Width 2", "/Height 3"} {
		if !strings.Contains(got, want) {
			t.Fatalf("image dict = %q, missing %q", got, want)
		}
	}
}

func TestGenerateEmbedsPDFImageXObject(t *testing.T) {
	tmpDir := t.TempDir()
	outputName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 100, Height: 160, DPI: 100}}}
	imageData := testPNG(t, 2, 3)
	c := &content.Content{
		SrcName: "book.fb2",
		CoverID: "cover",
		ImagesIndex: fb2.BookImages{"cover": &fb2.BookImage{
			MimeType: "image/png",
			Data:     imageData,
			Dim: struct {
				Width  int
				Height int
			}{Width: 2, Height: 3},
		}},
		Book: &fb2.FictionBook{
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Image Book"},
				Coverpage: []fb2.InlineImage{{Href: "#cover"}},
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
		"/XObject << /Im1",
		"/Subtype /Image",
		"/ColorSpace /DeviceRGB",
		"/Width 2",
		"/Height 3",
	} {
		if !strings.Contains(pdfText, want) {
			t.Fatalf("generated PDF does not contain %q", want)
		}
	}
}

func testPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := testImage(width, height)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func testJPEG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := testImage(width, height)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

func testImage(width int, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.NRGBA{R: uint8(40 + x), G: uint8(80 + y), B: 120, A: 255})
		}
	}
	return img
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
					ID:    "debug-section",
					Title: &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: "Chapter"}}}}}},
					Content: []fb2.FlowItem{{
						Kind: fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{Style: "has-dropcap", Text: []fb2.InlineSegment{
							{Text: "Debug body "},
							{Kind: fb2.InlineLink, Href: "https://example.com", Children: []fb2.InlineSegment{{Text: "link"}}},
							{Text: "."},
						}},
					}},
				}},
			}},
		},
	}

	if err := Generate(context.Background(), c, outputName, cfg, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var structurePlan pdfDebugStructurePlan
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-structure-plan.json"), &structurePlan)
	if len(structurePlan.Units) == 0 || structurePlan.Units[0].ID != "debug-section" || structurePlan.Units[0].Kind != "section" {
		t.Fatalf("debug structure plan = %#v, want debug section unit", structurePlan)
	}

	blockData, err := os.ReadFile(filepath.Join(tmpDir, "pdf-text-blocks.json"))
	if err != nil {
		t.Fatalf("read pdf-text-blocks.json: %v", err)
	}
	if !bytes.Contains(blockData, []byte(`"Chapter"`)) || !bytes.Contains(blockData, []byte(`"page-break"`)) || !bytes.Contains(blockData, []byte(`"style_name"`)) || !bytes.Contains(blockData, []byte(`"style_classes": "has-dropcap"`)) {
		t.Fatalf("pdf-text-blocks.json missing expected content: %s", blockData)
	}

	var styles []pdfDebugResolvedStyle
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-resolved-styles.json"), &styles)
	if len(styles) == 0 || styles[0].Name == "" {
		t.Fatalf("debug resolved styles = %#v, want named styles", styles)
	}
	traceData, err := os.ReadFile(filepath.Join(tmpDir, "pdf-style-trace.txt"))
	if err != nil {
		t.Fatalf("read pdf-style-trace.txt: %v", err)
	}
	if !bytes.Contains(traceData, []byte("=== PDF Style Trace ===")) || !bytes.Contains(traceData, []byte("ASSIGN")) || !bytes.Contains(traceData, []byte("COLLAPSE")) {
		t.Fatalf("pdf-style-trace.txt missing expected content: %s", traceData)
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
	if len(pages[1].Anchors) == 0 || pages[1].Anchors[0] != "debug-section" {
		t.Fatalf("body page anchors = %#v, want debug-section", pages[1].Anchors)
	}

	var links []pdfDebugLink
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-links.json"), &links)
	if len(links) != 1 || links[0].Href != "https://example.com" || links[0].ObjectID == 0 {
		t.Fatalf("debug links = %#v, want external link with object id", links)
	}

	var images []pdfDebugImage
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-images.json"), &images)
	if images == nil {
		t.Fatalf("debug images should unmarshal to an empty array, got nil")
	}

	var fonts []pdfDebugFont
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-fonts.json"), &fonts)
	if len(fonts) != 1 || fonts[0].PostScriptName == "" || fonts[0].UsedGlyphCount == 0 {
		t.Fatalf("debug fonts = %#v, want one used font", fonts)
	}
}

func readJSONDebugFile(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal %s: %v", filepath.Base(path), err)
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
		"/Dest [11 0 R /Fit]",
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
