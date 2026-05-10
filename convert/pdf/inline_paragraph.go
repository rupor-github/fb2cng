package pdf

import (
	"fmt"
	"math"
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

func layoutInlineParagraph(registry *pdfFontRegistry, baseFace *builtinFontFace, text string, runs []pdfInlineRun, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
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

	words, err := inlineParagraphWords(registry, runs, style)
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
	breaks := chooseInlineParagraphBreaks(words, spaceFragment.Width, style, maxWidth)
	lines := make([]paragraphLine, 0, len(breaks))
	start := 0
	for i, br := range breaks {
		fragments, text, width := inlineParagraphLineFragments(words[start:br.End], spaceFragment)
		shaped, err := shapeText(baseFace, text)
		if err != nil {
			return nil, fmt.Errorf("shape inline line text: %w", err)
		}
		indent := 0.0
		if start == 0 {
			indent = min(max(style.FirstLineIndent, 0), maxWidth)
		}
		available := max(maxWidth-indent, 1)
		line := paragraphLine{
			Text:              shaped,
			Width:             width,
			Indent:            indent,
			JustificationGaps: max(br.End-start-1, 0),
			Fragments:         fragments,
		}
		if style.Align == textAlignJustify && i != len(breaks)-1 && line.JustificationGaps > 0 && width < available {
			line.ExtraWordSpacing = (available - width) / float64(line.JustificationGaps)
		}
		lines = append(lines, line)
		start = br.End
	}
	return lines, nil
}

func hasInlineStyle(runs []pdfInlineRun) bool {
	for _, run := range runs {
		if run.Bold || run.Italic || run.Strikethrough || run.Subscript || run.Superscript || run.Code {
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

func inlineParagraphWords(registry *pdfFontRegistry, runs []pdfInlineRun, base paragraphStyle) ([]paragraphInlineWord, error) {
	words := make([]paragraphInlineWord, 0)
	current := paragraphInlineWord{}
	flushCurrent := func() {
		if strings.TrimSpace(current.Text) == "" {
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
		fragment, err := inlineRunFragment(registry, base, run, text)
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
	return inlineRunFragment(registry, base, pdfInlineRun{}, " ")
}

func inlineRunFragment(registry *pdfFontRegistry, base paragraphStyle, run pdfInlineRun, text string) (paragraphLineFragment, error) {
	style := inlineRunParagraphStyle(base, run)
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
		BaselineShift: inlineRunBaselineShift(base, run),
	}, nil
}

func inlineRunParagraphStyle(base paragraphStyle, run pdfInlineRun) paragraphStyle {
	style := base
	style.Bold = style.Bold || run.Bold
	style.Italic = style.Italic || run.Italic
	style.Strikethrough = style.Strikethrough || run.Strikethrough
	if run.Code {
		style.FontFamily = "monospace"
	}
	if run.Subscript || run.Superscript {
		style.FontSize *= pdfInlineScriptScale
		style.LetterSpacing *= pdfInlineScriptScale
	}
	return style
}

func inlineRunBaselineShift(base paragraphStyle, run pdfInlineRun) float64 {
	switch {
	case run.Superscript:
		return base.FontSize * pdfInlineSuperscriptRise
	case run.Subscript:
		return -base.FontSize * pdfInlineSubscriptDrop
	default:
		return 0
	}
}

func chooseInlineParagraphBreaks(words []paragraphInlineWord, spaceWidth float64, style paragraphStyle, maxWidth float64) []paragraphBreak {
	n := len(words)
	if n == 0 {
		return nil
	}
	cost := make([]float64, n+1)
	next := make([]paragraphBreak, n)
	for i := range cost {
		cost[i] = math.Inf(1)
	}
	cost[n] = 0
	for i := n - 1; i >= 0; i-- {
		width := 0.0
		for j := i; j < n; j++ {
			if j > i {
				width += spaceWidth
			}
			width += words[j].Width
			indent := 0.0
			if i == 0 {
				indent = min(max(style.FirstLineIndent, 0), maxWidth)
			}
			available := max(maxWidth-indent, 1)
			lineCost := paragraphLineCost(width, available, j == n-1, i == j)
			if math.IsInf(lineCost, 1) {
				break
			}
			candidate := lineCost + cost[j+1]
			if candidate < cost[i] {
				cost[i] = candidate
				next[i] = paragraphBreak{End: j + 1}
			}
		}
		if math.IsInf(cost[i], 1) {
			next[i] = paragraphBreak{End: i + 1}
			cost[i] = cost[i+1] + 1_000_000
		}
	}
	breaks := make([]paragraphBreak, 0, n)
	for i := 0; i < n; {
		br := next[i]
		if br.End <= i || br.End > n {
			br = paragraphBreak{End: i + 1}
		}
		breaks = append(breaks, br)
		i = br.End
	}
	return breaks
}

func inlineParagraphLineFragments(words []paragraphInlineWord, space paragraphLineFragment) ([]paragraphLineFragment, string, float64) {
	fragments := make([]paragraphLineFragment, 0, len(words)*2)
	var text strings.Builder
	width := 0.0
	for i, word := range words {
		if i > 0 {
			fragments = append(fragments, space)
			text.WriteByte(' ')
			width += space.Width
		}
		fragments = append(fragments, word.Fragments...)
		text.WriteString(word.Text)
		width += word.Width
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
