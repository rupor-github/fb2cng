package pdf

import "testing"

func TestPDFPageLineDrawnWidthIncludesJustificationSpacing(t *testing.T) {
	line := pdfPageLine{
		FontSize:         10,
		LetterSpacing:    1,
		ExtraWordSpacing: 2,
		ExtraCharSpacing: 0.25,
		Text: shapedText{Glyphs: []shapedGlyph{
			{GlyphID: 1, Rune: 'A', Width: 500},
			{GlyphID: 2, Rune: ' ', Width: 250},
			{GlyphID: 3, Rune: 'B', Width: 500},
		}},
	}

	if got, want := pdfPageLineAdvanceWidth(line), 14.5; got != want {
		t.Fatalf("advance width = %v, want %v", got, want)
	}
	if got, want := pdfPageLineDrawnWidth(line), 17.0; got != want {
		t.Fatalf("drawn width = %v, want %v", got, want)
	}
}

func TestPDFPageLineDrawnWidthMatchesFragmentDrawingCursor(t *testing.T) {
	line := pdfPageLine{
		ExtraCharSpacing: 1,
		Fragments: []pdfPageLineFragment{
			{Width: 6, Text: shapedText{Glyphs: []shapedGlyph{{GlyphID: 1, Rune: 'A', Width: 600}}}},
			{Width: 4, ImageID: "inline", ImageHeight: 10},
			{Width: 5, Text: shapedText{Glyphs: []shapedGlyph{{GlyphID: 2, Rune: 'B', Width: 500}}}},
		},
	}

	if got, want := pdfPageLineAdvanceWidth(line), 15.0; got != want {
		t.Fatalf("advance width = %v, want %v", got, want)
	}
	if got, want := pdfPageLineDrawnWidth(line), 17.0; got != want {
		t.Fatalf("drawn width = %v, want fragment cursor width %v", got, want)
	}
}

func TestPDFPageLineDrawnWidthAddsWordSpacingAfterFragmentSpace(t *testing.T) {
	line := pdfPageLine{
		ExtraWordSpacing: 3,
		ExtraCharSpacing: 1,
		Fragments: []pdfPageLineFragment{
			{Width: 8, Text: shapedText{Glyphs: []shapedGlyph{
				{GlyphID: 1, Rune: 'A', Width: 500},
				{GlyphID: 2, Rune: ' ', Width: 300},
			}}},
			{Width: 5, Text: shapedText{Glyphs: []shapedGlyph{{GlyphID: 3, Rune: 'B', Width: 500}}}},
		},
	}

	if got, want := pdfPageLineDrawnWidth(line), 18.0; got != want {
		t.Fatalf("drawn width = %v, want fragment cursor width %v", got, want)
	}
}

func TestPDFPageLineVisualBoundsUseGlyphInkBounds(t *testing.T) {
	line := pdfPageLine{
		X:        10,
		FontSize: 10,
		Text: shapedText{Glyphs: []shapedGlyph{
			{GlyphID: 1, Rune: 'T', Width: 500, Advance: 500, HasAdvance: true, InkLeft: -50, InkRight: 480, HasInkBounds: true},
			{GlyphID: 2, Rune: 'j', Width: 300, Advance: 300, HasAdvance: true, InkLeft: 20, InkRight: 360, HasInkBounds: true},
		}},
	}

	left, right, ok := pdfPageLineVisualBounds(line)
	if !ok {
		t.Fatal("visual bounds ok = false, want true")
	}
	if left != 9.5 || right != 18.6 {
		t.Fatalf("visual bounds = %v/%v, want 9.5/18.6", left, right)
	}
}

func TestPDFPageLineVisualBoundsIgnoreSpaces(t *testing.T) {
	line := pdfPageLine{
		X:        10,
		FontSize: 10,
		Text: shapedText{Glyphs: []shapedGlyph{
			{GlyphID: 1, Rune: ' ', Width: 500, Advance: 500, HasAdvance: true, InkLeft: 0, InkRight: 0, HasInkBounds: true},
		}},
	}

	_, _, ok := pdfPageLineVisualBounds(line)
	if ok {
		t.Fatal("visual bounds ok = true for space-only line, want false")
	}
}

func TestPDFPageLineXAdjustedForVisualRightPreservesLeftEdge(t *testing.T) {
	line := pdfPageLine{
		X:        10,
		FontSize: 10,
		Text: shapedText{Glyphs: []shapedGlyph{{
			GlyphID:      1,
			Rune:         'j',
			Width:        500,
			Advance:      500,
			HasAdvance:   true,
			InkLeft:      0,
			InkRight:     550,
			HasInkBounds: true,
		}}},
	}
	if got, want := pdfPageLineXAdjustedForVisualRight(line, 5), 10.0; got != want {
		t.Fatalf("adjusted x = %v, want visible left edge preserved at %v", got, want)
	}
}

func TestPDFPageLineXAdjustedForVisualRightConsumesLeftSlack(t *testing.T) {
	line := pdfPageLine{
		X:        10,
		FontSize: 10,
		Text: shapedText{Glyphs: []shapedGlyph{{
			GlyphID:      1,
			Rune:         'j',
			Width:        500,
			Advance:      500,
			HasAdvance:   true,
			InkLeft:      50,
			InkRight:     550,
			HasInkBounds: true,
		}}},
	}
	if got, want := pdfPageLineXAdjustedForVisualRight(line, 5), 9.5; got != want {
		t.Fatalf("adjusted x = %v, want shift consuming left ink slack %v", got, want)
	}
}

func TestPDFPageLineVisualOverflowUsesVisualRight(t *testing.T) {
	line := pdfPageLine{
		X:        10,
		FontSize: 10,
		Text: shapedText{Glyphs: []shapedGlyph{{
			GlyphID:      1,
			Rune:         'j',
			Width:        500,
			Advance:      500,
			HasAdvance:   true,
			InkLeft:      0,
			InkRight:     550,
			HasInkBounds: true,
		}}},
		BreakStats: paragraphLineBreakStats{AvailableWidth: 5},
	}
	if got, want := pdfPageLineVisualOverflow(line), 0.5; got != want {
		t.Fatalf("visual overflow = %v, want %v", got, want)
	}
	if got := pdfPageLineOverflow(line); got != 0 {
		t.Fatalf("advance overflow = %v, want zero", got)
	}
}

func TestPDFPageLineOverflowUsesTolerance(t *testing.T) {
	line := pdfPageLine{
		FontSize: 10,
		Text:     shapedText{Glyphs: []shapedGlyph{{GlyphID: 1, Rune: 'A', Width: 500}}},
		BreakStats: paragraphLineBreakStats{
			AvailableWidth: 4.9995,
		},
	}
	if got := pdfPageLineOverflow(line); got != 0 {
		t.Fatalf("overflow = %v, want tolerated zero overflow", got)
	}

	line.BreakStats.AvailableWidth = 4.5
	if got, want := pdfPageLineOverflow(line), 0.5; got != want {
		t.Fatalf("overflow = %v, want %v", got, want)
	}
}
