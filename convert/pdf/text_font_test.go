package pdf

import (
	"bytes"
	"strings"
	"testing"

	"fbc/convert/pdf/docwriter"
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
	resources, err := preparePDFFontResources(map[pdfFontKey]map[uint16]shapedGlyph{
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
