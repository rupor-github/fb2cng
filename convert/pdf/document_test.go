package pdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"

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

func TestPageSizePointsRequiresConfiguredDPI(t *testing.T) {
	_, _, err := pageSizePoints(config.ScreenConfig{Width: 300, Height: 600})
	if err == nil || !strings.Contains(err.Error(), "invalid pdf screen dpi") {
		t.Fatalf("pageSizePoints() error = %v, want invalid dpi", err)
	}
}

func TestGenerateLogsTemporaryAndFinalOutputPaths(t *testing.T) {
	tmpDir := t.TempDir()
	temporaryName := filepath.Join(tmpDir, ".book.pdf.123.tmp")
	finalName := filepath.Join(tmpDir, "book.pdf")
	cfg := &config.DocumentConfig{Images: config.ImagesConfig{Screen: config.ScreenConfig{Width: 1264, Height: 1680, DPI: 300}}}
	c := &content.Content{
		SrcName: "book.fb2",
		Book:    &fb2.FictionBook{Description: fb2.Description{TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Test Book"}}}},
	}
	core, logs := observer.New(zapcore.DebugLevel)

	if err := Generate(context.Background(), c, temporaryName, cfg, zap.New(core), finalName); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	entries := logs.FilterMessage("Writing PDF").All()
	if len(entries) != 1 {
		t.Fatalf("Writing PDF log entries = %d, want 1", len(entries))
	}
	fields := map[string]string{}
	for _, field := range entries[0].Context {
		if field.Type == zapcore.StringType {
			fields[field.Key] = field.String
		}
	}
	if fields["file"] != finalName {
		t.Fatalf("file log field = %q, want final output %q", fields["file"], finalName)
	}
	if fields["temporary_file"] != temporaryName {
		t.Fatalf("temporary_file log field = %q, want %q", fields["temporary_file"], temporaryName)
	}
}

func TestBuildPDFDocument(t *testing.T) {
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
	if !strings.Contains(string(data), "+NotoSansMono-Regular") {
		t.Fatalf("generated PDF does not contain embedded subset CustomMono font")
	}
}

func TestGeneratePDFWithThirdPartyCSSFontHonorsNoSubsettingRestriction(t *testing.T) {
	fontData, err := gunzipFont(notoMonoRegularGZ)
	if err != nil {
		t.Fatalf("gunzipFont() error = %v", err)
	}
	fontData = append([]byte(nil), fontData...)
	if !patchFontEmbeddingFSType(fontData, 0x0102) {
		t.Fatal("patchFontEmbeddingFSType() = false")
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
		Debug:   true,
		WorkDir: tmpDir,
		Book: &fb2.FictionBook{
			Stylesheets: []fb2.Stylesheet{{
				Type: "text/css",
				Data: `
					@font-face { font-family: ThirdPartyMono; src: url("#third-party-mono"); font-weight: 400; font-style: normal; }
					p.third-party { font-family: ThirdPartyMono; }
				`,
				Resources: []fb2.StylesheetResource{{OriginalURL: "#third-party-mono", MimeType: "font/ttf", Data: fontData}},
			}},
			Description: fb2.Description{TitleInfo: fb2.TitleInfo{BookTitle: fb2.TextField{Value: "Restricted Font Book"}}},
			Bodies: []fb2.Body{{
				Kind: fb2.BodyMain,
				Sections: []fb2.Section{{Content: []fb2.FlowItem{{
					Kind:      fb2.FlowParagraph,
					Paragraph: &fb2.Paragraph{Style: "third-party", Text: []fb2.InlineSegment{{Text: "Third-party embedded TTF."}}},
				}}}},
			}},
		},
	}
	core, logs := observer.New(zapcore.DebugLevel)

	if err := Generate(context.Background(), c, outputName, cfg, zap.New(core)); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	limitationEntries := logs.FilterMessage("Loaded PDF @font-face resource with limitations").All()
	if len(limitationEntries) != 1 {
		t.Fatalf("stylesheet font limitation log entries = %d, want 1", len(limitationEntries))
	}
	fields := pdfZapStringFields(limitationEntries[0])
	if fields["family"] != "ThirdPartyMono" || fields["pdf_subsetting_status"] != "disabled_by_font_fs_type" {
		t.Fatalf("stylesheet font limitation fields = %#v, want disabled-by-fsType ThirdPartyMono", fields)
	}
	if !pdfZapStringSliceFieldContains(limitationEntries[0], "limitations", "font_disallows_subsetting_full_font_will_be_embedded") {
		t.Fatalf("stylesheet font limitations missing no-subsetting note: %#v", limitationEntries[0].Context)
	}

	restrictionEntries := logs.FilterMessage("PDF font has embedding restrictions").All()
	if len(restrictionEntries) != 1 {
		t.Fatalf("embedding restriction log entries = %d, want 1", len(restrictionEntries))
	}
	boolFields := pdfZapBoolFields(restrictionEntries[0])
	if !boolFields["restricted_license_embedding"] || !boolFields["no_subsetting"] {
		t.Fatalf("embedding restriction fields = %#v, want restricted and no_subsetting", boolFields)
	}

	data, err := os.ReadFile(outputName)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.Contains(string(data), "+NotoSansMono-Regular") {
		t.Fatalf("generated PDF contains subset-prefixed restricted font")
	}
	if !strings.Contains(string(data), "/BaseFont /NotoSansMono-Regular") {
		t.Fatalf("generated PDF does not contain full embedded restricted font base name")
	}

	var fonts []pdfDebugFont
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-fonts.json"), &fonts)
	var restrictedFont *pdfDebugFont
	for i := range fonts {
		if fonts[i].Family == "ThirdPartyMono" {
			restrictedFont = &fonts[i]
			break
		}
	}
	if restrictedFont == nil {
		t.Fatalf("debug fonts = %#v, want ThirdPartyMono entry", fonts)
	}
	if restrictedFont.Subset || restrictedFont.PDFBaseFont != "NotoSansMono-Regular" {
		t.Fatalf("debug restricted font = %#v, want full unprefixed font", *restrictedFont)
	}
	if restrictedFont.OriginalFontFileSize != len(fontData) || restrictedFont.EmbeddedFontFileSize != len(fontData) {
		t.Fatalf("debug restricted font sizes = %#v, want original and embedded size %d", *restrictedFont, len(fontData))
	}
	if len(restrictedFont.PDFCIDs) == 0 || len(restrictedFont.SubsetGlyphIDs) != 0 {
		t.Fatalf("debug restricted font glyph ids = %#v, want PDF CIDs but no subset GIDs", *restrictedFont)
	}
}

func pdfZapBoolFields(entry observer.LoggedEntry) map[string]bool {
	fields := map[string]bool{}
	for _, field := range entry.Context {
		if field.Type == zapcore.BoolType {
			fields[field.Key] = field.Integer == 1
		}
	}
	return fields
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
						Kind: fb2.FlowParagraph,
						Paragraph: &fb2.Paragraph{
							Text: []fb2.InlineSegment{{Text: "This is the first native PDF body paragraph with selectable text."}},
						},
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
		"+NotoSerif-Regular",
		"+NotoSerif-Bold",
	} {
		if !strings.Contains(pdfText, want) {
			t.Errorf("generated PDF does not contain %q", want)
		}
	}
}
