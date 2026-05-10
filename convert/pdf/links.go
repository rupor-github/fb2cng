package pdf

import (
	"slices"
	"strings"

	"fbc/convert/pdf/docwriter"
)

func addLinkAnnotations(page *pdfPage, block pdfTextBlock, line paragraphLine, searchStart int, x float64, y float64, fontSize float64) {
	if len(block.Links) == 0 || len(line.Text.Glyphs) == 0 {
		return
	}
	lineText := shapedRunes(line.Text)
	lineStart, lineEnd, ok := lineRuneRange(block.Text, lineText, searchStart)
	if !ok {
		return
	}
	for _, link := range block.Links {
		start := max(link.Start, lineStart)
		end := min(link.End, lineEnd)
		if start >= end || strings.TrimSpace(link.Href) == "" {
			continue
		}
		x1 := x + glyphAdvanceRange(line.Text.Glyphs, 0, start-lineStart, fontSize)
		x2 := x + glyphAdvanceRange(line.Text.Glyphs, 0, end-lineStart, fontSize)
		if x2 <= x1 {
			continue
		}
		page.Annotations = append(page.Annotations, pdfLinkAnnotation{
			Rect: pdfRect{
				X1: x1,
				Y1: y - fontSize*0.2,
				X2: x2,
				Y2: y + fontSize,
			},
			Href: link.Href,
		})
	}
}

func nextLineSearchStart(text string, line paragraphLine, searchStart int) int {
	lineText := shapedRunes(line.Text)
	_, end, ok := lineRuneRange(text, lineText, searchStart)
	if !ok {
		return searchStart
	}
	return end
}

func lineRuneRange(text string, lineText string, searchStart int) (int, int, bool) {
	lineText = strings.TrimSuffix(lineText, "-")
	lineText = strings.TrimSpace(lineText)
	if lineText == "" {
		return searchStart, searchStart, false
	}
	runes := []rune(text)
	lineRunes := []rune(lineText)
	for start := max(searchStart, 0); start+len(lineRunes) <= len(runes); start++ {
		if string(runes[start:start+len(lineRunes)]) == lineText {
			return start, start + len(lineRunes), true
		}
	}
	return searchStart, searchStart, false
}

func glyphAdvanceRange(glyphs []shapedGlyph, start int, end int, fontSize float64) float64 {
	start = max(start, 0)
	end = min(max(end, start), len(glyphs))
	width := 0
	for _, glyph := range glyphs[start:end] {
		width += glyph.Width
	}
	return float64(width) * fontSize / 1000.0
}

func assignAnnotationObjectIDs(pages []pdfPage, nextObjectID *int) {
	for i := range pages {
		for j := range pages[i].Annotations {
			pages[i].Annotations[j].ObjectID = *nextObjectID
			(*nextObjectID)++
		}
	}
}

func writeAnnotationObjects(writer *docwriter.Writer, pages []pdfPage) error {
	for _, page := range pages {
		for _, annot := range page.Annotations {
			dict := docwriter.Dict{
				"Border":  docwriter.Array{docwriter.Integer(0), docwriter.Integer(0), docwriter.Integer(0)},
				"Rect":    rectArray(annot.Rect),
				"Subtype": docwriter.Name("Link"),
				"Type":    docwriter.Name("Annot"),
			}
			if target, ok := strings.CutPrefix(annot.Href, "#"); ok && target != "" {
				dict["Dest"] = docwriter.HexString([]byte(target))
			} else {
				dict["A"] = docwriter.Dict{
					"S":   docwriter.Name("URI"),
					"URI": docwriter.HexString([]byte(annot.Href)),
				}
			}
			if err := writer.Object(annot.ObjectID, dict); err != nil {
				return err
			}
		}
	}
	return nil
}

func rectArray(rect pdfRect) docwriter.Array {
	return docwriter.Array{
		docwriter.Number(rect.X1),
		docwriter.Number(rect.Y1),
		docwriter.Number(rect.X2),
		docwriter.Number(rect.Y2),
	}
}

func namedDestinations(pages []pdfPage) docwriter.Dict {
	anchors := make(map[string]int)
	for i := range pages {
		for _, id := range pages[i].Anchors {
			if _, exists := anchors[id]; !exists {
				anchors[id] = pages[i].ObjectID
			}
		}
	}
	if len(anchors) == 0 {
		return nil
	}

	ids := make([]string, 0, len(anchors))
	for id := range anchors {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	items := make(docwriter.Array, 0, len(ids)*2)
	for _, id := range ids {
		items = append(items,
			docwriter.HexString([]byte(id)),
			docwriter.Array{docwriter.Ref{ObjectNumber: anchors[id]}, docwriter.Name("Fit")},
		)
	}
	return docwriter.Dict{
		"Dests": docwriter.Dict{
			"Names": items,
		},
	}
}
