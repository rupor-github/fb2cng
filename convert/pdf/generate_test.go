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
		"/Root 1 0 R",
		"/Info 5 0 R",
		"xref\n0 6",
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
