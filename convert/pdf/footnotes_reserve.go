package pdf

// Printed footnotes can use at most this fraction of a normal content page on
// their source page. Longer note queues continue on inserted footnote pages.
const pdfPrintedFootnoteDefaultTextAreaFraction = 0.35

func pdfPrintedFootnoteTextAreaHeight(doc pdfDocumentSpec, styles *pdfStyleResolver) float64 {
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	_, _, contentTop, contentBottom := pdfContentMargins(doc, styles, pdfDefaultPageMargin, false)
	contentHeight := max(doc.PageHeight-contentTop-contentBottom, 0)
	if contentHeight <= pdfBaseLineHeight {
		return max(contentHeight, 0)
	}
	return min(contentHeight*pdfPrintedFootnoteDefaultTextAreaFraction, contentHeight-pdfBaseLineHeight)
}

func buildPDFPrintedFootnotePagePlansAndReserves(
	doc pdfDocumentSpec,
	pages []pdfPage,
	footnoteTextHeight float64,
) ([]pdfPrintedFootnotePagePlan, []float64, error) {
	plans, err := buildPDFPrintedFootnotePagePlans(doc, pages, footnoteTextHeight)
	if err != nil {
		return nil, nil, err
	}
	reserves := pdfPrintedFootnotePlanReserves(doc, plans, len(pages), footnoteTextHeight)
	return plans, reserves, nil
}

func pdfPrintedFootnotePlanReserves(
	doc pdfDocumentSpec,
	plans []pdfPrintedFootnotePagePlan,
	pageCount int,
	footnoteTextHeight float64,
) []float64 {
	// Convert each page plan into the vertical amount the main-text paginator must
	// leave empty at the bottom of the corresponding source page.
	if len(plans) == 0 || pageCount <= 0 || footnoteTextHeight <= 0 {
		return nil
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	contentLeft, contentRight, contentTop, contentBottom := pdfContentMargins(doc, styles, pdfDefaultPageMargin, false)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	separator := pdfPrintedFootnoteSeparatorMetricsForArea(doc, styles, contentLeft, contentWidth, contentBottom, footnoteTextHeight)
	top := doc.PageHeight - contentTop
	maxReserve := max(top-contentBottom-pdfBaseLineHeight, 0)
	if maxReserve <= 0 {
		return nil
	}
	reserves := make([]float64, pageCount)
	for _, plan := range plans {
		if plan.PageIndex < 0 || plan.PageIndex >= pageCount || len(plan.QueuePages) == 0 {
			continue
		}
		reserve := min(pdfPrintedFootnotePagePlanReserve(plan, footnoteTextHeight, separator), maxReserve)
		if reserve <= 0 {
			continue
		}
		reserves[plan.PageIndex] = reserve
	}
	if !pdfHasAnyPageBottomReserve(reserves) {
		return nil
	}
	return reserves
}

func pdfPrintedFootnotePagePlanReserve(
	plan pdfPrintedFootnotePagePlan,
	footnoteTextHeight float64,
	separator pdfPrintedFootnoteSeparatorMetrics,
) float64 {
	// Reserve only the first footnote chunk on the source page. If the queue spans
	// more chunks, those chunks become separate inserted pages and do not consume
	// additional space from the source page.
	if len(plan.QueuePages) == 0 || footnoteTextHeight <= 0 {
		return 0
	}
	chunkTop, chunkBottom, ok := pdfPageYBounds(plan.QueuePages[0])
	if !ok {
		return 0
	}
	chunkHeight := footnoteTextHeight
	if len(plan.QueuePages) == 1 {
		chunkHeight = min(max(chunkTop-chunkBottom, 0), footnoteTextHeight)
	}
	return chunkHeight + separator.Reserve
}

func pdfHasAnyPageBottomReserve(reserves []float64) bool {
	for _, reserve := range reserves {
		if reserve > 0 {
			return true
		}
	}
	return false
}
