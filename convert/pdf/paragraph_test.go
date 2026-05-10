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
	for i, line := range lines[:len(lines)-1] {
		if line.JustificationGaps > 0 && line.ExtraWordSpacing <= 0 {
			t.Fatalf("line %d extra word spacing = %v, want positive", i, line.ExtraWordSpacing)
		}
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

func TestJustifiedGlyphArrayAddsNegativeAdjustmentsAfterSpaces(t *testing.T) {
	got := justifiedGlyphArray([]shapedGlyph{
		{GlyphID: 1, Rune: 'A'},
		{GlyphID: 2, Rune: ' '},
		{GlyphID: 3, Rune: 'B'},
	}, 2, 10)
	want := "[<0001> <0002> -200 <0003>]"
	if got != want {
		t.Fatalf("justifiedGlyphArray() = %q, want %q", got, want)
	}
}
