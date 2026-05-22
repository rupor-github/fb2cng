package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"fbc/convert/pdf/docwriter"
	"fbc/fb2"
)

func TestShapeTextAndFontResourceObjects(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeText(face, "Test Ж")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	if len(shaped.Glyphs) != len([]rune("Test Ж")) {
		t.Fatalf("glyph count = %d, want %d", len(shaped.Glyphs), len([]rune("Test Ж")))
	}
	for _, glyph := range shaped.Glyphs {
		if glyph.GlyphID == 0 {
			t.Fatalf("rune %U mapped to .notdef", glyph.Rune)
		}
		if glyph.Width <= 0 {
			t.Fatalf("glyph %d width = %d, want positive", glyph.GlyphID, glyph.Width)
		}
	}

	objects, err := fontResourceObjects(face, shaped.Used, fontObjectIDs{
		Type0Font:      6,
		CIDFont:        7,
		FontDescriptor: 8,
		FontFile:       9,
		ToUnicode:      10,
	})
	if err != nil {
		t.Fatalf("fontResourceObjects() error = %v", err)
	}
	if len(objects.FontFileData) == 0 {
		t.Error("FontFileData is empty")
	}
	for _, want := range []string{
		"/Subtype /Type0",
		"/Encoding /Identity-H",
		"/DescendantFonts [7 0 R]",
		"/ToUnicode 10 0 R",
	} {
		if got := docwriter.Format(objects.Type0Font); !strings.Contains(got, want) {
			t.Errorf("Type0 font dictionary %q does not contain %q", got, want)
		}
	}
	for _, want := range []string{
		"/Subtype /CIDFontType2",
		"/CIDToGIDMap /Identity",
		"/FontDescriptor 8 0 R",
		"/W [",
	} {
		if got := docwriter.Format(objects.CIDFont); !strings.Contains(got, want) {
			t.Errorf("CID font dictionary %q does not contain %q", got, want)
		}
	}
	if !bytes.Contains(objects.ToUnicode, []byte("begincmap")) {
		t.Error("ToUnicode CMap does not contain begincmap")
	}
	if !bytes.Contains(objects.ToUnicode, []byte("0416")) {
		t.Error("ToUnicode CMap does not contain Cyrillic Ж mapping")
	}
}

func TestShapeTextUsesBuiltInSymbolFallbackForGenericFonts(t *testing.T) {
	registry := newPDFFontRegistry(nil, nil)
	face, key, err := fontForStyle(registry, paragraphStyle{FontFamily: "serif"})
	if err != nil {
		t.Fatalf("fontForStyle() error = %v", err)
	}
	shaped, err := shapeText(face, "≤→●A")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	if len(shaped.Glyphs) != 4 {
		t.Fatalf("glyph count = %d, want 4", len(shaped.Glyphs))
	}
	wants := []pdfFontKey{
		{Family: pdfBuiltinFontFamilyMath},
		{Family: pdfBuiltinFontFamilyMath},
		{Family: pdfBuiltinFontFamilySymbols2},
		key,
	}
	for i, want := range wants {
		glyph := shaped.Glyphs[i]
		if glyph.GlyphID == 0 || glyph.Missing != pdfMissingGlyphNone {
			t.Fatalf("glyph %d = %#v, want real glyph", i, glyph)
		}
		if glyph.FontKey != want {
			t.Fatalf("glyph %d font key = %#v, want %#v", i, glyph.FontKey, want)
		}
	}
}

func TestShapeTextDoesNotUseSymbolFallbackForCustomFonts(t *testing.T) {
	fontData, err := gunzipFont(notoSerifRegularGZ)
	if err != nil {
		t.Fatalf("gunzipFont() error = %v", err)
	}
	face, err := loadRawFont("CustomSerif", fontData, false, false)
	if err != nil {
		t.Fatalf("loadRawFont() error = %v", err)
	}
	shaped, err := shapeText(face, "≤")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	if len(shaped.Glyphs) != 1 {
		t.Fatalf("glyph count = %d, want 1", len(shaped.Glyphs))
	}
	if shaped.Glyphs[0].Missing != pdfMissingGlyphPrintable || shaped.Glyphs[0].GlyphID != 0 {
		t.Fatalf("glyph = %#v, want synthetic missing glyph", shaped.Glyphs[0])
	}
}

func TestPDFPageLineWithFontFragmentsSplitsBuiltInSymbolFallback(t *testing.T) {
	registry := newPDFFontRegistry(nil, nil)
	face, key, err := fontForStyle(registry, paragraphStyle{FontFamily: "serif"})
	if err != nil {
		t.Fatalf("fontForStyle() error = %v", err)
	}
	shaped, err := shapeText(face, "A≤→●B")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	line := pdfPageLine{Text: shaped, FontKey: key, FontSize: 10, LetterSpacing: 1}
	line = pdfPageLineWithFontFragments(line)
	if len(line.Fragments) != 4 {
		t.Fatalf("fragment count = %d, want 4: %#v", len(line.Fragments), line.Fragments)
	}
	wantKeys := []pdfFontKey{key, {Family: pdfBuiltinFontFamilyMath}, {Family: pdfBuiltinFontFamilySymbols2}, key}
	for i, want := range wantKeys {
		if line.Fragments[i].FontKey != want {
			t.Fatalf("fragment %d font key = %#v, want %#v", i, line.Fragments[i].FontKey, want)
		}
	}
	if got := string([]rune{line.Fragments[1].Text.Glyphs[0].Rune, line.Fragments[1].Text.Glyphs[1].Rune}); got != "≤→" {
		t.Fatalf("math fragment text = %q, want ≤→", got)
	}
	width := 0.0
	for _, fragment := range line.Fragments {
		width += fragment.Width
	}
	if width != shapedWidthPointsWithSpacing(shaped, line.FontSize, line.LetterSpacing) {
		t.Fatalf("fragment width sum = %v, want original width %v", width, shapedWidthPointsWithSpacing(shaped, line.FontSize, line.LetterSpacing))
	}
}

func TestShapeTextClassifiesAndLogsMissingGlyphs(t *testing.T) {
	fontData, err := os.ReadFile(filepath.Join("..", "..", "build", "fonts_compression", "bookerly-regular_9_5.ttf"))
	if err != nil {
		t.Fatalf("read test font: %v", err)
	}
	face, err := loadRawFont("BookerlySubset", fontData, false, false)
	if err != nil {
		t.Fatalf("loadRawFont() error = %v", err)
	}
	core, logs := observer.New(zapcore.DebugLevel)
	seen := make(map[pdfMissingGlyphLogKey]bool)
	var mu sync.Mutex
	face = pdfFontFaceWithLogger(
		face,
		zap.New(core),
		pdfFontKey{Family: "Bookerly"},
		seen,
		&mu,
	)

	shaped, err := shapeText(face, "A ́á ́á")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	if len(shaped.Glyphs) != 7 {
		t.Fatalf("glyph count = %d, want 7", len(shaped.Glyphs))
	}
	if shaped.Glyphs[1].Missing != pdfMissingGlyphSpace || shaped.Glyphs[1].Width <= 0 {
		t.Fatalf("space glyph = %#v, want missing space with positive width", shaped.Glyphs[1])
	}
	if shaped.Glyphs[2].Missing != pdfMissingGlyphCombining || shaped.Glyphs[2].Width != 0 {
		t.Fatalf("combining glyph = %#v, want zero-width missing combining mark", shaped.Glyphs[2])
	}
	if shaped.Glyphs[3].Missing != pdfMissingGlyphPrintable || shaped.Glyphs[3].Width <= 0 {
		t.Fatalf("printable glyph = %#v, want missing printable with positive width", shaped.Glyphs[3])
	}
	if _, ok := shaped.Used[0]; ok {
		t.Fatalf("used glyph map contains CID 0: %#v", shaped.Used)
	}

	if _, err := shapeText(face, " ́á"); err != nil {
		t.Fatalf("repeat shapeText() error = %v", err)
	}
	entries := logs.FilterMessage("Using synthetic PDF missing glyph").All()
	if len(entries) != 3 {
		t.Fatalf("missing glyph log entries = %d, want deduplicated 3", len(entries))
	}
	kinds := map[string]bool{}
	for _, entry := range entries {
		for _, field := range entry.Context {
			if field.Key == "kind" && field.Type == zapcore.StringType {
				kinds[field.String] = true
			}
		}
	}
	for _, want := range []string{"space", "combining-mark", "printable"} {
		if !kinds[want] {
			t.Fatalf("missing log kind %q in %#v", want, kinds)
		}
	}
}

func TestPDFFontRegistryLoadsStylesheetFontFace(t *testing.T) {
	fontData, err := gunzipFont(notoMonoRegularGZ)
	if err != nil {
		t.Fatalf("gunzipFont() error = %v", err)
	}
	book := &fb2.FictionBook{Stylesheets: []fb2.Stylesheet{{
		Type: "text/css",
		Data: `
			@font-face { font-family: CustomMono; src: url("#custom-mono"); font-weight: 400; font-style: normal; }
			p.custom { font-family: CustomMono; }
		`,
		Resources: []fb2.StylesheetResource{{OriginalURL: "#custom-mono", MimeType: "font/ttf", Data: fontData}},
	}}}

	registry := newPDFFontRegistry(book, nil)
	face, key, err := fontForStyle(registry, paragraphStyle{FontFamily: "CustomMono"})
	if err != nil {
		t.Fatalf("fontForStyle() error = %v", err)
	}
	if key.Family != "CustomMono" {
		t.Fatalf("font key family = %q, want CustomMono", key.Family)
	}
	if face == nil || face.PostScriptName != "NotoSansMono-Regular" {
		t.Fatalf("font face = %#v, want embedded NotoSansMono-Regular", face)
	}
}

func TestPreparePDFFontResources(t *testing.T) {
	sans, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont(sans-serif) error = %v", err)
	}
	serifBold, err := builtinFont("serif", true, false)
	if err != nil {
		t.Fatalf("builtinFont(serif bold) error = %v", err)
	}
	sansText, err := shapeText(sans, "Sans")
	if err != nil {
		t.Fatalf("shapeText(sans) error = %v", err)
	}
	serifText, err := shapeText(serifBold, "Serif")
	if err != nil {
		t.Fatalf("shapeText(serif) error = %v", err)
	}
	nextObjectID := 20
	resources, err := preparePDFFontResources(nil, map[pdfFontKey]map[uint16]shapedGlyph{
		{Family: "serif", Bold: true}: serifText.Used,
		{Family: "sans-serif"}:        sansText.Used,
	}, &nextObjectID)
	if err != nil {
		t.Fatalf("preparePDFFontResources() error = %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("font resources = %d, want 2", len(resources))
	}
	if resources[0].Name != "F1" || resources[1].Name != "F2" || nextObjectID != 30 {
		t.Fatalf("resources = %#v nextObjectID=%d, want F1/F2 and next id 30", resources, nextObjectID)
	}
}

func TestWrapText(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	lines, err := wrapText(face, "one two three four", 10, 55)
	if err != nil {
		t.Fatalf("wrapText() error = %v", err)
	}
	if len(lines) < 2 {
		t.Fatalf("wrapText() produced %d lines, want at least 2", len(lines))
	}
	for _, line := range lines {
		if shapedWidthPoints(line, 10) > 55 {
			t.Errorf("wrapped line width = %v, want <= 55", shapedWidthPoints(line, 10))
		}
	}
}

func TestShapedWidthPointsWithSpacing(t *testing.T) {
	text := shapedText{Glyphs: []shapedGlyph{{Width: 500}, {Width: 250}, {Width: 250}}}
	if got := shapedWidthPointsWithSpacing(text, 10, 1.5); got != 13 {
		t.Fatalf("shapedWidthPointsWithSpacing() = %v, want 13", got)
	}
}

func TestGlyphHex(t *testing.T) {
	got := docwriter.Format(glyphHex([]shapedGlyph{{GlyphID: 1}, {GlyphID: 0x0416}}))
	if got != "<00010416>" {
		t.Errorf("glyphHex() = %q, want %q", got, "<00010416>")
	}
}

func TestUTF16BEHex(t *testing.T) {
	if got := utf16BEHex('Ж'); got != "0416" {
		t.Errorf("utf16BEHex('Ж') = %q, want 0416", got)
	}
	if got := utf16BEHex('😀'); got != "D83DDE00" {
		t.Errorf("utf16BEHex('😀') = %q, want D83DDE00", got)
	}
}
