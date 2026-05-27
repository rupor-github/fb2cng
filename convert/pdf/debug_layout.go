package pdf

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

func pdfPageLineText(line pdfPageLine) string {
	if len(line.Fragments) == 0 {
		return shapedRunes(line.Text)
	}
	var b strings.Builder
	for _, fragment := range line.Fragments {
		b.WriteString(shapedRunes(fragment.Text))
	}
	return b.String()
}

func pdfDebugLineBreakStats(stats paragraphLineBreakStats) *pdfDebugLineBreak {
	if stats.AvailableWidth <= 0 || math.IsInf(stats.AdjustmentRatio, 0) || math.IsInf(stats.Badness, 0) || math.IsInf(stats.Demerits, 0) {
		return nil
	}
	return &pdfDebugLineBreak{
		AvailableWidth:  stats.AvailableWidth,
		AdjustmentRatio: stats.AdjustmentRatio,
		Badness:         stats.Badness,
		Demerits:        stats.Demerits,
		Fitness:         paragraphFitnessString(stats.Fitness),
		Hyphenated:      stats.Hyphenated,
		Emergency:       stats.Emergency,
		SingleWord:      stats.SingleWord,
	}
}

func pdfDebugLineGlyphs(pages []pdfPage) []pdfDebugGlyphLine {
	out := make([]pdfDebugGlyphLine, 0)
	for pageIndex, page := range pages {
		for lineIndex, line := range page.Lines {
			glyphs := pdfDebugGlyphsForLine(line)
			if len(glyphs) == 0 {
				continue
			}
			out = append(out, pdfDebugGlyphLine{
				Page:         pageIndex + 1,
				Line:         lineIndex + 1,
				Text:         pdfPageLineText(line),
				X:            line.X,
				Y:            line.Y,
				FontSize:     line.FontSize,
				FontResource: line.FontName,
				Glyphs:       glyphs,
			})
		}
	}
	return out
}

func pdfDebugGlyphsForLine(line pdfPageLine) []pdfDebugGlyph {
	if len(line.Fragments) == 0 {
		return pdfDebugGlyphs(line.Text.Glyphs, 0, line.X, line.Y, line.FontSize, line.LetterSpacing+line.ExtraCharSpacing, line.ExtraWordSpacing)
	}

	glyphs := make([]pdfDebugGlyph, 0)
	currentX := line.X
	for fragmentIndex, fragment := range line.Fragments {
		fragmentGlyphs := pdfDebugGlyphs(
			fragment.Text.Glyphs,
			fragmentIndex+1,
			currentX,
			line.Y+fragment.BaselineShift,
			fragment.FontSize,
			fragment.LetterSpacing+line.ExtraCharSpacing,
			line.ExtraWordSpacing,
		)
		glyphs = append(glyphs, fragmentGlyphs...)
		currentX += pdfPageFragmentAdvance(line, fragment, fragmentIndex != len(line.Fragments)-1)
	}
	return glyphs
}

func pdfDebugGlyphs(
	glyphs []shapedGlyph,
	fragment int,
	x float64,
	y float64,
	fontSize float64,
	letterSpacing float64,
	extraWordSpacing float64,
) []pdfDebugGlyph {
	out := make([]pdfDebugGlyph, 0, len(glyphs))
	currentX := x
	for i, glyph := range glyphs {
		glyphXOffset := glyphOffsetPoints(glyph.XOffset, fontSize)
		glyphYOffset := glyphOffsetPoints(glyph.YOffset, fontSize)
		entry := pdfDebugGlyph{
			Index:    i,
			Fragment: fragment,
			PDFCID:   glyph.GlyphID,
			Source:   glyphUnicodeText(glyph),
			Rune:     pdfDebugRune(glyph.Rune),
			X:        currentX + glyphXOffset,
			Y:        y + glyphYOffset,
			Advance:  glyphAdvancePoints(glyph, fontSize),
			XOffset:  glyphXOffset,
			YOffset:  glyphYOffset,
		}
		if glyph.Missing != pdfMissingGlyphNone {
			entry.MissingGlyph = glyph.Missing.String()
		}
		out = append(out, entry)
		currentX += glyphAdvancePoints(glyph, fontSize)
		if i != len(glyphs)-1 {
			currentX += letterSpacing
		}
		if glyph.Rune == ' ' && i != len(glyphs)-1 {
			currentX += extraWordSpacing
		}
	}
	return out
}

func pdfDebugRune(r rune) string {
	if r == 0 {
		return ""
	}
	return fmt.Sprintf("U+%04X", r)
}

func pdfDebugJustificationLines(pages []pdfPage) []pdfDebugJustificationLine {
	out := make([]pdfDebugJustificationLine, 0)
	for pageIndex, page := range pages {
		for lineIndex, line := range page.Lines {
			debugLine, ok := pdfDebugJustificationLineFor(pageIndex+1, lineIndex+1, line)
			if ok {
				out = append(out, debugLine)
			}
		}
	}
	return out
}

func pdfDebugJustificationLineFor(pageNumber int, lineNumber int, line pdfPageLine) (pdfDebugJustificationLine, bool) {
	justified := pdfPageLineIsJustified(line)
	if !justified && !line.BreakStats.Emergency && !line.BreakStats.Hyphenated && pdfPageLineOverflow(line) == 0 &&
		pdfPageLineVisualOverflow(line) == 0 {
		return pdfDebugJustificationLine{}, false
	}
	naturalWidth := pdfPageLineAdvanceWidth(line)
	drawnWidth := pdfPageLineDrawnWidth(line)
	available := pdfPageLineAvailableWidth(line)
	glyphCount := pdfDebugLineGlyphCount(line)
	gaps := pdfPageLineJustificationSpaceCount(line)
	debugLine := pdfDebugJustificationLine{
		Page:                       pageNumber,
		Line:                       lineNumber,
		Text:                       pdfPageLineText(line),
		Decision:                   pdfDebugJustificationDecision(line, naturalWidth, available, glyphCount, gaps),
		NaturalWidth:               naturalWidth,
		DrawnWidth:                 drawnWidth,
		AvailableWidth:             available,
		Residual:                   available - naturalWidth,
		JustificationGaps:          gaps,
		GlyphCount:                 glyphCount,
		ExtraWordSpacing:           line.ExtraWordSpacing,
		ExtraCharSpacing:           line.ExtraCharSpacing,
		BreakCandidateSummary:      "selected_break_recorded; rejected_candidates_not_retained",
		RejectedCandidatesRecorded: false,
		LineBreak:                  pdfDebugLineBreakStats(line.BreakStats),
	}
	pdfDebugPopulateJustificationCaps(&debugLine, line.FontSize)
	return debugLine, true
}

func pdfDebugLineGlyphCount(line pdfPageLine) int {
	if len(line.Fragments) == 0 {
		return len(line.Text.Glyphs)
	}
	count := 0
	for _, fragment := range line.Fragments {
		count += len(fragment.Text.Glyphs)
	}
	return count
}

func pdfDebugJustificationDecision(line pdfPageLine, naturalWidth float64, available float64, glyphCount int, gaps int) string {
	if !pdfPageLineIsJustified(line) {
		switch {
		case line.BreakStats.Emergency:
			return "emergency_break_unjustified"
		case line.BreakStats.Hyphenated:
			return "hyphenated_break_unjustified"
		case pdfPageLineOverflow(line) > 0 || pdfPageLineVisualOverflow(line) > 0:
			return "overflow_unjustified"
		default:
			return "not_justified"
		}
	}
	if naturalWidth > available+pdfLineWidthTolerance {
		return pdfDebugJustificationShrinkDecision(line, naturalWidth-available, glyphCount, gaps)
	}
	return pdfDebugJustificationStretchDecision(line, available-naturalWidth, glyphCount, gaps)
}

func pdfDebugJustificationStretchDecision(line pdfPageLine, residual float64, glyphCount int, gaps int) string {
	if gaps <= 0 || residual <= pdfLineWidthTolerance {
		return "justified_no_adjustment"
	}
	wordCap, charCap := paragraphJustificationStretchCaps(line.FontSize)
	wordCapped := residual/float64(gaps) > wordCap
	remaining := residual - min(residual/float64(gaps), wordCap)*float64(gaps)
	if remaining <= pdfLineWidthTolerance || glyphCount < 2 {
		if wordCapped {
			return "stretch_word_spacing_capped"
		}
		return "stretch_word_spacing"
	}
	charCapped := remaining/float64(glyphCount-1) > charCap
	if wordCapped && charCapped {
		return "stretch_word_and_char_spacing_capped"
	}
	if wordCapped {
		return "stretch_word_spacing_capped_with_tracking"
	}
	if charCapped {
		return "stretch_char_spacing_capped"
	}
	return "stretch_word_and_char_spacing"
}

func pdfDebugJustificationShrinkDecision(line pdfPageLine, overflow float64, glyphCount int, gaps int) string {
	if gaps <= 0 || overflow <= pdfLineWidthTolerance {
		return "justified_no_adjustment"
	}
	wordCap, charCap := paragraphJustificationShrinkCaps(line.FontSize)
	wordCapped := overflow/float64(gaps) > wordCap
	remaining := overflow - min(overflow/float64(gaps), wordCap)*float64(gaps)
	if remaining <= pdfLineWidthTolerance || glyphCount < 2 {
		if wordCapped {
			return "shrink_word_spacing_capped"
		}
		return "shrink_word_spacing"
	}
	charCapped := remaining/float64(glyphCount-1) > charCap
	if wordCapped && charCapped {
		return "shrink_word_and_char_spacing_capped"
	}
	if wordCapped {
		return "shrink_word_spacing_capped_with_tracking"
	}
	if charCapped {
		return "shrink_char_spacing_capped"
	}
	return "shrink_word_and_char_spacing"
}

func pdfDebugPopulateJustificationCaps(line *pdfDebugJustificationLine, fontSize float64) {
	if line == nil || line.JustificationGaps <= 0 {
		return
	}
	if line.Residual >= 0 {
		wordCap, charCap := paragraphJustificationStretchCaps(fontSize)
		line.WordSpacingCap = wordCap
		wordSpacing := min(line.Residual/float64(line.JustificationGaps), line.WordSpacingCap)
		line.WordSpacingCapped = line.Residual/float64(line.JustificationGaps) > line.WordSpacingCap
		line.ResidualAfterWordSpacing = line.Residual - wordSpacing*float64(line.JustificationGaps)
		if line.ResidualAfterWordSpacing > pdfLineWidthTolerance && line.GlyphCount > 1 {
			line.CharSpacingCap = charCap
			charSpacing := min(line.ResidualAfterWordSpacing/float64(line.GlyphCount-1), line.CharSpacingCap)
			line.CharSpacingCapped = line.ResidualAfterWordSpacing/float64(line.GlyphCount-1) > line.CharSpacingCap
			line.ResidualAfterCharSpacing = line.ResidualAfterWordSpacing - charSpacing*float64(line.GlyphCount-1)
		}
		return
	}
	overflow := -line.Residual
	wordCap, charCap := paragraphJustificationShrinkCaps(fontSize)
	line.WordSpacingCap = wordCap
	wordShrink := min(overflow/float64(line.JustificationGaps), line.WordSpacingCap)
	line.WordSpacingCapped = overflow/float64(line.JustificationGaps) > line.WordSpacingCap
	line.ResidualAfterWordSpacing = -(overflow - wordShrink*float64(line.JustificationGaps))
	if -line.ResidualAfterWordSpacing > pdfLineWidthTolerance && line.GlyphCount > 1 {
		line.CharSpacingCap = charCap
		charShrink := min((-line.ResidualAfterWordSpacing)/float64(line.GlyphCount-1), line.CharSpacingCap)
		line.CharSpacingCapped = (-line.ResidualAfterWordSpacing)/float64(line.GlyphCount-1) > line.CharSpacingCap
		line.ResidualAfterCharSpacing = line.ResidualAfterWordSpacing + charShrink*float64(line.GlyphCount-1)
	}
}

func pdfDebugPages(pages []pdfPage) ([]pdfDebugPage, []pdfDebugImage, []pdfDebugLink) {
	debugPages := make([]pdfDebugPage, 0, len(pages))
	debugImages := make([]pdfDebugImage, 0)
	debugLinks := make([]pdfDebugLink, 0)
	for i, page := range pages {
		debugPage := pdfDebugPage{
			Number:      i + 1,
			ObjectID:    page.ObjectID,
			ContentID:   page.ContentID,
			Anchors:     slices.Clone(page.Anchors),
			Lines:       make([]pdfDebugLine, 0, len(page.Lines)),
			Images:      make([]pdfDebugImage, 0, len(page.Images)),
			Backgrounds: make([]pdfDebugBackground, 0, len(page.Backgrounds)),
			Borders:     make([]pdfDebugBorder, 0, len(page.Borders)),
			Strokes:     make([]pdfDebugStroke, 0, len(page.Strokes)),
			Links:       make([]pdfDebugLink, 0, len(page.Annotations)),
		}
		for _, line := range page.Lines {
			advanceWidth := pdfPageLineAdvanceWidth(line)
			drawnWidth := pdfPageLineDrawnWidth(line)
			visualLeft, visualRight, visualOK := pdfPageLineVisualBounds(line)
			if !visualOK {
				visualLeft, visualRight = 0, 0
			}
			debugPage.Lines = append(debugPage.Lines, pdfDebugLine{
				Text:             pdfPageLineText(line),
				X:                line.X,
				Y:                line.Y,
				FontSize:         line.FontSize,
				LetterSpacing:    line.LetterSpacing,
				FontResource:     line.FontName,
				FontFamily:       line.FontKey.Family,
				FontWeight:       pdfCSSFontWeightString(line.FontKey.Bold),
				FontStyle:        pdfCSSFontStyleString(line.FontKey.Italic),
				Color:            line.Color.String(),
				Underline:        line.Underline,
				Strikethrough:    line.Strikethrough,
				Width:            drawnWidth,
				AdvanceWidth:     advanceWidth,
				DrawnWidth:       drawnWidth,
				AvailableWidth:   pdfPageLineAvailableWidth(line),
				RightEdge:        line.X + drawnWidth,
				VisualLeft:       visualLeft,
				VisualRight:      visualRight,
				Overflow:         pdfPageLineOverflow(line),
				VisualOverflow:   pdfPageLineVisualOverflow(line),
				Justified:        pdfPageLineIsJustified(line),
				ExtraWordSpacing: line.ExtraWordSpacing,
				ExtraCharSpacing: line.ExtraCharSpacing,
				LineBreak:        pdfDebugLineBreakStats(line.BreakStats),
			})
		}
		for _, background := range page.Backgrounds {
			debugPage.Backgrounds = append(debugPage.Backgrounds, pdfDebugBackground{
				X:      background.X,
				Y:      background.Y,
				Width:  background.Width,
				Height: background.Height,
				Color:  background.Color.String(),
			})
		}
		for _, border := range page.Borders {
			debugPage.Borders = append(debugPage.Borders, pdfDebugBorder{
				X:         border.X,
				Y:         border.Y,
				Width:     border.Width,
				Height:    border.Height,
				LineWidth: border.LineWidth,
				Color:     border.Color.String(),
			})
		}
		for _, stroke := range page.Strokes {
			debugPage.Strokes = append(debugPage.Strokes, pdfDebugStroke{
				X1:        stroke.X1,
				Y1:        stroke.Y1,
				X2:        stroke.X2,
				Y2:        stroke.Y2,
				LineWidth: stroke.LineWidth,
				Color:     stroke.Color.String(),
			})
		}
		for _, image := range page.Images {
			debugImage := pdfDebugImage{
				Page:         i + 1,
				ImageID:      image.ImageID,
				ResourceName: image.Name,
				X:            image.X,
				Y:            image.Y,
				Width:        image.Width,
				Height:       image.Height,
			}
			debugPage.Images = append(debugPage.Images, debugImage)
			debugImages = append(debugImages, debugImage)
		}
		for _, link := range page.Annotations {
			debugLink := pdfDebugLink{
				Page:     i + 1,
				ObjectID: link.ObjectID,
				Href:     link.Href,
				Internal: strings.HasPrefix(link.Href, "#"),
				Rect: pdfDebugRect{
					X1: link.Rect.X1,
					Y1: link.Rect.Y1,
					X2: link.Rect.X2,
					Y2: link.Rect.Y2,
				},
			}
			debugPage.Links = append(debugPage.Links, debugLink)
			debugLinks = append(debugLinks, debugLink)
		}
		debugPages = append(debugPages, debugPage)
	}
	return debugPages, debugImages, debugLinks
}
