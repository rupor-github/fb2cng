package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
	"golang.org/x/text/language"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

func TestGenerate_MinimalPDF(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Document.Images.Screen.Width = 1264
	cfg.Document.Images.Screen.Height = 1680

	c := &content.Content{
		SrcName:      "test.fb2",
		OutputFormat: common.OutputFmtEpub2,
		Book: &fb2.FictionBook{
			Description: fb2.Description{
				TitleInfo: fb2.TitleInfo{
					BookTitle: fb2.TextField{Value: "Test Book"},
					Lang:      language.English,
					Authors: []fb2.Author{
						{FirstName: "John", LastName: "Doe"},
					},
				},
				DocumentInfo: fb2.DocumentInfo{ID: "test-pdf"},
			},
			Bodies: []fb2.Body{{
				Kind:  fb2.BodyMain,
				Title: simpleTitle("Book Title"),
				Sections: []fb2.Section{{
					ID:    "chap1",
					Title: simpleTitle("Chapter 1"),
				}},
			}},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "test.pdf")
	if err := Generate(context.Background(), c, outputPath, &cfg.Document, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output pdf: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("generated PDF is empty")
	}
	if !strings.HasPrefix(string(data), "%PDF-") {
		t.Fatalf("generated file does not look like PDF: %q", string(data[:min(len(data), 8)]))
	}
}

func simpleTitle(text string) *fb2.Title {
	return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: text}}}}}}
}
