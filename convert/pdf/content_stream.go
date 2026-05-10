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
		if len(line.Text.Glyphs) == 0 || line.FontName == "" {
			continue
		}
		if line.FontName != currentFontName || line.FontSize != currentFontSize {
			fmt.Fprintf(&buf, "/%s %s Tf\n", line.FontName, docwriter.FormatNumber(line.FontSize))
			currentFontName = line.FontName
			currentFontSize = line.FontSize
		}
		if line.LetterSpacing != currentLetterSpacing {
			fmt.Fprintf(&buf, "%s Tc\n", docwriter.FormatNumber(line.LetterSpacing))
			currentLetterSpacing = line.LetterSpacing
		}
		if !colorInitialized || line.Color != currentColor {
			fmt.Fprintf(&buf, "%s\n", line.Color.contentOperator())
			currentColor = line.Color
			colorInitialized = true
		}
		fmt.Fprintf(&buf, "1 0 0 1 %s %s Tm\n", docwriter.FormatNumber(line.X), docwriter.FormatNumber(line.Y))
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

func pageTextDecorations(page pdfPage) []byte {
	var buf bytes.Buffer
	for _, line := range page.Lines {
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

func writeDecorationLine(buf *bytes.Buffer, x1, y, x2 float64) {
	fmt.Fprintf(buf, "%s %s m %s %s l S\n",
		docwriter.FormatNumber(x1),
		docwriter.FormatNumber(y),
		docwriter.FormatNumber(x2),
		docwriter.FormatNumber(y))
}

func decoratedLineWidth(line pdfPageLine) float64 {
	width := shapedWidthPointsWithSpacing(line.Text, line.FontSize, line.LetterSpacing)
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
