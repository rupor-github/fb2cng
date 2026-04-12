package pdf

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
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

func TestGenerate_RendersComplexDocument(t *testing.T) {
	cfg, err := config.LoadConfiguration("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Document.Images.Screen.Width = 1264
	cfg.Document.Images.Screen.Height = 1680

	book := &fb2.FictionBook{
		Stylesheets: []fb2.Stylesheet{{
			Type: "text/css",
			Data: `body { margin: 24pt 18pt 30pt 18pt; } .section-title-h2 { page-break-before: always; } .annotation { margin: 10pt 0; } .poem { margin: 12pt 0 12pt 24pt; }`,
		}},
		Description: fb2.Description{
			TitleInfo: fb2.TitleInfo{
				BookTitle: fb2.TextField{Value: "Complex Book"},
				Lang:      language.English,
				Authors: []fb2.Author{{
					FirstName: "Jane",
					LastName:  "Doe",
				}},
			},
			DocumentInfo: fb2.DocumentInfo{ID: "complex-pdf"},
		},
		Bodies: []fb2.Body{{
			Kind:  fb2.BodyMain,
			Title: simpleTitle("Book Title"),
			Image: &fb2.Image{Href: "#body-cover", Alt: "Body cover"},
			Sections: []fb2.Section{{
				ID:    "chapter-1",
				Title: simpleTitle("Chapter One"),
				Annotation: &fb2.Flow{Items: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Annotation paragraph."}}},
				}}},
				Content: []fb2.FlowItem{
					{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Style: "has-dropcap", Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Alpha paragraph with "}, {Kind: fb2.InlineStrong, Children: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "bold"}}}, {Kind: fb2.InlineText, Text: " text and "}, {Kind: fb2.InlineLink, Href: "https://example.com", Children: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "external link"}}}, {Kind: fb2.InlineText, Text: "."}}}},
					{Kind: fb2.FlowPoem, Poem: &fb2.Poem{Subtitles: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Poem subtitle"}}}}, Stanzas: []fb2.Stanza{{Verses: []fb2.Paragraph{{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "First verse"}}}, {Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Second verse"}}}}}}}},
					{Kind: fb2.FlowTable, Table: &fb2.Table{Rows: []fb2.TableRow{{Cells: []fb2.TableCell{{Header: true, Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "H1"}}}, {Header: true, Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "H2"}}}}}, {Cells: []fb2.TableCell{{Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "V1"}}}, {Content: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "V2"}}}}}}}},
					{Kind: fb2.FlowSection, Section: &fb2.Section{ID: "chapter-1-1", Title: simpleTitle("Nested Section"), Content: []fb2.FlowItem{{Kind: fb2.FlowParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: "Nested content."}}}}}}},
				},
			}},
		}},
	}
	book.SetBodyTitlePageBreak(true)
	book.SetSectionPageBreaks(map[int]bool{2: true})

	c := &content.Content{
		SrcName:      "complex.fb2",
		OutputFormat: common.OutputFmtPdf,
		Book:         book,
		CoverID:      "body-cover",
		ImagesIndex: fb2.BookImages{
			"body-cover": {MimeType: "image/png", Data: tinyPNG(t), Dim: struct{ Width, Height int }{Width: 16, Height: 16}, Filename: "body-cover.png"},
		},
	}

	outputPath := filepath.Join(t.TempDir(), "complex.pdf")
	if err := Generate(context.Background(), c, outputPath, &cfg.Document, zaptest.NewLogger(t)); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output pdf: %v", err)
	}
	if len(data) == 0 || !strings.HasPrefix(string(data), "%PDF-") {
		t.Fatalf("generated file does not look like PDF")
	}
}

func simpleTitle(text string) *fb2.Title {
	return &fb2.Title{Items: []fb2.TitleItem{{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: text}}}}}}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 0x22, G: 0x66, B: 0xaa, A: 0xff})
	img.Set(1, 0, color.NRGBA{R: 0xaa, G: 0x66, B: 0x22, A: 0xff})
	img.Set(0, 1, color.NRGBA{R: 0x33, G: 0x99, B: 0x55, A: 0xff})
	img.Set(1, 1, color.NRGBA{R: 0xee, G: 0xdd, B: 0x55, A: 0xff})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
