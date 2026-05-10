package pdf

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"fbc/convert/pdf/internal/pdfdoc"
)

type shapedGlyph struct {
	GlyphID uint16
	Rune    rune
	Width   int
}

type shapedText struct {
	Glyphs []shapedGlyph
	Used   map[uint16]shapedGlyph
}

func shapeText(face *builtinFontFace, text string) (shapedText, error) {
	if face == nil || face.Font == nil {
		return shapedText{}, fmt.Errorf("font face is required")
	}

	shaped := shapedText{
		Glyphs: make([]shapedGlyph, 0, len(text)),
		Used:   make(map[uint16]shapedGlyph),
	}
	var buf sfnt.Buffer
	ppem := fixed.I(face.UnitsPerEm)
	for _, r := range text {
		gid, err := face.Font.GlyphIndex(&buf, r)
		if err != nil {
			return shapedText{}, fmt.Errorf("map rune %U to glyph: %w", r, err)
		}
		advance, err := face.Font.GlyphAdvance(&buf, gid, ppem, font.HintingNone)
		if err != nil {
			return shapedText{}, fmt.Errorf("read glyph %d advance: %w", gid, err)
		}
		glyph := shapedGlyph{
			GlyphID: uint16(gid),
			Rune:    r,
			Width:   fontUnitsToPDFWidth(advance.Round(), face.UnitsPerEm),
		}
		shaped.Glyphs = append(shaped.Glyphs, glyph)
		if glyph.GlyphID != 0 {
			shaped.Used[glyph.GlyphID] = glyph
		}
	}
	return shaped, nil
}

func wrapText(face *builtinFontFace, text string, fontSize, maxWidth float64) ([]shapedText, error) {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []shapedText{{Used: make(map[uint16]shapedGlyph)}}, nil
	}

	lines := make([]shapedText, 0, 2)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		shapedCandidate, err := shapeText(face, candidate)
		if err != nil {
			return nil, err
		}
		if line == "" || shapedWidthPoints(shapedCandidate, fontSize) <= maxWidth {
			line = candidate
			continue
		}

		shapedLine, err := shapeText(face, line)
		if err != nil {
			return nil, err
		}
		lines = append(lines, shapedLine)
		line = word
	}
	if line != "" {
		shapedLine, err := shapeText(face, line)
		if err != nil {
			return nil, err
		}
		lines = append(lines, shapedLine)
	}
	return lines, nil
}

func shapedWidthPoints(text shapedText, fontSize float64) float64 {
	width := 0
	for _, glyph := range text.Glyphs {
		width += glyph.Width
	}
	return float64(width) * fontSize / 1000.0
}

func mergeUsedGlyphs(items ...shapedText) map[uint16]shapedGlyph {
	used := make(map[uint16]shapedGlyph)
	for _, item := range items {
		for id, glyph := range item.Used {
			used[id] = glyph
		}
	}
	return used
}

func fontUnitsToPDFWidth(width, unitsPerEm int) int {
	if unitsPerEm <= 0 {
		return width
	}
	return (width*1000 + unitsPerEm/2) / unitsPerEm
}

func glyphHex(glyphs []shapedGlyph) pdfdoc.HexString {
	data := make([]byte, 0, len(glyphs)*2)
	for _, glyph := range glyphs {
		data = append(data, byte(glyph.GlyphID>>8), byte(glyph.GlyphID))
	}
	return pdfdoc.HexString(data)
}

func fontResourceObjects(face *builtinFontFace, used map[uint16]shapedGlyph, objectIDs fontObjectIDs) (fontObjects, error) {
	if face == nil {
		return fontObjects{}, fmt.Errorf("font face is required")
	}
	if len(used) == 0 {
		return fontObjects{}, fmt.Errorf("at least one used glyph is required")
	}

	fontName := pdfdoc.Name(face.PostScriptName)
	return fontObjects{
		Type0Font: pdfdoc.Dict{
			"BaseFont":        fontName,
			"DescendantFonts": pdfdoc.Array{pdfdoc.Ref{ObjectNumber: objectIDs.CIDFont}},
			"Encoding":        pdfdoc.Name("Identity-H"),
			"Subtype":         pdfdoc.Name("Type0"),
			"ToUnicode":       pdfdoc.Ref{ObjectNumber: objectIDs.ToUnicode},
			"Type":            pdfdoc.Name("Font"),
		},
		CIDFont: pdfdoc.Dict{
			"BaseFont":      fontName,
			"CIDSystemInfo": cidSystemInfo("Adobe", "Identity"),
			"CIDToGIDMap":   pdfdoc.Name("Identity"),
			"DW":            pdfdoc.Integer(1000),
			"FontDescriptor": pdfdoc.Ref{
				ObjectNumber: objectIDs.FontDescriptor,
			},
			"Subtype": pdfdoc.Name("CIDFontType2"),
			"Type":    pdfdoc.Name("Font"),
			"W":       widthsArray(used),
		},
		FontDescriptor: pdfdoc.Dict{
			"Ascent":      pdfdoc.Integer(face.Ascent),
			"CapHeight":   pdfdoc.Integer(face.CapHeight),
			"Descent":     pdfdoc.Integer(face.Descent),
			"Flags":       pdfdoc.Integer(face.Flags),
			"FontBBox":    intArray(face.BBox[:]...),
			"FontFile2":   pdfdoc.Ref{ObjectNumber: objectIDs.FontFile},
			"FontName":    fontName,
			"ItalicAngle": pdfdoc.Integer(face.ItalicAngle),
			"StemV":       pdfdoc.Integer(80),
			"Type":        pdfdoc.Name("FontDescriptor"),
		},
		FontFile: pdfdoc.Dict{
			"Length1": pdfdoc.Integer(len(face.Data)),
		},
		FontFileData: face.Data,
		ToUnicode:    toUnicodeCMap(used),
	}, nil
}

type fontObjectIDs struct {
	Type0Font      int
	CIDFont        int
	FontDescriptor int
	FontFile       int
	ToUnicode      int
}

type fontObjects struct {
	Type0Font      pdfdoc.Dict
	CIDFont        pdfdoc.Dict
	FontDescriptor pdfdoc.Dict
	FontFile       pdfdoc.Dict
	FontFileData   []byte
	ToUnicode      []byte
}

func cidSystemInfo(registry, ordering string) pdfdoc.Dict {
	return pdfdoc.Dict{
		"Ordering":   pdfdoc.HexString([]byte(ordering)),
		"Registry":   pdfdoc.HexString([]byte(registry)),
		"Supplement": pdfdoc.Integer(0),
	}
}

func widthsArray(used map[uint16]shapedGlyph) pdfdoc.Array {
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, int(id))
	}
	slices.Sort(ids)

	items := make(pdfdoc.Array, 0, len(ids)*2)
	for _, id := range ids {
		glyph := used[uint16(id)]
		items = append(items, pdfdoc.Integer(id), pdfdoc.Array{pdfdoc.Integer(glyph.Width)})
	}
	return items
}

func intArray(values ...int) pdfdoc.Array {
	items := make(pdfdoc.Array, 0, len(values))
	for _, value := range values {
		items = append(items, pdfdoc.Integer(value))
	}
	return items
}

func toUnicodeCMap(used map[uint16]shapedGlyph) []byte {
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, int(id))
	}
	slices.Sort(ids)

	var buf bytes.Buffer
	buf.WriteString("/CIDInit /ProcSet findresource begin\n")
	buf.WriteString("12 dict begin\n")
	buf.WriteString("begincmap\n")
	buf.WriteString("/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	buf.WriteString("/CMapName /FBCToUnicode def\n")
	buf.WriteString("/CMapType 2 def\n")
	buf.WriteString("1 begincodespacerange\n")
	buf.WriteString("<0000> <FFFF>\n")
	buf.WriteString("endcodespacerange\n")
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		fmt.Fprintf(&buf, "%d beginbfchar\n", end-start)
		for _, id := range ids[start:end] {
			glyph := used[uint16(id)]
			fmt.Fprintf(&buf, "<%04X> <%s>\n", id, utf16BEHex(glyph.Rune))
		}
		buf.WriteString("endbfchar\n")
	}
	buf.WriteString("endcmap\n")
	buf.WriteString("CMapName currentdict /CMap defineresource pop\n")
	buf.WriteString("end\n")
	buf.WriteString("end\n")
	return buf.Bytes()
}

func utf16BEHex(r rune) string {
	words := utf16.Encode([]rune{r})
	var b strings.Builder
	for _, word := range words {
		fmt.Fprintf(&b, "%04X", word)
	}
	return b.String()
}
