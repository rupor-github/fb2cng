package pdf

func pdfPageLineWithFontFragments(line pdfPageLine) pdfPageLine {
	if len(line.Fragments) == 0 {
		if !shapedTextUsesMultiplePDFFonts(line.Text, line.FontKey) {
			return line
		}
		line.Fragments = splitPDFPageLineFragmentByFont(pdfPageLineFragment{
			Text:          line.Text,
			Width:         line.WidthFromText(),
			FontSize:      line.FontSize,
			LetterSpacing: line.LetterSpacing,
			FontKey:       line.FontKey,
			Color:         line.Color,
			Underline:     line.Underline,
			Strikethrough: line.Strikethrough,
		})
		return line
	}
	fragments := make([]pdfPageLineFragment, 0, len(line.Fragments))
	for _, fragment := range line.Fragments {
		fragments = append(fragments, splitPDFPageLineFragmentByFont(fragment)...)
	}
	line.Fragments = fragments
	return line
}

func (line pdfPageLine) WidthFromText() float64 {
	return shapedWidthPointsWithSpacing(line.Text, line.FontSize, line.LetterSpacing)
}

func shapedTextUsesMultiplePDFFonts(text shapedText, defaultKey pdfFontKey) bool {
	for _, glyph := range text.Glyphs {
		if glyph.Missing != pdfMissingGlyphNone || glyph.GlyphID == 0 {
			continue
		}
		if pdfActualGlyphFontKey(glyph, defaultKey) != defaultKey {
			return true
		}
	}
	return false
}

func splitPDFPageLineFragmentByFont(fragment pdfPageLineFragment) []pdfPageLineFragment {
	if !shapedTextUsesMultiplePDFFonts(fragment.Text, fragment.FontKey) {
		return []pdfPageLineFragment{fragment}
	}
	out := make([]pdfPageLineFragment, 0, 2)
	start := 0
	for start < len(fragment.Text.Glyphs) {
		key := pdfActualGlyphFontKey(fragment.Text.Glyphs[start], fragment.FontKey)
		end := start + 1
		for end < len(fragment.Text.Glyphs) && pdfActualGlyphFontKey(fragment.Text.Glyphs[end], fragment.FontKey) == key {
			end++
		}
		out = append(out, pdfPageLineFontFragment(fragment, key, fragment.Text.Glyphs[start:end], end < len(fragment.Text.Glyphs)))
		start = end
	}
	return out
}

func pdfPageLineFontFragment(template pdfPageLineFragment, key pdfFontKey, glyphs []shapedGlyph, hasFollowingGlyph bool) pdfPageLineFragment {
	used := make(map[uint16]shapedGlyph)
	for _, glyph := range glyphs {
		if glyph.GlyphID != 0 && glyph.Missing == pdfMissingGlyphNone {
			used[glyph.GlyphID] = glyph
		}
	}
	fragment := template
	fragment.FontKey = key
	fragment.Text = shapedText{Glyphs: glyphs, Used: used}
	fragment.Width = shapedWidthPointsWithSpacing(fragment.Text, fragment.FontSize, fragment.LetterSpacing)
	if hasFollowingGlyph && fragment.LetterSpacing != 0 {
		fragment.Width += fragment.LetterSpacing
	}
	return fragment
}

func collectPDFLineUsedGlyphs(used map[pdfFontKey]map[uint16]shapedGlyph, line pdfPageLine) {
	if len(line.Fragments) != 0 {
		for _, fragment := range line.Fragments {
			collectPDFTextUsedGlyphs(used, fragment.Text, fragment.FontKey)
		}
		return
	}
	collectPDFTextUsedGlyphs(used, line.Text, line.FontKey)
}

func collectPDFTextUsedGlyphs(used map[pdfFontKey]map[uint16]shapedGlyph, text shapedText, defaultKey pdfFontKey) {
	if defaultKey.Family == "" {
		defaultKey = pdfFontKey{Family: "serif"}
	}
	for _, glyph := range text.Glyphs {
		if glyph.GlyphID == 0 || glyph.Missing != pdfMissingGlyphNone {
			continue
		}
		key := pdfActualGlyphFontKey(glyph, defaultKey)
		fontUsed := used[key]
		if fontUsed == nil {
			fontUsed = make(map[uint16]shapedGlyph)
			used[key] = fontUsed
		}
		fontUsed[glyph.GlyphID] = glyph
	}
}

func pdfActualGlyphFontKey(glyph shapedGlyph, defaultKey pdfFontKey) pdfFontKey {
	if glyph.FontKey.Family != "" {
		return glyph.FontKey
	}
	return defaultKey
}
