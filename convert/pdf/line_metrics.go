package pdf

import "math"

const pdfLineWidthTolerance = 0.001

func pdfPageLineAdvanceWidth(line pdfPageLine) float64 {
	if len(line.Fragments) == 0 {
		return shapedWidthPointsWithSpacing(line.Text, line.FontSize, line.LetterSpacing)
	}
	width := 0.0
	for _, fragment := range line.Fragments {
		width += fragment.Width
	}
	return width
}

func pdfPageLineDrawnWidth(line pdfPageLine) float64 {
	if len(line.Fragments) == 0 {
		width := pdfPageLineAdvanceWidth(line)
		width += line.ExtraCharSpacing * float64(max(len(line.Text.Glyphs)-1, 0))
		width += line.ExtraWordSpacing * float64(justificationSpaceCount(line.Text.Glyphs))
		return width
	}

	width := 0.0
	for i, fragment := range line.Fragments {
		width += fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
		if i != len(line.Fragments)-1 {
			width += line.ExtraCharSpacing
		}
		if line.ExtraWordSpacing != 0 && i != len(line.Fragments)-1 && fragmentEndsWithSpace(fragment) {
			width += line.ExtraWordSpacing
		}
	}
	return width
}

func pdfPageLineAvailableWidth(line pdfPageLine) float64 {
	available := line.BreakStats.AvailableWidth
	if available <= 0 || math.IsInf(available, 0) || math.IsNaN(available) {
		return 0
	}
	return available
}

func pdfPageLineOverflow(line pdfPageLine) float64 {
	available := pdfPageLineAvailableWidth(line)
	if available <= 0 {
		return 0
	}
	overflow := pdfPageLineDrawnWidth(line) - available
	if overflow <= pdfLineWidthTolerance {
		return 0
	}
	return overflow
}

func pdfPageLineVisualBounds(line pdfPageLine) (float64, float64, bool) {
	if len(line.Fragments) == 0 {
		left, right, ok := shapedTextVisualBounds(
			line.Text,
			line.FontSize,
			line.LetterSpacing+line.ExtraCharSpacing,
			line.ExtraWordSpacing,
		)
		return line.X + left, line.X + right, ok
	}

	left := math.Inf(1)
	right := math.Inf(-1)
	ok := false
	currentX := line.X
	for i, fragment := range line.Fragments {
		fragmentLeft, fragmentRight, fragmentOK := shapedTextVisualBounds(
			fragment.Text,
			fragment.FontSize,
			fragment.LetterSpacing+line.ExtraCharSpacing,
			line.ExtraWordSpacing,
		)
		if fragmentOK {
			left = min(left, currentX+fragmentLeft)
			right = max(right, currentX+fragmentRight)
			ok = true
		}
		currentX += fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
		if i != len(line.Fragments)-1 {
			currentX += line.ExtraCharSpacing
		}
		if line.ExtraWordSpacing != 0 && i != len(line.Fragments)-1 && fragmentEndsWithSpace(fragment) {
			currentX += line.ExtraWordSpacing
		}
	}
	return left, right, ok
}

func shapedTextVisualBounds(text shapedText, fontSize float64, letterSpacing float64, extraWordSpacing float64) (float64, float64, bool) {
	left := math.Inf(1)
	right := math.Inf(-1)
	currentX := 0.0
	ok := false
	for i, glyph := range text.Glyphs {
		glyphLeft, glyphRight := shapedGlyphInkBounds(glyph)
		if glyphLeft != glyphRight {
			left = min(left, currentX+float64(glyphLeft)*fontSize/1000.0)
			right = max(right, currentX+float64(glyphRight)*fontSize/1000.0)
			ok = true
		}
		currentX += glyphAdvancePoints(glyph, fontSize)
		if i != len(text.Glyphs)-1 {
			currentX += letterSpacing
		}
		if glyph.Rune == ' ' && i != len(text.Glyphs)-1 {
			currentX += extraWordSpacing
		}
	}
	return left, right, ok
}

func pdfPageLineIsJustified(line pdfPageLine) bool {
	return line.ExtraWordSpacing != 0 || line.ExtraCharSpacing != 0
}
