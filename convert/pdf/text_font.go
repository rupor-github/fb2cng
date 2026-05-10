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

	"fbc/convert/pdf/docwriter"
)

type pdfFontKey struct {
	Family string
	Bold   bool
	Italic bool
}

type pdfFontResource struct {
	Key     pdfFontKey
	Name    string
	Face    *builtinFontFace
	Used    map[uint16]shapedGlyph
	Objects fontObjects
	IDs     fontObjectIDs
}

type shapedGlyph struct {
	GlyphID uint16
	Rune    rune
	Width   int
}

type shapedText struct {
	Glyphs []shapedGlyph
	Used   map[uint16]shapedGlyph
}

func normalizedPDFFontFamily(family string) string {
	family = strings.TrimSpace(family)
	if family == "" {
		return "serif"
	}
	return family
}

func pdfFontKeyForStyle(style paragraphStyle) pdfFontKey {
	return pdfFontKey{Family: normalizedPDFFontFamily(style.FontFamily), Bold: style.Bold, Italic: style.Italic}
}

func fontForStyle(registry *pdfFontRegistry, style paragraphStyle) (*builtinFontFace, pdfFontKey, error) {
	key := pdfFontKeyForStyle(style)
	face, err := fontForKey(registry, key)
	if err != nil {
		return nil, pdfFontKey{}, err
	}
	return face, key, nil
}

func fontForKey(registry *pdfFontRegistry, key pdfFontKey) (*builtinFontFace, error) {
	if registry != nil {
		return registry.fontForKey(key)
	}
	return builtinFont(key.Family, key.Bold, key.Italic)
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
	return shapedWidthPointsWithSpacing(text, fontSize, 0)
}

func shapedWidthPointsWithSpacing(text shapedText, fontSize float64, letterSpacing float64) float64 {
	width := 0
	for _, glyph := range text.Glyphs {
		width += glyph.Width
	}
	points := float64(width) * fontSize / 1000.0
	if letterSpacing != 0 && len(text.Glyphs) > 1 {
		points += letterSpacing * float64(len(text.Glyphs)-1)
	}
	return points
}

func fontUnitsToPDFWidth(width, unitsPerEm int) int {
	if unitsPerEm <= 0 {
		return width
	}
	return (width*1000 + unitsPerEm/2) / unitsPerEm
}

func glyphHex(glyphs []shapedGlyph) docwriter.HexString {
	data := make([]byte, 0, len(glyphs)*2)
	for _, glyph := range glyphs {
		data = append(data, byte(glyph.GlyphID>>8), byte(glyph.GlyphID))
	}
	return docwriter.HexString(data)
}

func preparePDFFontResources(registry *pdfFontRegistry, used map[pdfFontKey]map[uint16]shapedGlyph, nextObjectID *int) ([]pdfFontResource, error) {
	keys := make([]pdfFontKey, 0, len(used))
	for key, glyphs := range used {
		if len(glyphs) == 0 {
			continue
		}
		keys = append(keys, key)
	}
	slices.SortFunc(keys, comparePDFFontKeys)

	resources := make([]pdfFontResource, 0, len(keys))
	for i, key := range keys {
		face, err := fontForKey(registry, key)
		if err != nil {
			return nil, err
		}
		ids := fontObjectIDs{
			Type0Font:      *nextObjectID,
			CIDFont:        *nextObjectID + 1,
			FontDescriptor: *nextObjectID + 2,
			FontFile:       *nextObjectID + 3,
			ToUnicode:      *nextObjectID + 4,
		}
		*nextObjectID += 5
		objects, err := fontResourceObjects(face, used[key], ids)
		if err != nil {
			return nil, err
		}
		resources = append(resources, pdfFontResource{
			Key:     key,
			Name:    fmt.Sprintf("F%d", i+1),
			Face:    face,
			Used:    used[key],
			IDs:     ids,
			Objects: objects,
		})
	}
	return resources, nil
}

func comparePDFFontKeys(a, b pdfFontKey) int {
	if c := strings.Compare(a.Family, b.Family); c != 0 {
		return c
	}
	if a.Bold != b.Bold {
		if !a.Bold {
			return -1
		}
		return 1
	}
	if a.Italic != b.Italic {
		if !a.Italic {
			return -1
		}
		return 1
	}
	return 0
}

func assignPDFFontResourceNames(pages []pdfPage, resources []pdfFontResource) {
	names := make(map[pdfFontKey]string, len(resources))
	for _, resource := range resources {
		names[resource.Key] = resource.Name
	}
	for pageIndex := range pages {
		for lineIndex := range pages[pageIndex].Lines {
			pages[pageIndex].Lines[lineIndex].FontName = names[pages[pageIndex].Lines[lineIndex].FontKey]
			for fragmentIndex := range pages[pageIndex].Lines[lineIndex].Fragments {
				fragment := &pages[pageIndex].Lines[lineIndex].Fragments[fragmentIndex]
				fragment.FontName = names[fragment.FontKey]
			}
		}
	}
}

func pageFontResources(resources []pdfFontResource) docwriter.Dict {
	fonts := docwriter.Dict{}
	for _, resource := range resources {
		if resource.Name == "" {
			continue
		}
		fonts[resource.Name] = docwriter.Ref{ObjectNumber: resource.IDs.Type0Font}
	}
	return fonts
}

func writePDFFontObjects(writer *docwriter.Writer, resources []pdfFontResource) error {
	for _, resource := range resources {
		if err := writer.Object(resource.IDs.Type0Font, resource.Objects.Type0Font); err != nil {
			return err
		}
		if err := writer.Object(resource.IDs.CIDFont, resource.Objects.CIDFont); err != nil {
			return err
		}
		if err := writer.Object(resource.IDs.FontDescriptor, resource.Objects.FontDescriptor); err != nil {
			return err
		}
		if err := writer.StreamObject(resource.IDs.FontFile, resource.Objects.FontFile, resource.Objects.FontFileData); err != nil {
			return err
		}
		if err := writer.StreamObject(resource.IDs.ToUnicode, docwriter.Dict{}, resource.Objects.ToUnicode); err != nil {
			return err
		}
	}
	return nil
}

func fontResourceObjects(face *builtinFontFace, used map[uint16]shapedGlyph, objectIDs fontObjectIDs) (fontObjects, error) {
	if face == nil {
		return fontObjects{}, fmt.Errorf("font face is required")
	}
	if len(used) == 0 {
		return fontObjects{}, fmt.Errorf("at least one used glyph is required")
	}

	fontName := docwriter.Name(face.PostScriptName)
	return fontObjects{
		Type0Font: docwriter.Dict{
			"BaseFont":        fontName,
			"DescendantFonts": docwriter.Array{docwriter.Ref{ObjectNumber: objectIDs.CIDFont}},
			"Encoding":        docwriter.Name("Identity-H"),
			"Subtype":         docwriter.Name("Type0"),
			"ToUnicode":       docwriter.Ref{ObjectNumber: objectIDs.ToUnicode},
			"Type":            docwriter.Name("Font"),
		},
		CIDFont: docwriter.Dict{
			"BaseFont":      fontName,
			"CIDSystemInfo": cidSystemInfo("Adobe", "Identity"),
			"CIDToGIDMap":   docwriter.Name("Identity"),
			"DW":            docwriter.Integer(1000),
			"FontDescriptor": docwriter.Ref{
				ObjectNumber: objectIDs.FontDescriptor,
			},
			"Subtype": docwriter.Name("CIDFontType2"),
			"Type":    docwriter.Name("Font"),
			"W":       widthsArray(used),
		},
		FontDescriptor: docwriter.Dict{
			"Ascent":      docwriter.Integer(face.Ascent),
			"CapHeight":   docwriter.Integer(face.CapHeight),
			"Descent":     docwriter.Integer(face.Descent),
			"Flags":       docwriter.Integer(face.Flags),
			"FontBBox":    intArray(face.BBox[:]...),
			"FontFile2":   docwriter.Ref{ObjectNumber: objectIDs.FontFile},
			"FontName":    fontName,
			"ItalicAngle": docwriter.Integer(face.ItalicAngle),
			"StemV":       docwriter.Integer(80),
			"Type":        docwriter.Name("FontDescriptor"),
		},
		FontFile: docwriter.Dict{
			"Length1": docwriter.Integer(len(face.Data)),
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
	Type0Font      docwriter.Dict
	CIDFont        docwriter.Dict
	FontDescriptor docwriter.Dict
	FontFile       docwriter.Dict
	FontFileData   []byte
	ToUnicode      []byte
}

func cidSystemInfo(registry, ordering string) docwriter.Dict {
	return docwriter.Dict{
		"Ordering":   docwriter.HexString([]byte(ordering)),
		"Registry":   docwriter.HexString([]byte(registry)),
		"Supplement": docwriter.Integer(0),
	}
}

func widthsArray(used map[uint16]shapedGlyph) docwriter.Array {
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, int(id))
	}
	slices.Sort(ids)

	items := make(docwriter.Array, 0, len(ids)*2)
	for _, id := range ids {
		glyph := used[uint16(id)]
		items = append(items, docwriter.Integer(id), docwriter.Array{docwriter.Integer(glyph.Width)})
	}
	return items
}

func intArray(values ...int) docwriter.Array {
	items := make(docwriter.Array, 0, len(values))
	for _, value := range values {
		items = append(items, docwriter.Integer(value))
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
