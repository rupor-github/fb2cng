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
	contentLeft, contentRight, _, contentBottom := pdfPageContentMargins(doc, styles, margin)
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
		for i := 1; i < len(plan.QueuePages); i++ {
			continuation := pdfPage{}
			appendPDFPrintedFootnotePage(&continuation, plan.QueuePages[i], separator)
			out = append(out, continuation)
		}
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
