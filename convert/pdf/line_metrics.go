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

func pdfPageLineIsJustified(line pdfPageLine) bool {
	return line.ExtraWordSpacing != 0 || line.ExtraCharSpacing != 0
}
