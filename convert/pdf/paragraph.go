package pdf

import (
	"fmt"
	"math"
	"strings"
	"unicode"
)

const softHyphen = '\u00ad'

const (
	paragraphLinePenalty              = 10.0
	paragraphHyphenPenalty            = 1200.0
	paragraphConsecutiveHyphenPenalty = 4500.0
	paragraphFitnessPenalty           = 900.0
	paragraphEmergencyPenalty         = 1_000_000.0
)

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
	LetterSpacing   float64
	FirstLineIndent float64
	Align           textAlign
	VerticalAlign   textVerticalAlign
	Color           pdfColor
	Underline       bool
	Strikethrough   bool
	Hyphenation     paragraphHyphenation
	Hyphenator      paragraphHyphenator
}

type textVerticalAlign int

const (
	textVerticalAlignBaseline textVerticalAlign = iota
	textVerticalAlignSub
	textVerticalAlignSuper
)

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
	Fragments         []paragraphLineFragment
}

type paragraphLineFragment struct {
	Text          shapedText
	Width         float64
	FontSize      float64
	LetterSpacing float64
	FontKey       pdfFontKey
	Color         pdfColor
	Underline     bool
	Strikethrough bool
	BaselineShift float64
}

type paragraphWord struct {
	Text string
}

type paragraphUnit struct {
	Text            string
	Width           float64
	WordIndex       int
	EndWord         bool
	HyphenAfter     bool
	HyphenText      string
	HyphenWidth     float64
	Fragments       []paragraphLineFragment
	HyphenFragments []paragraphLineFragment
}

type paragraphBreak struct {
	End         int
	HyphenAfter bool
}

type paragraphBreakCandidate struct {
	Break   paragraphBreak
	Cost    float64
	Fitness paragraphFitness
}

type paragraphFitness int

const (
	paragraphFitnessTight paragraphFitness = iota
	paragraphFitnessDecent
	paragraphFitnessLoose
	paragraphFitnessVeryLoose
)

type paragraphBreakState struct {
	Cost       float64
	Prev       int
	PrevState  int
	Break      paragraphBreak
	Fitness    paragraphFitness
	Hyphenated bool
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

	hyphenWidth, err := plainHyphenWidth(face, style)
	if err != nil {
		return nil, err
	}
	units, err := paragraphUnits(face, words, style, hyphenWidth)
	if err != nil {
		return nil, err
	}
	return assembleParagraphLines(face, units, style, maxWidth)
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

func plainHyphenWidth(face *builtinFontFace, style paragraphStyle) (float64, error) {
	hyphen, err := shapeText(face, "-")
	if err != nil {
		return 0, fmt.Errorf("shape hyphen: %w", err)
	}
	return shapedWidthPointsWithSpacing(hyphen, style.FontSize, style.LetterSpacing) + max(style.LetterSpacing, 0), nil
}

func plainSpaceWidth(face *builtinFontFace, style paragraphStyle) (float64, error) {
	space, err := shapeText(face, " ")
	if err != nil {
		return 0, fmt.Errorf("shape space: %w", err)
	}
	return shapedWidthPointsWithSpacing(space, style.FontSize, style.LetterSpacing), nil
}

func paragraphUnits(face *builtinFontFace, words []paragraphWord, style paragraphStyle, hyphenWidth float64) ([]paragraphUnit, error) {
	units := make([]paragraphUnit, 0, len(words))
	for i, word := range words {
		parts := hyphenatedWordParts(word.Text, style.Hyphenator, style.Hyphenation)
		for j, part := range parts {
			shaped, err := shapeText(face, part)
			if err != nil {
				return nil, fmt.Errorf("shape word segment %q: %w", part, err)
			}
			units = append(units, paragraphUnit{
				Text:        part,
				Width:       shapedWidthPointsWithSpacing(shaped, style.FontSize, style.LetterSpacing),
				WordIndex:   i,
				EndWord:     j == len(parts)-1,
				HyphenAfter: j != len(parts)-1,
				HyphenText:  "-",
				HyphenWidth: hyphenWidth,
			})
		}
	}
	return units, nil
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

func assembleParagraphLines(face *builtinFontFace, units []paragraphUnit, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	spaceWidth, err := plainSpaceWidth(face, style)
	if err != nil {
		return nil, err
	}
	breaks := chooseParagraphBreaks(units, spaceWidth, style, maxWidth)
	lines := make([]paragraphLine, 0, len(breaks))
	start := 0
	for i, br := range breaks {
		lineText := joinUnits(units[start:br.End], br.HyphenAfter)
		shaped, err := shapeText(face, lineText)
		if err != nil {
			return nil, fmt.Errorf("shape line: %w", err)
		}

		width := shapedWidthPointsWithSpacing(shaped, style.FontSize, style.LetterSpacing)
		indent := paragraphLineIndent(start, style, maxWidth)
		available := max(maxWidth-indent, 1)
		line := paragraphLine{
			Text:              shaped,
			Width:             width,
			Indent:            indent,
			JustificationGaps: countJustificationGaps(units[start:br.End]),
		}
		line.ExtraWordSpacing = paragraphExtraWordSpacing(style, i == len(breaks)-1, width, available, line.JustificationGaps)
		lines = append(lines, line)
		start = br.End
	}
	return lines, nil
}

func chooseParagraphBreaks(units []paragraphUnit, spaceWidth float64, style paragraphStyle, maxWidth float64) []paragraphBreak {
	n := len(units)
	if n == 0 {
		return nil
	}

	const statesPerBreak = 8
	states := make([][statesPerBreak]paragraphBreakState, n+1)
	for i := range states {
		for j := range states[i] {
			states[i][j].Cost = math.Inf(1)
			states[i][j].Prev = -1
			states[i][j].PrevState = -1
		}
	}
	states[0][stateIndex(paragraphFitnessDecent, false)] = paragraphBreakState{Cost: 0, Prev: -1, PrevState: -1, Fitness: paragraphFitnessDecent}

	for start := 0; start < n; start++ {
		for stateIdx, state := range states[start] {
			if math.IsInf(state.Cost, 1) {
				continue
			}
			for _, candidate := range paragraphBreakCandidates(units, start, spaceWidth, style, maxWidth, state.Fitness, state.Hyphenated) {
				end := candidate.Break.End
				fitness := candidate.Fitness
				nextStateIdx := stateIndex(fitness, candidate.Break.HyphenAfter)
				cost := state.Cost + candidate.Cost
				if cost < states[end][nextStateIdx].Cost {
					states[end][nextStateIdx] = paragraphBreakState{
						Cost:       cost,
						Prev:       start,
						PrevState:  stateIdx,
						Break:      candidate.Break,
						Fitness:    fitness,
						Hyphenated: candidate.Break.HyphenAfter,
					}
				}
			}
		}
	}

	bestState := -1
	bestCost := math.Inf(1)
	for i, state := range states[n] {
		if state.Cost < bestCost {
			bestCost = state.Cost
			bestState = i
		}
	}
	if bestState >= 0 && !math.IsInf(bestCost, 1) {
		return paragraphBreaksFromStates(states, n, bestState)
	}
	return emergencyParagraphBreaks(units, spaceWidth, style, maxWidth)
}

func paragraphBreakCandidates(units []paragraphUnit, start int, spaceWidth float64, style paragraphStyle, maxWidth float64, previousFitness paragraphFitness, previousHyphenated bool) []paragraphBreakCandidate {
	candidates := make([]paragraphBreakCandidate, 0)
	width := 0.0
	for end := start; end < len(units); end++ {
		if end > start && units[end].WordIndex != units[end-1].WordIndex {
			width += spaceWidth
		}
		width += units[end].Width
		if !units[end].EndWord && !units[end].HyphenAfter {
			continue
		}

		lineWidth := width
		if units[end].HyphenAfter {
			lineWidth += units[end].HyphenWidth
		}
		indent := paragraphLineIndent(start, style, maxWidth)
		available := max(maxWidth-indent, 1)
		gaps := countJustificationGaps(units[start : end+1])
		last := end == len(units)-1
		lineCost := paragraphLineDemerits(lineWidth, available, gaps, start == 0, last, units[start].WordIndex == units[end].WordIndex, units[end].HyphenAfter, previousHyphenated, previousFitness)
		if math.IsInf(lineCost, 1) {
			if lineWidth > available {
				// Later candidates only get wider until the next line start, so there is no
				// useful non-emergency continuation from this start.
				break
			}
			// This candidate is too loose to be useful, but adding more material may
			// produce a well-balanced line.
			continue
		}
		candidates = append(candidates, paragraphBreakCandidate{
			Break:   paragraphBreak{End: end + 1, HyphenAfter: units[end].HyphenAfter},
			Cost:    lineCost,
			Fitness: paragraphLineFitness(lineWidth, available, gaps, last, units[start].WordIndex == units[end].WordIndex),
		})
	}
	if len(candidates) == 0 {
		end := min(start+1, len(units))
		candidates = append(candidates, paragraphBreakCandidate{
			Break:   paragraphBreak{End: end, HyphenAfter: false},
			Cost:    paragraphEmergencyPenalty,
			Fitness: paragraphFitnessVeryLoose,
		})
	}
	return candidates
}

func paragraphLineDemerits(width, available float64, gaps int, firstLine bool, last bool, singleWord bool, hyphenated bool, previousHyphenated bool, previousFitness paragraphFitness) float64 {
	ratio, emergency := paragraphAdjustmentRatio(width, available, gaps, last, singleWord)
	if math.IsInf(ratio, 1) {
		return math.Inf(1)
	}
	badness := paragraphBadness(ratio)
	if last {
		badness = paragraphFinalLineBadness(width, available, firstLine)
	}
	fitness := paragraphFitnessClass(ratio)
	demerits := math.Pow(paragraphLinePenalty+badness, 2)
	if emergency {
		demerits += paragraphEmergencyPenalty
		if firstLine {
			demerits += paragraphEmergencyPenalty * 10
		}
	}
	if hyphenated {
		demerits += paragraphHyphenPenalty
		if previousHyphenated {
			demerits += paragraphConsecutiveHyphenPenalty
		}
	}
	if !last && math.Abs(float64(fitness-previousFitness)) > 1 {
		demerits += paragraphFitnessPenalty
	}
	if singleWord && !last {
		demerits += 250
	}
	return demerits
}

func paragraphAdjustmentRatio(width, available float64, gaps int, last bool, singleWord bool) (float64, bool) {
	delta := available - width
	if last {
		if width > available && !singleWord {
			return math.Inf(1), false
		}
		if width > available {
			return 1, true
		}
		return 0, false
	}
	if delta >= 0 {
		if gaps == 0 {
			if singleWord {
				return min(delta/max(available, 1), 1.5), false
			}
			return math.Inf(1), false
		}
		stretch := max(float64(gaps)*max(available*0.02, 0.5), float64(gaps)*2)
		return delta / stretch, false
	}
	if gaps == 0 || singleWord {
		return 1, true
	}
	shrink := max(float64(gaps)*max(available*0.006, 0.2), float64(gaps)*0.75)
	ratio := -delta / shrink
	if ratio > 2.0 {
		return math.Inf(1), false
	}
	return -ratio, false
}

func paragraphBadness(ratio float64) float64 {
	ratio = math.Abs(ratio)
	if ratio > 3 {
		return math.Inf(1)
	}
	return min(10_000, 100*math.Pow(ratio, 3))
}

func paragraphFinalLineBadness(width, available float64, firstLine bool) float64 {
	if firstLine || available <= 0 || width >= available*0.35 {
		return 0
	}
	ratio := (available*0.35 - width) / available
	return ratio * ratio * 250
}

func paragraphLineFitness(width, available float64, gaps int, last bool, singleWord bool) paragraphFitness {
	if last {
		return paragraphFitnessDecent
	}
	ratio, _ := paragraphAdjustmentRatio(width, available, gaps, last, singleWord)
	if math.IsInf(ratio, 1) {
		return paragraphFitnessVeryLoose
	}
	return paragraphFitnessClass(ratio)
}

func paragraphFitnessClass(ratio float64) paragraphFitness {
	switch {
	case ratio < -0.5:
		return paragraphFitnessTight
	case ratio <= 0.5:
		return paragraphFitnessDecent
	case ratio <= 1:
		return paragraphFitnessLoose
	default:
		return paragraphFitnessVeryLoose
	}
}

func stateIndex(fitness paragraphFitness, hyphenated bool) int {
	idx := int(fitness) * 2
	if hyphenated {
		idx++
	}
	return idx
}

func paragraphBreaksFromStates(states [][8]paragraphBreakState, end int, stateIdx int) []paragraphBreak {
	breaks := make([]paragraphBreak, 0)
	for end > 0 && stateIdx >= 0 {
		state := states[end][stateIdx]
		if state.Prev < 0 || state.Break.End <= state.Prev {
			break
		}
		breaks = append(breaks, state.Break)
		end, stateIdx = state.Prev, state.PrevState
	}
	for i, j := 0, len(breaks)-1; i < j; i, j = i+1, j-1 {
		breaks[i], breaks[j] = breaks[j], breaks[i]
	}
	if len(breaks) == 0 {
		return nil
	}
	return breaks
}

func emergencyParagraphBreaks(units []paragraphUnit, spaceWidth float64, style paragraphStyle, maxWidth float64) []paragraphBreak {
	breaks := make([]paragraphBreak, 0)
	for start := 0; start < len(units); {
		width := 0.0
		best := start + 1
		bestHyphen := false
		for end := start; end < len(units); end++ {
			if end > start && units[end].WordIndex != units[end-1].WordIndex {
				width += spaceWidth
			}
			width += units[end].Width
			lineWidth := width
			if units[end].HyphenAfter {
				lineWidth += units[end].HyphenWidth
			}
			if !units[end].EndWord && !units[end].HyphenAfter {
				continue
			}
			indent := paragraphLineIndent(start, style, maxWidth)
			available := max(maxWidth-indent, 1)
			if lineWidth <= available || best == start+1 {
				best = end + 1
				bestHyphen = units[end].HyphenAfter
			}
			if lineWidth > available && best != start+1 {
				break
			}
		}
		breaks = append(breaks, paragraphBreak{End: best, HyphenAfter: bestHyphen})
		start = best
	}
	return breaks
}

func paragraphLineIndent(start int, style paragraphStyle, maxWidth float64) float64 {
	if start != 0 {
		return 0
	}
	return min(max(style.FirstLineIndent, 0), maxWidth)
}

func paragraphExtraWordSpacing(style paragraphStyle, last bool, width, available float64, gaps int) float64 {
	if style.Align != textAlignJustify || last || gaps <= 0 || width >= available {
		return 0
	}
	extra := (available - width) / float64(gaps)
	if extra > max(style.FontSize*0.55, 2.5) {
		return 0
	}
	return extra
}

func joinUnits(units []paragraphUnit, hyphenAfter bool) string {
	var b strings.Builder
	for i, unit := range units {
		if i > 0 && unit.WordIndex != units[i-1].WordIndex {
			b.WriteByte(' ')
		}
		b.WriteString(unit.Text)
	}
	if hyphenAfter && len(units) > 0 {
		b.WriteString(units[len(units)-1].HyphenText)
	}
	return b.String()
}

func countJustificationGaps(units []paragraphUnit) int {
	gaps := 0
	for i := 1; i < len(units); i++ {
		if units[i].WordIndex != units[i-1].WordIndex {
			gaps++
		}
	}
	return gaps
}
