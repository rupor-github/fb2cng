package pdf

import "strings"

func shapedRunes(text shapedText) string {
	var b strings.Builder
	for _, glyph := range text.Glyphs {
		b.WriteString(glyphUnicodeText(glyph))
	}
	return b.String()
}
