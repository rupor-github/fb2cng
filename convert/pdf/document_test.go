package pdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

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
		"/Root 1 0 R",
		"/Info 5 0 R",
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

func TestGeneratePDFEmbedsStylesheetFontFace(t *testing.T) {
	fontData, err := gunzipFont(notoMonoRegularGZ)
	if err != nil {
		t.Fatalf("gunzipFont() error = %v", err)
	}
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
			Stylesheets: []fb2.Stylesheet{{
				Type: "text/css",
				Data: `
					@font-face { font-family: CustomMono; src: url("#custom-mono"); font-weight: 400; font-style: normal; }
					p.custom { font-family: CustomMono; }
				`,
				Resources: []fb2.StylesheetResource{{OriginalURL: "#custom-mono", MimeType: "font/ttf", Data: fontData}},
			}},
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Font Book"}}},
			Bodies: []fb2.Body{{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{Content: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Style: "custom", Text: []fb2.InlineSegment{{Text: "Custom embedded font paragraph."}}},
				}}}},
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
	if !strings.Contains(string(data), "/BaseFont /NotoSansMono-Regular") {
		t.Fatalf("generated PDF does not contain embedded CustomMono font")
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
		"/Count 1",
		"/Kids [3 0 R]",
		"/FontFile2",
		"/ToUnicode",
		"/Outlines",
		"/Type /Outlines",
		"/Dest [3 0 R /Fit]",
		"/Names [<636861707465722D31> [3 0 R /Fit]]",
		"/BaseFont /NotoSerif-Regular",
		"/BaseFont /NotoSerif-Bold",
	} {
		if !strings.Contains(pdfText, want) {
			t.Errorf("generated PDF does not contain %q", want)
		}
	}
}
