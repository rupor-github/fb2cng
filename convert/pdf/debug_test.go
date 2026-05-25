package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"

	"fbc/common"
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
			Stylesheets: []fb2.Stylesheet{{Type: "text/css", Data: `p { margin: 0; }`}},
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
	parsedCSS, err := os.ReadFile(filepath.Join(tmpDir, "parsed-stylesheet.css"))
	if err != nil {
		t.Fatalf("read parsed-stylesheet.css: %v", err)
	}
	if !bytes.Contains(parsedCSS, []byte("p {")) || !bytes.Contains(parsedCSS, []byte("margin: 0")) {
		t.Fatalf("parsed-stylesheet.css missing expected CSS: %s", parsedCSS)
	}

	pageData, err := os.ReadFile(filepath.Join(tmpDir, "pdf-layout-pages.json"))
	if err != nil {
		t.Fatalf("read pdf-layout-pages.json: %v", err)
	}
	var pages []pdfDebugPage
	if err := json.Unmarshal(pageData, &pages); err != nil {
		t.Fatalf("unmarshal pdf-layout-pages.json: %v", err)
	}
	if len(pages) < 1 {
		t.Fatalf("debug pages = %d, want at least 1", len(pages))
	}
	if len(pages[0].Anchors) == 0 || pages[0].Anchors[0] != "debug-section" {
		t.Fatalf("body page anchors = %#v, want debug-section", pages[0].Anchors)
	}
	lineBreakFound := false
	for _, page := range pages {
		for _, line := range page.Lines {
			if line.Width != line.DrawnWidth || line.AdvanceWidth <= 0 || line.RightEdge <= line.X || line.VisualRight <= line.VisualLeft {
				t.Fatalf("debug line metrics = %#v, want populated width diagnostics", line)
			}
			if line.LineBreak == nil {
				continue
			}
			lineBreakFound = true
			if line.LineBreak.AvailableWidth <= 0 || line.LineBreak.Demerits <= 0 || line.LineBreak.Fitness == "" {
				t.Fatalf("debug line break = %#v, want populated diagnostics", line.LineBreak)
			}
			if line.AvailableWidth != line.LineBreak.AvailableWidth {
				t.Fatalf("debug line available width = %v, line_break available width = %v", line.AvailableWidth, line.LineBreak.AvailableWidth)
			}
		}
	}
	if !lineBreakFound {
		t.Fatal("pdf-layout-pages.json missing line_break diagnostics")
	}

	var lineGlyphs []pdfDebugGlyphLine
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-line-glyphs.json"), &lineGlyphs)
	if len(lineGlyphs) == 0 || len(lineGlyphs[0].Glyphs) == 0 || lineGlyphs[0].Glyphs[0].PDFCID == 0 {
		t.Fatalf("debug line glyphs = %#v, want compact positioned glyph diagnostics", lineGlyphs)
	}

	var justification []pdfDebugJustificationLine
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-justification.json"), &justification)
	if justification == nil {
		t.Fatalf("debug justification should unmarshal to an empty array, got nil")
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
		if font.OriginalFontFileSize <= 0 || font.EmbeddedFontFileSize <= 0 || !font.Subset {
			t.Fatalf("debug font = %#v, want original/subset font file sizes", font)
		}
		if font.EmbeddedFontFileStreamSize <= 0 || font.EmbeddedFontFileStreamSize >= font.EmbeddedFontFileSize || font.ToUnicodeStreamSize <= 0 {
			t.Fatalf("debug font = %#v, want compressed font and ToUnicode stream sizes", font)
		}
		if font.OutlineKind != "truetype" || font.PDFCIDFontSubtype != "CIDFontType2" || font.PDFEmbeddedFontFile != "FontFile2" {
			t.Fatalf("debug font = %#v, want TrueType PDF font program metadata", font)
		}
	}

	var printedFootnotes pdfDebugPrintedFootnotes
	readJSONDebugFile(t, filepath.Join(tmpDir, "pdf-printed-footnotes.json"), &printedFootnotes)
	if printedFootnotes.Enabled || printedFootnotes.FinalPageCount != len(pages) {
		t.Fatalf("debug printed footnotes = %#v, want disabled summary for non-printed document", printedFootnotes)
	}
}

func TestPDFDebugLineGlyphsAndJustificationSummaries(t *testing.T) {
	line := pdfPageLine{
		X:             10,
		Y:             100,
		FontSize:      10,
		LetterSpacing: 0,
		FontName:      "F1",
		Text: shapedText{Glyphs: []shapedGlyph{
			{GlyphID: 1, Rune: 'A', Source: "A", Width: 600, Advance: 600, HasAdvance: true},
			{GlyphID: 2, Rune: ' ', Source: " ", Width: 250, Advance: 250, HasAdvance: true},
			{GlyphID: 3, Rune: 'B', Source: "B", Width: 600, Advance: 600, HasAdvance: true},
		}},
		ExtraWordSpacing: 5,
		ExtraCharSpacing: 0.25,
		BreakStats: paragraphLineBreakStats{
			AvailableWidth:  20,
			AdjustmentRatio: 1,
			Badness:         100,
			Demerits:        10_000,
			Fitness:         paragraphFitnessLoose,
		},
	}
	pages := []pdfPage{{Lines: []pdfPageLine{line}}}

	glyphLines := pdfDebugLineGlyphs(pages)
	if len(glyphLines) != 1 || len(glyphLines[0].Glyphs) != 3 {
		t.Fatalf("glyph lines = %#v, want one compact three-glyph line", glyphLines)
	}
	if glyphLines[0].Glyphs[0].X != 10 || glyphLines[0].Glyphs[1].X != 16.25 || glyphLines[0].Glyphs[2].X != 24 {
		t.Fatalf("glyph positions = %#v, want advances with character and word spacing", glyphLines[0].Glyphs)
	}
	if glyphLines[0].Glyphs[0].PDFCID != 1 || glyphLines[0].Glyphs[0].Rune != "U+0041" || glyphLines[0].Glyphs[0].Source != "A" {
		t.Fatalf("first glyph debug = %#v, want PDF CID, source, and rune", glyphLines[0].Glyphs[0])
	}

	justification := pdfDebugJustificationLines(pages)
	if len(justification) != 1 {
		t.Fatalf("justification = %#v, want one justified line summary", justification)
	}
	summary := justification[0]
	if summary.Decision != "stretch_word_and_char_spacing_capped" || !summary.WordSpacingCapped || !summary.CharSpacingCapped {
		t.Fatalf("justification summary = %#v, want capped word/char spacing decision", summary)
	}
	if summary.JustificationGaps != 1 || summary.GlyphCount != 3 || summary.LineBreak == nil || summary.LineBreak.Fitness != "loose" {
		t.Fatalf("justification summary = %#v, want line-break and gap diagnostics", summary)
	}
	if summary.RejectedCandidatesRecorded || !strings.Contains(summary.BreakCandidateSummary, "rejected_candidates_not_retained") {
		t.Fatalf("justification summary = %#v, rejected candidate retention should be explicit", summary)
	}
}

func TestPDFDebugFontsReportsOriginalCIDsAndSubsetGlyphIDs(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeText(face, "Tiny")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	objects, err := fontResourceObjects(face, shaped.Used, fontObjectIDs{
		Type0Font:      1,
		CIDFont:        2,
		FontDescriptor: 3,
		FontFile:       4,
		ToUnicode:      5,
	})
	if err != nil {
		t.Fatalf("fontResourceObjects() error = %v", err)
	}

	fonts := pdfDebugFonts([]pdfFontResource{{
		Key:     pdfFontKey{Family: "serif"},
		Name:    "F1",
		Face:    face,
		Used:    shaped.Used,
		CIDMap:  objects.CIDMap,
		Objects: objects,
	}})
	if len(fonts) != 1 {
		t.Fatalf("debug fonts = %#v, want one font", fonts)
	}
	font := fonts[0]
	if !font.Subset ||
		len(font.OriginalGlyphIDs) != font.UsedGlyphCount ||
		len(font.PDFCIDs) != font.UsedGlyphCount ||
		len(font.SubsetGlyphIDs) < font.UsedGlyphCount {
		t.Fatalf("debug font glyph summary = %#v, want original glyphs, PDF CIDs, and subset GIDs", font)
	}
	if !slices.Equal(font.UsedGlyphIDs, font.OriginalGlyphIDs) {
		t.Fatalf("used glyph IDs = %v, original glyph IDs = %v, want legacy field to report original IDs", font.UsedGlyphIDs, font.OriginalGlyphIDs)
	}

	for _, originalGlyphID := range font.OriginalGlyphIDs {
		mappedCID, ok := objects.CIDMap[originalGlyphID]
		if !ok {
			t.Fatalf("original glyph %d missing from CID map %#v", originalGlyphID, objects.CIDMap)
		}
		if !slices.Contains(font.PDFCIDs, mappedCID) {
			t.Fatalf("PDF CIDs = %v, want mapped CID %d for original glyph %d", font.PDFCIDs, mappedCID, originalGlyphID)
		}
		if !slices.Contains(font.SubsetGlyphIDs, mappedCID) {
			t.Fatalf("subset glyph IDs = %v, want subset GID %d", font.SubsetGlyphIDs, mappedCID)
		}
		entry, ok := pdfDebugFontGlyphIDMapEntry(font.GlyphIDMap, originalGlyphID)
		if !ok || !entry.Used || entry.PDFCID != mappedCID || entry.SubsetGlyphID != mappedCID {
			t.Fatalf("glyph ID map entry for original glyph %d = %#v/%t, want used PDF CID/subset GID %d", originalGlyphID, entry, ok, mappedCID)
		}
	}
}

func pdfDebugFontGlyphIDMapEntry(entries []pdfDebugFontGlyphIDMap, originalGlyphID uint16) (pdfDebugFontGlyphIDMap, bool) {
	for _, entry := range entries {
		if entry.OriginalGlyphID == originalGlyphID {
			return entry, true
		}
	}
	return pdfDebugFontGlyphIDMap{}, false
}

func TestPDFDebugPrintedFootnotesSummaryIncludesPlansReservesAndContinuationPacks(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	longBody := strings.Repeat("long footnote text ", 120) + "ENDMARK"
	doc := pdfDocumentSpec{
		PageWidth:  260,
		PageHeight: 180,
		Content:    &content.Content{OutputFormat: common.OutputFmtPdf, FootnotesMode: common.FootnotesModeFloatRenumbered},
		PrintedFootnotes: map[string]pdfPrintedFootnote{
			"n1": {
				ID: "n1",
				BodyBlocks: []pdfTextBlock{{
					Kind:           pdfBlockParagraph,
					Text:           longBody,
					Runs:           []pdfInlineRun{{Text: longBody}},
					StyleClasses:   pdfStyleFootnote,
					ContextClasses: pdfStyleFootnote,
				}},
			},
		},
		Blocks: []pdfTextBlock{{
			Kind: pdfBlockParagraph,
			Text: "Body 1",
			Runs: []pdfInlineRun{{Text: "Body "}, {Text: "1", StyleClasses: pdfStyleLinkFootnote, FootnoteID: "n1", Superscript: true}},
		}},
	}

	pages, _, printedFootnotes, err := layoutPDFDocumentPages(doc, face)
	if err != nil {
		t.Fatalf("layoutPDFDocumentPages() error = %v", err)
	}
	if len(pages) < 2 {
		t.Fatalf("pages = %d, want continuation pages for long footnote", len(pages))
	}
	if !printedFootnotes.Enabled || printedFootnotes.PlanCount != 1 || printedFootnotes.ReserveCount == 0 {
		t.Fatalf("printed footnote summary = %#v, want enabled plan and reserve summaries", printedFootnotes)
	}
	if len(printedFootnotes.Plans) != 1 || len(printedFootnotes.Plans[0].Refs) != 1 || len(printedFootnotes.Plans[0].Queue) != 1 || printedFootnotes.Plans[0].ContinuationPages == 0 {
		t.Fatalf("printed footnote plans = %#v, want refs, queue, and continuation count", printedFootnotes.Plans)
	}
	if len(printedFootnotes.PackedContinuationChunks) == 0 || printedFootnotes.ContinuationPageCount == 0 {
		t.Fatalf("printed footnote continuation summary = %#v, want packed continuation chunks", printedFootnotes)
	}
	if printedFootnotes.SkippedCount != len(printedFootnotes.Skipped) || printedFootnotes.OverflowCount != len(printedFootnotes.Overflow) {
		t.Fatalf("printed footnote case counts = skipped %d/%d overflow %d/%d", printedFootnotes.SkippedCount, len(printedFootnotes.Skipped), printedFootnotes.OverflowCount, len(printedFootnotes.Overflow))
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
