package pdf

import (
	"fmt"
	"strconv"
	"strings"
)

func applyPDFPageLocalFootnoteReferenceLabels(
	pages []pdfPage,
	fonts *pdfFontRegistry,
	used map[pdfFontKey]map[uint16]shapedGlyph,
	resolver *pdfStyleResolver,
	shapers *pdfTextShaperCache,
) error {
	for pageIndex := range pages {
		labels := make(map[string]string)
		nextLabel := 1
		for lineIndex := range pages[pageIndex].Lines {
			line := &pages[pageIndex].Lines[lineIndex]
			changed := false
			for fragmentIndex := range line.Fragments {
				fragment := &line.Fragments[fragmentIndex]
				if strings.TrimSpace(fragment.FootnoteID) == "" || strings.TrimSpace(fragment.LinkHref) != "" {
					continue
				}
				label, ok := labels[fragment.FootnoteID]
				if !ok {
					label = strconv.Itoa(nextLabel)
					nextLabel++
					labels[fragment.FootnoteID] = label
				}
				if strings.TrimSpace(fragment.ImageID) != "" {
					continue
				}
				labelText := pdfDecoratedFootnoteReferenceLabel(resolver, fragment.StyleClasses, label)
				face, err := resolvePDFFontFace(fonts, fragment.FontKey)
				if err != nil {
					return fmt.Errorf("shape page-local footnote label %q: %w", labelText, err)
				}
				shaped, err := shapeTextWithCache(shapers, face, labelText)
				if err != nil {
					return fmt.Errorf("shape page-local footnote label %q: %w", labelText, err)
				}
				fragment.Text = shaped
				fragment.Width = shapedWidthPoints(shaped, fragment.FontSize, fragment.LetterSpacing)
				collectPDFTextUsedGlyphs(used, shaped, fragment.FontKey)
				changed = true
			}
			if changed {
				line.Text = shapedTextFromPageLineFragments(line.Fragments)
				rejustifyPDFPageLineAfterFragmentRelabel(line)
			}
		}
	}
	return nil
}

func rejustifyPDFPageLineAfterFragmentRelabel(line *pdfPageLine) {
	if line == nil || !pdfPageLineIsJustified(*line) {
		return
	}
	available := pdfPageLineAvailableWidth(*line)
	if available <= 0 {
		return
	}
	width := pdfPageLineAdvanceWidth(*line)
	gaps := pdfPageLineJustificationSpaceCount(*line)
	if gaps <= 0 {
		line.ExtraWordSpacing = 0
		line.ExtraCharSpacing = 0
		return
	}
	style := paragraphStyle{FontSize: line.FontSize, Align: textAlignJustify}
	line.ExtraWordSpacing, line.ExtraCharSpacing = paragraphJustificationSpacing(style, false, width, available, gaps, len(line.Text.Glyphs))
}

func shapedTextFromPageLineFragments(fragments []pdfPageLineFragment) shapedText {
	shaped := shapedText{Used: make(map[uint16]shapedGlyph)}
	for _, fragment := range fragments {
		shaped.Glyphs = append(shaped.Glyphs, fragment.Text.Glyphs...)
		for glyphID, glyph := range fragment.Text.Used {
			shaped.Used[glyphID] = glyph
		}
	}
	return shaped
}
