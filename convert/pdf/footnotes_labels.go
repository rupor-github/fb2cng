package pdf

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
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
				visibleLabel := pdfPageLocalFootnoteReferenceText(shapedRunes(fragment.Text), label)
				face, err := fontForKey(fonts, fragment.FontKey)
				if err != nil {
					return fmt.Errorf("shape page-local footnote label %q: %w", label, err)
				}
				shaped, err := shapeText(face, visibleLabel)
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

func pdfPageLocalFootnoteReferenceText(current string, label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return current
	}
	current = strings.TrimSpace(current)
	if current == "" {
		return label
	}
	runes := []rune(current)
	firstCore := 0
	for firstCore < len(runes) && !unicode.IsLetter(runes[firstCore]) && !unicode.IsDigit(runes[firstCore]) {
		firstCore++
	}
	lastCore := len(runes) - 1
	for lastCore >= firstCore && !unicode.IsLetter(runes[lastCore]) && !unicode.IsDigit(runes[lastCore]) {
		lastCore--
	}
	if firstCore > lastCore {
		return label
	}
	return string(runes[:firstCore]) + label + string(runes[lastCore+1:])
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
