package pdf

type pdfPrintedFootnotePagePlan struct {
	PageIndex         int
	Refs              []pdfPrintedFootnoteRef
	Queue             []pdfPrintedFootnoteQueueEntry
	QueuePages        []pdfPage
	UsedGlyphs        map[pdfFontKey]map[uint16]shapedGlyph
	ContinuationPages int
}

func buildPDFPrintedFootnotePagePlans(
	doc pdfDocumentSpec,
	pages []pdfPage,
	footnoteAreaHeight float64,
) ([]pdfPrintedFootnotePagePlan, error) {
	if len(doc.PrintedFootnotes) == 0 || len(pages) == 0 {
		return nil, nil
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	plans := make([]pdfPrintedFootnotePagePlan, 0, len(pages))
	for pageIndex := range pages {
		refs := pdfPrintedFootnotePageRefs(doc, pages[pageIndex])
		queue := buildPDFPrintedFootnoteQueue(doc, refs)
		if len(queue) == 0 {
			continue
		}
		pageFootnoteAreaHeight := pdfPrintedFootnoteSourceTextAreaHeight(doc, styles, pages[pageIndex], footnoteAreaHeight)
		queuePages, used, err := layoutPDFPrintedFootnoteQueue(doc, queue, pageFootnoteAreaHeight)
		if err != nil {
			return nil, err
		}
		plans = append(plans, pdfPrintedFootnotePagePlan{
			PageIndex:         pageIndex,
			Refs:              append([]pdfPrintedFootnoteRef(nil), refs...),
			Queue:             append([]pdfPrintedFootnoteQueueEntry(nil), queue...),
			QueuePages:        queuePages,
			UsedGlyphs:        used,
			ContinuationPages: max(len(queuePages)-1, 0),
		})
	}
	return plans, nil
}

func pdfPrintedFootnoteSourceTextAreaHeight(
	doc pdfDocumentSpec,
	styles *pdfStyleResolver,
	page pdfPage,
	defaultHeight float64,
) float64 {
	if defaultHeight <= 0 {
		return defaultHeight
	}
	_, mainBottom, ok := pdfPageYBounds(page)
	if !ok {
		return defaultHeight
	}
	contentLeft, contentRight, contentTop, contentBottom := pdfPageContentMargins(doc, styles, pdfDefaultPageMargin)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	contentHeight := max(doc.PageHeight-contentTop-contentBottom, 0)
	if contentHeight <= 0 {
		return defaultHeight
	}
	separator := pdfPrintedFootnoteSeparatorMetricsForArea(doc, styles, contentLeft, contentWidth, contentBottom, defaultHeight)
	availableTextHeight := min(max(mainBottom-contentBottom-separator.Reserve, 0), max(contentHeight-separator.Reserve, 0))
	return max(defaultHeight, availableTextHeight)
}
