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
		width += pdfPageFragmentAdvance(line, fragment, i != len(line.Fragments)-1)
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

func pdfPageLineVisualOverflow(line pdfPageLine) float64 {
	available := pdfPageLineAvailableWidth(line)
	if available <= 0 {
		return 0
	}
	return pdfPageLineVisualOverflowForAvailable(line, available)
}

func pdfPageLineXAdjustedForVisualRight(line pdfPageLine, available float64) float64 {
	if available <= 0 {
		return line.X
	}
	overflow := pdfPageLineVisualOverflowForAvailable(line, available)
	if overflow <= 0 {
		return line.X
	}
	visualLeft, _, ok := pdfPageLineVisualBounds(line)
	if !ok {
		return line.X
	}
	// Preserve the intended left edge. A right-edge safety correction may consume
	// blank left-side ink slack (for example after leading whitespace), but it must
	// not move visible text to the left of the line origin; justified text needs
	// both edges to remain stable.
	leftSlack := max(visualLeft-line.X, 0)
	if leftSlack <= 0 {
		return line.X
	}
	return line.X - min(overflow, leftSlack)
}

func pdfPageLineVisualOverflowForAvailable(line pdfPageLine, available float64) float64 {
	_, visualRight, ok := pdfPageLineVisualBounds(line)
	if !ok {
		return 0
	}
	overflow := visualRight - (line.X + available)
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
		currentX += pdfPageFragmentAdvance(line, fragment, i != len(line.Fragments)-1)
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

func pdfPageLineJustificationSpaceCount(line pdfPageLine) int {
	if len(line.Fragments) == 0 {
		return justificationSpaceCount(line.Text.Glyphs)
	}
	count := 0
	for i, fragment := range line.Fragments {
		count += pdfPageFragmentJustificationSpaceCount(fragment, i != len(line.Fragments)-1)
	}
	return count
}

func pdfPageFragmentAdvance(line pdfPageLine, fragment pdfPageLineFragment, includeTrailing bool) float64 {
	advance := fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
	if includeTrailing {
		advance += line.ExtraCharSpacing
	}
	advance += line.ExtraWordSpacing * float64(pdfPageFragmentJustificationSpaceCount(fragment, includeTrailing))
	return advance
}

func paragraphFragmentAdvance(line paragraphLine, fragment paragraphLineFragment, includeTrailing bool) float64 {
	advance := fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
	if includeTrailing {
		advance += line.ExtraCharSpacing
	}
	advance += line.ExtraWordSpacing * float64(paragraphFragmentJustificationSpaceCount(fragment, includeTrailing))
	return advance
}

func pdfPageFragmentJustificationSpaceCount(fragment pdfPageLineFragment, includeTrailing bool) int {
	count := justificationSpaceCount(fragment.Text.Glyphs)
	if includeTrailing && fragmentEndsWithSpace(fragment) {
		count++
	}
	return count
}

func paragraphFragmentJustificationSpaceCount(fragment paragraphLineFragment, includeTrailing bool) int {
	count := justificationSpaceCount(fragment.Text.Glyphs)
	if includeTrailing && paragraphFragmentEndsWithSpace(fragment) {
		count++
	}
	return count
}
