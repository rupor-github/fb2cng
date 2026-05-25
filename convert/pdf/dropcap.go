package pdf

import "strings"

const (
	// Match KFX/KP3 semantics: normalized dropcaps always use one initial
	// character, and CSS font-size on .dropcap is converted to a line count.
	pdfDropcapDefaultChars = 1
	pdfDropcapDefaultLines = 3
	pdfDropcapMinLines     = 2
	pdfDropcapMaxLines     = 10

	// Native PDF renders the dropcap glyph explicitly, while KFX delegates the
	// actual glyph scaling to KP3 through dropcap_lines. We therefore derive a
	// synthetic render font size from the reserved dropcap area. CapHeightFactor
	// approximates how much of that font size is occupied by an uppercase glyph;
	// increasing it makes the rendered capital smaller.
	pdfDropcapCapHeightFactor = 0.76

	// Do not fill the full dropcap_lines area with black glyph pixels. KP3 leaves
	// a little breathing room before the first full-width line below the dropcap;
	// this factor keeps the exclusion at the requested line count while rendering
	// the visible glyph slightly shorter.
	pdfDropcapVisualFillFactor = 0.90
)

type pdfDropcapCSSConfig struct {
	Chars int
	Lines int
}

type pdfActiveDropcap struct {
	Page             *pdfPage
	X                float64
	TopY             float64
	BottomY          float64
	ExclusionWidth   float64
	Lines            int
	Char             string
	BodySearchOffset int
}

type pdfDropcapLayout struct {
	Run              pdfInlineRun
	BodyRuns         []pdfInlineRun
	BodyText         string
	Fragment         paragraphLineFragment
	Style            paragraphStyle
	Lines            int
	PaddingRight     float64
	ExclusionWidth   float64
	ReservedHeight   float64
	BaselineY        float64
	TopY             float64
	BottomY          float64
	BodySearchOffset int
}

func hasPDFStyleClass(classes string, className string) bool {
	for _, class := range strings.Fields(classes) {
		if class == className {
			return true
		}
	}
	return false
}

func addPDFDropcapInlineRun(runs []pdfInlineRun) []pdfInlineRun {
	if len(runs) == 0 {
		return runs
	}
	result := make([]pdfInlineRun, 0, len(runs)+1)
	applied := false
	for _, run := range runs {
		if applied || run.Text == "" {
			result = append(result, run)
			continue
		}
		if inlineRunHasStyleClass(run, pdfStyleDropcap) {
			return runs
		}
		runes := []rune(run.Text)
		if len(runes) == 0 {
			result = append(result, run)
			continue
		}
		dropcap := run
		dropcap.Text = string(runes[0])
		dropcap.StyleClasses = joinStyleClasses(run.StyleClasses, pdfStyleDropcap)
		result = append(result, dropcap)
		if len(runes) > 1 {
			rest := run
			rest.Text = string(runes[1:])
			result = append(result, rest)
		}
		applied = true
	}
	return result
}

func pdfBlockStartsDropcap(block pdfTextBlock) bool {
	return block.Kind == pdfBlockParagraph && hasPDFStyleClass(block.StyleClasses, "has-dropcap")
}

func splitPDFDropcapRuns(runs []pdfInlineRun) (pdfInlineRun, []pdfInlineRun, bool) {
	for i, run := range runs {
		if run.Text == "" {
			continue
		}
		if !inlineRunHasStyleClass(run, pdfStyleDropcap) {
			return pdfInlineRun{}, runs, false
		}
		runes := []rune(run.Text)
		if len(runes) == 0 {
			return pdfInlineRun{}, runs, false
		}
		dropcap := run
		dropcap.Text = string(runes[0])
		body := make([]pdfInlineRun, 0, len(runs))
		body = append(body, runs[:i]...)
		if len(runes) > 1 {
			rest := run
			rest.Text = string(runes[1:])
			rest.StyleClasses = removeInlineRunStyleClass(rest.StyleClasses, pdfStyleDropcap)
			body = append(body, rest)
		}
		body = append(body, runs[i+1:]...)
		return dropcap, body, true
	}
	return pdfInlineRun{}, runs, false
}

func pdfDropcapLineCount(resolver *pdfStyleResolver, block pdfTextBlock, style paragraphStyle) int {
	if style.FontSizeSpec.Set && style.FontSizeSpec.Value > 0 {
		return clampPDFDropcapLines(int(style.FontSizeSpec.Value + 0.5))
	}
	if resolver != nil {
		for _, class := range strings.Fields(block.StyleClasses) {
			if cfg, ok := resolver.dropcaps[class]; ok && cfg.Lines > 0 {
				return clampPDFDropcapLines(cfg.Lines)
			}
		}
	}
	return pdfDropcapDefaultLines
}

func clampPDFDropcapLines(lines int) int {
	return max(pdfDropcapMinLines, min(pdfDropcapMaxLines, lines))
}

func pdfDropcapPaddingRight(resolver *pdfStyleResolver, run pdfInlineRun, fontSize float64) float64 {
	if resolver == nil {
		return 0
	}
	fallback := resolver.namedStyle(pdfStyleParagraph)
	paddingRight := 0.0
	apply := func(style pdfBlockResolvedStyle) {
		if style.PaddingRightSpec.Set {
			paddingRight = pdfResolveCSSLengthSpec(style.PaddingRightSpec, fontSize)
			return
		}
		if style.PaddingRight != fallback.PaddingRight {
			paddingRight = style.PaddingRight
		}
	}
	for _, class := range strings.Fields(run.StyleClasses) {
		if style, ok := resolver.styles[class]; ok {
			apply(style)
		}
	}
	for _, name := range inlineRunContextDescendantStyleNames(resolver, run) {
		if style, ok := resolver.styles[name]; ok {
			apply(style)
		}
	}
	return paddingRight
}

func repeatPDFDropcapInset(inset float64, lines int) paragraphLineShape {
	if inset <= 0 || lines <= 0 {
		return paragraphLineShape{}
	}
	shape := paragraphLineShape{InitialInsets: make([]float64, lines)}
	for i := range shape.InitialInsets {
		shape.InitialInsets[i] = inset
	}
	return shape
}

func pdfActiveDropcapShape(active *pdfActiveDropcap, page *pdfPage, block pdfTextBlock, firstBaselineY float64, style paragraphStyle) paragraphLineShape {
	if active == nil || active.Page != page || block.Kind != pdfBlockParagraph || active.ExclusionWidth <= 0 {
		return paragraphLineShape{}
	}
	lineHeight := pdfEffectiveParagraphLineHeight(style)
	if lineHeight <= 0 {
		return paragraphLineShape{}
	}
	insets := make([]float64, 0, active.Lines)
	for i := 0; i < pdfDropcapMaxLines*2; i++ {
		lineY := firstBaselineY - float64(i)*lineHeight
		if !pdfLineOverlapsDropcap(lineY, lineHeight, style.FontSize, active) {
			if lineY+style.FontSize < active.BottomY {
				break
			}
			insets = append(insets, 0)
			continue
		}
		insets = append(insets, active.ExclusionWidth)
	}
	for len(insets) > 0 && insets[len(insets)-1] == 0 {
		insets = insets[:len(insets)-1]
	}
	return paragraphLineShape{InitialInsets: insets}
}

func pdfLineOverlapsDropcap(lineY float64, lineHeight float64, fontSize float64, active *pdfActiveDropcap) bool {
	if active == nil {
		return false
	}
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	lineTop := lineY + fontSize
	lineBottom := lineY - max(lineHeight-fontSize, 0)
	return lineTop > active.BottomY && lineBottom < active.TopY
}

func pdfDropcapExpired(active *pdfActiveDropcap, page *pdfPage, y float64) bool {
	return active == nil || active.Page != page || y <= active.BottomY+0.001
}

func pdfDropcapExpiredForLine(active *pdfActiveDropcap, page *pdfPage, lineY float64, lineHeight float64, fontSize float64) bool {
	if active == nil || active.Page != page {
		return true
	}
	if pdfLineOverlapsDropcap(lineY, lineHeight, fontSize, active) {
		return false
	}
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	return lineY+fontSize <= active.BottomY+0.001
}

func buildPDFDropcapLayout(doc pdfDocumentSpec, resolver *pdfStyleResolver, block pdfTextBlock, base paragraphStyle, baseFace *builtinFontFace, runs []pdfInlineRun, blockWidth float64, firstBaselineY float64) (pdfDropcapLayout, bool, error) {
	dropcapRun, bodyRuns, ok := splitPDFDropcapRuns(runs)
	if !ok || strings.TrimSpace(dropcapRun.Text) == "" {
		return pdfDropcapLayout{}, false, nil
	}
	bodyText := pdfDropcapBodyText(bodyRuns)
	cssDropcapStyle := inlineRunParagraphStyle(resolver, base, dropcapRun)
	lines := pdfDropcapLineCount(resolver, block, cssDropcapStyle)
	lineHeight := pdfEffectiveParagraphLineHeight(base)
	reservedHeight := float64(lines) * lineHeight
	paddingRight := pdfDropcapPaddingRight(resolver, dropcapRun, cssDropcapStyle.FontSize)
	dropcapStyle := cssDropcapStyle
	dropcapStyle.FontSize = pdfDropcapRenderFontSize(reservedHeight)
	dropcapStyle.FontSizeSpec = pdfCSSLengthSpec{}
	dropcapStyle.LineHeight = reservedHeight
	dropcapStyle.LineHeightSpec = pdfCSSLengthSpec{}
	dropcapStyle.LineHeightExplicit = true
	dropcapFace, dropcapKey, err := fontForStyle(doc.Fonts, dropcapStyle)
	if err != nil {
		return pdfDropcapLayout{}, false, err
	}
	shaped, err := shapeTextWithCache(doc.TextShapers, dropcapFace, dropcapRun.Text)
	if err != nil {
		return pdfDropcapLayout{}, false, err
	}
	fragment := paragraphLineFragment{
		Text:          shaped,
		Width:         shapedWidthPointsWithSpacing(shaped, dropcapStyle.FontSize, dropcapStyle.LetterSpacing),
		FontSize:      dropcapStyle.FontSize,
		LetterSpacing: dropcapStyle.LetterSpacing,
		FontKey:       dropcapKey,
		Color:         dropcapStyle.Color,
		Underline:     dropcapStyle.Underline,
		Strikethrough: dropcapStyle.Strikethrough,
		LinkHref:      dropcapRun.LinkHref,
		AnchorID:      dropcapRun.AnchorID,
	}
	topY := firstBaselineY + base.FontSize*pdfDropcapCapHeightFactor
	dropcapAscent := dropcapStyle.FontSize * pdfDropcapCapHeightFactor
	layout := pdfDropcapLayout{
		Run:              dropcapRun,
		BodyRuns:         bodyRuns,
		BodyText:         bodyText,
		Fragment:         fragment,
		Style:            dropcapStyle,
		Lines:            lines,
		PaddingRight:     paddingRight,
		ExclusionWidth:   fragment.Width + paddingRight,
		ReservedHeight:   reservedHeight,
		BaselineY:        topY - dropcapAscent,
		TopY:             topY,
		BottomY:          topY - reservedHeight*pdfDropcapVisualFillFactor,
		BodySearchOffset: len([]rune(dropcapRun.Text)),
	}
	return layout, true, nil
}

func pdfDropcapRenderFontSize(reservedHeight float64) float64 {
	if reservedHeight <= 0 {
		return pdfBaseFontSize
	}
	return reservedHeight * pdfDropcapVisualFillFactor / pdfDropcapCapHeightFactor
}

func pdfTraceResolvedDropcap(resolver *pdfStyleResolver, blockIndex int, block pdfTextBlock, layout pdfDropcapLayout, base paragraphStyle) {
	if resolver == nil {
		return
	}
	resolver.tracer.traceDropcapResolved(blockIndex, block, layout, base)
}

func pdfDropcapBodyText(bodyRuns []pdfInlineRun) string {
	text := plainInlineRunText(bodyRuns)
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return text
}
