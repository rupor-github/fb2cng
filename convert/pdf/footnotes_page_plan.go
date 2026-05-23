package pdf

type pdfPrintedFootnotePagePlan struct {
	PageIndex         int
	Refs              []string
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
	plans := make([]pdfPrintedFootnotePagePlan, 0, len(pages))
	for pageIndex := range pages {
		refs := pdfPrintedFootnotePageRefs(doc, pages[pageIndex])
		queue := buildPDFPrintedFootnoteQueue(doc, refs)
		if len(queue) == 0 {
			continue
		}
		queuePages, used, err := layoutPDFPrintedFootnoteQueue(doc, queue, footnoteAreaHeight)
		if err != nil {
			return nil, err
		}
		plans = append(plans, pdfPrintedFootnotePagePlan{
			PageIndex:         pageIndex,
			Refs:              append([]string(nil), refs...),
			Queue:             append([]pdfPrintedFootnoteQueueEntry(nil), queue...),
			QueuePages:        queuePages,
			UsedGlyphs:        used,
			ContinuationPages: max(len(queuePages)-1, 0),
		})
	}
	return plans, nil
}
