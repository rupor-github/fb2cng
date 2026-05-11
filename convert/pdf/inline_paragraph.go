package pdf

import (
	"fmt"
	"strings"
)

const (
	pdfInlineScriptScale     = 0.72
	pdfInlineSuperscriptRise = 0.34
	pdfInlineSubscriptDrop   = 0.18
)

type paragraphInlineWord struct {
	Text      string
	Fragments []paragraphLineFragment
	Width     float64
}

type inlineGlyphPiece struct {
	Glyph    shapedGlyph
	Template paragraphLineFragment
}

func layoutInlineParagraph(registry *pdfFontRegistry, resolver *pdfStyleResolver, baseFace *builtinFontFace, text string, runs []pdfInlineRun, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	if len(runs) == 0 {
		runs = []pdfInlineRun{{Text: text}}
	}
	if !hasInlineStyle(runs) {
		return layoutParagraph(baseFace, plainInlineRunText(runs), style, maxWidth)
	}
	if style.FontSize <= 0 {
		return nil, fmt.Errorf("paragraph font size must be positive: %g", style.FontSize)
	}
	if maxWidth <= 0 {
		return nil, fmt.Errorf("paragraph width must be positive: %g", maxWidth)
	}

	words, err := inlineParagraphWords(registry, resolver, runs, style)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, nil
	}
	spaceFragment, err := inlineParagraphSpace(registry, style)
	if err != nil {
		return nil, err
	}
	units, err := inlineParagraphUnits(registry, words, style)
	if err != nil {
		return nil, err
	}
	breaks := chooseParagraphBreaks(units, spaceFragment.Width, style, maxWidth)
	lines := make([]paragraphLine, 0, len(breaks))
	start := 0
	previousHyphenated := false
	previousFitness := paragraphFitnessDecent
	for i, br := range breaks {
		fragments, lineText, width := inlineParagraphLineFragments(units[start:br.End], spaceFragment, br.HyphenAfter)
		shaped, err := shapeText(baseFace, lineText)
		if err != nil {
			return nil, fmt.Errorf("shape inline line text: %w", err)
		}
		indent := paragraphLineIndent(start, style, maxWidth)
		available := max(maxWidth-indent, 1)
		line := paragraphLine{
			Text:              shaped,
			Width:             width,
			Indent:            indent,
			JustificationGaps: countJustificationGaps(units[start:br.End]),
			Fragments:         fragments,
		}
		last := i == len(breaks)-1
		singleWord := units[start].WordIndex == units[br.End-1].WordIndex
		line.BreakStats = paragraphLineBreakStatsFor(width, available, line.JustificationGaps, start == 0, last, singleWord, br.HyphenAfter, previousHyphenated, previousFitness)
		line.ExtraWordSpacing, line.ExtraCharSpacing = paragraphJustificationSpacing(style, last, width, available, line.JustificationGaps, len(shaped.Glyphs))
		lines = append(lines, line)
		previousHyphenated = br.HyphenAfter
		previousFitness = line.BreakStats.Fitness
		start = br.End
	}
	return lines, nil
}

func hasInlineStyle(runs []pdfInlineRun) bool {
	for _, run := range runs {
		if run.StyleClasses != "" || run.Bold || run.Italic || run.Strikethrough || run.Subscript || run.Superscript || run.Code {
			return true
		}
	}
	return false
}

func plainInlineRunText(runs []pdfInlineRun) string {
	var b strings.Builder
	for _, run := range runs {
		b.WriteString(run.Text)
	}
	return strings.TrimSpace(b.String())
}

func inlineParagraphWords(registry *pdfFontRegistry, resolver *pdfStyleResolver, runs []pdfInlineRun, base paragraphStyle) ([]paragraphInlineWord, error) {
	words := make([]paragraphInlineWord, 0)
	current := paragraphInlineWord{}
	flushCurrent := func() {
		if strings.TrimSpace(strings.ReplaceAll(current.Text, string(softHyphen), "")) == "" {
			current = paragraphInlineWord{}
			return
		}
		words = append(words, current)
		current = paragraphInlineWord{}
	}
	appendSegment := func(run pdfInlineRun, text string) error {
		if text == "" {
			return nil
		}
		fragment, err := inlineRunFragment(registry, resolver, base, run, text)
		if err != nil {
			return err
		}
		current.Text += text
		current.Width += fragment.Width
		current.Fragments = append(current.Fragments, fragment)
		return nil
	}

	for _, run := range runs {
		var segment strings.Builder
		flushSegment := func() error {
			if segment.Len() == 0 {
				return nil
			}
			text := segment.String()
			segment.Reset()
			return appendSegment(run, text)
		}
		for _, r := range run.Text {
			if isBreakableSpace(r) {
				if err := flushSegment(); err != nil {
					return nil, err
				}
				flushCurrent()
				continue
			}
			segment.WriteRune(r)
		}
		if err := flushSegment(); err != nil {
			return nil, err
		}
	}
	flushCurrent()
	return words, nil
}

func inlineParagraphSpace(registry *pdfFontRegistry, base paragraphStyle) (paragraphLineFragment, error) {
	return inlineRunFragment(registry, nil, base, pdfInlineRun{}, " ")
}

func inlineRunFragment(registry *pdfFontRegistry, resolver *pdfStyleResolver, base paragraphStyle, run pdfInlineRun, text string) (paragraphLineFragment, error) {
	style := inlineRunParagraphStyle(resolver, base, run)
	face, key, err := fontForStyle(registry, style)
	if err != nil {
		return paragraphLineFragment{}, err
	}
	shaped, err := shapeText(face, text)
	if err != nil {
		return paragraphLineFragment{}, fmt.Errorf("shape inline text %q: %w", text, err)
	}
	return paragraphLineFragment{
		Text:          shaped,
		Width:         shapedWidthPointsWithSpacing(shaped, style.FontSize, style.LetterSpacing),
		FontSize:      style.FontSize,
		LetterSpacing: style.LetterSpacing,
		FontKey:       key,
		Color:         style.Color,
		Underline:     style.Underline,
		Strikethrough: style.Strikethrough,
		BaselineShift: inlineRunBaselineShift(base, style),
	}, nil
}

func inlineRunParagraphStyle(resolver *pdfStyleResolver, base paragraphStyle, run pdfInlineRun) paragraphStyle {
	style := inlineClassParagraphStyle(resolver, base, run.StyleClasses)
	style.Bold = style.Bold || run.Bold
	style.Italic = style.Italic || run.Italic
	style.Strikethrough = style.Strikethrough || run.Strikethrough
	if run.Code {
		style.FontFamily = "monospace"
	}
	if run.Subscript {
		style.VerticalAlign = textVerticalAlignSub
	}
	if run.Superscript {
		style.VerticalAlign = textVerticalAlignSuper
	}
	if style.VerticalAlign == textVerticalAlignSub || style.VerticalAlign == textVerticalAlignSuper {
		style.FontSize *= pdfInlineScriptScale
		style.LetterSpacing *= pdfInlineScriptScale
	}
	return style
}

func inlineClassParagraphStyle(resolver *pdfStyleResolver, base paragraphStyle, classes string) paragraphStyle {
	if resolver == nil || strings.TrimSpace(classes) == "" {
		return base
	}
	fallback := resolver.styles[pdfStyleParagraph].Paragraph
	style := base
	for _, class := range strings.Fields(classes) {
		classStyle, ok := resolver.styles[class]
		if !ok {
			continue
		}
		style = mergeInlineParagraphStyle(style, classStyle.Paragraph, fallback)
	}
	return style
}

func mergeInlineParagraphStyle(base, override, fallback paragraphStyle) paragraphStyle {
	if override.FontFamily != fallback.FontFamily {
		base.FontFamily = override.FontFamily
	}
	if override.Bold != fallback.Bold {
		base.Bold = override.Bold
	}
	if override.Italic != fallback.Italic {
		base.Italic = override.Italic
	}
	if override.FontSize != fallback.FontSize {
		base.FontSize = override.FontSize
	}
	if override.LineHeight != fallback.LineHeight {
		base.LineHeight = override.LineHeight
	}
	if override.LetterSpacing != fallback.LetterSpacing {
		base.LetterSpacing = override.LetterSpacing
	}
	if override.VerticalAlign != fallback.VerticalAlign {
		base.VerticalAlign = override.VerticalAlign
	}
	if override.Color != fallback.Color {
		base.Color = override.Color
	}
	if override.Underline != fallback.Underline {
		base.Underline = override.Underline
	}
	if override.Strikethrough != fallback.Strikethrough {
		base.Strikethrough = override.Strikethrough
	}
	return base
}

func inlineRunBaselineShift(base paragraphStyle, style paragraphStyle) float64 {
	switch style.VerticalAlign {
	case textVerticalAlignSuper:
		return base.FontSize * pdfInlineSuperscriptRise
	case textVerticalAlignSub:
		return -base.FontSize * pdfInlineSubscriptDrop
	default:
		return 0
	}
}

func inlineParagraphUnits(registry *pdfFontRegistry, words []paragraphInlineWord, style paragraphStyle) ([]paragraphUnit, error) {
	units := make([]paragraphUnit, 0, len(words))
	for wordIndex, word := range words {
		parts := hyphenatedWordParts(word.Text, style.Hyphenator, style.Hyphenation)
		pieces := inlineWordGlyphPieces(word)
		cursor := 0
		for partIndex, part := range parts {
			count := len([]rune(part))
			if cursor+count > len(pieces) {
				count = max(len(pieces)-cursor, 0)
			}
			fragments := inlinePiecesToFragments(pieces[cursor : cursor+count])
			cursor += count
			width := paragraphFragmentsWidth(fragments)
			hyphenFragments, hyphenWidth, err := inlineHyphenFragments(registry, fragments)
			if err != nil {
				return nil, err
			}
			units = append(units, paragraphUnit{
				Text:            part,
				Width:           width,
				WordIndex:       wordIndex,
				EndWord:         partIndex == len(parts)-1,
				HyphenAfter:     partIndex != len(parts)-1,
				HyphenText:      "-",
				HyphenWidth:     hyphenWidth,
				Fragments:       fragments,
				HyphenFragments: hyphenFragments,
			})
		}
	}
	return units, nil
}

func inlineWordGlyphPieces(word paragraphInlineWord) []inlineGlyphPiece {
	pieces := make([]inlineGlyphPiece, 0, len([]rune(word.Text)))
	for _, fragment := range word.Fragments {
		for _, glyph := range fragment.Text.Glyphs {
			if glyph.Rune == softHyphen {
				continue
			}
			pieces = append(pieces, inlineGlyphPiece{Glyph: glyph, Template: fragment})
		}
	}
	return pieces
}

func inlinePiecesToFragments(pieces []inlineGlyphPiece) []paragraphLineFragment {
	if len(pieces) == 0 {
		return nil
	}
	fragments := make([]paragraphLineFragment, 0, len(pieces))
	start := 0
	for start < len(pieces) {
		end := start + 1
		for end < len(pieces) && sameInlineFragmentStyle(pieces[start].Template, pieces[end].Template) {
			end++
		}
		fragments = append(fragments, inlinePiecesFragment(pieces[start:end], pieces[start].Template))
		start = end
	}
	return fragments
}

func sameInlineFragmentStyle(a, b paragraphLineFragment) bool {
	return a.FontSize == b.FontSize &&
		a.LetterSpacing == b.LetterSpacing &&
		a.FontKey == b.FontKey &&
		a.Color == b.Color &&
		a.Underline == b.Underline &&
		a.Strikethrough == b.Strikethrough &&
		a.BaselineShift == b.BaselineShift
}

func inlinePiecesFragment(pieces []inlineGlyphPiece, template paragraphLineFragment) paragraphLineFragment {
	glyphs := make([]shapedGlyph, 0, len(pieces))
	used := make(map[uint16]shapedGlyph)
	for _, piece := range pieces {
		glyphs = append(glyphs, piece.Glyph)
		if piece.Glyph.GlyphID != 0 {
			used[piece.Glyph.GlyphID] = piece.Glyph
		}
	}
	fragment := template
	fragment.Text = shapedText{Glyphs: glyphs, Used: used}
	fragment.Width = shapedWidthPointsWithSpacing(fragment.Text, fragment.FontSize, fragment.LetterSpacing)
	return fragment
}

func inlineHyphenFragments(registry *pdfFontRegistry, fragments []paragraphLineFragment) ([]paragraphLineFragment, float64, error) {
	if len(fragments) == 0 {
		return nil, 0, nil
	}
	style := fragments[len(fragments)-1]
	face, err := fontForKey(registry, style.FontKey)
	if err != nil {
		return nil, 0, err
	}
	shaped, err := shapeText(face, "-")
	if err != nil {
		return nil, 0, fmt.Errorf("shape inline hyphen: %w", err)
	}
	style.Text = shaped
	style.Width = shapedWidthPointsWithSpacing(shaped, style.FontSize, style.LetterSpacing) + max(style.LetterSpacing, 0)
	return []paragraphLineFragment{style}, style.Width, nil
}

func paragraphFragmentsWidth(fragments []paragraphLineFragment) float64 {
	width := 0.0
	for _, fragment := range fragments {
		width += fragment.Width
	}
	return width
}

func inlineParagraphLineFragments(units []paragraphUnit, space paragraphLineFragment, hyphenAfter bool) ([]paragraphLineFragment, string, float64) {
	fragments := make([]paragraphLineFragment, 0, len(units)*2)
	var text strings.Builder
	width := 0.0
	for i, unit := range units {
		if i > 0 && unit.WordIndex != units[i-1].WordIndex {
			fragments = append(fragments, space)
			text.WriteByte(' ')
			width += space.Width
		}
		fragments = append(fragments, unit.Fragments...)
		text.WriteString(unit.Text)
		width += unit.Width
	}
	if hyphenAfter && len(units) > 0 {
		last := units[len(units)-1]
		fragments = append(fragments, last.HyphenFragments...)
		text.WriteString(last.HyphenText)
		width += last.HyphenWidth
	}
	return fragments, text.String(), width
}

func pageLineFragments(fragments []paragraphLineFragment) []pdfPageLineFragment {
	if len(fragments) == 0 {
		return nil
	}
	out := make([]pdfPageLineFragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, pdfPageLineFragment{
			Text:          fragment.Text,
			Width:         fragment.Width,
			FontSize:      fragment.FontSize,
			LetterSpacing: fragment.LetterSpacing,
			FontKey:       fragment.FontKey,
			Color:         fragment.Color,
			Underline:     fragment.Underline,
			Strikethrough: fragment.Strikethrough,
			BaselineShift: fragment.BaselineShift,
		})
	}
	return out
}
