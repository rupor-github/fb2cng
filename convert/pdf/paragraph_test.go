package pdf

import "testing"

func layoutParagraph(face *builtinFontFace, text string, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	return layoutParagraphWithShape(face, text, style, maxWidth, paragraphLineShape{})
}

func chooseParagraphBreaks(units []paragraphUnit, spaceWidth float64, style paragraphStyle, maxWidth float64) []paragraphBreak {
	return chooseBreaksWithShape(units, spaceWidth, style, maxWidth, paragraphLineShape{})
}

func paragraphLineJustificationAvailable(line paragraphLine, fontSize float64, letterSpacing float64, available float64) float64 {
	return paragraphJustificationAvailableForOverhang(available, paragraphLineVisualRightReserve(line, fontSize, letterSpacing))
}

func TestLayoutParagraphBalancesLinesAndJustifies(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{
		FontSize:        10,
		LineHeight:      12,
		FirstLineIndent: 12,
		Align:           textAlignJustify,
	}
	lines, err := layoutParagraph(face, "one two three four five six seven eight nine", style, 75)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) < 3 {
		t.Fatalf("layoutParagraph() produced %d lines, want at least 3", len(lines))
	}
	for i, line := range lines {
		available := 75.0 - line.Indent
		if line.Width > available+0.001 {
			t.Fatalf("line %d width = %v, available = %v", i, line.Width, available)
		}
		if i == 0 && line.Indent != 12 {
			t.Fatalf("first line indent = %v, want 12", line.Indent)
		}
		if i > 0 && line.Indent != 0 {
			t.Fatalf("line %d indent = %v, want 0", i, line.Indent)
		}
	}
	sawJustifiedLine := false
	for _, line := range lines[:len(lines)-1] {
		if line.ExtraWordSpacing > 0 {
			sawJustifiedLine = true
		}
	}
	if !sawJustifiedLine {
		t.Fatal("layoutParagraph() produced no justified non-final lines")
	}
	if got := lines[len(lines)-1].ExtraWordSpacing; got != 0 {
		t.Fatalf("last line extra word spacing = %v, want 0", got)
	}
}

type fakeHyphenator map[string]string

func (h fakeHyphenator) Hyphenate(s string) string {
	if v, ok := h[s]; ok {
		return v
	}
	return s
}

func containsHyphen(text string) bool {
	for _, r := range text {
		if isHyphenLikeBreakRune(r) {
			return true
		}
	}
	return false
}

func TestParagraphDemeritsTreatHyphenationAsUsefulCorrection(t *testing.T) {
	hyphenated := paragraphLineDemerits(49, 50, 2, false, false, false, true, false, paragraphFitnessDecent)
	loose := paragraphLineDemerits(42, 50, 2, false, false, false, false, false, paragraphFitnessDecent)
	if hyphenated >= loose {
		t.Fatalf("hyphenated demerits = %v, loose demerits = %v, want hyphenation preferred over loose spacing", hyphenated, loose)
	}
}

func TestLayoutParagraphUsesHyphenationPoints(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}

	style := paragraphStyle{
		FontSize:   10,
		LineHeight: 12,
		Hyphenator: fakeHyphenator{"hyphenation": "hy\u00adphenation"},
	}
	prefix, err := shapeTextWithCache(nil, face, "hy-")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	suffix, err := shapeTextWithCache(nil, face, "phenation")
	if err != nil {
		t.Fatalf("shape suffix error = %v", err)
	}
	full, err := shapeTextWithCache(nil, face, "hyphenation")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	maxWidth := max(shapedWidthPoints(prefix, style.FontSize), shapedWidthPoints(suffix, style.FontSize)) + 1
	if maxWidth >= shapedWidthPoints(full, style.FontSize) {
		t.Fatal("test width is not narrow enough to require hyphenation")
	}

	lines, err := layoutParagraph(face, "hyphenation", style, maxWidth)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("layoutParagraph() produced %d lines, want 2", len(lines))
	}
	if got := shapedRunes(lines[0].Text); got != "hy-" {
		t.Fatalf("first line = %q, want %q", got, "hy-")
	}
	if got := shapedRunes(lines[1].Text); got != "phenation" {
		t.Fatalf("second line = %q, want %q", got, "phenation")
	}
}

func TestLayoutParagraphDropsUnbrokenSoftHyphen(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	lines, err := layoutParagraph(face, "hy\u00adphenation", paragraphStyle{FontSize: 10, LineHeight: 12}, 1000)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("layoutParagraph() produced %d lines, want 1", len(lines))
	}
	if got := shapedRunes(lines[0].Text); got != "hyphenation" {
		t.Fatalf("line = %q, want %q", got, "hyphenation")
	}
}

func TestLayoutParagraphBreaksAfterHardHyphenWithoutExtraHyphen(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationNone}
	prefix, err := shapeTextWithCache(nil, face, "alpha-")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	full, err := shapeTextWithCache(nil, face, "alpha-beta")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	maxWidth := shapedWidthPoints(prefix, style.FontSize) + 0.1
	if maxWidth >= shapedWidthPoints(full, style.FontSize) {
		t.Fatal("test width is not narrow enough to require hard hyphen break")
	}

	lines, err := layoutParagraph(face, "alpha-beta", style, maxWidth)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("layoutParagraph() produced %d lines, want 2", len(lines))
	}
	if got := shapedRunes(lines[0].Text); got != "alpha-" {
		t.Fatalf("first line = %q, want hard hyphen without extra inserted hyphen", got)
	}
	if got := shapedRunes(lines[1].Text); got != "beta" {
		t.Fatalf("second line = %q, want beta", got)
	}
	if !lines[0].BreakStats.Hyphenated {
		t.Fatalf("first line break stats = %#v, want hyphenated hard-break diagnostic", lines[0].BreakStats)
	}
}

func TestLayoutParagraphBreaksAfterPunctuationWithoutHyphenPenalty(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationNone}
	prefix, err := shapeTextWithCache(nil, face, "alpha/")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}

	lines, err := layoutParagraph(face, "alpha/beta", style, shapedWidthPoints(prefix, style.FontSize)+0.1)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("layoutParagraph() produced %d lines, want 2", len(lines))
	}
	if got := shapedRunes(lines[0].Text); got != "alpha/" {
		t.Fatalf("first line = %q, want punctuation break without inserted hyphen", got)
	}
	if lines[0].BreakStats.Hyphenated {
		t.Fatalf("first line break stats = %#v, want punctuation break without hyphen penalty", lines[0].BreakStats)
	}
}

func TestLayoutParagraphHonorsHyphenationModes(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	prefix, err := shapeTextWithCache(nil, face, "hy-")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	suffix, err := shapeTextWithCache(nil, face, "phenation")
	if err != nil {
		t.Fatalf("shape suffix error = %v", err)
	}
	maxWidth := max(shapedWidthPoints(prefix, 10), shapedWidthPoints(suffix, 10)) + 1

	tests := []struct {
		name                  string
		text                  string
		style                 paragraphStyle
		wantLines             int
		wantFirst             string
		wantSecond            string
		wantEmergencyNoHyphen bool
	}{
		{
			name: "auto uses dictionary hyphenation",
			text: "hyphenation",
			style: paragraphStyle{
				FontSize:    10,
				LineHeight:  12,
				Hyphenation: paragraphHyphenationAuto,
				Hyphenator:  fakeHyphenator{"hyphenation": "hy\u00adphenation"},
			},
			wantLines:  2,
			wantFirst:  "hy-",
			wantSecond: "phenation",
		},
		{
			name: "none disables dictionary and manual hyphenation",
			text: "hy\u00adphenation",
			style: paragraphStyle{
				FontSize:    10,
				LineHeight:  12,
				Hyphenation: paragraphHyphenationNone,
				Hyphenator:  fakeHyphenator{"hyphenation": "hy\u00adphenation"},
			},
			wantEmergencyNoHyphen: true,
		},
		{
			name:       "manual honors source soft hyphen only",
			text:       "hy\u00adphenation",
			style:      paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationManual},
			wantLines:  2,
			wantFirst:  "hy-",
			wantSecond: "phenation",
		},
		{
			name: "manual ignores dictionary hyphenation",
			text: "hyphenation",
			style: paragraphStyle{
				FontSize:    10,
				LineHeight:  12,
				Hyphenation: paragraphHyphenationManual,
				Hyphenator:  fakeHyphenator{"hyphenation": "hy\u00adphenation"},
			},
			wantEmergencyNoHyphen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := layoutParagraph(face, tt.text, tt.style, maxWidth)
			if err != nil {
				t.Fatalf("layoutParagraph() error = %v", err)
			}
			if tt.wantEmergencyNoHyphen {
				if len(lines) < 2 {
					t.Fatalf("layoutParagraph() produced %d lines, want emergency wrapping", len(lines))
				}
				var joined string
				sawEmergency := false
				for _, line := range lines {
					lineText := shapedRunes(line.Text)
					joined += lineText
					if line.BreakStats.Hyphenated || containsHyphen(lineText) {
						t.Fatalf("line %q was hyphenated despite hyphens disabled/manual-only", lineText)
					}
					sawEmergency = sawEmergency || line.BreakStats.Emergency
				}
				if joined != "hyphenation" {
					t.Fatalf("joined emergency lines = %q, want hyphenation", joined)
				}
				if !sawEmergency {
					t.Fatalf("line break stats = %#v, want emergency diagnostic", lines)
				}
				return
			}
			if len(lines) != tt.wantLines {
				t.Fatalf("layoutParagraph() produced %d lines, want %d", len(lines), tt.wantLines)
			}
			if got := shapedRunes(lines[0].Text); got != tt.wantFirst {
				t.Fatalf("first line = %q, want %q", got, tt.wantFirst)
			}
			if tt.wantSecond != "" {
				if got := shapedRunes(lines[1].Text); got != tt.wantSecond {
					t.Fatalf("second line = %q, want %q", got, tt.wantSecond)
				}
			}
		})
	}
}

func TestLayoutParagraphEmergencyWrapsLongUnbreakableWord(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationNone}
	text := "supercalifragilistic"
	prefix, err := shapeTextWithCache(nil, face, "super")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	maxWidth := shapedWidthPoints(prefix, style.FontSize) + 0.5

	lines, err := layoutParagraph(face, text, style, maxWidth)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) < 2 {
		t.Fatalf("layoutParagraph() produced %d lines, want emergency wrapping", len(lines))
	}
	var joined string
	for i, line := range lines {
		joined += shapedRunes(line.Text)
		if line.Width > maxWidth+pdfLineWidthTolerance {
			t.Fatalf("line %d %q width = %v, max = %v", i, shapedRunes(line.Text), line.Width, maxWidth)
		}
	}
	if joined != text {
		t.Fatalf("joined emergency lines = %q, want %q", joined, text)
	}
	if !lines[0].BreakStats.Emergency {
		t.Fatalf("first line break stats = %#v, want emergency diagnostic", lines[0].BreakStats)
	}
}

func TestLayoutParagraphEmergencyDoesNotSplitLigatureCluster(t *testing.T) {
	face, err := builtinFont("serif", true, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shapedLigature, err := shapeTextWithCache(nil, face, "fi")
	if err != nil {
		t.Fatalf("shape fi error = %v", err)
	}
	if len(shapedLigature.Glyphs) != 1 || shapedLigature.Glyphs[0].Source != "fi" {
		t.Fatalf("test font did not shape fi as a ligature: %#v", shapedLigature.Glyphs)
	}
	style := paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationNone}
	lines, err := layoutParagraph(face, "fifi", style, shapedWidthPoints(shapedLigature, style.FontSize)+0.1)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("layoutParagraph() produced %d lines, want two ligature clusters", len(lines))
	}
	for i, line := range lines {
		if got := shapedRunes(line.Text); got != "fi" {
			t.Fatalf("line %d = %q, want whole fi ligature cluster", i, got)
		}
	}
}

func TestLayoutParagraphEmergencyKeepsCombiningMarksWithBase(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationNone}
	cluster, err := shapeTextWithCache(nil, face, "a\u0301")
	if err != nil {
		t.Fatalf("shape combining error = %v", err)
	}
	text := "a\u0301a\u0301a\u0301"
	lines, err := layoutParagraph(face, text, style, shapedWidthPoints(cluster, style.FontSize)+0.1)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) < 2 {
		t.Fatalf("layoutParagraph() produced %d lines, want emergency wrapping", len(lines))
	}
	var joined string
	for i, line := range lines {
		lineText := shapedRunes(line.Text)
		joined += lineText
		if startsWithCombiningMark(lineText) {
			t.Fatalf("line %d starts with a combining mark: %q", i, lineText)
		}
	}
	if joined != text {
		t.Fatalf("joined emergency lines = %q, want %q", joined, text)
	}
}

func TestParagraphBreaksReserveTerminalVisualOverhang(t *testing.T) {
	units := []paragraphUnit{
		{Text: "left", Width: 5, WordIndex: 0, EndWord: true},
		{Text: "right", Width: 5, WordIndex: 1, EndWord: true, RightOverhang: 5},
	}
	breaks := chooseParagraphBreaks(units, 1, paragraphStyle{FontSize: 10}, 11)
	if len(breaks) != 2 {
		t.Fatalf("breaks = %#v, want two lines because terminal overhang exceeds width", breaks)
	}
	if breaks[0].End != 1 {
		t.Fatalf("first break = %#v, want before overhanging terminal word", breaks[0])
	}
}

func TestLayoutParagraphAvoidsIsolatedSingleWordWhenLooseJustifiedLineFits(t *testing.T) {
	face, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{FontSize: 10.08, LineHeight: 12, Align: textAlignJustify, Hyphenation: paragraphHyphenationNone}
	text := "Настоя́щая должностнáя инстру́кция определя́ет обя́занности " +
		"(постоя́нные рабóты), правá и отве́тственность ли́ц, занима́ющих или " +
		"назнача́емых на дóлжность инженéра."
	lines, err := layoutParagraph(face, text, style, 279.36)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	for i, line := range lines {
		if got := shapedRunes(line.Text); got == "обя́занности" {
			t.Fatalf("line %d isolated %q in %#v", i, got, line.BreakStats)
		}
	}
}

func TestParagraphBreaksPreferFillableJustifiedLines(t *testing.T) {
	units := []paragraphUnit{
		{Text: "one", Width: 13, GlyphCount: 4, WordIndex: 0, EndWord: true},
		{Text: "two", Width: 13, GlyphCount: 4, WordIndex: 1, EndWord: true},
		{Text: "three", Width: 13, GlyphCount: 4, WordIndex: 2, EndWord: true},
		{Text: "four", Width: 13, GlyphCount: 4, WordIndex: 3, EndWord: true},
	}
	breaks := chooseParagraphBreaks(units, 1, paragraphStyle{FontSize: 10, Align: textAlignJustify}, 44)
	if len(breaks) != 2 {
		t.Fatalf("breaks = %#v, want two lines", breaks)
	}
	if breaks[0].End != 3 {
		t.Fatalf("first break = %#v, want fillable three-word justified line", breaks[0])
	}
}

func TestParagraphJustificationIgnoresNonTerminalOverhangReserve(t *testing.T) {
	style := paragraphStyle{FontSize: 10, Align: textAlignJustify}
	available := paragraphJustificationAvailableForOverhang(14.5, 0)
	word, char := paragraphJustificationSpacing(style, false, 12.5, available, 1, 3)
	if word != 2 || char != 0 {
		t.Fatalf("justification spacing = %v/%v, want full right-edge fill without non-terminal reserve", word, char)
	}
}

func TestParagraphJustificationReservesTerminalVisualOverhang(t *testing.T) {
	style := paragraphStyle{FontSize: 10, Align: textAlignJustify}
	line := paragraphLine{
		Text: shapedText{Glyphs: []shapedGlyph{
			{GlyphID: 1, Rune: 'A', Source: "A", Width: 500, Advance: 500, HasAdvance: true, InkLeft: 0, InkRight: 500, HasInkBounds: true},
			{GlyphID: 2, Rune: ' ', Source: " ", Width: 250, Advance: 250, HasAdvance: true},
			{GlyphID: 3, Rune: 'T', Source: "T", Width: 500, Advance: 500, HasAdvance: true, InkLeft: 0, InkRight: 650, HasInkBounds: true},
		}},
		Width:             12.5,
		JustificationGaps: 1,
	}
	available := paragraphLineJustificationAvailable(line, style.FontSize, 0, 14.5)
	if available != 13 {
		t.Fatalf("justification available = %v, want terminal overhang reserve", available)
	}
	word, char := paragraphJustificationSpacing(style, false, line.Width, available, line.JustificationGaps, len(line.Text.Glyphs))
	if word != 0.5 || char != 0 {
		t.Fatalf("justification spacing = %v/%v, want spacing reduced by visual reserve", word, char)
	}
}

func TestParagraphJustificationShrinksOverfullLines(t *testing.T) {
	style := paragraphStyle{FontSize: 10, Align: textAlignJustify}
	word, char := paragraphJustificationSpacing(style, false, 104, 100, 4, 20)
	if word != -1 || char != 0 {
		t.Fatalf("justification shrink = %v/%v, want negative word spacing", word, char)
	}
}

func TestParagraphJustificationUsesCharacterSpacingAndFillsRightEdge(t *testing.T) {
	word, char := paragraphJustificationSpacing(paragraphStyle{FontSize: 10, Align: textAlignJustify}, false, 90, 100, 2, 20)
	if word != 4 {
		t.Fatalf("word spacing = %v, want soft-capped word spacing", word)
	}
	if char <= 0 || char > 0.25 {
		t.Fatalf("char spacing = %v, want positive capped spacing", char)
	}
	if got := 90 + word*2 + char*19; got < 100-pdfLineWidthTolerance || got > 100+pdfLineWidthTolerance {
		t.Fatalf("justified width = %v, want 100 (word=%v char=%v)", got, word, char)
	}
}

func TestParagraphJustificationFallsBackToWordSpacingForLargeResidualStretch(t *testing.T) {
	word, char := paragraphJustificationSpacing(paragraphStyle{FontSize: 10, Align: textAlignJustify}, false, 70, 100, 2, 20)
	if char != 0.25 {
		t.Fatalf("char spacing = %v, want capped tracking", char)
	}
	if got := 70 + word*2 + char*19; got < 100-pdfLineWidthTolerance || got > 100+pdfLineWidthTolerance {
		t.Fatalf("justified width = %v, want 100 (word=%v char=%v)", got, word, char)
	}
}

func TestLayoutParagraphRecordsLineBreakStats(t *testing.T) {
	face, err := builtinFont("sans-serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	style := paragraphStyle{
		FontSize:    10,
		LineHeight:  12,
		Hyphenator:  fakeHyphenator{"hyphenation": "hy\u00adphenation"},
		Hyphenation: paragraphHyphenationAuto,
	}
	prefix, err := shapeTextWithCache(nil, face, "hy-")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	suffix, err := shapeTextWithCache(nil, face, "phenation")
	if err != nil {
		t.Fatalf("shape suffix error = %v", err)
	}
	maxWidth := max(shapedWidthPoints(prefix, style.FontSize), shapedWidthPoints(suffix, style.FontSize)) + 1
	lines, err := layoutParagraph(face, "hyphenation", style, maxWidth)
	if err != nil {
		t.Fatalf("layoutParagraph() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("layoutParagraph() produced %d lines, want 2", len(lines))
	}
	stats := lines[0].BreakStats
	if stats.AvailableWidth <= 0 || stats.Demerits <= 0 {
		t.Fatalf("line break stats = %#v, want populated diagnostics", stats)
	}
	if !stats.Hyphenated || !stats.SingleWord {
		t.Fatalf("line break stats = %#v, want hyphenated single-word line", stats)
	}
	if got := paragraphFitnessString(stats.Fitness); got == "unknown" {
		t.Fatalf("fitness string = %q", got)
	}
}

func TestParagraphBreakerAvoidsShortFinalLineWhenBalancedBreaksFit(t *testing.T) {
	units := []paragraphUnit{
		{Text: "one", Width: 20, WordIndex: 0, EndWord: true},
		{Text: "two", Width: 20, WordIndex: 1, EndWord: true},
		{Text: "three", Width: 20, WordIndex: 2, EndWord: true},
		{Text: "four", Width: 20, WordIndex: 3, EndWord: true},
		{Text: "five", Width: 20, WordIndex: 4, EndWord: true},
	}

	breaks := chooseParagraphBreaks(units, 5, paragraphStyle{FontSize: 10}, 70)
	if len(breaks) != 2 {
		t.Fatalf("chooseParagraphBreaks() produced %#v, want two balanced lines", breaks)
	}
	if breaks[0].End != 3 || breaks[1].End != 5 {
		t.Fatalf("chooseParagraphBreaks() = %#v, want 3/2 word split", breaks)
	}
}

func TestParagraphDemeritsPenalizeConsecutiveHyphenatedLines(t *testing.T) {
	first := paragraphLineDemerits(40, 45, 0, false, false, true, true, false, paragraphFitnessDecent)
	consecutive := paragraphLineDemerits(40, 45, 0, false, false, true, true, true, paragraphFitnessDecent)
	if consecutive <= first {
		t.Fatalf("consecutive hyphen demerits = %v, want > first hyphen demerits %v", consecutive, first)
	}
}

func TestLayoutInlineParagraphHyphenatesStyledRuns(t *testing.T) {
	baseFace, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	boldFace, err := builtinFont("serif", true, false)
	if err != nil {
		t.Fatalf("builtinFont() bold error = %v", err)
	}
	prefix, err := shapeTextWithCache(nil, boldFace, "hy-")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	suffix, err := shapeTextWithCache(nil, boldFace, "phenation")
	if err != nil {
		t.Fatalf("shape suffix error = %v", err)
	}
	style := paragraphStyle{
		FontSize:    10,
		LineHeight:  12,
		Hyphenator:  fakeHyphenator{"hyphenation": "hy\u00adphenation"},
		Hyphenation: paragraphHyphenationAuto,
	}

	maxWidth := max(shapedWidthPoints(prefix, style.FontSize), shapedWidthPoints(suffix, style.FontSize)) + 1
	lines, err := layoutInlineWithShape(
		pdfDocumentSpec{},
		nil,
		nil,
		baseFace,
		"hyphenation",
		[]pdfInlineRun{{Text: "hyphenation", Bold: true}},
		style,
		maxWidth,
		paragraphLineShape{},
	)
	if err != nil {
		t.Fatalf("layoutInlineWithShape() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("layoutInlineWithShape() produced %d lines, want 2", len(lines))
	}
	if got := shapedRunes(lines[0].Text); got != "hy-" {
		t.Fatalf("first line = %q, want hyphenated prefix", got)
	}
	if len(lines[0].Fragments) == 0 || !lines[0].Fragments[0].FontKey.Bold {
		t.Fatalf("first line fragments = %#v, want bold fragment preserved", lines[0].Fragments)
	}
}

func TestLayoutInlineParagraphEmergencyWrapsStyledLongWord(t *testing.T) {
	baseFace, err := builtinFont("serif", false, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	boldFace, err := builtinFont("serif", true, false)
	if err != nil {
		t.Fatalf("builtinFont() bold error = %v", err)
	}
	style := paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationNone}
	prefix, err := shapeTextWithCache(nil, boldFace, "super")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	text := "supercalifragilistic"
	maxWidth := shapedWidthPoints(prefix, style.FontSize) + 0.5

	lines, err := layoutInlineWithShape(
		pdfDocumentSpec{},
		nil,
		nil,
		baseFace,
		text,
		[]pdfInlineRun{{Text: text, Bold: true}},
		style,
		maxWidth,
		paragraphLineShape{},
	)
	if err != nil {
		t.Fatalf("layoutInlineWithShape() error = %v", err)
	}
	if len(lines) < 2 {
		t.Fatalf("layoutInlineWithShape() produced %d lines, want emergency wrapping", len(lines))
	}
	var joined string
	for i, line := range lines {
		joined += shapedRunes(line.Text)
		if len(line.Fragments) == 0 || !line.Fragments[0].FontKey.Bold {
			t.Fatalf("line %d fragments = %#v, want bold fragments preserved", i, line.Fragments)
		}
		if line.Width > maxWidth+pdfLineWidthTolerance {
			t.Fatalf("line %d %q width = %v, max = %v", i, shapedRunes(line.Text), line.Width, maxWidth)
		}
	}
	if joined != text {
		t.Fatalf("joined emergency lines = %q, want %q", joined, text)
	}
}

func TestInlineParagraphUnitsKeepLigatureWordsIntact(t *testing.T) {
	face, err := builtinFont("serif", true, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeTextWithCache(nil, face, "fiend")
	if err != nil {
		t.Fatalf("shape text error = %v", err)
	}
	if len(shaped.Glyphs) >= len([]rune("fiend")) {
		t.Fatalf("test font did not shape fi ligature: %#v", shaped.Glyphs)
	}
	word := paragraphInlineWord{
		Text: "fiend",
		Fragments: []paragraphLineFragment{{
			Text:     shaped,
			Width:    shapedWidthPoints(shaped, 10),
			FontSize: 10,
			FontKey:  face.Key,
		}},
	}
	units, err := inlineParagraphUnits(nil, nil, []paragraphInlineWord{word}, paragraphStyle{
		FontSize:    10,
		Hyphenation: paragraphHyphenationAuto,
		Hyphenator:  fakeHyphenator{"fiend": "fi\u00adend"},
	})
	if err != nil {
		t.Fatalf("inlineParagraphUnits() error = %v", err)
	}
	if len(units) != 1 || units[0].Text != "fiend" || !units[0].EndWord {
		t.Fatalf("units = %#v, want ligature word kept as one unit", units)
	}
}

func TestBreakableWordsKeepsNoBreakSpaceInsideWord(t *testing.T) {
	got := breakableWords("one  two\u00a0three\tfour")
	want := []string{"one", "two\u00a0three", "four"}
	if len(got) != len(want) {
		t.Fatalf("breakableWords() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("breakableWords()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPositionedGlyphArrayAddsNegativeAdjustmentsAfterSpaces(t *testing.T) {
	got := positionedGlyphArray([]shapedGlyph{
		{GlyphID: 1, Rune: 'A'},
		{GlyphID: 2, Rune: ' '},
		{GlyphID: 3, Rune: 'B'},
	}, 2, 10)
	want := "[<0001> <0002> -200 <0003>]"
	if got != want {
		t.Fatalf("positionedGlyphArray() = %q, want %q", got, want)
	}
}
