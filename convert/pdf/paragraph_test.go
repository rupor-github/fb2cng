package pdf

import "testing"

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
	for i, line := range lines[:len(lines)-1] {
		if line.ExtraWordSpacing > max(style.FontSize*0.40, 3.0) {
			t.Fatalf("line %d extra word spacing = %v, want capped", i, line.ExtraWordSpacing)
		}
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
	prefix, err := shapeText(face, "hy-")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	full, err := shapeText(face, "hyphenation")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	maxWidth := shapedWidthPoints(prefix, style.FontSize) + 1
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
	prefix, err := shapeText(face, "alpha-")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	full, err := shapeText(face, "alpha-beta")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
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
	prefix, err := shapeText(face, "alpha/")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
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
	prefix, err := shapeText(face, "hy-")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	maxWidth := shapedWidthPoints(prefix, 10) + 1

	tests := []struct {
		name       string
		text       string
		style      paragraphStyle
		wantLines  int
		wantFirst  string
		wantSecond string
	}{
		{
			name:       "auto uses dictionary hyphenation",
			text:       "hyphenation",
			style:      paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationAuto, Hyphenator: fakeHyphenator{"hyphenation": "hy\u00adphenation"}},
			wantLines:  2,
			wantFirst:  "hy-",
			wantSecond: "phenation",
		},
		{
			name:      "none disables dictionary and manual hyphenation",
			text:      "hy\u00adphenation",
			style:     paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationNone, Hyphenator: fakeHyphenator{"hyphenation": "hy\u00adphenation"}},
			wantLines: 1,
			wantFirst: "hyphenation",
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
			name:      "manual ignores dictionary hyphenation",
			text:      "hyphenation",
			style:     paragraphStyle{FontSize: 10, LineHeight: 12, Hyphenation: paragraphHyphenationManual, Hyphenator: fakeHyphenator{"hyphenation": "hy\u00adphenation"}},
			wantLines: 1,
			wantFirst: "hyphenation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := layoutParagraph(face, tt.text, tt.style, maxWidth)
			if err != nil {
				t.Fatalf("layoutParagraph() error = %v", err)
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

func TestParagraphJustificationUsesCharacterSpacingAfterWordSpacingCap(t *testing.T) {
	word, char := paragraphJustificationSpacing(paragraphStyle{FontSize: 10, Align: textAlignJustify}, false, 70, 100, 2, 20)
	if word != 4 {
		t.Fatalf("word spacing = %v, want capped 4", word)
	}
	if char <= 0 || char > 0.7 {
		t.Fatalf("char spacing = %v, want positive capped spacing", char)
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
	prefix, err := shapeText(face, "hy-")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	lines, err := layoutParagraph(face, "hyphenation", style, shapedWidthPoints(prefix, style.FontSize)+1)
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
	prefix, err := shapeText(boldFace, "hy-")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
	}
	style := paragraphStyle{
		FontSize:    10,
		LineHeight:  12,
		Hyphenator:  fakeHyphenator{"hyphenation": "hy\u00adphenation"},
		Hyphenation: paragraphHyphenationAuto,
	}

	lines, err := layoutInlineParagraph(pdfDocumentSpec{}, nil, nil, baseFace, "hyphenation", []pdfInlineRun{{Text: "hyphenation", Bold: true}}, style, shapedWidthPoints(prefix, style.FontSize)+1)
	if err != nil {
		t.Fatalf("layoutInlineParagraph() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("layoutInlineParagraph() produced %d lines, want 2", len(lines))
	}
	if got := shapedRunes(lines[0].Text); got != "hy-" {
		t.Fatalf("first line = %q, want hyphenated prefix", got)
	}
	if len(lines[0].Fragments) == 0 || !lines[0].Fragments[0].FontKey.Bold {
		t.Fatalf("first line fragments = %#v, want bold fragment preserved", lines[0].Fragments)
	}
}

func TestInlineParagraphUnitsKeepLigatureWordsIntact(t *testing.T) {
	face, err := builtinFont("serif", true, false)
	if err != nil {
		t.Fatalf("builtinFont() error = %v", err)
	}
	shaped, err := shapeText(face, "fiend")
	if err != nil {
		t.Fatalf("shapeText() error = %v", err)
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
	units, err := inlineParagraphUnits(nil, []paragraphInlineWord{word}, paragraphStyle{
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
