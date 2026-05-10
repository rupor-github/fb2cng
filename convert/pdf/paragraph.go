package pdf

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

const softHyphen = '\u00ad'

type textAlign int

const (
	textAlignLeft textAlign = iota
	textAlignCenter
	textAlignRight
	textAlignJustify
)

type paragraphStyle struct {
	FontFamily      string
	Bold            bool
	Italic          bool
	FontSize        float64
	LineHeight      float64
	FirstLineIndent float64
	Align           textAlign
	Hyphenation     paragraphHyphenation
	Hyphenator      paragraphHyphenator
}

type paragraphHyphenation int

const (
	paragraphHyphenationAuto paragraphHyphenation = iota
	paragraphHyphenationNone
	paragraphHyphenationManual
)

type paragraphHyphenator interface {
	Hyphenate(string) string
}

type paragraphLine struct {
	Text              shapedText
	Width             float64
	Indent            float64
	ExtraWordSpacing  float64
	JustificationGaps int
}

type paragraphWord struct {
	Text string
}

type paragraphAtom struct {
	Text        string
	Width       float64
	WordIndex   int
	EndWord     bool
	HyphenAfter bool
}

type paragraphBreak struct {
	End         int
	HyphenAfter bool
}

func layoutParagraph(face *builtinFontFace, text string, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	if style.FontSize <= 0 {
		return nil, fmt.Errorf("paragraph font size must be positive: %g", style.FontSize)
	}
	if maxWidth <= 0 {
		return nil, fmt.Errorf("paragraph width must be positive: %g", maxWidth)
	}

	words := paragraphWords(text)
	if len(words) == 0 {
		return nil, nil
	}

	space, err := shapeText(face, " ")
	if err != nil {
		return nil, fmt.Errorf("shape space: %w", err)
	}
	spaceWidth := shapedWidthPoints(space, style.FontSize)
	hyphen, err := shapeText(face, "-")
	if err != nil {
		return nil, fmt.Errorf("shape hyphen: %w", err)
	}
	hyphenWidth := shapedWidthPoints(hyphen, style.FontSize)

	atoms, err := paragraphAtoms(face, words, style)
	if err != nil {
		return nil, err
	}
	breaks := chooseParagraphBreaks(atoms, spaceWidth, hyphenWidth, style, maxWidth)
	lines := make([]paragraphLine, 0, len(breaks))
	start := 0
	for i, br := range breaks {
		lineText := joinAtoms(atoms[start:br.End], br.HyphenAfter)
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
			JustificationGaps: countJustificationGaps(atoms[start:br.End]),
		}
		if style.Align == textAlignJustify && i != len(breaks)-1 && line.JustificationGaps > 0 && width < available {
			line.ExtraWordSpacing = (available - width) / float64(line.JustificationGaps)
		}
		lines = append(lines, line)
		start = br.End
	}
	return lines, nil
}

func paragraphWords(text string) []paragraphWord {
	parts := breakableWords(text)
	words := make([]paragraphWord, 0, len(parts))
	for _, part := range parts {
		words = append(words, paragraphWord{Text: part})
	}
	return words
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

func paragraphAtoms(face *builtinFontFace, words []paragraphWord, style paragraphStyle) ([]paragraphAtom, error) {
	atoms := make([]paragraphAtom, 0, len(words))
	for i, word := range words {
		parts := hyphenatedWordParts(word.Text, style.Hyphenator, style.Hyphenation)
		for j, part := range parts {
			shaped, err := shapeText(face, part)
			if err != nil {
				return nil, fmt.Errorf("shape word segment %q: %w", part, err)
			}
			atoms = append(atoms, paragraphAtom{
				Text:        part,
				Width:       shapedWidthPoints(shaped, style.FontSize),
				WordIndex:   i,
				EndWord:     j == len(parts)-1,
				HyphenAfter: j != len(parts)-1,
			})
		}
	}
	return atoms, nil
}

func hyphenatedWordParts(word string, hyphenator paragraphHyphenator, mode paragraphHyphenation) []string {
	if word == "" {
		return nil
	}

	hyphenated := word
	switch mode {
	case paragraphHyphenationNone:
		return []string{strings.ReplaceAll(word, string(softHyphen), "")}
	case paragraphHyphenationManual:
		// Honor only explicit soft hyphens already present in the source text.
	case paragraphHyphenationAuto:
		if hyphenator != nil {
			hyphenated = hyphenator.Hyphenate(word)
		}
	default:
		if hyphenator != nil {
			hyphenated = hyphenator.Hyphenate(word)
		}
	}

	parts := strings.FieldsFunc(hyphenated, func(r rune) bool { return r == softHyphen })
	if len(parts) == 0 {
		return []string{strings.ReplaceAll(word, string(softHyphen), "")}
	}
	return parts
}

func chooseParagraphBreaks(atoms []paragraphAtom, spaceWidth float64, hyphenWidth float64, style paragraphStyle, maxWidth float64) []paragraphBreak {
	n := len(atoms)
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
			if j > i && atoms[j].WordIndex != atoms[j-1].WordIndex {
				width += spaceWidth
			}
			width += atoms[j].Width
			if !atoms[j].EndWord && !atoms[j].HyphenAfter {
				continue
			}

			lineWidth := width
			if atoms[j].HyphenAfter {
				lineWidth += hyphenWidth
			}
			indent := 0.0
			if i == 0 {
				indent = min(max(style.FirstLineIndent, 0), maxWidth)
			}
			available := max(maxWidth-indent, 1)
			singleWord := atoms[i].WordIndex == atoms[j].WordIndex
			lineCost := paragraphLineCost(lineWidth, available, j == n-1, singleWord)
			if math.IsInf(lineCost, 1) {
				break
			}
			if atoms[j].HyphenAfter {
				lineCost += 80
			}
			candidate := lineCost + cost[j+1]
			if candidate < cost[i] {
				cost[i] = candidate
				next[i] = paragraphBreak{End: j + 1, HyphenAfter: atoms[j].HyphenAfter}
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

func joinAtoms(atoms []paragraphAtom, hyphenAfter bool) string {
	var b strings.Builder
	for i, atom := range atoms {
		if i > 0 && atom.WordIndex != atoms[i-1].WordIndex {
			b.WriteByte(' ')
		}
		b.WriteString(atom.Text)
	}
	if hyphenAfter {
		b.WriteByte('-')
	}
	return b.String()
}

func countJustificationGaps(atoms []paragraphAtom) int {
	gaps := 0
	for i := 1; i < len(atoms); i++ {
		if atoms[i].WordIndex != atoms[i-1].WordIndex {
			gaps++
		}
	}
	return gaps
}
