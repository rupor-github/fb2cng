package pdf

import "strings"

func addPDFPageLine(page *pdfPage, used map[pdfFontKey]map[uint16]shapedGlyph, line pdfPageLine) {
	if line.FontKey.Family == "" {
		line.FontKey = pdfFontKey{Family: "serif"}
	}
	line = pdfPageLineWithFontFragments(line)
	page.Lines = append(page.Lines, line)
	collectPDFLineUsedGlyphs(used, line)
}

func addPDFPageAnchor(page *pdfPage, id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	for _, existing := range page.Anchors {
		if existing == id {
			return
		}
	}
	page.Anchors = append(page.Anchors, id)
}

func addPDFParagraphFragmentAnchors(page *pdfPage, line paragraphLine) {
	for _, fragment := range line.Fragments {
		addPDFPageAnchor(page, fragment.AnchorID)
	}
}

func addPDFInlineImages(page *pdfPage, line paragraphLine, x float64, y float64) {
	currentX := x
	for i, fragment := range line.Fragments {
		if fragment.ImageID != "" && fragment.Width > 0 && fragment.ImageHeight > 0 {
			page.Images = append(page.Images, pdfPageImage{
				ImageID: fragment.ImageID,
				X:       currentX,
				Y:       y + fragment.BaselineShift,
				Width:   fragment.Width,
				Height:  fragment.ImageHeight,
			})
		}
		currentX += paragraphFragmentAdvance(line, fragment, i != len(line.Fragments)-1)
	}
}

func addPDFBlockDecoration(page *pdfPage, style pdfBlockResolvedStyle, x, topY, width, bottomY float64) {
	if page == nil || width <= 0 || topY <= bottomY {
		return
	}
	height := topY - bottomY
	if style.HasBackground {
		page.Backgrounds = append(page.Backgrounds, pdfPageRect{
			X:      x,
			Y:      bottomY,
			Width:  width,
			Height: height,
			Color:  style.BackgroundColor,
		})
	}
	if style.HasBorder && style.BorderWidth > 0 {
		page.Borders = append(page.Borders, pdfPageBorder{
			X:         x,
			Y:         bottomY,
			Width:     width,
			Height:    height,
			LineWidth: style.BorderWidth,
			Color:     style.BorderColor,
		})
	}
}
