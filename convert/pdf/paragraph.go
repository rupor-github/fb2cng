package pdf

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	contentText "fbc/content/text"
)

const (
	paragraphLinePenalty              = 10.0
	paragraphHyphenPenalty            = 350.0
	paragraphConsecutiveHyphenPenalty = 1800.0
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
	FontFamily          string
	Bold                bool
	HasBold             bool
	Italic              bool
	HasItalic           bool
	FontSize            float64
	FontSizeSpec        pdfCSSLengthSpec
	LineHeight          float64
	LineHeightSpec      pdfCSSLengthSpec
	LineHeightExplicit  bool
	LetterSpacing       float64
	LetterSpacingSpec   pdfCSSLengthSpec
	FirstLineIndent     float64
	FirstLineIndentSpec pdfCSSLengthSpec
	HasFirstLineIndent  bool
	Align               textAlign
	HasAlign            bool
	VerticalAlign       textVerticalAlign
	HasVerticalAlign    bool
	Color               pdfColor
	Underline           bool
	HasUnderline        bool
	Strikethrough       bool
	HasStrikethrough    bool
	PreserveSpace       bool
	HasPreserveSpace    bool
	NoWrap              bool
	HasNoWrap           bool
	Hyphenation         paragraphHyphenation
	HasHyphenation      bool
	Hyphenator          paragraphHyphenator
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

type paragraphLineShape struct {
	InitialInsets []float64
}

type paragraphLine struct {
	Text              shapedText
	Width             float64
	Indent            float64
	ExtraWordSpacing  float64
	ExtraCharSpacing  float64
	JustificationGaps int
	Fragments         []paragraphLineFragment
	BreakStats        paragraphLineBreakStats
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
	StyleClasses  string
	LinkHref      string
	AnchorID      string
	FootnoteID    string
	ImageID       string
	ImageHeight   float64
}

type paragraphWord struct {
	Text string
}

type paragraphWordPart struct {
	Text       string
	BreakAfter bool
	Hyphenated bool
	HyphenText string
}

type paragraphUnit struct {
	Text                string
	Width               float64
	WordIndex           int
	EndWord             bool
	BreakAfter          bool
	Hyphenated          bool
	HyphenText          string
	HyphenWidth         float64
	GlyphCount          int
	HyphenGlyphCount    int
	RightOverhang       float64
	HyphenRightOverhang float64
	EmergencyBreakAfter bool
	Fragments           []paragraphLineFragment
	HyphenFragments     []paragraphLineFragment
}

type paragraphBreak struct {
	End         int
	HyphenAfter bool
	Hyphenated  bool
	Emergency   bool
}

type paragraphBreakCandidate struct {
	Break   paragraphBreak
	Cost    float64
	Fitness paragraphFitness
}

type paragraphLineBreakStats struct {
	AvailableWidth  float64
	AdjustmentRatio float64
	Badness         float64
	Demerits        float64
	Fitness         paragraphFitness
	Hyphenated      bool
	Emergency       bool
	SingleWord      bool
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
	ShapeLine  int
}

func layoutParagraph(face *builtinFontFace, text string, style paragraphStyle, maxWidth float64) ([]paragraphLine, error) {
	return layoutParagraphWithShape(face, text, style, maxWidth, paragraphLineShape{})
}

func layoutParagraphWithShape(face *builtinFontFace, text string, style paragraphStyle, maxWidth float64, shape paragraphLineShape) ([]paragraphLine, error) {
	if style.FontSize <= 0 {
		return nil, fmt.Errorf("paragraph font size must be positive: %g", style.FontSize)
	}
	if maxWidth <= 0 {
		return nil, fmt.Errorf("paragraph width must be positive: %g", maxWidth)
	}

	words := paragraphWords(text, style.NoWrap)
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
	units, err = splitPlainEmergencyParagraphUnits(face, units, style, maxWidth, shape)
	if err != nil {
		return nil, err
	}
	return assembleParagraphLines(face, units, style, maxWidth, shape)
}

func paragraphWords(text string, noWrap bool) []paragraphWord {
	parts := breakableWords(text)
	if noWrap && len(parts) > 0 {
		return []paragraphWord{{Text: strings.Join(parts, " ")}}
	}
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

func paragraphUnits(face *builtinFontFace, words []paragraphWord, style paragraphStyle, softHyphenWidth float64) ([]paragraphUnit, error) {
	units := make([]paragraphUnit, 0, len(words))
	for i, word := range words {
		parts := hyphenatedWordParts(word.Text, style.Hyphenator, pdfEffectiveHyphenation(style))
		for j, part := range parts {
			shaped, err := shapeText(face, part.Text)
			if err != nil {
				return nil, fmt.Errorf("shape word segment %q: %w", part.Text, err)
			}
			width := shapedWidthPointsWithSpacing(shaped, style.FontSize, style.LetterSpacing)
			hyphenWidth := 0.0
			hyphenGlyphCount := 0
			hyphenOverhang := 0.0
			if part.HyphenText != "" {
				hyphenWidth = softHyphenWidth
				hyphen, err := shapeText(face, part.HyphenText)
				if err != nil {
					return nil, fmt.Errorf("shape hyphen %q: %w", part.HyphenText, err)
				}
				hyphenGlyphCount = len(hyphen.Glyphs)
				hyphenated, err := shapeText(face, part.Text+part.HyphenText)
				if err != nil {
					return nil, fmt.Errorf("shape hyphenated word segment %q: %w", part.Text+part.HyphenText, err)
				}
				hyphenOverhang = shapedTextRightOverhang(hyphenated, style.FontSize, style.LetterSpacing)
			}
			units = append(units, paragraphUnit{
				Text:                part.Text,
				Width:               width,
				WordIndex:           i,
				EndWord:             j == len(parts)-1,
				BreakAfter:          part.BreakAfter,
				Hyphenated:          part.Hyphenated,
				HyphenText:          part.HyphenText,
				HyphenWidth:         hyphenWidth,
				GlyphCount:          len(shaped.Glyphs),
				HyphenGlyphCount:    hyphenGlyphCount,
				RightOverhang:       shapedTextRightOverhang(shaped, style.FontSize, style.LetterSpacing),
				HyphenRightOverhang: hyphenOverhang,
			})
		}
	}
	return units, nil
}

func splitPlainEmergencyParagraphUnits(
	face *builtinFontFace,
	units []paragraphUnit,
	style paragraphStyle,
	maxWidth float64,
	shape paragraphLineShape,
) ([]paragraphUnit, error) {
	if style.NoWrap || len(units) == 0 {
		return units, nil
	}

	splitWidth := paragraphEmergencySplitWidth(style, maxWidth, shape)
	out := make([]paragraphUnit, 0, len(units))
	for _, unit := range units {
		if !paragraphUnitNeedsEmergencySplit(unit, splitWidth) {
			out = append(out, unit)
			continue
		}
		pieces, err := splitPlainEmergencyParagraphUnit(face, unit, style)
		if err != nil {
			return nil, err
		}
		if len(pieces) <= 1 {
			out = append(out, unit)
			continue
		}
		out = append(out, pieces...)
	}
	return out, nil
}

func paragraphEmergencySplitWidth(style paragraphStyle, maxWidth float64, shape paragraphLineShape) float64 {
	minAvailable := max(maxWidth, 1)
	consider := func(indent float64) {
		minAvailable = min(minAvailable, max(maxWidth-max(indent, 0), 1))
	}
	consider(style.FirstLineIndent)
	for i, inset := range shape.InitialInsets {
		indent := inset
		if i == 0 {
			indent += style.FirstLineIndent
		}
		consider(indent)
	}
	return minAvailable
}

func paragraphUnitNeedsEmergencySplit(unit paragraphUnit, splitWidth float64) bool {
	if unit.Text == "" || len([]rune(unit.Text)) < 2 {
		return false
	}
	return unit.Width+unit.HyphenWidth > splitWidth+pdfLineWidthTolerance
}

func splitPlainEmergencyParagraphUnit(face *builtinFontFace, unit paragraphUnit, style paragraphStyle) ([]paragraphUnit, error) {
	shaped, err := shapeText(face, unit.Text)
	if err != nil {
		return nil, fmt.Errorf("shape emergency word segment %q: %w", unit.Text, err)
	}
	clusters := shapedTextEmergencyClusters(shaped, unit.Text)
	if len(clusters) <= 1 {
		return nil, nil
	}

	pieces := make([]paragraphUnit, 0, len(clusters))
	for i, cluster := range clusters {
		piece := paragraphUnit{
			Text:                cluster.Text,
			Width:               shapedWidthPointsWithSpacing(cluster.TextShape, style.FontSize, style.LetterSpacing),
			WordIndex:           unit.WordIndex,
			GlyphCount:          len(cluster.TextShape.Glyphs),
			RightOverhang:       shapedTextRightOverhang(cluster.TextShape, style.FontSize, style.LetterSpacing),
			EmergencyBreakAfter: i != len(clusters)-1,
		}
		if i == len(clusters)-1 {
			piece.EndWord = unit.EndWord
			piece.BreakAfter = unit.BreakAfter
			piece.Hyphenated = unit.Hyphenated
			piece.HyphenText = unit.HyphenText
			piece.HyphenWidth = unit.HyphenWidth
			piece.HyphenGlyphCount = unit.HyphenGlyphCount
			piece.HyphenRightOverhang = unit.HyphenRightOverhang
		}
		pieces = append(pieces, piece)
	}
	return pieces, nil
}

type paragraphEmergencyCluster struct {
	Text      string
	TextShape shapedText
}

func shapedTextEmergencyClusters(shaped shapedText, text string) []paragraphEmergencyCluster {
	if len(shaped.Glyphs) == 0 {
		return nil
	}

	runes := []rune(text)
	clusters := make([]paragraphEmergencyCluster, 0, len(shaped.Glyphs))
	for _, glyph := range shaped.Glyphs {
		start := min(max(glyph.ClusterStart, 0), len(runes))
		end := min(max(glyph.ClusterEnd, start), len(runes))
		if end == start {
			end = min(start+1, len(runes))
		}
		glyphText := string(runes[start:end])
		last := len(clusters) - 1
		if last >= 0 && sameShapedGlyphCluster(clusters[last].TextShape.Glyphs[0], glyph) {
			clusters[last].TextShape.Glyphs = append(clusters[last].TextShape.Glyphs, glyph)
			if glyph.GlyphID != 0 {
				clusters[last].TextShape.Used[glyph.GlyphID] = glyph
			}
			continue
		}
		clusters = append(clusters, paragraphEmergencyCluster{
			Text: glyphText,
			TextShape: shapedText{
				Glyphs: []shapedGlyph{glyph},
				Used:   shapedGlyphUsedMap([]shapedGlyph{glyph}),
			},
		})
	}
	return mergeLeadingCombiningEmergencyClusters(clusters)
}

func sameShapedGlyphCluster(a, b shapedGlyph) bool {
	return a.ClusterStart == b.ClusterStart && a.ClusterEnd == b.ClusterEnd
}

func shapedGlyphUsedMap(glyphs []shapedGlyph) map[uint16]shapedGlyph {
	used := make(map[uint16]shapedGlyph)
	for _, glyph := range glyphs {
		if glyph.GlyphID != 0 {
			used[glyph.GlyphID] = glyph
		}
	}
	return used
}

func mergeLeadingCombiningEmergencyClusters(clusters []paragraphEmergencyCluster) []paragraphEmergencyCluster {
	merged := make([]paragraphEmergencyCluster, 0, len(clusters))
	for _, cluster := range clusters {
		if len(merged) > 0 && startsWithCombiningMark(cluster.Text) {
			last := &merged[len(merged)-1]
			last.Text += cluster.Text
			last.TextShape.Glyphs = append(last.TextShape.Glyphs, cluster.TextShape.Glyphs...)
			for id, glyph := range cluster.TextShape.Used {
				last.TextShape.Used[id] = glyph
			}
			continue
		}
		merged = append(merged, cluster)
	}
	return merged
}

func startsWithCombiningMark(text string) bool {
	for _, r := range text {
		return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r)
	}
	return false
}

func pdfEffectiveHyphenation(style paragraphStyle) paragraphHyphenation {
	if style.NoWrap {
		return paragraphHyphenationNone
	}
	return style.Hyphenation
}

func hyphenatedWordParts(word string, hyphenator paragraphHyphenator, mode paragraphHyphenation) []paragraphWordPart {
	if word == "" {
		return nil
	}

	hyphenated := word
	switch mode {
	case paragraphHyphenationNone:
		return punctuationWordParts(strings.ReplaceAll(word, contentText.SOFTHYPHEN, ""))
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

	segments := strings.Split(hyphenated, contentText.SOFTHYPHEN)
	parts := make([]paragraphWordPart, 0, len(segments))
	for segmentIndex, segment := range segments {
		segmentParts := punctuationWordParts(segment)
		if len(segmentParts) == 0 {
			continue
		}
		if segmentIndex != len(segments)-1 {
			last := len(segmentParts) - 1
			segmentParts[last].BreakAfter = true
			segmentParts[last].Hyphenated = true
			segmentParts[last].HyphenText = "-"
		}
		parts = append(parts, segmentParts...)
	}
	if len(parts) == 0 {
		return punctuationWordParts(strings.ReplaceAll(word, contentText.SOFTHYPHEN, ""))
	}
	return parts
}

func punctuationWordParts(word string) []paragraphWordPart {
	if word == "" {
		return nil
	}

	parts := make([]paragraphWordPart, 0, 1)
	var b strings.Builder
	for _, r := range word {
		b.WriteRune(r)
		if !isWordInternalBreakRune(r) {
			continue
		}
		parts = append(parts, paragraphWordPart{
			Text:       b.String(),
			BreakAfter: true,
			Hyphenated: isHyphenLikeBreakRune(r),
		})
		b.Reset()
	}
	if b.Len() > 0 {
		parts = append(parts, paragraphWordPart{Text: b.String()})
	}
	return parts
}

func isWordInternalBreakRune(r rune) bool {
	switch r {
	case '-', '\u2010', '\u2012', '\u2013', '\u2014', '/', '\u2044':
		return true
	default:
		return false
	}
}

func isHyphenLikeBreakRune(r rune) bool {
	switch r {
	case '-', '\u2010', '\u2012', '\u2013', '\u2014':
		return true
	default:
		return false
	}
}

func assembleParagraphLines(face *builtinFontFace, units []paragraphUnit, style paragraphStyle, maxWidth float64, shape paragraphLineShape) ([]paragraphLine, error) {
	spaceWidth, err := plainSpaceWidth(face, style)
	if err != nil {
		return nil, err
	}
	breaks := chooseBreaksWithShape(units, spaceWidth, style, maxWidth, shape)
	lines := make([]paragraphLine, 0, len(breaks))
	start := 0
	previousHyphenated := false
	previousFitness := paragraphFitnessDecent
	for i, br := range breaks {
		lineText := joinUnits(units[start:br.End], br.HyphenAfter)
		shaped, err := shapeText(face, lineText)
		if err != nil {
			return nil, fmt.Errorf("shape line: %w", err)
		}

		width := shapedWidthPointsWithSpacing(shaped, style.FontSize, style.LetterSpacing)
		indent := paragraphLineIndentForLine(start, i, style, maxWidth, shape)
		available := max(maxWidth-indent, 1)
		line := paragraphLine{
			Text:              shaped,
			Width:             width,
			Indent:            indent,
			JustificationGaps: countJustificationGaps(units[start:br.End]),
		}
		last := i == len(breaks)-1
		singleWord := units[start].WordIndex == units[br.End-1].WordIndex
		terminalOverhang := paragraphBreakTerminalOverhangFor(units[br.End-1], br.HyphenAfter)
		visualMetricWidth := width + terminalOverhang
		line.BreakStats = lineBreakStats(
			visualMetricWidth,
			available,
			line.JustificationGaps,
			start == 0,
			last,
			singleWord,
			br.Hyphenated,
			previousHyphenated,
			previousFitness,
		)
		if br.Emergency {
			line.BreakStats.Emergency = true
			line.BreakStats.Demerits += paragraphEmergencyPenalty
		}
		spacingAvailable := paragraphJustificationAvailableForOverhang(available, terminalOverhang)
		line.ExtraWordSpacing, line.ExtraCharSpacing = paragraphJustificationSpacing(
			style,
			last,
			width,
			spacingAvailable,
			line.JustificationGaps,
			len(shaped.Glyphs),
		)
		lines = append(lines, line)
		previousHyphenated = br.Hyphenated
		previousFitness = line.BreakStats.Fitness
		start = br.End
	}
	return lines, nil
}

func chooseBreaksWithShape(units []paragraphUnit, spaceWidth float64, style paragraphStyle, maxWidth float64, shape paragraphLineShape) []paragraphBreak {
	n := len(units)
	if n == 0 {
		return nil
	}

	shapeStates := max(len(shape.InitialInsets)+1, 1)
	statesPerBreak := shapeStates * 8
	states := make([][]paragraphBreakState, n+1)
	for i := range states {
		states[i] = make([]paragraphBreakState, statesPerBreak)
		for j := range states[i] {
			states[i][j].Cost = math.Inf(1)
			states[i][j].Prev = -1
			states[i][j].PrevState = -1
		}
	}
	states[0][stateIndexWithShape(paragraphFitnessDecent, false, 0, shapeStates)] = paragraphBreakState{Cost: 0, Prev: -1, PrevState: -1, Fitness: paragraphFitnessDecent, ShapeLine: 0}

	for start := 0; start < n; start++ {
		for stateIdx, state := range states[start] {
			if math.IsInf(state.Cost, 1) {
				continue
			}
			for _, candidate := range paragraphBreakCandidates(units, start, spaceWidth, style, maxWidth, shape, state.ShapeLine, state.Fitness, state.Hyphenated) {
				end := candidate.Break.End
				fitness := candidate.Fitness
				nextShapeLine := min(state.ShapeLine+1, shapeStates-1)
				nextStateIdx := stateIndexWithShape(fitness, candidate.Break.Hyphenated, nextShapeLine, shapeStates)
				cost := state.Cost + candidate.Cost
				if cost < states[end][nextStateIdx].Cost {
					states[end][nextStateIdx] = paragraphBreakState{
						Cost:       cost,
						Prev:       start,
						PrevState:  stateIdx,
						Break:      candidate.Break,
						Fitness:    fitness,
						Hyphenated: candidate.Break.Hyphenated,
						ShapeLine:  nextShapeLine,
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
	return emergencyParagraphBreaks(units, spaceWidth, style, maxWidth, shape)
}

func paragraphBreakCandidates(
	units []paragraphUnit,
	start int,
	spaceWidth float64,
	style paragraphStyle,
	maxWidth float64,
	shape paragraphLineShape,
	lineIndex int,
	previousFitness paragraphFitness,
	previousHyphenated bool,
) []paragraphBreakCandidate {
	candidates := make([]paragraphBreakCandidate, 0)
	width := 0.0
	for end := start; end < len(units); end++ {
		if end > start && units[end].WordIndex != units[end-1].WordIndex {
			width += spaceWidth
		}
		width += units[end].Width

		if units[end].EmergencyBreakAfter {
			if candidate, ok := paragraphBreakCandidateFor(
				units, start, end, width, style, maxWidth, shape, lineIndex, previousFitness, previousHyphenated, true,
			); ok {
				candidates = append(candidates, candidate)
			}
		}
		if !units[end].EndWord && !units[end].BreakAfter {
			continue
		}

		candidate, ok := paragraphBreakCandidateFor(
			units, start, end, width, style, maxWidth, shape, lineIndex, previousFitness, previousHyphenated, false,
		)
		if !ok {
			indent := paragraphLineIndentForLine(start, lineIndex, style, maxWidth, shape)
			available := max(maxWidth-indent, 1)
			if width+paragraphBreakTerminalOverhang(units[end]) > available {
				// Later candidates only get wider until the next line start, so there is no
				// useful non-emergency continuation from this start.
				break
			}
			// This candidate is too loose to be useful, but adding more material may
			// produce a well-balanced line.
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		end := min(start+1, len(units))
		candidates = append(candidates, paragraphBreakCandidate{
			Break:   paragraphBreak{End: end, Emergency: true},
			Cost:    paragraphEmergencyPenalty,
			Fitness: paragraphFitnessVeryLoose,
		})
	}
	return candidates
}

func paragraphBreakCandidateFor(
	units []paragraphUnit,
	start int,
	end int,
	width float64,
	style paragraphStyle,
	maxWidth float64,
	shape paragraphLineShape,
	lineIndex int,
	previousFitness paragraphFitness,
	previousHyphenated bool,
	emergency bool,
) (paragraphBreakCandidate, bool) {
	lineWidth := width
	hyphenAfter := !emergency && units[end].HyphenText != ""
	if hyphenAfter {
		lineWidth += units[end].HyphenWidth
	}
	indent := paragraphLineIndentForLine(start, lineIndex, style, maxWidth, shape)
	available := max(maxWidth-indent, 1)
	gaps := countJustificationGaps(units[start : end+1])
	last := end == len(units)-1
	terminalOverhang := paragraphBreakTerminalOverhangFor(units[end], hyphenAfter)
	metricWidth := lineWidth + terminalOverhang
	singleWord := units[start].WordIndex == units[end].WordIndex
	if !emergency && singleWord && metricWidth > available && paragraphRangeHasEmergencyBreak(units[start:end]) {
		return paragraphBreakCandidate{}, false
	}
	hyphenated := !emergency && units[end].Hyphenated
	lineCost := paragraphLineDemerits(
		metricWidth,
		available,
		gaps,
		start == 0,
		last,
		singleWord,
		hyphenated,
		previousHyphenated,
		previousFitness,
	)
	if math.IsInf(lineCost, 1) {
		return paragraphBreakCandidate{}, false
	}
	if emergency {
		lineCost += paragraphEmergencyPenalty
	}
	return paragraphBreakCandidate{
		Break: paragraphBreak{
			End:         end + 1,
			HyphenAfter: hyphenAfter,
			Hyphenated:  hyphenated,
			Emergency:   emergency,
		},
		Cost:    lineCost,
		Fitness: paragraphLineFitness(metricWidth, available, gaps, last, singleWord),
	}, true
}

func paragraphRangeHasEmergencyBreak(units []paragraphUnit) bool {
	for _, unit := range units {
		if unit.EmergencyBreakAfter {
			return true
		}
	}
	return false
}

func paragraphLineDemerits(width, available float64, gaps int, firstLine bool, last bool, singleWord bool, hyphenated bool, previousHyphenated bool, previousFitness paragraphFitness) float64 {
	return lineBreakStats(width, available, gaps, firstLine, last, singleWord, hyphenated, previousHyphenated, previousFitness).Demerits
}

func lineBreakStats(width, available float64, gaps int, firstLine bool, last bool, singleWord bool, hyphenated bool, previousHyphenated bool, previousFitness paragraphFitness) paragraphLineBreakStats {
	ratio, emergency := paragraphAdjustmentRatio(width, available, gaps, last, singleWord)
	if math.IsInf(ratio, 1) {
		return paragraphLineBreakStats{
			AvailableWidth:  available,
			AdjustmentRatio: ratio,
			Badness:         math.Inf(1),
			Demerits:        math.Inf(1),
			Fitness:         paragraphFitnessVeryLoose,
			Hyphenated:      hyphenated,
			SingleWord:      singleWord,
		}
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
		unused := max(available-width, 0) / max(available, 1)
		demerits += 5_000 + unused*unused*20_000
	}
	return paragraphLineBreakStats{
		AvailableWidth:  available,
		AdjustmentRatio: ratio,
		Badness:         badness,
		Demerits:        demerits,
		Fitness:         fitness,
		Hyphenated:      hyphenated,
		Emergency:       emergency,
		SingleWord:      singleWord,
	}
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
		// Use paragraph-wide raggedness tolerance for break choice, not only the
		// amount of spacing we are willing to add while drawing justified text. A
		// line with few word gaps may be visually better left slightly ragged than
		// replaced by a very short single-word line.
		stretch := max(available*0.20, float64(gaps)*max(available*0.02, 0.5))
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

func paragraphFitnessString(fitness paragraphFitness) string {
	switch fitness {
	case paragraphFitnessTight:
		return "tight"
	case paragraphFitnessDecent:
		return "decent"
	case paragraphFitnessLoose:
		return "loose"
	case paragraphFitnessVeryLoose:
		return "very-loose"
	default:
		return "unknown"
	}
}

func stateIndex(fitness paragraphFitness, hyphenated bool) int {
	idx := int(fitness) * 2
	if hyphenated {
		idx++
	}
	return idx
}

func stateIndexWithShape(fitness paragraphFitness, hyphenated bool, shapeLine int, shapeStates int) int {
	shapeLine = min(max(shapeLine, 0), max(shapeStates-1, 0))
	return shapeLine*8 + stateIndex(fitness, hyphenated)
}

func paragraphBreaksFromStates(states [][]paragraphBreakState, end int, stateIdx int) []paragraphBreak {
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

func emergencyParagraphBreaks(units []paragraphUnit, spaceWidth float64, style paragraphStyle, maxWidth float64, shape paragraphLineShape) []paragraphBreak {
	breaks := make([]paragraphBreak, 0)
	for start, lineIndex := 0, 0; start < len(units); lineIndex++ {
		width := 0.0
		best := start + 1
		bestHyphen := false
		bestHyphenated := false
		bestEmergency := true
		for end := start; end < len(units); end++ {
			if end > start && units[end].WordIndex != units[end-1].WordIndex {
				width += spaceWidth
			}
			width += units[end].Width
			canNormalBreak := units[end].EndWord || units[end].BreakAfter
			canEmergencyBreak := units[end].EmergencyBreakAfter
			if !canNormalBreak && !canEmergencyBreak {
				continue
			}
			hyphenAfter := canNormalBreak && !canEmergencyBreak && units[end].HyphenText != ""
			lineWidth := width
			if hyphenAfter {
				lineWidth += units[end].HyphenWidth
			}
			indent := paragraphLineIndentForLine(start, lineIndex, style, maxWidth, shape)
			available := max(maxWidth-indent, 1)
			if lineWidth <= available || best == start+1 {
				best = end + 1
				bestHyphen = hyphenAfter
				bestHyphenated = hyphenAfter && units[end].Hyphenated
				bestEmergency = canEmergencyBreak
			}
			if lineWidth > available && best != start+1 {
				break
			}
		}
		breaks = append(breaks, paragraphBreak{End: best, HyphenAfter: bestHyphen, Hyphenated: bestHyphenated, Emergency: bestEmergency})
		start = best
	}
	return breaks
}

func paragraphLineIndentForLine(start int, lineIndex int, style paragraphStyle, maxWidth float64, shape paragraphLineShape) float64 {
	indent := 0.0
	if start == 0 {
		indent += max(style.FirstLineIndent, 0)
	}
	if lineIndex >= 0 && lineIndex < len(shape.InitialInsets) {
		indent += max(shape.InitialInsets[lineIndex], 0)
	}
	return min(indent, maxWidth)
}

func shapedTextRightOverhang(text shapedText, fontSize float64, letterSpacing float64) float64 {
	width := shapedWidthPointsWithSpacing(text, fontSize, letterSpacing)
	_, right, ok := shapedTextVisualBounds(text, fontSize, letterSpacing, 0)
	if !ok {
		return 0
	}
	return max(right-width, 0)
}

func paragraphBreakTerminalOverhang(unit paragraphUnit) float64 {
	return paragraphBreakTerminalOverhangFor(unit, unit.HyphenText != "")
}

func paragraphBreakTerminalOverhangFor(unit paragraphUnit, hyphenAfter bool) float64 {
	if hyphenAfter {
		return unit.HyphenRightOverhang
	}
	return unit.RightOverhang
}

func paragraphJustificationAvailableForOverhang(available float64, terminalOverhang float64) float64 {
	if available <= 1 {
		return available
	}
	return max(available-max(terminalOverhang, 0), 1)
}

func paragraphLineVisualRightReserve(line paragraphLine, fontSize float64, letterSpacing float64) float64 {
	var right float64
	var ok bool
	if len(line.Fragments) == 0 {
		_, right, ok = shapedTextVisualBounds(line.Text, fontSize, letterSpacing, 0)
	} else {
		right, ok = paragraphFragmentLineVisualRight(line)
	}
	if !ok {
		return 0
	}
	return max(right-line.Width, 0)
}

func paragraphFragmentLineVisualRight(line paragraphLine) (float64, bool) {
	right := math.Inf(-1)
	currentX := 0.0
	ok := false
	for _, fragment := range line.Fragments {
		_, fragmentRight, fragmentOK := shapedTextVisualBounds(fragment.Text, fragment.FontSize, fragment.LetterSpacing, 0)
		if fragmentOK {
			right = max(right, currentX+fragmentRight)
			ok = true
		}
		currentX += fragment.Width
	}
	return right, ok
}

func paragraphJustificationSpacing(style paragraphStyle, last bool, width, available float64, gaps int, glyphs int) (float64, float64) {
	if style.Align != textAlignJustify || last || gaps <= 0 || width == available {
		return 0, 0
	}
	remaining := available - width
	if remaining < 0 {
		return paragraphJustificationShrink(style, -remaining, gaps, glyphs)
	}
	wordCap, charCap := paragraphJustificationStretchCaps(style.FontSize)
	wordSpacing := min(remaining/float64(gaps), wordCap)
	remaining -= wordSpacing * float64(gaps)
	if remaining <= 0 || glyphs < 2 {
		return wordSpacing, 0
	}

	// When word spacing alone would create rivers, distribute the remaining
	// adjustment as small character spacing. This is closer to book composition:
	// spaces carry most of the stretch, but tiny tracking changes can make the
	// margin even without obvious holes between words. The caps are soft: if the
	// paragraph breaker selected this line, keep text-align: justify semantics and
	// put any residual stretch back into word spaces so the right edge remains flush.
	charSpacing := min(remaining/float64(glyphs-1), charCap)
	remaining -= charSpacing * float64(glyphs-1)
	if remaining > pdfLineWidthTolerance {
		wordSpacing += remaining / float64(gaps)
	}
	return wordSpacing, charSpacing
}

func paragraphJustificationShrink(style paragraphStyle, overflow float64, gaps int, glyphs int) (float64, float64) {
	wordCap, charCap := paragraphJustificationShrinkCaps(style.FontSize)
	wordShrink := min(overflow/float64(gaps), wordCap)
	overflow -= wordShrink * float64(gaps)
	if overflow <= 0 || glyphs < 2 {
		return -wordShrink, 0
	}

	// Keep negative tracking conservative: it is a last small correction after
	// spaces have absorbed most of the shrink, not a substitute for better breaks.
	charShrink := min(overflow/float64(glyphs-1), charCap)
	return -wordShrink, -charShrink
}

func paragraphJustificationStretchCaps(fontSize float64) (float64, float64) {
	return max(fontSize*0.40, 3.0), 0.25
}

func paragraphJustificationShrinkCaps(fontSize float64) (float64, float64) {
	return max(fontSize*0.18, 1.0), min(max(fontSize*0.025, 0.12), 0.35)
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
