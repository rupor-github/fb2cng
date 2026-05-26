package pdf

import (
	"slices"
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
	shaped, err := shapeTextWithCache(nil, face, "Subset")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	subset, ok, err := subsetTrueTypeFont(face.Data, shaped.Used)
	if err != nil {
		t.Fatalf("subsetTrueTypeFont() error = %v", err)
	}
	if !ok {
		t.Fatal("subsetTrueTypeFont() ok = false, want true for bundled TTF")
	}
	if len(subset.Data) >= len(face.Data) {
		t.Fatalf("subset size = %d, original size = %d, want smaller subset", len(subset.Data), len(face.Data))
	}
	if _, ok := subset.GlyphMap[0]; !ok {
		t.Fatal("subset glyph map does not include .notdef")
	}
	for glyphID := range shaped.Used {
		if mapped, ok := subset.GlyphMap[glyphID]; !ok {
			t.Fatalf("used glyph %d missing from subset glyph map", glyphID)
		} else if mapped >= uint16(len(subset.GlyphMap)) {
			t.Fatalf("used glyph %d maps to out-of-range subset glyph %d", glyphID, mapped)
		}
	}
	if _, err := sfnt.Parse(subset.Data); err != nil {
		t.Fatalf("sfnt.Parse(subset) error = %v", err)
	}
}

func TestSubsetTrueTypeFontBuildsCompactGlyphProgram(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeTextWithCache(nil, face, "Tiny")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	subset, ok, err := subsetTrueTypeFont(face.Data, shaped.Used)
	if err != nil {
		t.Fatalf("subsetTrueTypeFont() error = %v", err)
	}
	if !ok {
		t.Fatal("subsetTrueTypeFont() ok = false, want true")
	}
	tables, err := parseTTFTables(subset.Data)
	if err != nil {
		t.Fatalf("parseTTFTables(subset) error = %v", err)
	}
	maxp := tables.Records["maxp"].Data
	if len(maxp) < 6 {
		t.Fatalf("subset maxp too short")
	}
	if got, want := int(maxp[4])<<8|int(maxp[5]), len(subset.GlyphMap); got != want {
		t.Fatalf("subset maxp numGlyphs = %d, want compact glyph count %d", got, want)
	}
	head := tables.Records["head"].Data
	locFormat := int(head[50])<<8 | int(head[51])
	loca := tables.Records["loca"].Data
	entrySize := 4
	if locFormat == 0 {
		entrySize = 2
	}
	if got, want := len(loca), (len(subset.GlyphMap)+1)*entrySize; got != want {
		t.Fatalf("subset loca length = %d, want %d", got, want)
	}
	post := tables.Records["post"].Data
	if len(post) != 32 {
		t.Fatalf("subset post length = %d, want compact format 3.0 length 32", len(post))
	}
	if got := uint32(post[0])<<24 | uint32(post[1])<<16 | uint32(post[2])<<8 | uint32(post[3]); got != 0x00030000 {
		t.Fatalf("subset post format = 0x%08X, want 0x00030000", got)
	}
	for _, tag := range []string{"GDEF", "GPOS", "GSUB", "DSIG"} {
		if _, ok := tables.Records[tag]; ok {
			t.Fatalf("subset retained unused layout/signature table %q", tag)
		}
	}
}

func TestSubsetTrueTypeFontIncludesAndRemapsCompositeGlyphComponents(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeTextWithCache(nil, face, "é")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	if len(shaped.Glyphs) != 1 {
		t.Fatalf("shaped glyphs = %#v, want one precomposed glyph", shaped.Glyphs)
	}
	originalCompositeGID := shaped.Glyphs[0].GlyphID
	originalTables, originalLoca := parseTTFSubsetTablesAndLoca(t, face.Data)
	originalDeps, err := compositeGlyphDependencies(originalTables.Records["glyf"].Data, originalLoca, int(originalCompositeGID))
	if err != nil {
		t.Fatalf("compositeGlyphDependencies(original) error = %v", err)
	}
	if len(originalDeps) == 0 {
		t.Fatalf("glyph %d has no composite dependencies; choose a composite regression sample", originalCompositeGID)
	}

	subset, ok, err := subsetTrueTypeFont(face.Data, shaped.Used)
	if err != nil {
		t.Fatalf("subsetTrueTypeFont() error = %v", err)
	}
	if !ok {
		t.Fatal("subsetTrueTypeFont() ok = false, want true")
	}
	if _, err := sfnt.Parse(subset.Data); err != nil {
		t.Fatalf("sfnt.Parse(subset) error = %v", err)
	}

	mappedCompositeGID, ok := subset.GlyphMap[originalCompositeGID]
	if !ok {
		t.Fatalf("composite glyph %d missing from subset glyph map %#v", originalCompositeGID, subset.GlyphMap)
	}
	wantMappedDeps := make([]int, 0, len(originalDeps))
	for _, originalDep := range originalDeps {
		mappedDep, ok := subset.GlyphMap[uint16(originalDep)]
		if !ok {
			t.Fatalf("composite dependency %d missing from subset glyph map %#v", originalDep, subset.GlyphMap)
		}
		wantMappedDeps = append(wantMappedDeps, int(mappedDep))
	}
	if slices.Equal(wantMappedDeps, originalDeps) {
		t.Fatalf("mapped dependencies = %v, original dependencies = %v, want remapped component IDs", wantMappedDeps, originalDeps)
	}

	subsetTables, subsetLoca := parseTTFSubsetTablesAndLoca(t, subset.Data)
	gotMappedDeps, err := compositeGlyphDependencies(subsetTables.Records["glyf"].Data, subsetLoca, int(mappedCompositeGID))
	if err != nil {
		t.Fatalf("compositeGlyphDependencies(subset) error = %v", err)
	}
	if !slices.Equal(gotMappedDeps, wantMappedDeps) {
		t.Fatalf("subset composite deps = %v, want remapped deps %v from original deps %v", gotMappedDeps, wantMappedDeps, originalDeps)
	}
}

func parseTTFSubsetTablesAndLoca(t *testing.T, data []byte) (ttfSubsetTables, []uint32) {
	t.Helper()
	tables, err := parseTTFTables(data)
	if err != nil {
		t.Fatalf("parseTTFTables() error = %v", err)
	}
	head := tables.Records["head"].Data
	maxp := tables.Records["maxp"].Data
	if len(head) < 52 || len(maxp) < 6 {
		t.Fatalf("head/maxp too short")
	}
	locFormat := int16(uint16(head[50])<<8 | uint16(head[51]))
	numGlyphs := int(uint16(maxp[4])<<8 | uint16(maxp[5]))
	loca, err := parseLocaOffsets(tables.Records["loca"].Data, numGlyphs, locFormat)
	if err != nil {
		t.Fatalf("parseLocaOffsets() error = %v", err)
	}
	return tables, loca
}

func TestFontResourceObjectsEmbedsSubsetFontFile(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeTextWithCache(nil, face, "Tiny")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
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
	if len(objects.CIDMap) == 0 {
		t.Fatal("font resource objects did not expose subset CID map")
	}
	for glyphID := range shaped.Used {
		if _, ok := objects.CIDMap[glyphID]; !ok {
			t.Fatalf("used glyph %d missing from font CID map", glyphID)
		}
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
	shaped, err := shapeTextWithCache(nil, face, "Tiny")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
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
