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

func TestFontResourceObjectsEmbedsOpenTypeCFFFontFile(t *testing.T) {
	fontData := fakeOpenTypeFontWithTable("CFF ")
	face := &builtinFontFace{
		PostScriptName: "CFFFont-Regular",
		Data:           fontData,
		Ascent:         900,
		Descent:        -200,
		CapHeight:      700,
		BBox:           [4]int{-50, -200, 1000, 900},
		Flags:          1 << 5,
	}
	objects, err := fontResourceObjects(face, map[uint16]shapedGlyph{
		1: {GlyphID: 1, Source: "A", Width: 600, Advance: 600, HasAdvance: true},
	}, fontObjectIDs{
		Type0Font:      1,
		CIDFont:        2,
		FontDescriptor: 3,
		FontFile:       4,
		ToUnicode:      5,
	})
	if err != nil {
		t.Fatalf("fontResourceObjects() error = %v", err)
	}
	if got := objects.CIDFont["Subtype"]; got != docwriter.Name("CIDFontType0") {
		t.Fatalf("CIDFont Subtype = %#v, want CIDFontType0", got)
	}
	if _, ok := objects.CIDFont["CIDToGIDMap"]; ok {
		t.Fatalf("CIDFont has CIDToGIDMap for CFF font: %#v", objects.CIDFont)
	}
	if _, ok := objects.FontDescriptor["FontFile2"]; ok {
		t.Fatalf("FontDescriptor has FontFile2 for CFF font: %#v", objects.FontDescriptor)
	}
	if got := objects.FontDescriptor["FontFile3"]; got != (docwriter.Ref{ObjectNumber: 4}) {
		t.Fatalf("FontDescriptor FontFile3 = %#v, want ref to font file", got)
	}
	if got := objects.FontFile["Subtype"]; got != docwriter.Name("OpenType") {
		t.Fatalf("FontFile Subtype = %#v, want OpenType", got)
	}
	if _, ok := objects.FontFile["Length1"]; ok {
		t.Fatalf("FontFile has Length1 for FontFile3 stream: %#v", objects.FontFile)
	}
	if string(objects.Type0Font["BaseFont"].(docwriter.Name)) != face.PostScriptName {
		t.Fatalf("BaseFont = %#v, want unprefixed full-font name", objects.Type0Font["BaseFont"])
	}
	if len(objects.FontFileData) != len(fontData) {
		t.Fatalf("FontFileData len = %d, want %d", len(objects.FontFileData), len(fontData))
	}
}

func fakeOpenTypeFontWithTable(tag string) []byte {
	data := make([]byte, 12+16+4)
	copy(data[:4], "OTTO")
	data[5] = 1
	copy(data[12:16], tag)
	data[12+11] = byte(12 + 16)
	data[12+15] = 4
	copy(data[12+16:], []byte{1, 2, 3, 4})
	return data
}

func TestFontResourceObjectsHonorsNoSubsettingFlag(t *testing.T) {
	fontData, err := gunzipFont(notoSerifRegularGZ)
	if err != nil {
		t.Fatalf("gunzipFont() error = %v", err)
	}
	fontData = append([]byte(nil), fontData...)
	if !patchFontEmbeddingFSType(fontData, 0x0100) {
		t.Fatal("patchFontEmbeddingFSType() = false")
	}
	face, err := loadRawFont("NoSubsetSerif", fontData, false, false)
	if err != nil {
		t.Fatalf("loadRawFont() error = %v", err)
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
	if len(objects.FontFileData) != len(face.Data) {
		t.Fatalf("font file size = %d, original size = %d, want full font for no-subsetting flag", len(objects.FontFileData), len(face.Data))
	}
	baseFont, ok := objects.Type0Font["BaseFont"].(docwriter.Name)
	if !ok || strings.Contains(string(baseFont), "+") {
		t.Fatalf("BaseFont = %#v, want unprefixed full-font name", objects.Type0Font["BaseFont"])
	}
}
