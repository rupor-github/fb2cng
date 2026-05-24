package pdf

import (
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"

	"fbc/convert/pdf/docwriter"
)

func TestSubsetTrueTypeFontKeepsUsedGlyphsAndParses(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeText(face, "Subset")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	subset, ok, err := subsetTrueTypeFont(face.Data, shaped.Used)
	if err != nil {
		t.Fatalf("subsetTrueTypeFont() error = %v", err)
	}
	if !ok {
		t.Fatal("subsetTrueTypeFont() ok = false, want true for bundled TTF")
	}
	if len(subset) >= len(face.Data) {
		t.Fatalf("subset size = %d, original size = %d, want smaller subset", len(subset), len(face.Data))
	}
	if _, err := sfnt.Parse(subset); err != nil {
		t.Fatalf("sfnt.Parse(subset) error = %v", err)
	}
}

func TestFontResourceObjectsEmbedsSubsetFontFile(t *testing.T) {
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
	if len(objects.FontFileData) >= len(face.Data) {
		t.Fatalf("font file size = %d, original size = %d, want subset embedded", len(objects.FontFileData), len(face.Data))
	}
	if got := int(objects.FontFile["Length1"].(docwriter.Integer)); got != len(objects.FontFileData) {
		t.Fatalf("Length1 = %d, font data len = %d", got, len(objects.FontFileData))
	}
	baseFont, ok := objects.Type0Font["BaseFont"].(docwriter.Name)
	if !ok || !strings.Contains(string(baseFont), "+"+face.PostScriptName) {
		t.Fatalf("BaseFont = %#v, want subset prefix plus PostScript name", objects.Type0Font["BaseFont"])
	}
	descriptorFont, ok := objects.FontDescriptor["FontName"].(docwriter.Name)
	if !ok || descriptorFont != baseFont {
		t.Fatalf("FontDescriptor FontName = %#v, want matching BaseFont %q", objects.FontDescriptor["FontName"], baseFont)
	}
}
