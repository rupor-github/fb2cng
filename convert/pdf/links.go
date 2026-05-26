package pdf

import (
	"slices"
	"strings"

	"fbc/convert/pdf/docwriter"
)

func addLinkAnnotations(page *pdfPage, block pdfTextBlock, line paragraphLine, searchStart int, x float64, y float64, fontSize float64) {
	if addFragmentLinkAnnotations(page, line, x, y) {
		return
	}
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
		x1 := x + glyphSourceOffsetX(line.Text.Glyphs, start-lineStart, fontSize, false)
		x2 := x + glyphSourceOffsetX(line.Text.Glyphs, end-lineStart, fontSize, true)
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

func addFragmentLinkAnnotations(page *pdfPage, line paragraphLine, x float64, y float64) bool {
	currentX := x
	added := false
	for i, fragment := range line.Fragments {
		href := strings.TrimSpace(fragment.LinkHref)
		if href != "" && fragment.Width > 0 && len(fragment.Text.Glyphs) > 0 {
			page.Annotations = append(page.Annotations, pdfLinkAnnotation{
				Rect: pdfRect{
					X1: currentX,
					Y1: y + fragment.BaselineShift - fragment.FontSize*0.2,
					X2: currentX + fragment.Width,
					Y2: y + fragment.BaselineShift + fragment.FontSize,
				},
				Href: href,
			})
			added = true
		}
		if href != "" && fragment.ImageID != "" && fragment.Width > 0 && fragment.ImageHeight > 0 {
			page.Annotations = append(page.Annotations, pdfLinkAnnotation{
				Rect: pdfRect{
					X1: currentX,
					Y1: y + fragment.BaselineShift,
					X2: currentX + fragment.Width,
					Y2: y + fragment.BaselineShift + fragment.ImageHeight,
				},
				Href: href,
			})
			added = true
		}
		currentX += paragraphFragmentAdvance(line, fragment, i != len(line.Fragments)-1)
	}
	return added
}

func paragraphFragmentEndsWithSpace(fragment paragraphLineFragment) bool {
	glyphs := fragment.Text.Glyphs
	return len(glyphs) != 0 && glyphs[len(glyphs)-1].Rune == ' '
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

func glyphSourceOffsetX(glyphs []shapedGlyph, sourceOffset int, fontSize float64, trailing bool) float64 {
	if sourceOffset <= 0 {
		return 0
	}
	x := 0.0
	for _, glyph := range glyphs {
		advance := glyphAdvancePoints(glyph, fontSize)
		if glyph.ClusterEnd <= glyph.ClusterStart {
			if sourceOffset <= 0 {
				return x
			}
			x += advance
			sourceOffset--
			continue
		}
		if sourceOffset <= glyph.ClusterStart {
			return x
		}
		if sourceOffset < glyph.ClusterEnd {
			if trailing {
				return x + advance
			}
			return x
		}
		x += advance
	}
	return x
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
