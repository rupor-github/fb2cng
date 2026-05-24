package pdf

import (
	"fmt"
	"slices"
	"strings"

	contentText "fbc/content/text"
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
	Newline  bool
}

func layoutInlineParagraph(doc pdfDocumentSpec, registry *pdfFontRegistry, resolver *pdfStyleResolver, baseFace *builtinFontFace, text string, runs []pdfInlineRun, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	return layoutInlineParagraphWithShape(doc, registry, resolver, baseFace, text, runs, style, maxWidth, paragraphLineShape{})
}

func layoutInlineParagraphWithShape(doc pdfDocumentSpec, registry *pdfFontRegistry, resolver *pdfStyleResolver, baseFace *builtinFontFace, text string, runs []pdfInlineRun, style paragraphStyle, maxWidth float64, shape paragraphLineShape) ([]paragraphLine, error) {
	if len(runs) == 0 {
		runs = []pdfInlineRun{{Text: text}}
	}
	if style.PreserveSpace {
		return layoutPreformattedParagraph(doc, registry, resolver, runs, style, maxWidth)
	}
	if !hasInlineStyle(runs) {
		return layoutParagraphWithShape(baseFace, plainInlineRunText(runs), style, maxWidth, shape)
	}
	if style.FontSize <= 0 {
		return nil, fmt.Errorf("paragraph font size must be positive: %g", style.FontSize)
	}
	if maxWidth <= 0 {
		return nil, fmt.Errorf("paragraph width must be positive: %g", maxWidth)
	}

	words, err := inlineParagraphWords(doc, registry, resolver, runs, style, maxWidth)
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
	breaks := chooseParagraphBreaksWithShape(units, spaceFragment.Width, style, maxWidth, shape)
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
		indent := paragraphLineIndentForLine(start, i, style, maxWidth, shape)
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
		terminalOverhang := paragraphBreakTerminalOverhang(units[br.End-1])
		visualMetricWidth := width + terminalOverhang
		line.BreakStats = paragraphLineBreakStatsFor(visualMetricWidth, available, line.JustificationGaps, start == 0, last, singleWord, br.Hyphenated, previousHyphenated, previousFitness)
		spacingAvailable := paragraphJustificationAvailableForOverhang(available, terminalOverhang)
		line.ExtraWordSpacing, line.ExtraCharSpacing = paragraphJustificationSpacing(style, last, width, spacingAvailable, line.JustificationGaps, len(shaped.Glyphs))
		lines = append(lines, line)
		previousHyphenated = br.Hyphenated
		previousFitness = line.BreakStats.Fitness
		start = br.End
	}
	return lines, nil
}

func hasInlineStyle(runs []pdfInlineRun) bool {
	for _, run := range runs {
		if run.StyleClasses != "" || run.LinkHref != "" || run.AnchorID != "" || run.FootnoteID != "" || run.ImageID != "" ||
			run.Bold || run.Italic || run.Underline || run.Strikethrough || run.Subscript || run.Superscript || run.Code {
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

func layoutPreformattedParagraph(doc pdfDocumentSpec, registry *pdfFontRegistry, resolver *pdfStyleResolver, runs []pdfInlineRun, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	if style.FontSize <= 0 {
		return nil, fmt.Errorf("paragraph font size must be positive: %g", style.FontSize)
	}
	if maxWidth <= 0 {
		return nil, fmt.Errorf("paragraph width must be positive: %g", maxWidth)
	}
	pieces, err := preformattedPieces(doc, registry, resolver, runs, style, maxWidth)
	if err != nil {
		return nil, err
	}
	if len(pieces) == 0 {
		return nil, nil
	}
	linePieces := wrapPreformattedPieces(pieces, maxWidth)
	lines := make([]paragraphLine, 0, len(linePieces))
	for _, pieces := range linePieces {
		fragments := inlinePiecesToFragments(pieces)
		line := paragraphLine{
			Fragments: fragments,
			Width:     paragraphFragmentsWidth(fragments),
		}
		line.Text = shapedTextFromFragments(fragments)
		line.BreakStats = paragraphLineBreakStatsFor(line.Width, maxWidth, 0, len(lines) == 0, false, false, false, false, paragraphFitnessDecent)
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		last := len(lines) - 1
		lines[last].BreakStats = paragraphLineBreakStatsFor(lines[last].Width, maxWidth, 0, last == 0, true, false, false, false, paragraphFitnessDecent)
	}
	return lines, nil
}

func preformattedPieces(doc pdfDocumentSpec, registry *pdfFontRegistry, resolver *pdfStyleResolver, runs []pdfInlineRun, base paragraphStyle, maxWidth float64) ([]inlineGlyphPiece, error) {
	pieces := make([]inlineGlyphPiece, 0)
	for _, run := range runs {
		text := strings.ReplaceAll(run.Text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		if run.ImageID != "" {
			fragment, err := inlineRunFragment(doc, registry, resolver, base, run, "", maxWidth)
			if err != nil {
				return nil, err
			}
			pieces = append(pieces, inlineGlyphPiece{Template: fragment})
			continue
		}
		parts := strings.Split(text, "\n")
		for i, part := range parts {
			if part != "" {
				fragment, err := inlineRunFragment(doc, registry, resolver, base, run, part, maxWidth)
				if err != nil {
					return nil, err
				}
				for _, glyph := range fragment.Text.Glyphs {
					pieces = append(pieces, inlineGlyphPiece{Glyph: glyph, Template: fragment})
				}
			}
			if i != len(parts)-1 {
				pieces = append(pieces, inlineGlyphPiece{Newline: true})
			}
		}
	}
	return pieces, nil
}

func wrapPreformattedPieces(pieces []inlineGlyphPiece, maxWidth float64) [][]inlineGlyphPiece {
	lines := make([][]inlineGlyphPiece, 0)
	current := make([]inlineGlyphPiece, 0)
	currentWidth := 0.0
	lastBreak := -1
	flush := func(line []inlineGlyphPiece) {
		line = trimTrailingBreakableSpacePieces(line)
		lines = append(lines, append([]inlineGlyphPiece(nil), line...))
	}
	for _, piece := range pieces {
		if piece.Newline {
			flush(current)
			current = current[:0]
			currentWidth = 0
			lastBreak = -1
			continue
		}
		pieceWidth := preformattedPieceWidth(piece)
		if len(current) > 0 && currentWidth+pieceWidth > maxWidth {
			breakAt := len(current)
			if lastBreak > 0 && !allBreakableSpacePieces(current[:lastBreak]) {
				breakAt = lastBreak
			}
			flush(current[:breakAt])
			current = append([]inlineGlyphPiece{}, trimLeadingBreakableSpacePieces(current[breakAt:])...)
			currentWidth = preformattedPiecesWidth(current)
			lastBreak = lastBreakIndex(current)
		}
		current = append(current, piece)
		currentWidth += pieceWidth
		if isBreakableSpace(piece.Glyph.Rune) {
			lastBreak = len(current)
		}
	}
	if len(current) > 0 || len(lines) == 0 {
		flush(current)
	}
	return lines
}

func trimTrailingBreakableSpacePieces(pieces []inlineGlyphPiece) []inlineGlyphPiece {
	for len(pieces) > 0 && !pieces[len(pieces)-1].Newline && isBreakableSpace(pieces[len(pieces)-1].Glyph.Rune) {
		pieces = pieces[:len(pieces)-1]
	}
	return pieces
}

func trimLeadingBreakableSpacePieces(pieces []inlineGlyphPiece) []inlineGlyphPiece {
	for len(pieces) > 0 && !pieces[0].Newline && isBreakableSpace(pieces[0].Glyph.Rune) {
		pieces = pieces[1:]
	}
	return pieces
}

func allBreakableSpacePieces(pieces []inlineGlyphPiece) bool {
	for _, piece := range pieces {
		if piece.Newline || !isBreakableSpace(piece.Glyph.Rune) {
			return false
		}
	}
	return len(pieces) > 0
}

func lastBreakIndex(pieces []inlineGlyphPiece) int {
	last := -1
	for i, piece := range pieces {
		if !piece.Newline && isBreakableSpace(piece.Glyph.Rune) {
			last = i + 1
		}
	}
	return last
}

func preformattedPiecesWidth(pieces []inlineGlyphPiece) float64 {
	width := 0.0
	for _, piece := range pieces {
		width += preformattedPieceWidth(piece)
	}
	return width
}

func preformattedPieceWidth(piece inlineGlyphPiece) float64 {
	if piece.Template.ImageID != "" {
		return piece.Template.Width
	}
	return float64(piece.Glyph.Width) * piece.Template.FontSize / 1000.0
}

func shapedTextFromFragments(fragments []paragraphLineFragment) shapedText {
	shaped := shapedText{Used: make(map[uint16]shapedGlyph)}
	for _, fragment := range fragments {
		shaped.Glyphs = append(shaped.Glyphs, fragment.Text.Glyphs...)
		for id, glyph := range fragment.Text.Used {
			shaped.Used[id] = glyph
		}
	}
	return shaped
}

func inlineParagraphWords(doc pdfDocumentSpec, registry *pdfFontRegistry, resolver *pdfStyleResolver, runs []pdfInlineRun, base paragraphStyle, maxWidth float64) ([]paragraphInlineWord, error) {
	words := make([]paragraphInlineWord, 0)
	current := paragraphInlineWord{}
	flushCurrent := func() {
		if strings.TrimSpace(strings.ReplaceAll(current.Text, contentText.SOFTHYPHEN, "")) == "" && len(current.Fragments) == 0 {
			current = paragraphInlineWord{}
			return
		}
		words = append(words, current)
		current = paragraphInlineWord{}
	}
	appendSegment := func(run pdfInlineRun, text string) error {
		if text == "" && run.ImageID == "" {
			return nil
		}
		fragment, err := inlineRunFragment(doc, registry, resolver, base, run, text, maxWidth)
		if err != nil {
			return err
		}
		current.Text += text
		current.Width += fragment.Width
		current.Fragments = append(current.Fragments, fragment)
		return nil
	}

	for _, run := range runs {
		if run.ImageID != "" {
			if err := appendSegment(run, ""); err != nil {
				return nil, err
			}
			continue
		}
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
			if isBreakableSpace(r) && !base.NoWrap {
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
	return inlineRunFragment(pdfDocumentSpec{}, registry, nil, base, pdfInlineRun{}, " ", 0)
}

func inlineRunFragment(doc pdfDocumentSpec, registry *pdfFontRegistry, resolver *pdfStyleResolver, base paragraphStyle, run pdfInlineRun, text string, maxWidth float64) (paragraphLineFragment, error) {
	style := inlineRunParagraphStyle(resolver, base, run)
	face, key, err := fontForStyle(registry, style)
	if err != nil {
		return paragraphLineFragment{}, err
	}
	if run.ImageID != "" {
		width, height, baselineShift := inlineImageFragmentSize(doc, run.ImageID, style, face, maxWidth)
		return paragraphLineFragment{
			Width:         width,
			FontSize:      style.FontSize,
			FontKey:       key,
			Color:         style.Color,
			Underline:     style.Underline,
			Strikethrough: style.Strikethrough,
			BaselineShift: baselineShift,
			StyleClasses:  run.StyleClasses,
			LinkHref:      run.LinkHref,
			AnchorID:      run.AnchorID,
			FootnoteID:    run.FootnoteID,
			ImageID:       run.ImageID,
			ImageHeight:   height,
		}, nil
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
		StyleClasses:  run.StyleClasses,
		LinkHref:      run.LinkHref,
		AnchorID:      run.AnchorID,
		FootnoteID:    run.FootnoteID,
	}, nil
}

func inlineImageFragmentSize(doc pdfDocumentSpec, imageID string, style paragraphStyle, face *builtinFontFace, maxWidth float64) (float64, float64, float64) {
	lineHeight := max(style.LineHeight, style.FontSize)
	if lineHeight <= 0 {
		lineHeight = pdfBaseLineHeight
	}
	lineAscent, lineDescent := inlineLineBoxMetrics(face, style, lineHeight)
	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	targetHeight := lineHeight
	width, height := targetHeight, targetHeight
	if img := doc.Images[imageID]; img != nil {
		widthPx, heightPx := pdfImagePixelSize(img)
		if widthPx > 0 && heightPx > 0 {
			scale := fontSize / pdfKP3PixelsPerEm
			width = float64(widthPx) * scale
			height = float64(heightPx) * scale
		}
	}
	if maxWidth > 0 && width > maxWidth {
		scale := maxWidth / width
		width *= scale
		height *= scale
	}
	baselineShift := -lineDescent + max((lineHeight-height)/2, 0)
	if height > lineAscent+lineDescent {
		baselineShift = -lineDescent
	}
	return width, height, baselineShift
}

func inlineLineBoxMetrics(face *builtinFontFace, style paragraphStyle, lineHeight float64) (float64, float64) {
	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	if face == nil || face.UnitsPerEm <= 0 {
		ascent := fontSize * 0.8
		descent := fontSize * 0.2
		leading := max(lineHeight-ascent-descent, 0) / 2
		return ascent + leading, descent + leading
	}
	ascent := float64(face.Ascent) * fontSize / float64(face.UnitsPerEm)
	descent := float64(face.Descent) * fontSize / float64(face.UnitsPerEm)
	if ascent <= 0 || descent < 0 {
		ascent = fontSize * 0.8
		descent = fontSize * 0.2
	}
	leading := max(lineHeight-ascent-descent, 0) / 2
	return ascent + leading, descent + leading
}

func inlineRunParagraphStyle(resolver *pdfStyleResolver, base paragraphStyle, run pdfInlineRun) paragraphStyle {
	style := inlineClassParagraphStyle(resolver, base, run)
	style = applyInlineRunDirectStyle(style, run, true)
	if inlineRunIsFootnoteLink(run) {
		if resolverHasDefaultFootnoteLinkStyle(resolver, run) {
			contextStyle := inlineFootnoteContextParagraphStyle(resolver, base, run)
			factor := inlineFootnoteLinkFontSizeFactor(run)
			style.FontSize = contextStyle.FontSize * factor
			style.LetterSpacing = contextStyle.LetterSpacing * factor
		}
		return style
	}
	if style.VerticalAlign == textVerticalAlignSub || style.VerticalAlign == textVerticalAlignSuper {
		style.FontSize *= pdfInlineScriptScale
		style.LetterSpacing *= pdfInlineScriptScale
	}
	return style
}

func applyInlineRunDirectStyle(style paragraphStyle, run pdfInlineRun, includeScript bool) paragraphStyle {
	style.Bold = style.Bold || run.Bold
	style.Italic = style.Italic || run.Italic
	style.Underline = style.Underline || run.Underline
	style.Strikethrough = style.Strikethrough || run.Strikethrough
	if run.Code {
		style.FontFamily = "monospace"
		if !inlineRunHasStyleClass(run, pdfStyleCode) {
			style.FontSize *= 0.70
		}
	}
	if includeScript && run.Subscript {
		style.VerticalAlign = textVerticalAlignSub
	}
	if includeScript && run.Superscript {
		style.VerticalAlign = textVerticalAlignSuper
	}
	return style
}

func inlineFootnoteContextParagraphStyle(resolver *pdfStyleResolver, base paragraphStyle, run pdfInlineRun) paragraphStyle {
	contextRun := run
	contextRun.StyleClasses = removeInlineRunStyleClass(run.StyleClasses, pdfStyleLinkFootnote)
	style := inlineClassParagraphStyle(resolver, base, contextRun)
	return applyInlineRunDirectStyle(style, contextRun, false)
}

func inlineRunIsFootnoteLink(run pdfInlineRun) bool {
	return inlineRunHasStyleClass(run, pdfStyleLinkFootnote)
}

func inlineRunHasSuperscriptContext(run pdfInlineRun) bool {
	return run.Superscript
}

func inlineRunHeadingLevel(run pdfInlineRun) int {
	for _, class := range strings.Fields(run.ContextClasses) {
		switch class {
		case "h1", pdfStyleBodyTitle, pdfStyleChapterTitle, pdfStyleBodyTitleHeader, pdfStyleChapterTitleHeader:
			return 1
		case "h2", pdfStyleSectionTitleH2, pdfStyleSectionTitleHeader:
			return 2
		case "h3":
			return 3
		case "h4":
			return 4
		case "h5":
			return 5
		case "h6":
			return 6
		}
	}
	return 0
}

func inlineFootnoteLinkFontSizeFactor(run pdfInlineRun) float64 {
	if inlineRunHeadingLevel(run) > 0 {
		if inlineRunHasSuperscriptContext(run) {
			return 0.70
		}
		return 0.90
	}
	if inlineRunHasSuperscriptContext(run) {
		return 0.75
	}
	return 0.80
}

func resolverHasDefaultFootnoteLinkStyle(resolver *pdfStyleResolver, run pdfInlineRun) bool {
	if resolver == nil || resolver.styles == nil {
		return false
	}
	style, ok := resolver.styles[pdfStyleLinkFootnote]
	if !ok || !paragraphStyleLooksLikeDefaultFootnoteLink(style.Paragraph) {
		return false
	}
	fallback := resolver.namedStyle(pdfStyleParagraph).Paragraph
	for _, descStyleName := range inlineRunContextDescendantStyleNames(resolver, run) {
		if !strings.Contains(descStyleName, pdfStyleLinkFootnote) {
			continue
		}
		descStyle := resolver.styles[descStyleName]
		if paragraphStyleOverridesFootnoteSizing(descStyle.Paragraph, fallback) {
			return false
		}
	}
	return true
}

func paragraphStyleLooksLikeDefaultFootnoteLink(style paragraphStyle) bool {
	return style.VerticalAlign == textVerticalAlignSuper && pdfCSSSpecScale(style.FontSizeSpec, 0.8)
}

func paragraphStyleOverridesFootnoteSizing(style paragraphStyle, fallback paragraphStyle) bool {
	if style.FontSizeSpec.Set && !pdfCSSSpecScale(style.FontSizeSpec, 0.8) {
		return true
	}
	if !style.FontSizeSpec.Set && style.FontSize != fallback.FontSize {
		return true
	}
	return (style.HasVerticalAlign || style.VerticalAlign != fallback.VerticalAlign) && style.VerticalAlign != textVerticalAlignSuper
}

func pdfCSSSpecScale(spec pdfCSSLengthSpec, scale float64) bool {
	if !spec.Set {
		return false
	}
	switch spec.Unit {
	case "em":
		return inlineFloatEqual(spec.Value, scale)
	case "%":
		return inlineFloatEqual(spec.Value, scale*100)
	default:
		return false
	}
}

func inlineFloatEqual(a float64, b float64) bool {
	if a > b {
		return a-b < 0.0001
	}
	return b-a < 0.0001
}

func removeInlineRunStyleClass(classes string, remove string) string {
	if strings.TrimSpace(classes) == "" {
		return ""
	}
	kept := make([]string, 0, len(strings.Fields(classes)))
	for _, class := range strings.Fields(classes) {
		if class != remove {
			kept = append(kept, class)
		}
	}
	return strings.Join(kept, " ")
}

func inlineRunHasStyleClass(run pdfInlineRun, className string) bool {
	for _, class := range strings.Fields(run.StyleClasses) {
		if class == className {
			return true
		}
	}
	return false
}

func inlineClassParagraphStyle(resolver *pdfStyleResolver, base paragraphStyle, run pdfInlineRun) paragraphStyle {
	if resolver == nil {
		return base
	}
	fallback := resolver.styles[pdfStyleParagraph].Paragraph
	style := base
	for _, class := range strings.Fields(run.StyleClasses) {
		if inlineRunClassAlreadyAppliedByBlockContext(run, class) {
			continue
		}
		classStyle, ok := resolver.styles[class]
		if !ok {
			continue
		}
		style = mergeInlineParagraphStyle(style, classStyle.Paragraph, fallback)
	}
	for _, descStyleName := range inlineRunContextDescendantStyleNames(resolver, run) {
		descStyle, ok := resolver.styles[descStyleName]
		if !ok {
			continue
		}
		style = mergeInlineParagraphStyle(style, descStyle.Paragraph, fallback)
	}
	return style
}

func inlineRunClassAlreadyAppliedByBlockContext(run pdfInlineRun, class string) bool {
	return class == pdfStyleCode && run.Code && inlineRunHasContextClass(run, pdfStyleCode)
}

func inlineRunHasContextClass(run pdfInlineRun, className string) bool {
	for _, class := range strings.Fields(run.ContextClasses) {
		if class == className {
			return true
		}
	}
	return false
}

func inlineRunsWithContext(runs []pdfInlineRun, contextClasses string) []pdfInlineRun {
	contextClasses = strings.TrimSpace(contextClasses)
	if contextClasses == "" || len(runs) == 0 {
		return runs
	}
	withContext := make([]pdfInlineRun, len(runs))
	for i := range runs {
		withContext[i] = runs[i]
		withContext[i].ContextClasses = joinStyleClasses(contextClasses, runs[i].ContextClasses)
	}
	return withContext
}

func inlineRunContextClassesForBlock(block pdfTextBlock) string {
	return joinStyleClasses(block.ContextClasses, pdfElementTagForBlock(block), pdfStyleNameForBlock(block), block.StyleClasses)
}

func inlineRunContextDescendantStyleNames(resolver *pdfStyleResolver, run pdfInlineRun) []string {
	if resolver == nil {
		return nil
	}
	ancestors := []string{pdfStyleHTML, pdfStyleBody}
	ancestors = append(ancestors, strings.Fields(run.ContextClasses)...)
	candidates := inlineRunSelectorCandidates(run)
	var names []string
	for _, ancestor := range ancestors {
		for _, candidate := range candidates {
			name := ancestor + "--" + candidate
			if _, ok := resolver.styles[name]; ok {
				names = appendUniqueString(names, name)
			}
		}
	}
	return names
}

func inlineRunSelectorCandidates(run pdfInlineRun) []string {
	classList := strings.Fields(run.StyleClasses)
	candidates := make([]string, 0, len(classList)+1)
	for _, class := range classList {
		candidates = appendUniqueString(candidates, class)
	}
	if run.Code || stringListContains(classList, pdfStyleCode) {
		candidates = appendUniqueString(candidates, "code")
		for _, class := range classList {
			candidates = appendUniqueString(candidates, "code."+class)
		}
	}
	return candidates
}

func appendUniqueString(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func stringListContains(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

func mergeInlineParagraphStyle(base, override, fallback paragraphStyle) paragraphStyle {
	if override.FontFamily != fallback.FontFamily {
		base.FontFamily = override.FontFamily
	}
	if override.HasBold || override.Bold != fallback.Bold {
		base.Bold = override.Bold
		base.HasBold = override.HasBold
	}
	if override.HasItalic || override.Italic != fallback.Italic {
		base.Italic = override.Italic
		base.HasItalic = override.HasItalic
	}
	if override.FontSizeSpec.Set {
		base.FontSize = pdfResolveCSSFontSizeSpec(override.FontSizeSpec, base.FontSize)
		base.FontSizeSpec = override.FontSizeSpec
	} else if override.FontSize != fallback.FontSize {
		base.FontSize = override.FontSize
		base.FontSizeSpec = pdfCSSLengthSpec{}
	}
	lineHeightOverride := override
	if override.FontSizeSpec.Set && !override.LineHeightExplicit {
		if override.LineHeight != fallback.LineHeight && override.FontSize > 0 {
			base.LineHeight = override.LineHeight * base.FontSize / override.FontSize
			lineHeightOverride.LineHeight = fallback.LineHeight
		} else {
			lineHeightOverride.LineHeight = fallback.LineHeight
		}
	}
	base = mergePDFLineHeightOverride(base, lineHeightOverride, fallback)
	if override.LineHeightSpec.Set {
		base.LineHeight = pdfResolveCSSLineHeightSpec(override.LineHeightSpec, base.FontSize)
		base.LineHeightSpec = override.LineHeightSpec
		base.LineHeightExplicit = true
	}
	if override.LetterSpacingSpec.Set {
		base.LetterSpacing = pdfResolveCSSLengthSpec(override.LetterSpacingSpec, base.FontSize)
		base.LetterSpacingSpec = override.LetterSpacingSpec
	} else if override.LetterSpacing != fallback.LetterSpacing {
		base.LetterSpacing = override.LetterSpacing
		base.LetterSpacingSpec = pdfCSSLengthSpec{}
	}
	if override.HasVerticalAlign || override.VerticalAlign != fallback.VerticalAlign {
		base.VerticalAlign = override.VerticalAlign
		base.HasVerticalAlign = override.HasVerticalAlign
	}
	if override.Color != fallback.Color {
		base.Color = override.Color
	}
	if override.HasUnderline || override.Underline != fallback.Underline {
		base.Underline = override.Underline
		base.HasUnderline = override.HasUnderline
	}
	if override.HasStrikethrough || override.Strikethrough != fallback.Strikethrough {
		base.Strikethrough = override.Strikethrough
		base.HasStrikethrough = override.HasStrikethrough
	}
	if override.HasPreserveSpace || override.PreserveSpace != fallback.PreserveSpace {
		base.PreserveSpace = override.PreserveSpace
		base.HasPreserveSpace = override.HasPreserveSpace
	}
	if override.HasNoWrap || override.NoWrap != fallback.NoWrap {
		base.NoWrap = override.NoWrap
		base.HasNoWrap = override.HasNoWrap
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
		if inlineWordHasImage(word) {
			units = append(units, paragraphUnit{
				Text:       word.Text,
				Width:      paragraphFragmentsWidth(word.Fragments),
				WordIndex:  wordIndex,
				EndWord:    true,
				GlyphCount: paragraphFragmentsGlyphCount(word.Fragments),
				Fragments:  word.Fragments,
			})
			continue
		}
		if style.NoWrap || inlineWordHasMultiRuneGlyph(word) {
			width := paragraphFragmentsWidth(word.Fragments)
			units = append(units, paragraphUnit{
				Text:          word.Text,
				Width:         width,
				WordIndex:     wordIndex,
				EndWord:       true,
				GlyphCount:    paragraphFragmentsGlyphCount(word.Fragments),
				RightOverhang: paragraphFragmentsRightOverhang(word.Fragments, width),
				Fragments:     word.Fragments,
			})
			continue
		}
		parts := hyphenatedWordParts(word.Text, style.Hyphenator, pdfEffectiveHyphenation(style))
		pieces := inlineWordGlyphPieces(word)
		cursor := 0
		for partIndex, part := range parts {
			count := len([]rune(part.Text))
			if cursor+count > len(pieces) {
				count = max(len(pieces)-cursor, 0)
			}
			fragments := inlinePiecesToFragments(pieces[cursor : cursor+count])
			cursor += count
			width := paragraphFragmentsWidth(fragments)
			var hyphenFragments []paragraphLineFragment
			hyphenWidth := 0.0
			if part.HyphenText != "" {
				var err error
				hyphenFragments, hyphenWidth, err = inlineHyphenFragments(registry, fragments)
				if err != nil {
					return nil, err
				}
			}
			hyphenOverhang := 0.0
			if len(hyphenFragments) != 0 {
				hyphenatedFragments := slices.Concat(fragments, hyphenFragments)
				hyphenOverhang = paragraphFragmentsRightOverhang(hyphenatedFragments, width+hyphenWidth)
			}
			units = append(units, paragraphUnit{
				Text:                part.Text,
				Width:               width,
				WordIndex:           wordIndex,
				EndWord:             partIndex == len(parts)-1,
				BreakAfter:          part.BreakAfter,
				Hyphenated:          part.Hyphenated,
				HyphenText:          part.HyphenText,
				HyphenWidth:         hyphenWidth,
				GlyphCount:          paragraphFragmentsGlyphCount(fragments),
				HyphenGlyphCount:    paragraphFragmentsGlyphCount(hyphenFragments),
				RightOverhang:       paragraphFragmentsRightOverhang(fragments, width),
				HyphenRightOverhang: hyphenOverhang,
				Fragments:           fragments,
				HyphenFragments:     hyphenFragments,
			})
		}
	}
	return units, nil
}

func inlineWordHasImage(word paragraphInlineWord) bool {
	for _, fragment := range word.Fragments {
		if fragment.ImageID != "" {
			return true
		}
	}
	return false
}

func inlineWordHasMultiRuneGlyph(word paragraphInlineWord) bool {
	for _, fragment := range word.Fragments {
		for _, glyph := range fragment.Text.Glyphs {
			if glyph.ClusterEnd-glyph.ClusterStart > 1 || len([]rune(glyphUnicodeText(glyph))) > 1 {
				return true
			}
		}
	}
	return false
}

func inlineWordGlyphPieces(word paragraphInlineWord) []inlineGlyphPiece {
	pieces := make([]inlineGlyphPiece, 0, len([]rune(word.Text)))
	for _, fragment := range word.Fragments {
		for _, glyph := range fragment.Text.Glyphs {
			if string(glyph.Rune) == contentText.SOFTHYPHEN {
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
		a.BaselineShift == b.BaselineShift &&
		a.LinkHref == b.LinkHref &&
		a.AnchorID == b.AnchorID &&
		a.FootnoteID == b.FootnoteID &&
		a.ImageID == b.ImageID &&
		a.ImageHeight == b.ImageHeight
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

func paragraphFragmentsGlyphCount(fragments []paragraphLineFragment) int {
	count := 0
	for _, fragment := range fragments {
		count += len(fragment.Text.Glyphs)
	}
	return count
}

func paragraphFragmentsRightOverhang(fragments []paragraphLineFragment, width float64) float64 {
	right, ok := paragraphFragmentLineVisualRight(paragraphLine{Width: width, Fragments: fragments})
	if !ok {
		return 0
	}
	return max(right-width, 0)
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
			StyleClasses:  fragment.StyleClasses,
			LinkHref:      fragment.LinkHref,
			AnchorID:      fragment.AnchorID,
			FootnoteID:    fragment.FootnoteID,
			ImageID:       fragment.ImageID,
			ImageHeight:   fragment.ImageHeight,
		})
	}
	return out
}
