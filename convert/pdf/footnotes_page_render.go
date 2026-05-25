package pdf

const (
	pdfPrintedFootnoteContinuationMarkerChevrons       = 3
	pdfPrintedFootnoteContinuationMarkerMinStrokeWidth = 0.5
	pdfPrintedFootnoteContinuationMarkerHeightFactor   = 5.0
	pdfPrintedFootnoteContinuationMarkerMinHeight      = 3.0
	pdfPrintedFootnoteContinuationMarkerWidthFactor    = 0.55
	pdfPrintedFootnoteContinuationMarkerGapFactor      = 2.5
	pdfPrintedFootnoteContinuationMarkerMinGap         = 1.5
	pdfPrintedFootnoteContinuationMarkerInsetFactor    = 6.0
	pdfPrintedFootnoteContinuationMarkerMinInset       = 4.0
)

type pdfPrintedFootnoteSeparatorMetrics struct {
	Reserve     float64
	LineWidth   float64
	SpaceBefore float64
	SpaceAfter  float64
	X           float64
	Width       float64
	Y           float64
	Color       pdfColor
}

func pdfDebugPrintedFootnoteSkipped(debug *pdfDebugPrintedFootnotes, kind string, reason string, pageIndex int, queuePageIndex int, value float64, limit float64) {
	if debug == nil {
		return
	}
	debug.Skipped = append(debug.Skipped, pdfDebugPrintedFootnoteCase{
		Kind:           kind,
		Reason:         reason,
		PageIndex:      pageIndex,
		QueuePageIndex: queuePageIndex,
		Value:          value,
		Limit:          limit,
	})
}

func pdfDebugPrintedFootnoteOverflow(debug *pdfDebugPrintedFootnotes, kind string, reason string, pageIndex int, queuePageIndex int, value float64, limit float64) {
	if debug == nil {
		return
	}
	debug.Overflow = append(debug.Overflow, pdfDebugPrintedFootnoteCase{
		Kind:           kind,
		Reason:         reason,
		PageIndex:      pageIndex,
		QueuePageIndex: queuePageIndex,
		Value:          value,
		Limit:          limit,
	})
}

func recordFootnoteChunkDebug(
	debug *pdfDebugPrintedFootnotes,
	sourcePageIndex int,
	queuePageIndex int,
	continuationPageIndex int,
	chunkTop float64,
	chunkBottom float64,
	shift float64,
	placedTop float64,
	placedBottom float64,
) {
	if debug == nil {
		return
	}
	debug.PackedContinuationChunks = append(debug.PackedContinuationChunks, pdfDebugPrintedFootnoteContinuationPack{
		SourcePageIndex:       sourcePageIndex,
		QueuePageIndex:        queuePageIndex,
		ContinuationPageIndex: continuationPageIndex,
		ChunkTop:              chunkTop,
		ChunkBottom:           chunkBottom,
		ChunkHeight:           max(chunkTop-chunkBottom, 0),
		ShiftY:                shift,
		PlacedTop:             placedTop,
		PlacedBottom:          placedBottom,
	})
}

func appendPDFPrintedFootnotePagePlans(
	doc pdfDocumentSpec,
	pages []pdfPage,
	plans []pdfPrintedFootnotePagePlan,
	footnoteTextHeight float64,
	used map[pdfFontKey]map[uint16]shapedGlyph,
	debug *pdfDebugPrintedFootnotes,
) []pdfPage {
	if len(pages) == 0 || len(plans) == 0 {
		return pages
	}
	plansByPage := make(map[int]pdfPrintedFootnotePagePlan, len(plans))
	for _, plan := range plans {
		if plan.PageIndex < 0 || plan.PageIndex >= len(pages) {
			pdfDebugPrintedFootnoteSkipped(debug, "plan", "page_index_out_of_range", plan.PageIndex, 0, 0, float64(len(pages)))
			continue
		}
		if len(plan.QueuePages) == 0 {
			pdfDebugPrintedFootnoteSkipped(debug, "plan", "empty_queue_pages", plan.PageIndex, 0, 0, 0)
			continue
		}
		plansByPage[plan.PageIndex] = plan
	}
	if len(plansByPage) == 0 {
		return pages
	}

	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	contentLeft, contentRight, contentTop, contentBottom := pdfPageContentMargins(doc, styles, pdfDefaultPageMargin)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	separator := pdfPrintedFootnoteSeparatorMetricsForArea(doc, styles, contentLeft, contentWidth, contentBottom, footnoteTextHeight)

	out := make([]pdfPage, 0, len(pages)+len(plans))
	for pageIndex, page := range pages {
		plan, ok := plansByPage[pageIndex]
		if !ok {
			out = append(out, page)
			continue
		}
		footnotePage, pageSeparator := bottomAlignPDFPrintedFootnotePage(plan.QueuePages[0], separator, contentBottom)
		appendPDFPrintedFootnotePage(&page, footnotePage, pageSeparator)
		mergePDFUsedGlyphs(used, plan.UsedGlyphs)
		out = append(out, page)
		out = appendPDFPrintedFootnoteContinuationPages(out, doc, plan.PageIndex, plan.QueuePages[1:], contentTop, contentBottom, separator, debug)
		pdfDebugPrintedFootnotesSyncCounts(debug)
	}
	return out
}

func bottomAlignPDFPrintedFootnotePage(
	footnotePage pdfPage,
	separator pdfPrintedFootnoteSeparatorMetrics,
	contentBottom float64,
) (pdfPage, pdfPrintedFootnoteSeparatorMetrics) {
	chunkTop, chunkBottom, ok := pdfPageYBounds(footnotePage)
	if !ok {
		return footnotePage, separator
	}
	shift := contentBottom - chunkBottom
	separator.Y = chunkTop + shift + separator.SpaceAfter
	return shiftPDFPageY(footnotePage, shift), separator
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
	page.Strokes = append(page.Strokes, footnotePage.Strokes...)
	page.Lines = append(page.Lines, footnotePage.Lines...)
	page.Images = append(page.Images, footnotePage.Images...)
	page.Anchors = append(page.Anchors, footnotePage.Anchors...)
	page.Annotations = append(page.Annotations, footnotePage.Annotations...)
}

type pdfPrintedFootnoteContinuationChunk struct {
	Page           pdfPage
	QueuePageIndex int
	Top            float64
	Bottom         float64
	Height         float64
}

func appendPDFPrintedFootnoteContinuationPages(
	out []pdfPage,
	doc pdfDocumentSpec,
	sourcePageIndex int,
	queuePages []pdfPage,
	contentTop float64,
	contentBottom float64,
	separator pdfPrintedFootnoteSeparatorMetrics,
	debug *pdfDebugPrintedFootnotes,
) []pdfPage {
	if len(queuePages) == 0 {
		return out
	}
	pageTop := doc.PageHeight - contentTop
	pageBottom := contentBottom
	textTop := pageTop - max(separator.Reserve, 0)
	if textTop <= pageBottom {
		textTop = pageTop
	}
	pageCapacity := textTop - pageBottom

	var chunks []pdfPrintedFootnoteContinuationChunk
	chunkGroupHeight := 0.0
	flushChunks := func() {
		if len(chunks) == 0 {
			return
		}
		continuationPageIndex := len(out)
		continuation := bottomAlignFootnoteContinuations(
			chunks,
			separator,
			pageBottom,
			sourcePageIndex,
			continuationPageIndex,
			debug,
		)
		out = append(out, continuation)
		chunks = nil
		chunkGroupHeight = 0
	}

	for queuePageIndex, queuePage := range queuePages {
		chunkTop, chunkBottom, ok := pdfPageYBounds(queuePage)
		if !ok {
			pdfDebugPrintedFootnoteSkipped(debug, "continuation_chunk", "empty_bounds", sourcePageIndex, queuePageIndex+1, 0, 0)
			continue
		}
		chunkHeight := chunkTop - chunkBottom
		if chunkHeight > pageCapacity {
			pdfDebugPrintedFootnoteOverflow(debug, "continuation_chunk", "chunk_taller_than_page_capacity", sourcePageIndex, queuePageIndex+1, chunkHeight, pageCapacity)
		}
		if len(chunks) > 0 && chunkGroupHeight+chunkHeight > pageCapacity {
			flushChunks()
		}
		chunks = append(chunks, pdfPrintedFootnoteContinuationChunk{
			Page:           queuePage,
			QueuePageIndex: queuePageIndex + 1,
			Top:            chunkTop,
			Bottom:         chunkBottom,
			Height:         chunkHeight,
		})
		chunkGroupHeight += chunkHeight
		if chunkHeight > pageCapacity {
			flushChunks()
		}
	}
	flushChunks()
	return out
}

func bottomAlignFootnoteContinuations(
	chunks []pdfPrintedFootnoteContinuationChunk,
	separator pdfPrintedFootnoteSeparatorMetrics,
	contentBottom float64,
	sourcePageIndex int,
	continuationPageIndex int,
	debug *pdfDebugPrintedFootnotes,
) pdfPage {
	continuation := pdfPage{}
	if len(chunks) == 0 {
		return continuation
	}
	groupHeight := 0.0
	for _, chunk := range chunks {
		groupHeight += chunk.Height
	}
	cursor := contentBottom + groupHeight
	addFootnoteContinuationSeparator(&continuation, separator, cursor+max(separator.SpaceAfter, 0))
	for _, chunk := range chunks {
		shift := cursor - chunk.Top
		placedTop := chunk.Top + shift
		placedBottom := chunk.Bottom + shift
		recordFootnoteChunkDebug(debug, sourcePageIndex, chunk.QueuePageIndex, continuationPageIndex, chunk.Top, chunk.Bottom, shift, placedTop, placedBottom)
		appendPDFPrintedFootnotePage(&continuation, shiftPDFPageY(chunk.Page, shift), pdfPrintedFootnoteSeparatorMetrics{})
		cursor = placedBottom
	}
	return continuation
}

func addFootnoteContinuationSeparator(page *pdfPage, separator pdfPrintedFootnoteSeparatorMetrics, lineY float64) {
	if page == nil || separator.LineWidth <= 0 || separator.Width <= 0 {
		return
	}
	page.Backgrounds = append(page.Backgrounds, pdfPageRect{
		X:      separator.X,
		Y:      lineY,
		Width:  separator.Width,
		Height: separator.LineWidth,
		Color:  separator.Color,
	})
	appendPDFPrintedFootnoteContinuationMarker(page, separator, lineY)
}

func appendPDFPrintedFootnoteContinuationMarker(page *pdfPage, separator pdfPrintedFootnoteSeparatorMetrics, separatorY float64) {
	if page == nil || separator.LineWidth <= 0 || separator.Width <= 0 {
		return
	}
	strokeWidth := max(separator.LineWidth, pdfPrintedFootnoteContinuationMarkerMinStrokeWidth)
	chevronHeight := max(strokeWidth*pdfPrintedFootnoteContinuationMarkerHeightFactor, pdfPrintedFootnoteContinuationMarkerMinHeight)
	chevronWidth := chevronHeight * pdfPrintedFootnoteContinuationMarkerWidthFactor
	gap := max(strokeWidth*pdfPrintedFootnoteContinuationMarkerGapFactor, pdfPrintedFootnoteContinuationMarkerMinGap)
	markerWidth := float64(pdfPrintedFootnoteContinuationMarkerChevrons)*chevronWidth + float64(pdfPrintedFootnoteContinuationMarkerChevrons-1)*gap
	inset := min(max(separator.Width-markerWidth, 0), max(strokeWidth*pdfPrintedFootnoteContinuationMarkerInsetFactor, pdfPrintedFootnoteContinuationMarkerMinInset))
	startX := separator.X + separator.Width - markerWidth - inset
	centerY := separatorY + separator.LineWidth/2
	for i := range pdfPrintedFootnoteContinuationMarkerChevrons {
		x := startX + float64(i)*(chevronWidth+gap)
		midX := x + chevronWidth
		page.Strokes = append(page.Strokes,
			pdfPageStroke{X1: x, Y1: centerY + chevronHeight/2, X2: midX, Y2: centerY, LineWidth: strokeWidth, Color: separator.Color},
			pdfPageStroke{X1: x, Y1: centerY - chevronHeight/2, X2: midX, Y2: centerY, LineWidth: strokeWidth, Color: separator.Color},
		)
	}
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
	shifted.Strokes = append([]pdfPageStroke(nil), page.Strokes...)
	for i := range shifted.Strokes {
		shifted.Strokes[i].Y1 += dy
		shifted.Strokes[i].Y2 += dy
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
	for _, stroke := range page.Strokes {
		padding := max(stroke.LineWidth/2, 0)
		include(max(stroke.Y1, stroke.Y2)+padding, min(stroke.Y1, stroke.Y2)-padding)
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
	spaceBefore := max(style.SpaceBefore, 0)
	spaceAfter := max(style.SpaceAfter, 0)
	return pdfPrintedFootnoteSeparatorMetrics{
		Reserve:     spaceBefore + lineWidth + spaceAfter,
		LineWidth:   lineWidth,
		SpaceBefore: spaceBefore,
		SpaceAfter:  spaceAfter,
		X:           contentLeft + style.MarginLeft,
		Width:       min(max(width, 1), contentWidth),
		Y:           contentBottom + max(footnoteTextHeight, 0) + spaceAfter,
		Color:       style.BorderColor,
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
