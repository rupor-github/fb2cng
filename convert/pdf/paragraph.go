package pdf

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

type textAlign int

const (
	textAlignLeft textAlign = iota
	textAlignCenter
	textAlignRight
	textAlignJustify
)

type paragraphStyle struct {
	FontSize        float64
	LineHeight      float64
	FirstLineIndent float64
	Align           textAlign
}

type paragraphLine struct {
	Text              shapedText
	Width             float64
	Indent            float64
	ExtraWordSpacing  float64
	JustificationGaps int
}

type paragraphWord struct {
	Text   string
	Shape  shapedText
	Width  float64
	Spaces int
}

func layoutParagraph(face *builtinFontFace, text string, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	if style.FontSize <= 0 {
		return nil, fmt.Errorf("paragraph font size must be positive: %g", style.FontSize)
	}
	if maxWidth <= 0 {
		return nil, fmt.Errorf("paragraph width must be positive: %g", maxWidth)
	}

	words, err := shapeParagraphWords(face, text, style.FontSize)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, nil
	}

	space, err := shapeText(face, " ")
	if err != nil {
		return nil, fmt.Errorf("shape space: %w", err)
	}
	spaceWidth := shapedWidthPoints(space, style.FontSize)

	breaks := chooseParagraphBreaks(words, spaceWidth, style, maxWidth)
	lines := make([]paragraphLine, 0, len(breaks))
	start := 0
	for i, end := range breaks {
		lineText := joinWords(words[start:end])
		shaped, err := shapeText(face, lineText)
		if err != nil {
			return nil, fmt.Errorf("shape line: %w", err)
		}

		width := shapedWidthPoints(shaped, style.FontSize)
		indent := 0.0
		if start == 0 {
			indent = min(max(style.FirstLineIndent, 0), maxWidth)
		}
		available := max(maxWidth-indent, 1)
		line := paragraphLine{
			Text:              shaped,
			Width:             width,
			Indent:            indent,
			JustificationGaps: max(0, end-start-1),
		}
		if style.Align == textAlignJustify && i != len(breaks)-1 && line.JustificationGaps > 0 && width < available {
			line.ExtraWordSpacing = (available - width) / float64(line.JustificationGaps)
		}
		lines = append(lines, line)
		start = end
	}
	return lines, nil
}

func shapeParagraphWords(face *builtinFontFace, text string, fontSize float64) ([]paragraphWord, error) {
	parts := breakableWords(text)
	words := make([]paragraphWord, 0, len(parts))
	for _, part := range parts {
		shaped, err := shapeText(face, part)
		if err != nil {
			return nil, fmt.Errorf("shape word %q: %w", part, err)
		}
		words = append(words, paragraphWord{
			Text:  part,
			Shape: shaped,
			Width: shapedWidthPoints(shaped, fontSize),
		})
	}
	return words, nil
}

func breakableWords(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var words []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		words = append(words, b.String())
		b.Reset()
	}
	for _, r := range text {
		if isBreakableSpace(r) {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return words
}

func isBreakableSpace(r rune) bool {
	return unicode.IsSpace(r) && r != '\u00a0'
}

func chooseParagraphBreaks(words []paragraphWord, spaceWidth float64, style paragraphStyle, maxWidth float64) []int {
	n := len(words)
	if n == 0 {
		return nil
	}

	cost := make([]float64, n+1)
	next := make([]int, n)
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
			lineCost := paragraphLineCost(width, available, j == n-1, j == i)
			if math.IsInf(lineCost, 1) {
				break
			}
			candidate := lineCost + cost[j+1]
			if candidate < cost[i] {
				cost[i] = candidate
				next[i] = j + 1
			}
		}
		if math.IsInf(cost[i], 1) {
			next[i] = i + 1
			cost[i] = cost[i+1] + 1_000_000
		}
	}

	breaks := make([]int, 0, n)
	for i := 0; i < n; {
		j := next[i]
		if j <= i || j > n {
			j = i + 1
		}
		breaks = append(breaks, j)
		i = j
	}
	return breaks
}

func paragraphLineCost(width, available float64, last bool, singleWord bool) float64 {
	overfull := width - available
	if overfull > 0 {
		if singleWord {
			return 1_000_000 + overfull*overfull*100
		}
		return math.Inf(1)
	}
	if last {
		return 0
	}

	ratio := (available - width) / available
	cost := ratio * ratio * 1000
	if singleWord {
		cost += 50
	}
	return cost
}

func joinWords(words []paragraphWord) string {
	var b strings.Builder
	for i, word := range words {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(word.Text)
	}
	return b.String()
}
