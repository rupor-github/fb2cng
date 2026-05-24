package pdf

type pdfPrintedFootnoteSeparatorMetrics struct {
	Reserve   float64
	LineWidth float64
	X         float64
	Width     float64
	Y         float64
	Color     pdfColor
}

func appendPDFPrintedFootnotePagePlans(
	doc pdfDocumentSpec,
	pages []pdfPage,
	plans []pdfPrintedFootnotePagePlan,
	footnoteTextHeight float64,
	used map[pdfFontKey]map[uint16]shapedGlyph,
) []pdfPage {
	if len(pages) == 0 || len(plans) == 0 {
		return pages
	}
	plansByPage := make(map[int]pdfPrintedFootnotePagePlan, len(plans))
	for _, plan := range plans {
		if plan.PageIndex >= 0 && plan.PageIndex < len(pages) && len(plan.QueuePages) > 0 {
			plansByPage[plan.PageIndex] = plan
		}
	}
	if len(plansByPage) == 0 {
		return pages
	}

	const margin = 24.0
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	contentLeft, contentRight, contentTop, contentBottom := pdfPageContentMargins(doc, styles, margin)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	separator := pdfPrintedFootnoteSeparatorMetricsForArea(doc, styles, contentLeft, contentWidth, contentBottom, footnoteTextHeight)

	out := make([]pdfPage, 0, len(pages)+len(plans))
	for pageIndex, page := range pages {
		plan, ok := plansByPage[pageIndex]
		if !ok {
			out = append(out, page)
			continue
		}
		appendPDFPrintedFootnotePage(&page, plan.QueuePages[0], separator)
		mergePDFUsedGlyphs(used, plan.UsedGlyphs)
		out = append(out, page)
		out = appendPDFPrintedFootnoteContinuationPages(out, doc, plan.QueuePages[1:], contentTop, contentBottom)
	}
	return out
}

func appendPDFPrintedFootnotePage(page *pdfPage, footnotePage pdfPage, separator pdfPrintedFootnoteSeparatorMetrics) {
	if page == nil {
		return
	}
	if separator.LineWidth > 0 && separator.Width > 0 {
		page.Backgrounds = append(page.Backgrounds, pdfPageRect{
			X:      separator.X,
			Y:      separator.Y,
			Width:  separator.Width,
			Height: separator.LineWidth,
			Color:  separator.Color,
		})
	}
	page.Backgrounds = append(page.Backgrounds, footnotePage.Backgrounds...)
	page.Borders = append(page.Borders, footnotePage.Borders...)
	page.Lines = append(page.Lines, footnotePage.Lines...)
	page.Images = append(page.Images, footnotePage.Images...)
	page.Anchors = append(page.Anchors, footnotePage.Anchors...)
	page.Annotations = append(page.Annotations, footnotePage.Annotations...)
}

func appendPDFPrintedFootnoteContinuationPages(out []pdfPage, doc pdfDocumentSpec, queuePages []pdfPage, contentTop float64, contentBottom float64) []pdfPage {
	if len(queuePages) == 0 {
		return out
	}
	pageTop := doc.PageHeight - contentTop
	pageBottom := contentBottom
	var continuation pdfPage
	cursor := pageTop
	hasContent := false
	for _, queuePage := range queuePages {
		chunkTop, chunkBottom, ok := pdfPageYBounds(queuePage)
		if !ok {
			continue
		}
		chunkHeight := chunkTop - chunkBottom
		if hasContent && cursor-chunkHeight < pageBottom {
			out = append(out, continuation)
			continuation = pdfPage{}
			cursor = pageTop
			hasContent = false
		}
		shift := cursor - chunkTop
		appendPDFPrintedFootnotePage(&continuation, shiftPDFPageY(queuePage, shift), pdfPrintedFootnoteSeparatorMetrics{})
		cursor = chunkBottom + shift
		hasContent = true
	}
	if hasContent {
		out = append(out, continuation)
	}
	return out
}

func shiftPDFPageY(page pdfPage, dy float64) pdfPage {
	if dy == 0 {
		return page
	}
	shifted := page
	shifted.Backgrounds = append([]pdfPageRect(nil), page.Backgrounds...)
	for i := range shifted.Backgrounds {
		shifted.Backgrounds[i].Y += dy
	}
	shifted.Borders = append([]pdfPageBorder(nil), page.Borders...)
	for i := range shifted.Borders {
		shifted.Borders[i].Y += dy
	}
	shifted.Lines = append([]pdfPageLine(nil), page.Lines...)
	for i := range shifted.Lines {
		shifted.Lines[i].Y += dy
	}
	shifted.Images = append([]pdfPageImage(nil), page.Images...)
	for i := range shifted.Images {
		shifted.Images[i].Y += dy
	}
	shifted.Annotations = append([]pdfLinkAnnotation(nil), page.Annotations...)
	for i := range shifted.Annotations {
		shifted.Annotations[i].Rect.Y1 += dy
		shifted.Annotations[i].Rect.Y2 += dy
	}
	return shifted
}

func pdfPageYBounds(page pdfPage) (float64, float64, bool) {
	top := 0.0
	bottom := 0.0
	ok := false
	include := func(itemTop float64, itemBottom float64) {
		if itemTop < itemBottom {
			itemTop, itemBottom = itemBottom, itemTop
		}
		if !ok || itemTop > top {
			top = itemTop
		}
		if !ok || itemBottom < bottom {
			bottom = itemBottom
		}
		ok = true
	}
	for _, rect := range page.Backgrounds {
		include(rect.Y+rect.Height, rect.Y)
	}
	for _, border := range page.Borders {
		include(border.Y+border.Height, border.Y)
	}
	for _, line := range page.Lines {
		include(line.Y+line.FontSize, line.Y-line.FontSize*0.2)
	}
	for _, image := range page.Images {
		include(image.Y+image.Height, image.Y)
	}
	for _, annotation := range page.Annotations {
		include(annotation.Rect.Y2, annotation.Rect.Y1)
	}
	return top, bottom, ok
}

func pdfPrintedFootnoteSeparatorMetricsForArea(
	doc pdfDocumentSpec,
	styles *pdfStyleResolver,
	contentLeft float64,
	contentWidth float64,
	contentBottom float64,
	footnoteTextHeight float64,
) pdfPrintedFootnoteSeparatorMetrics {
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	style := styles.styleForBlock(pdfTextBlock{Kind: pdfBlockParagraph, StyleClasses: pdfStyleFootnoteSeparator, ContextClasses: pdfStyleFootnoteSeparator})
	lineWidth := style.BorderWidth
	if lineWidth <= 0 {
		lineWidth = 0.5
	}
	width := blockBoxWidth(contentWidth, style)
	return pdfPrintedFootnoteSeparatorMetrics{
		Reserve:   max(style.SpaceBefore, 0) + lineWidth + max(style.SpaceAfter, 0),
		LineWidth: lineWidth,
		X:         contentLeft + style.MarginLeft,
		Width:     min(max(width, 1), contentWidth),
		Y:         contentBottom + max(footnoteTextHeight, 0) + max(style.SpaceAfter, 0),
		Color:     style.BorderColor,
	}
}

func mergePDFUsedGlyphs(dst map[pdfFontKey]map[uint16]shapedGlyph, src map[pdfFontKey]map[uint16]shapedGlyph) {
	if dst == nil || len(src) == 0 {
		return
	}
	for key, glyphs := range src {
		if dst[key] == nil {
			dst[key] = make(map[uint16]shapedGlyph, len(glyphs))
		}
		for id, glyph := range glyphs {
			dst[key][id] = glyph
		}
	}
}
