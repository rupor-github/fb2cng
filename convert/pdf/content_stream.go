package pdf

import (
	"bytes"
	"fmt"

	"fbc/convert/pdf/docwriter"
)

func pageContent(page pdfPage) []byte {
	var buf bytes.Buffer
	for _, background := range page.Backgrounds {
		if background.Width <= 0 || background.Height <= 0 {
			continue
		}
		fmt.Fprintf(&buf, "q\n%s\n%s %s %s %s re f\nQ\n",
			background.Color.contentOperator(),
			docwriter.FormatNumber(background.X),
			docwriter.FormatNumber(background.Y),
			docwriter.FormatNumber(background.Width),
			docwriter.FormatNumber(background.Height))
	}
	for _, border := range page.Borders {
		if border.Width <= 0 || border.Height <= 0 || border.LineWidth <= 0 {
			continue
		}
		fmt.Fprintf(&buf, "q\n%s\n%s w\n%s %s %s %s re S\nQ\n",
			border.Color.strokeOperator(),
			docwriter.FormatNumber(border.LineWidth),
			docwriter.FormatNumber(border.X),
			docwriter.FormatNumber(border.Y),
			docwriter.FormatNumber(border.Width),
			docwriter.FormatNumber(border.Height))
	}
	for _, img := range page.Images {
		if img.Name == "" || img.Width <= 0 || img.Height <= 0 {
			continue
		}
		fmt.Fprintf(&buf, "q\n%s 0 0 %s %s %s cm\n/%s Do\nQ\n",
			docwriter.FormatNumber(img.Width),
			docwriter.FormatNumber(img.Height),
			docwriter.FormatNumber(img.X),
			docwriter.FormatNumber(img.Y),
			img.Name)
	}
	buf.WriteString("q\nBT\n")
	currentFontName := ""
	currentFontSize := -1.0
	currentLetterSpacing := 0.0
	currentColor := pdfColor{}
	colorInitialized := false
	for _, line := range page.Lines {
		if len(line.Fragments) != 0 {
			currentX := line.X
			for i, fragment := range line.Fragments {
				if len(fragment.Text.Glyphs) != 0 && fragment.FontName != "" {
					writeTextFragment(&buf, fragment.FontName, fragment.FontSize, fragment.LetterSpacing+line.ExtraCharSpacing, fragment.Color, currentX, line.Y+fragment.BaselineShift, fragment.Text.Glyphs, &currentFontName, &currentFontSize, &currentLetterSpacing, &currentColor, &colorInitialized)
				}
				currentX += fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
				if i != len(line.Fragments)-1 {
					currentX += line.ExtraCharSpacing
				}
				if line.ExtraWordSpacing != 0 && i != len(line.Fragments)-1 && fragmentEndsWithSpace(fragment) {
					currentX += line.ExtraWordSpacing
				}
			}
			continue
		}
		if len(line.Text.Glyphs) == 0 || line.FontName == "" {
			continue
		}
		writeTextState(&buf, line.FontName, line.FontSize, line.LetterSpacing+line.ExtraCharSpacing, line.Color, line.X, line.Y, &currentFontName, &currentFontSize, &currentLetterSpacing, &currentColor, &colorInitialized)
		if line.ExtraWordSpacing != 0 {
			fmt.Fprintf(&buf, "%s TJ\n", justifiedGlyphArray(line.Text.Glyphs, line.ExtraWordSpacing, line.FontSize))
			continue
		}
		fmt.Fprintf(&buf, "%s Tj\n", docwriter.Format(glyphHex(line.Text.Glyphs)))
	}
	buf.WriteString("ET\nQ\n")
	buf.Write(pageTextDecorations(page))
	return buf.Bytes()
}

func writeTextFragment(buf *bytes.Buffer, fontName string, fontSize float64, letterSpacing float64, color pdfColor, x float64, y float64, glyphs []shapedGlyph, currentFontName *string, currentFontSize *float64, currentLetterSpacing *float64, currentColor *pdfColor, colorInitialized *bool) {
	writeTextState(buf, fontName, fontSize, letterSpacing, color, x, y, currentFontName, currentFontSize, currentLetterSpacing, currentColor, colorInitialized)
	fmt.Fprintf(buf, "%s Tj\n", docwriter.Format(glyphHex(glyphs)))
}

func writeTextState(buf *bytes.Buffer, fontName string, fontSize float64, letterSpacing float64, color pdfColor, x float64, y float64, currentFontName *string, currentFontSize *float64, currentLetterSpacing *float64, currentColor *pdfColor, colorInitialized *bool) {
	if fontName != *currentFontName || fontSize != *currentFontSize {
		fmt.Fprintf(buf, "/%s %s Tf\n", fontName, docwriter.FormatNumber(fontSize))
		*currentFontName = fontName
		*currentFontSize = fontSize
	}
	if letterSpacing != *currentLetterSpacing {
		fmt.Fprintf(buf, "%s Tc\n", docwriter.FormatNumber(letterSpacing))
		*currentLetterSpacing = letterSpacing
	}
	if !*colorInitialized || color != *currentColor {
		fmt.Fprintf(buf, "%s\n", color.contentOperator())
		*currentColor = color
		*colorInitialized = true
	}
	fmt.Fprintf(buf, "1 0 0 1 %s %s Tm\n", docwriter.FormatNumber(x), docwriter.FormatNumber(y))
}

func fragmentEndsWithSpace(fragment pdfPageLineFragment) bool {
	glyphs := fragment.Text.Glyphs
	return len(glyphs) != 0 && glyphs[len(glyphs)-1].Rune == ' '
}

func pageTextDecorations(page pdfPage) []byte {
	var buf bytes.Buffer
	for _, line := range page.Lines {
		if len(line.Fragments) != 0 {
			writeFragmentDecorations(&buf, line)
			continue
		}
		if len(line.Text.Glyphs) == 0 || (!line.Underline && !line.Strikethrough) {
			continue
		}
		width := decoratedLineWidth(line)
		if width <= 0 {
			continue
		}
		thickness := max(line.FontSize/18, 0.4)
		fmt.Fprintf(&buf, "q\n%s\n%s w\n", line.Color.strokeOperator(), docwriter.FormatNumber(thickness))
		if line.Underline {
			y := line.Y - line.FontSize*0.12
			writeDecorationLine(&buf, line.X, y, line.X+width)
		}
		if line.Strikethrough {
			y := line.Y + line.FontSize*0.30
			writeDecorationLine(&buf, line.X, y, line.X+width)
		}
		buf.WriteString("Q\n")
	}
	return buf.Bytes()
}

func writeFragmentDecorations(buf *bytes.Buffer, line pdfPageLine) {
	currentX := line.X
	for i, fragment := range line.Fragments {
		if fragment.Width > 0 && (fragment.Underline || fragment.Strikethrough) && (len(fragment.Text.Glyphs) != 0 || fragment.ImageID != "") {
			thickness := max(fragment.FontSize/18, 0.4)
			fmt.Fprintf(buf, "q\n%s\n%s w\n", fragment.Color.strokeOperator(), docwriter.FormatNumber(thickness))
			if fragment.Underline {
				y := fragmentUnderlineY(line, fragment)
				writeDecorationLine(buf, currentX, y, currentX+fragment.Width)
			}
			if fragment.Strikethrough {
				y := fragmentStrikethroughY(line, fragment)
				writeDecorationLine(buf, currentX, y, currentX+fragment.Width)
			}
			buf.WriteString("Q\n")
		}
		currentX += fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
		if i != len(line.Fragments)-1 {
			currentX += line.ExtraCharSpacing
		}
		if line.ExtraWordSpacing != 0 && i != len(line.Fragments)-1 && fragmentEndsWithSpace(fragment) {
			currentX += line.ExtraWordSpacing
		}
	}
}

func fragmentUnderlineY(line pdfPageLine, fragment pdfPageLineFragment) float64 {
	return line.Y + fragment.BaselineShift - fragment.FontSize*0.12
}

func fragmentStrikethroughY(line pdfPageLine, fragment pdfPageLineFragment) float64 {
	if fragment.ImageID != "" && fragment.ImageHeight > 0 {
		return line.Y + fragment.BaselineShift + fragment.ImageHeight*0.5
	}
	return line.Y + fragment.BaselineShift + fragment.FontSize*0.30
}

func writeDecorationLine(buf *bytes.Buffer, x1, y, x2 float64) {
	fmt.Fprintf(buf, "%s %s m %s %s l S\n",
		docwriter.FormatNumber(x1),
		docwriter.FormatNumber(y),
		docwriter.FormatNumber(x2),
		docwriter.FormatNumber(y))
}

func decoratedLineWidth(line pdfPageLine) float64 {
	width := shapedWidthPointsWithSpacing(line.Text, line.FontSize, line.LetterSpacing)
	width += line.ExtraCharSpacing * float64(max(len(line.Text.Glyphs)-1, 0))
	if line.ExtraWordSpacing == 0 {
		return width
	}
	return width + line.ExtraWordSpacing*float64(justificationSpaceCount(line.Text.Glyphs))
}

func justificationSpaceCount(glyphs []shapedGlyph) int {
	count := 0
	for i, glyph := range glyphs {
		if glyph.Rune == ' ' && i != len(glyphs)-1 {
			count++
		}
	}
	return count
}

func justifiedGlyphArray(glyphs []shapedGlyph, extraWordSpacing, fontSize float64) string {
	adjustment := -extraWordSpacing * 1000 / fontSize
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, glyph := range glyphs {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(docwriter.Format(glyphHex([]shapedGlyph{glyph})))
		if glyph.Rune == ' ' && i != len(glyphs)-1 {
			buf.WriteByte(' ')
			buf.WriteString(docwriter.FormatNumber(adjustment))
		}
	}
	buf.WriteByte(']')
	return buf.String()
}
