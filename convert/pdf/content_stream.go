package pdf

import (
	"bytes"
	"fmt"

	"fbc/convert/pdf/docwriter"
)

func pageContent(page pdfPage) []byte {
	var buf bytes.Buffer
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
	currentFontSize := -1.0
	for _, line := range page.Lines {
		if len(line.Text.Glyphs) == 0 {
			continue
		}
		if line.FontSize != currentFontSize {
			fmt.Fprintf(&buf, "/F1 %s Tf\n", docwriter.FormatNumber(line.FontSize))
			currentFontSize = line.FontSize
		}
		fmt.Fprintf(&buf, "1 0 0 1 %s %s Tm\n", docwriter.FormatNumber(line.X), docwriter.FormatNumber(line.Y))
		if line.ExtraWordSpacing != 0 {
			fmt.Fprintf(&buf, "%s TJ\n", justifiedGlyphArray(line.Text.Glyphs, line.ExtraWordSpacing, line.FontSize))
			continue
		}
		fmt.Fprintf(&buf, "%s Tj\n", docwriter.Format(glyphHex(line.Text.Glyphs)))
	}
	buf.WriteString("ET\nQ\n")
	return buf.Bytes()
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
