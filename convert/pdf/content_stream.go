package pdf

import (
	"bytes"
	"fmt"

	"fbc/convert/pdf/docwriter"
)

type pdfMissingGlyphBox struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
	Color  pdfColor
}

type pdfTextDrawingState struct {
	FontName         string
	FontSize         float64
	LetterSpacing    float64
	Color            pdfColor
	ColorInitialized bool
}

func pageContent(page pdfPage) []byte {
	var buf bytes.Buffer
	missingGlyphBoxes := make([]pdfMissingGlyphBox, 0)
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
	for _, stroke := range page.Strokes {
		if stroke.LineWidth <= 0 || (stroke.X1 == stroke.X2 && stroke.Y1 == stroke.Y2) {
			continue
		}
		fmt.Fprintf(&buf, "q\n%s\n%s w\n1 J\n%s %s m %s %s l S\nQ\n",
			stroke.Color.strokeOperator(),
			docwriter.FormatNumber(stroke.LineWidth),
			docwriter.FormatNumber(stroke.X1),
			docwriter.FormatNumber(stroke.Y1),
			docwriter.FormatNumber(stroke.X2),
			docwriter.FormatNumber(stroke.Y2))
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
	textState := pdfTextDrawingState{FontSize: -1}
	for _, line := range page.Lines {
		if len(line.Fragments) != 0 {
			currentX := line.X
			for i, fragment := range line.Fragments {
				if len(fragment.Text.Glyphs) != 0 && fragment.FontName != "" {
					missingGlyphBoxes = append(missingGlyphBoxes, writeTextFragment(
						&buf,
						fragment.FontName,
						fragment.FontSize,
						fragment.LetterSpacing+line.ExtraCharSpacing,
						fragment.Color,
						currentX,
						line.Y+fragment.BaselineShift,
						fragment.Text.Glyphs,
						&textState,
					)...)
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
		missingGlyphBoxes = append(missingGlyphBoxes, writeTextGlyphs(
			&buf,
			line.FontName,
			line.FontSize,
			line.LetterSpacing+line.ExtraCharSpacing,
			line.Color,
			line.X,
			line.Y,
			line.Text.Glyphs,
			line.ExtraWordSpacing,
			&textState,
		)...)
	}
	buf.WriteString("ET\nQ\n")
	buf.Write(pageMissingGlyphBoxes(missingGlyphBoxes))
	buf.Write(pageTextDecorations(page))
	return buf.Bytes()
}

func writeTextFragment(
	buf *bytes.Buffer,
	fontName string,
	fontSize float64,
	letterSpacing float64,
	color pdfColor,
	x float64,
	y float64,
	glyphs []shapedGlyph,
	state *pdfTextDrawingState,
) []pdfMissingGlyphBox {
	return writeTextGlyphs(buf, fontName, fontSize, letterSpacing, color, x, y, glyphs, 0, state)
}

func writeTextGlyphs(
	buf *bytes.Buffer,
	fontName string,
	fontSize float64,
	letterSpacing float64,
	color pdfColor,
	x float64,
	y float64,
	glyphs []shapedGlyph,
	extraWordSpacing float64,
	state *pdfTextDrawingState,
) []pdfMissingGlyphBox {
	writeTextState(buf, fontName, fontSize, letterSpacing, color, x, y, state)
	if !hasSyntheticPDFGlyphs(glyphs) {
		if needsPositionedGlyphArray(glyphs, extraWordSpacing) {
			fmt.Fprintf(buf, "%s TJ\n", positionedGlyphArray(glyphs, extraWordSpacing, fontSize))
			return nil
		}
		fmt.Fprintf(buf, "%s Tj\n", docwriter.Format(glyphHex(glyphs)))
		return nil
	}
	glyphArray, boxes := syntheticGlyphArray(glyphs, fontSize, letterSpacing, extraWordSpacing, x, y, color)
	if glyphArray != "" {
		fmt.Fprintf(buf, "%s TJ\n", glyphArray)
	}
	return boxes
}

func hasSyntheticPDFGlyphs(glyphs []shapedGlyph) bool {
	for _, glyph := range glyphs {
		if glyph.Missing != pdfMissingGlyphNone || glyph.GlyphID == 0 {
			return true
		}
	}
	return false
}

func syntheticGlyphArray(
	glyphs []shapedGlyph,
	fontSize float64,
	letterSpacing float64,
	extraWordSpacing float64,
	startX float64,
	baselineY float64,
	color pdfColor,
) (string, []pdfMissingGlyphBox) {
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	boxes := make([]pdfMissingGlyphBox, 0)
	var buf bytes.Buffer
	buf.WriteByte('[')
	wroteItem := false
	x := startX
	for i, glyph := range glyphs {
		if glyph.Missing == pdfMissingGlyphNone && glyph.GlyphID != 0 {
			writePDFGlyphArrayItem(&buf, &wroteItem, docwriter.Format(glyphHex([]shapedGlyph{glyph})))
			if delta := shapedAdvanceAdjustment(glyph); delta != 0 && i != len(glyphs)-1 {
				writePDFGlyphArrayItem(&buf, &wroteItem, docwriter.FormatNumber(float64(delta)))
			}
			x += glyphAdvancePoints(glyph, fontSize)
		} else {
			if glyph.Missing == pdfMissingGlyphPrintable && glyph.Width > 0 {
				boxes = append(boxes, missingPDFGlyphBox(glyph, fontSize, x, baselineY, color))
			}
			advance := glyphAdvancePoints(glyph, fontSize)
			if advance != 0 {
				writePDFGlyphArrayItem(&buf, &wroteItem, docwriter.FormatNumber(-advance*1000/fontSize))
			}
			x += advance
		}
		if i != len(glyphs)-1 && letterSpacing != 0 && (glyph.Missing != pdfMissingGlyphNone || glyph.GlyphID == 0) {
			writePDFGlyphArrayItem(&buf, &wroteItem, docwriter.FormatNumber(-letterSpacing*1000/fontSize))
			x += letterSpacing
		}
		if glyph.Rune == ' ' && i != len(glyphs)-1 && extraWordSpacing != 0 {
			writePDFGlyphArrayItem(&buf, &wroteItem, docwriter.FormatNumber(-extraWordSpacing*1000/fontSize))
			x += extraWordSpacing
		}
	}
	buf.WriteByte(']')
	if !wroteItem {
		return "", boxes
	}
	return buf.String(), boxes
}

func writePDFGlyphArrayItem(buf *bytes.Buffer, wroteItem *bool, item string) {
	if *wroteItem {
		buf.WriteByte(' ')
	}
	buf.WriteString(item)
	*wroteItem = true
}

func missingPDFGlyphBox(glyph shapedGlyph, fontSize float64, x float64, baselineY float64, color pdfColor) pdfMissingGlyphBox {
	advance := glyphAdvancePoints(glyph, fontSize)
	if advance <= 0 {
		advance = fontSize * 0.5
	}
	return pdfMissingGlyphBox{
		X:      x,
		Y:      baselineY - fontSize*0.25,
		Width:  advance,
		Height: fontSize,
		Color:  color,
	}
}

func glyphAdvancePoints(glyph shapedGlyph, fontSize float64) float64 {
	return float64(shapedGlyphAdvanceWidth(glyph)) * fontSize / 1000.0
}

func pageMissingGlyphBoxes(boxes []pdfMissingGlyphBox) []byte {
	if len(boxes) == 0 {
		return nil
	}
	var buf bytes.Buffer
	for _, box := range boxes {
		if box.Width <= 0 || box.Height <= 0 {
			continue
		}
		thickness := max(min(box.Width, box.Height)/8, 0.35)
		x1 := box.X
		y1 := box.Y
		x2 := box.X + box.Width
		y2 := box.Y + box.Height
		fmt.Fprintf(&buf, "q\n%s\n%s w\n%s %s %s %s re S\n%s %s m %s %s l S\n%s %s m %s %s l S\nQ\n",
			box.Color.strokeOperator(),
			docwriter.FormatNumber(thickness),
			docwriter.FormatNumber(box.X),
			docwriter.FormatNumber(box.Y),
			docwriter.FormatNumber(box.Width),
			docwriter.FormatNumber(box.Height),
			docwriter.FormatNumber(x1),
			docwriter.FormatNumber(y1),
			docwriter.FormatNumber(x2),
			docwriter.FormatNumber(y2),
			docwriter.FormatNumber(x1),
			docwriter.FormatNumber(y2),
			docwriter.FormatNumber(x2),
			docwriter.FormatNumber(y1))
	}
	return buf.Bytes()
}

func writeTextState(
	buf *bytes.Buffer,
	fontName string,
	fontSize float64,
	letterSpacing float64,
	color pdfColor,
	x float64,
	y float64,
	state *pdfTextDrawingState,
) {
	if fontName != state.FontName || fontSize != state.FontSize {
		fmt.Fprintf(buf, "/%s %s Tf\n", fontName, docwriter.FormatNumber(fontSize))
		state.FontName = fontName
		state.FontSize = fontSize
	}
	if letterSpacing != state.LetterSpacing {
		fmt.Fprintf(buf, "%s Tc\n", docwriter.FormatNumber(letterSpacing))
		state.LetterSpacing = letterSpacing
	}
	if !state.ColorInitialized || color != state.Color {
		fmt.Fprintf(buf, "%s\n", color.contentOperator())
		state.Color = color
		state.ColorInitialized = true
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

func needsPositionedGlyphArray(glyphs []shapedGlyph, extraWordSpacing float64) bool {
	if extraWordSpacing != 0 {
		return true
	}
	for _, glyph := range glyphs {
		if shapedAdvanceAdjustment(glyph) != 0 {
			return true
		}
	}
	return false
}

func shapedAdvanceAdjustment(glyph shapedGlyph) int {
	return glyph.Width - shapedGlyphAdvanceWidth(glyph)
}

func positionedGlyphArray(glyphs []shapedGlyph, extraWordSpacing, fontSize float64) string {
	wordSpacingAdjustment := -extraWordSpacing * 1000 / fontSize
	var buf bytes.Buffer
	buf.WriteByte('[')
	wroteItem := false
	for i, glyph := range glyphs {
		writePDFGlyphArrayItem(&buf, &wroteItem, docwriter.Format(glyphHex([]shapedGlyph{glyph})))
		if i == len(glyphs)-1 {
			continue
		}
		adjustment := float64(shapedAdvanceAdjustment(glyph))
		if glyph.Rune == ' ' {
			adjustment += wordSpacingAdjustment
		}
		if adjustment != 0 {
			writePDFGlyphArrayItem(&buf, &wroteItem, docwriter.FormatNumber(adjustment))
		}
	}
	buf.WriteByte(']')
	return buf.String()
}
