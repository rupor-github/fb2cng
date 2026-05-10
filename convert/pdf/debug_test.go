package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/config"
	"fbc/content"
	"fbc/fb2"
)

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
	if len(fonts) < 2 {
		t.Fatalf("debug fonts = %#v, want multiple used fonts", fonts)
	}
	for _, font := range fonts {
		if font.ResourceName == "" || font.PostScriptName == "" || font.UsedGlyphCount == 0 {
			t.Fatalf("debug font = %#v, want resource name, PostScript name, and used glyphs", font)
		}
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
