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
				face, err := fontForKey(fonts, fragment.FontKey)
				if err != nil {
					return fmt.Errorf("shape page-local footnote label %q: %w", label, err)
				}
				shaped, err := shapeText(face, label)
				if err != nil {
					return fmt.Errorf("shape page-local footnote label %q: %w", label, err)
				}
				fragment.Text = shaped
				fragment.Width = shapedWidthPointsWithSpacing(shaped, fragment.FontSize, fragment.LetterSpacing)
				collectPDFTextUsedGlyphs(used, shaped, fragment.FontKey)
				changed = true
			}
			if changed {
				line.Text = shapedTextFromPageLineFragments(line.Fragments)
			}
		}
	}
	return nil
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
