package pdf

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf16"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/shaping"
	"go.uber.org/zap"
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
	CIDMap  map[uint16]uint16
	Objects fontObjects
	IDs     fontObjectIDs
}

type pdfMissingGlyphKind int

const (
	pdfMissingGlyphNone pdfMissingGlyphKind = iota
	pdfMissingGlyphSpace
	pdfMissingGlyphCombining
	pdfMissingGlyphPrintable
)

func (k pdfMissingGlyphKind) String() string {
	switch k {
	case pdfMissingGlyphSpace:
		return "space"
	case pdfMissingGlyphCombining:
		return "combining-mark"
	case pdfMissingGlyphPrintable:
		return "printable"
	default:
		return "none"
	}
}

type shapedGlyph struct {
	GlyphID uint16
	Rune    rune
	Source  string
	// Width is the nominal PDF glyph width used in font dictionaries. Advance is
	// the shaped advance used for layout; it can differ from Width after kerning.
	Width      int
	Advance    int
	HasAdvance bool
	// ClusterStart and ClusterEnd identify the source rune range represented by
	// this glyph. They are equal to one rune for the current simple shaper, and
	// will allow ligatures and multi-rune clusters once OpenType shaping lands.
	ClusterStart int
	ClusterEnd   int
	XOffset      int
	YOffset      int
	InkLeft      int
	InkRight     int
	HasInkBounds bool
	// FontKey overrides the surrounding line font for deliberate built-in companion
	// glyphs. Keeping this per glyph lets resource usage stay exact and keeps the
	// path to future font subsetting straightforward.
	FontKey pdfFontKey
	Missing pdfMissingGlyphKind
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
	face, err := builtinFont(key.Family, key.Bold, key.Italic)
	return pdfFontFaceWithLogger(face, nil, key, nil, nil), err
}

type pdfMissingGlyphLogKey struct {
	FontFamily     string
	Bold           bool
	Italic         bool
	PostScriptName string
	Rune           rune
	Kind           pdfMissingGlyphKind
}

type pdfMissingGlyphLogger struct {
	Log     *zap.Logger
	FontKey pdfFontKey
	Seen    map[pdfMissingGlyphLogKey]bool
	Mu      *sync.Mutex
}

func pdfFontFaceWithLogger(
	face *builtinFontFace,
	log *zap.Logger,
	key pdfFontKey,
	seen map[pdfMissingGlyphLogKey]bool,
	mu *sync.Mutex,
) *builtinFontFace {
	if face == nil {
		return face
	}
	clone := *face
	clone.Key = key
	if log == nil {
		return &clone
	}
	clone.MissingGlyphLog = &pdfMissingGlyphLogger{
		Log:     log,
		FontKey: key,
		Seen:    seen,
		Mu:      mu,
	}
	return &clone
}

type pdfShapeOptions struct{}

type pdfTextShaper interface {
	Shape(text string, opts pdfShapeOptions) (shapedText, error)
}

type simplePDFTextShaper struct {
	face *builtinFontFace
}

type openTypePDFTextShaper struct {
	face   *builtinFontFace
	shaper shaping.HarfbuzzShaper
}

func shapeText(face *builtinFontFace, text string) (shapedText, error) {
	if canShapeOpenTypeText(face, text) {
		var shaper pdfTextShaper = &openTypePDFTextShaper{face: face}
		return shaper.Shape(text, pdfShapeOptions{})
	}
	var shaper pdfTextShaper = simplePDFTextShaper{face: face}
	return shaper.Shape(text, pdfShapeOptions{})
}

func canShapeOpenTypeText(face *builtinFontFace, text string) bool {
	if face == nil || face.TextFace == nil {
		return false
	}
	for _, r := range text {
		if _, ok := face.TextFace.NominalGlyph(r); !ok {
			return false
		}
	}
	return true
}

func (s simplePDFTextShaper) Shape(text string, _ pdfShapeOptions) (shapedText, error) {
	face := s.face
	if face == nil || face.Font == nil {
		return shapedText{}, fmt.Errorf("font face is required")
	}

	runes := []rune(text)
	shaped := shapedText{
		Glyphs: make([]shapedGlyph, 0, len(runes)),
		Used:   make(map[uint16]shapedGlyph),
	}
	var buf sfnt.Buffer
	ppem := fixed.I(face.UnitsPerEm)
	for cluster, r := range runes {
		gid, err := face.Font.GlyphIndex(&buf, r)
		if err != nil {
			return shapedText{}, fmt.Errorf("map rune %U to glyph: %w", r, err)
		}
		advance, err := face.Font.GlyphAdvance(&buf, gid, ppem, font.HintingNone)
		if err != nil {
			return shapedText{}, fmt.Errorf("read glyph %d advance: %w", gid, err)
		}
		width := fontUnitsToPDFWidth(advance.Round(), face.UnitsPerEm)
		glyph := shapedGlyph{
			GlyphID:      uint16(gid),
			Rune:         r,
			Source:       string(r),
			Width:        width,
			Advance:      width,
			HasAdvance:   true,
			ClusterStart: cluster,
			ClusterEnd:   cluster + 1,
			InkLeft:      0,
			InkRight:     pdfSimpleGlyphInkRight(r, width),
			HasInkBounds: true,
			FontKey:      face.Key,
		}
		if glyph.GlyphID == 0 {
			fallbackGlyph, ok, err := pdfBuiltinSymbolFallbackGlyph(face, r)
			if err != nil {
				return shapedText{}, err
			}
			if ok {
				glyph = fallbackGlyph
			} else {
				glyph = missingPDFGlyph(face, r, glyph.Width)
			}
			glyph = pdfGlyphWithSource(glyph, r, cluster, cluster+1)
		}
		shaped.Glyphs = append(shaped.Glyphs, glyph)
		if glyph.GlyphID != 0 {
			shaped.Used[glyph.GlyphID] = glyph
		}
	}
	return shaped, nil
}

func pdfGlyphWithSource(glyph shapedGlyph, r rune, start, end int) shapedGlyph {
	glyph.Rune = r
	glyph.Source = string(r)
	glyph.ClusterStart = start
	glyph.ClusterEnd = end
	if !glyph.HasAdvance {
		glyph.Advance = glyph.Width
		glyph.HasAdvance = true
	}
	if !glyph.HasInkBounds {
		glyph.InkLeft = 0
		glyph.InkRight = pdfSimpleGlyphInkRight(r, shapedGlyphAdvanceWidth(glyph))
		glyph.HasInkBounds = true
	}
	return glyph
}

func shapeOpenTypeText(face *builtinFontFace, text string) (shapedText, error) {
	shaper := openTypePDFTextShaper{face: face}
	return shaper.Shape(text, pdfShapeOptions{})
}

func (s *openTypePDFTextShaper) Shape(text string, _ pdfShapeOptions) (shapedText, error) {
	face := s.face
	if face == nil || face.TextFace == nil {
		return shapedText{}, fmt.Errorf("OpenType font face is required")
	}

	runes := []rune(text)
	out := s.shaper.Shape(shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: di.DirectionLTR,
		Face:      face.TextFace,
		Size:      fixed.I(face.UnitsPerEm),
	})

	shaped := shapedText{
		Glyphs: make([]shapedGlyph, 0, len(out.Glyphs)),
		Used:   make(map[uint16]shapedGlyph),
	}
	seenClusters := make(map[int]bool)
	for _, hbGlyph := range out.Glyphs {
		if hbGlyph.GlyphID > math.MaxUint16 {
			return shapedText{}, fmt.Errorf("glyph id %d exceeds PDF CID width", hbGlyph.GlyphID)
		}
		start, end, source := shapedGlyphSource(runes, hbGlyph.TextIndex(), hbGlyph.RunesCount())
		r := firstRuneOrZero(source)
		if seenClusters[start] {
			source = ""
			r = 0
		} else {
			seenClusters[start] = true
		}
		xOffset := fixedFontUnitsToPDFWidth(hbGlyph.XOffset, face.UnitsPerEm)
		inkLeft := xOffset + fixedFontUnitsToPDFWidth(hbGlyph.XBearing, face.UnitsPerEm)
		inkRight := inkLeft + fixedFontUnitsToPDFWidth(hbGlyph.Width, face.UnitsPerEm)
		if inkRight < inkLeft {
			inkLeft, inkRight = inkRight, inkLeft
		}
		glyph := shapedGlyph{
			GlyphID:      uint16(hbGlyph.GlyphID),
			Rune:         r,
			Source:       source,
			Width:        floatFontUnitsToPDFWidth(face.TextFace.HorizontalAdvance(hbGlyph.GlyphID), face.UnitsPerEm),
			Advance:      fixedFontUnitsToPDFWidth(hbGlyph.Advance, face.UnitsPerEm),
			HasAdvance:   true,
			ClusterStart: start,
			ClusterEnd:   end,
			XOffset:      xOffset,
			YOffset:      fixedFontUnitsToPDFWidth(hbGlyph.YOffset, face.UnitsPerEm),
			InkLeft:      inkLeft,
			InkRight:     inkRight,
			HasInkBounds: true,
			FontKey:      face.Key,
		}
		if glyph.GlyphID == 0 {
			glyph = missingPDFGlyph(face, glyph.Rune, glyph.Width)
			glyph.Source = source
			glyph.ClusterStart = start
			glyph.ClusterEnd = end
		}
		shaped.Glyphs = append(shaped.Glyphs, glyph)
		if glyph.GlyphID != 0 {
			shaped.Used[glyph.GlyphID] = glyph
		}
	}
	return shaped, nil
}

func shapedGlyphSource(runes []rune, start, count int) (int, int, string) {
	start = min(max(start, 0), len(runes))
	if count <= 0 {
		count = 1
	}
	end := min(start+count, len(runes))
	if end < start {
		end = start
	}
	return start, end, string(runes[start:end])
}

func firstRuneOrZero(text string) rune {
	for _, r := range text {
		return r
	}
	return 0
}

func fixedFontUnitsToPDFWidth(value fixed.Int26_6, unitsPerEm int) int {
	return fontUnitsToPDFWidth(value.Round(), unitsPerEm)
}

func floatFontUnitsToPDFWidth(value float32, unitsPerEm int) int {
	return fontUnitsToPDFWidth(int(math.Round(float64(value))), unitsPerEm)
}

func pdfBuiltinSymbolFallbackGlyph(face *builtinFontFace, r rune) (shapedGlyph, bool, error) {
	if !pdfBuiltinSymbolFallbackEnabled(face) || !pdfRuneUsesBuiltinSymbolFallback(r) {
		return shapedGlyph{}, false, nil
	}
	for _, key := range pdfBuiltinSymbolFallbackKeys(r) {
		fallbackFace, err := builtinFont(key.Family, key.Bold, key.Italic)
		if err != nil {
			return shapedGlyph{}, false, err
		}
		glyph, ok, err := pdfFontGlyph(fallbackFace, key, r)
		if err != nil || ok {
			return glyph, ok, err
		}
	}
	return shapedGlyph{}, false, nil
}

func pdfBuiltinSymbolFallbackEnabled(face *builtinFontFace) bool {
	return face != nil && face.Builtin && !pdfFontKeyIsBuiltinSymbolFallback(face.Key)
}

func pdfFontKeyIsBuiltinSymbolFallback(key pdfFontKey) bool {
	switch key.Family {
	case pdfBuiltinFontFamilyMath, pdfBuiltinFontFamilySymbols, pdfBuiltinFontFamilySymbols2:
		return true
	default:
		return false
	}
}

func pdfRuneUsesBuiltinSymbolFallback(r rune) bool {
	for _, rng := range pdfBuiltinSymbolFallbackRanges {
		if r >= rng.lo && r <= rng.hi {
			return true
		}
	}
	return false
}

func pdfBuiltinSymbolFallbackKeys(r rune) []pdfFontKey {
	math := pdfFontKey{Family: pdfBuiltinFontFamilyMath}
	mono := pdfFontKey{Family: "monospace"}
	symbols := pdfFontKey{Family: pdfBuiltinFontFamilySymbols}
	symbols2 := pdfFontKey{Family: pdfBuiltinFontFamilySymbols2}
	if pdfRuneUsesMonoFallbackFirst(r) {
		return []pdfFontKey{mono, symbols2, symbols, math}
	}
	if pdfRuneUsesMathFallbackFirst(r) {
		return []pdfFontKey{math, symbols2, symbols, mono}
	}
	return []pdfFontKey{symbols2, symbols, math, mono}
}

func pdfRuneUsesMonoFallbackFirst(r rune) bool {
	for _, rng := range pdfBuiltinMonoFallbackFirstRanges {
		if r >= rng.lo && r <= rng.hi {
			return true
		}
	}
	return false
}

func pdfRuneUsesMathFallbackFirst(r rune) bool {
	for _, rng := range pdfBuiltinMathFallbackFirstRanges {
		if r >= rng.lo && r <= rng.hi {
			return true
		}
	}
	return false
}

func pdfFontGlyph(face *builtinFontFace, key pdfFontKey, r rune) (shapedGlyph, bool, error) {
	if face == nil || face.Font == nil {
		return shapedGlyph{}, false, fmt.Errorf("font face is required")
	}
	var buf sfnt.Buffer
	gid, err := face.Font.GlyphIndex(&buf, r)
	if err != nil {
		return shapedGlyph{}, false, fmt.Errorf("map rune %U to glyph: %w", r, err)
	}
	if gid == 0 {
		return shapedGlyph{}, false, nil
	}
	advance, err := face.Font.GlyphAdvance(&buf, gid, fixed.I(face.UnitsPerEm), font.HintingNone)
	if err != nil {
		return shapedGlyph{}, false, fmt.Errorf("read glyph %d advance: %w", gid, err)
	}
	width := fontUnitsToPDFWidth(advance.Round(), face.UnitsPerEm)
	return shapedGlyph{
		GlyphID:      uint16(gid),
		Rune:         r,
		Source:       string(r),
		Width:        width,
		Advance:      width,
		HasAdvance:   true,
		InkLeft:      0,
		InkRight:     pdfSimpleGlyphInkRight(r, width),
		HasInkBounds: true,
		FontKey:      key,
	}, true, nil
}

type pdfRuneRange struct {
	lo rune
	hi rune
}

var pdfBuiltinSymbolFallbackRanges = []pdfRuneRange{
	{0x2190, 0x21FF},   // Arrows
	{0x2200, 0x22FF},   // Mathematical Operators
	{0x2300, 0x23FF},   // Miscellaneous Technical
	{0x2400, 0x243F},   // Control Pictures
	{0x2440, 0x245F},   // Optical Character Recognition
	{0x2460, 0x24FF},   // Enclosed Alphanumerics
	{0x2500, 0x257F},   // Box Drawing
	{0x2580, 0x259F},   // Block Elements
	{0x25A0, 0x25FF},   // Geometric Shapes
	{0x2600, 0x26FF},   // Miscellaneous Symbols
	{0x2700, 0x27BF},   // Dingbats
	{0x27C0, 0x27EF},   // Miscellaneous Mathematical Symbols-A
	{0x27F0, 0x27FF},   // Supplemental Arrows-A
	{0x2800, 0x28FF},   // Braille Patterns
	{0x2900, 0x297F},   // Supplemental Arrows-B
	{0x2980, 0x29FF},   // Miscellaneous Mathematical Symbols-B
	{0x2A00, 0x2AFF},   // Supplemental Mathematical Operators
	{0x2B00, 0x2BFF},   // Miscellaneous Symbols and Arrows
	{0x1D400, 0x1D7FF}, // Mathematical Alphanumeric Symbols
	{0x1F000, 0x1FAFF}, // Supplemental symbol and pictographic blocks with monochrome Noto coverage
}

var pdfBuiltinMathFallbackFirstRanges = []pdfRuneRange{
	{0x2190, 0x22FF},
	{0x27C0, 0x27FF},
	{0x2900, 0x2AFF},
	{0x1D400, 0x1D7FF},
}

var pdfBuiltinMonoFallbackFirstRanges = []pdfRuneRange{
	{0x2500, 0x257F}, // Box Drawing
	{0x2580, 0x259F}, // Block Elements
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

func missingPDFGlyph(face *builtinFontFace, r rune, advanceWidth int) shapedGlyph {
	kind := pdfMissingGlyphPrintable
	width := advanceWidth
	if unicode.IsSpace(r) {
		kind = pdfMissingGlyphSpace
	} else if unicode.IsMark(r) {
		kind = pdfMissingGlyphCombining
		width = 0
	}
	if width < 0 {
		width = 0
	}
	if width == 0 && kind != pdfMissingGlyphCombining {
		width = 500
	}
	glyph := shapedGlyph{
		Rune:         r,
		Source:       string(r),
		Width:        width,
		Advance:      width,
		HasAdvance:   true,
		InkLeft:      0,
		InkRight:     pdfSimpleGlyphInkRight(r, width),
		HasInkBounds: true,
		Missing:      kind,
	}
	logPDFMissingGlyph(face, glyph)
	return glyph
}

func logPDFMissingGlyph(face *builtinFontFace, glyph shapedGlyph) {
	if face == nil || face.MissingGlyphLog == nil || face.MissingGlyphLog.Log == nil {
		return
	}
	logger := face.MissingGlyphLog
	key := pdfMissingGlyphLogKey{
		FontFamily:     normalizedPDFFontFamily(logger.FontKey.Family),
		Bold:           logger.FontKey.Bold,
		Italic:         logger.FontKey.Italic,
		PostScriptName: face.PostScriptName,
		Rune:           glyph.Rune,
		Kind:           glyph.Missing,
	}
	if pdfMissingGlyphAlreadyLogged(logger, key) {
		return
	}
	logger.Log.Warn("Using synthetic PDF missing glyph",
		zap.String("font_family", key.FontFamily),
		zap.Bool("bold", key.Bold),
		zap.Bool("italic", key.Italic),
		zap.String("font", key.PostScriptName),
		zap.String("rune", fmt.Sprintf("%U", glyph.Rune)),
		zap.String("char", string(glyph.Rune)),
		zap.String("kind", glyph.Missing.String()),
		zap.Int("advance_width", glyph.Width))
}

func pdfMissingGlyphAlreadyLogged(logger *pdfMissingGlyphLogger, key pdfMissingGlyphLogKey) bool {
	if logger.Seen == nil {
		return false
	}
	if logger.Mu != nil {
		logger.Mu.Lock()
		defer logger.Mu.Unlock()
	}
	if logger.Seen[key] {
		return true
	}
	logger.Seen[key] = true
	return false
}

func shapedWidthPoints(text shapedText, fontSize float64) float64 {
	return shapedWidthPointsWithSpacing(text, fontSize, 0)
}

func shapedWidthPointsWithSpacing(text shapedText, fontSize float64, letterSpacing float64) float64 {
	width := 0
	for _, glyph := range text.Glyphs {
		width += shapedGlyphAdvanceWidth(glyph)
	}
	points := float64(width) * fontSize / 1000.0
	if letterSpacing != 0 && len(text.Glyphs) > 1 {
		points += letterSpacing * float64(len(text.Glyphs)-1)
	}
	return points
}

func shapedGlyphAdvanceWidth(glyph shapedGlyph) int {
	if glyph.HasAdvance {
		return glyph.Advance
	}
	return glyph.Width
}

func shapedGlyphInkBounds(glyph shapedGlyph) (int, int) {
	if glyph.HasInkBounds {
		return glyph.InkLeft, glyph.InkRight
	}
	return 0, pdfSimpleGlyphInkRight(glyph.Rune, shapedGlyphAdvanceWidth(glyph))
}

func pdfSimpleGlyphInkRight(r rune, width int) int {
	if unicode.IsSpace(r) {
		return 0
	}
	return width
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
		if glyph.Missing != pdfMissingGlyphNone || glyph.GlyphID == 0 {
			continue
		}
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
			CIDMap:  objects.CIDMap,
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
	cidMaps := make(map[pdfFontKey]map[uint16]uint16, len(resources))
	for _, resource := range resources {
		names[resource.Key] = resource.Name
		cidMaps[resource.Key] = resource.CIDMap
	}
	for pageIndex := range pages {
		for lineIndex := range pages[pageIndex].Lines {
			line := &pages[pageIndex].Lines[lineIndex]
			line.FontName = names[line.FontKey]
			remapShapedTextGlyphIDs(&line.Text, cidMaps[line.FontKey])
			for fragmentIndex := range line.Fragments {
				fragment := &line.Fragments[fragmentIndex]
				fragment.FontName = names[fragment.FontKey]
				remapShapedTextGlyphIDs(&fragment.Text, cidMaps[fragment.FontKey])
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
		fontFileDict, fontFileData, err := compressedPDFStream(resource.Objects.FontFile, resource.Objects.FontFileData)
		if err != nil {
			return fmt.Errorf("compress font file stream %s: %w", resource.Face.PostScriptName, err)
		}
		if err := writer.StreamObject(resource.IDs.FontFile, fontFileDict, fontFileData); err != nil {
			return err
		}
		toUnicodeDict, toUnicodeData, err := compressedPDFStream(docwriter.Dict{}, resource.Objects.ToUnicode)
		if err != nil {
			return fmt.Errorf("compress ToUnicode stream %s: %w", resource.Face.PostScriptName, err)
		}
		if err := writer.StreamObject(resource.IDs.ToUnicode, toUnicodeDict, toUnicodeData); err != nil {
			return err
		}
	}
	return nil
}

func compressedPDFStream(dict docwriter.Dict, data []byte) (docwriter.Dict, []byte, error) {
	compressed, err := flateStream(data)
	if err != nil {
		return nil, nil, err
	}
	out := make(docwriter.Dict, len(dict)+1)
	for key, value := range dict {
		out[key] = value
	}
	out["Filter"] = docwriter.Name("FlateDecode")
	return out, compressed, nil
}

func compressedPDFStreamSize(data []byte) int {
	compressed, err := flateStream(data)
	if err != nil {
		return 0
	}
	return len(compressed)
}

func fontResourceObjects(face *builtinFontFace, used map[uint16]shapedGlyph, objectIDs fontObjectIDs) (fontObjects, error) {
	if face == nil {
		return fontObjects{}, fmt.Errorf("font face is required")
	}
	if len(used) == 0 {
		return fontObjects{}, fmt.Errorf("at least one used glyph is required")
	}

	program, err := pdfFontProgram(face.Data)
	if err != nil {
		return fontObjects{}, fmt.Errorf("classify font program %s: %w", face.PostScriptName, err)
	}
	fontNameString := face.PostScriptName
	fontFileData := face.Data
	encodedUsed := used
	cidMap := identityCIDMap(used)
	if program.TrueTypeOutlines && allowPDFTrueTypeSubsetting(face) {
		if subset, ok, err := subsetTrueTypeFont(face.Data, used); err != nil {
			return fontObjects{}, fmt.Errorf("subset TrueType font %s: %w", face.PostScriptName, err)
		} else if ok {
			fontFileData = subset.Data
			fontNameString = subsetPDFFontName(face.PostScriptName, used)
			cidMap = subset.GlyphMap
			encodedUsed, err = remapUsedGlyphsToCIDs(used, cidMap)
			if err != nil {
				return fontObjects{}, fmt.Errorf("remap TrueType subset glyphs for %s: %w", face.PostScriptName, err)
			}
		}
	}
	fontName := docwriter.Name(fontNameString)
	cidFont := docwriter.Dict{
		"BaseFont":      fontName,
		"CIDSystemInfo": cidSystemInfo("Adobe", "Identity"),
		"DW":            docwriter.Integer(1000),
		"FontDescriptor": docwriter.Ref{
			ObjectNumber: objectIDs.FontDescriptor,
		},
		"Subtype": program.CIDFontSubtype,
		"Type":    docwriter.Name("Font"),
		"W":       widthsArray(encodedUsed),
	}
	if program.CIDToGIDMapIdentity {
		cidFont["CIDToGIDMap"] = docwriter.Name("Identity")
	}
	fontDescriptor := docwriter.Dict{
		"Ascent":      docwriter.Integer(face.Ascent),
		"CapHeight":   docwriter.Integer(face.CapHeight),
		"Descent":     docwriter.Integer(face.Descent),
		"Flags":       docwriter.Integer(face.Flags),
		"FontBBox":    intArray(face.BBox[:]...),
		"FontName":    fontName,
		"ItalicAngle": docwriter.Integer(face.ItalicAngle),
		"StemV":       docwriter.Integer(80),
		"Type":        docwriter.Name("FontDescriptor"),
	}
	fontDescriptor[program.FontFileKey] = docwriter.Ref{ObjectNumber: objectIDs.FontFile}
	fontFile := docwriter.Dict{}
	if program.FontFileLength1 {
		fontFile["Length1"] = docwriter.Integer(len(fontFileData))
	}
	if program.FontFileSubtype != "" {
		fontFile["Subtype"] = program.FontFileSubtype
	}
	return fontObjects{
		Type0Font: docwriter.Dict{
			"BaseFont":        fontName,
			"DescendantFonts": docwriter.Array{docwriter.Ref{ObjectNumber: objectIDs.CIDFont}},
			"Encoding":        docwriter.Name("Identity-H"),
			"Subtype":         docwriter.Name("Type0"),
			"ToUnicode":       docwriter.Ref{ObjectNumber: objectIDs.ToUnicode},
			"Type":            docwriter.Name("Font"),
		},
		CIDFont:        cidFont,
		FontDescriptor: fontDescriptor,
		FontFile:       fontFile,
		FontFileData:   fontFileData,
		ToUnicode:      toUnicodeCMap(encodedUsed),
		CIDMap:         cidMap,
	}, nil
}

type pdfFontProgramInfo struct {
	OutlineKind         string
	TrueTypeOutlines    bool
	CIDFontSubtype      docwriter.Name
	CIDToGIDMapIdentity bool
	FontFileKey         string
	FontFileSubtype     docwriter.Name
	FontFileLength1     bool
}

func identityCIDMap(used map[uint16]shapedGlyph) map[uint16]uint16 {
	mapping := make(map[uint16]uint16, len(used))
	for glyphID := range used {
		mapping[glyphID] = glyphID
	}
	return mapping
}

func remapUsedGlyphsToCIDs(used map[uint16]shapedGlyph, cidMap map[uint16]uint16) (map[uint16]shapedGlyph, error) {
	remapped := make(map[uint16]shapedGlyph, len(used))
	for originalID, glyph := range used {
		cid, ok := cidMap[originalID]
		if !ok {
			return nil, fmt.Errorf("used glyph %d is missing from subset CID map", originalID)
		}
		glyph.GlyphID = cid
		remapped[cid] = glyph
	}
	return remapped, nil
}

func remapShapedTextGlyphIDs(text *shapedText, cidMap map[uint16]uint16) {
	if text == nil || len(cidMap) == 0 {
		return
	}
	for i := range text.Glyphs {
		glyph := &text.Glyphs[i]
		if glyph.GlyphID == 0 || glyph.Missing != pdfMissingGlyphNone {
			continue
		}
		if cid, ok := cidMap[glyph.GlyphID]; ok {
			glyph.GlyphID = cid
		}
	}
	if len(text.Used) == 0 {
		return
	}
	used := make(map[uint16]shapedGlyph, len(text.Used))
	for originalID, glyph := range text.Used {
		if cid, ok := cidMap[originalID]; ok {
			glyph.GlyphID = cid
			used[cid] = glyph
			continue
		}
		used[originalID] = glyph
	}
	text.Used = used
}

func pdfFontProgram(data []byte) (pdfFontProgramInfo, error) {
	if _, ok := rawTTFTable(data, "glyf"); ok {
		return pdfFontProgramInfo{
			OutlineKind:         "truetype",
			TrueTypeOutlines:    true,
			CIDFontSubtype:      docwriter.Name("CIDFontType2"),
			CIDToGIDMapIdentity: true,
			FontFileKey:         "FontFile2",
			FontFileLength1:     true,
		}, nil
	}
	if _, ok := rawTTFTable(data, "CFF "); ok {
		return pdfOpenTypeCFFProgram(), nil
	}
	if _, ok := rawTTFTable(data, "CFF2"); ok {
		return pdfOpenTypeCFF2Program(), nil
	}
	return pdfFontProgramInfo{}, fmt.Errorf("unsupported font outline tables")
}

func pdfOpenTypeCFFProgram() pdfFontProgramInfo {
	return pdfOpenTypeCFFProgramWithKind("opentype_cff")
}

func pdfOpenTypeCFF2Program() pdfFontProgramInfo {
	return pdfOpenTypeCFFProgramWithKind("opentype_cff2")
}

func pdfOpenTypeCFFProgramWithKind(kind string) pdfFontProgramInfo {
	return pdfFontProgramInfo{
		OutlineKind:     kind,
		CIDFontSubtype:  docwriter.Name("CIDFontType0"),
		FontFileKey:     "FontFile3",
		FontFileSubtype: docwriter.Name("OpenType"),
	}
}

func allowPDFTrueTypeSubsetting(face *builtinFontFace) bool {
	const noSubsetting = 0x0100
	return face.EmbeddingFSType&noSubsetting == 0
}

func subsetPDFFontName(postScriptName string, used map[uint16]shapedGlyph) string {
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, int(id))
	}
	slices.Sort(ids)

	h := fnv.New32a()
	_, _ = h.Write([]byte(postScriptName))
	for _, id := range ids {
		_, _ = fmt.Fprintf(h, ":%04X", id)
	}
	value := h.Sum32()
	prefix := make([]byte, 6)
	for i := range prefix {
		prefix[i] = byte('A' + value%26)
		value /= 26
	}
	return string(prefix) + "+" + postScriptName
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
	CIDMap         map[uint16]uint16
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
	for id, glyph := range used {
		if glyphUnicodeText(glyph) == "" {
			continue
		}
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
			fmt.Fprintf(&buf, "<%04X> <%s>\n", id, utf16BEHexString(glyphUnicodeText(glyph)))
		}
		buf.WriteString("endbfchar\n")
	}
	buf.WriteString("endcmap\n")
	buf.WriteString("CMapName currentdict /CMap defineresource pop\n")
	buf.WriteString("end\n")
	buf.WriteString("end\n")
	return buf.Bytes()
}

func glyphUnicodeText(glyph shapedGlyph) string {
	if glyph.Source != "" {
		return glyph.Source
	}
	if glyph.Rune != 0 {
		return string(glyph.Rune)
	}
	return ""
}

func utf16BEHex(r rune) string {
	return utf16BEHexString(string(r))
}

func utf16BEHexString(text string) string {
	words := utf16.Encode([]rune(text))
	var b strings.Builder
	for _, word := range words {
		fmt.Fprintf(&b, "%04X", word)
	}
	return b.String()
}
