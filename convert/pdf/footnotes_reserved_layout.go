package pdf

const pdfPrintedFootnoteReserveLayoutIterations = 8

type pdfPrintedFootnoteReservedLayout struct {
	Pages              []pdfPage
	UsedGlyphs         map[pdfFontKey]map[uint16]shapedGlyph
	Plans              []pdfPrintedFootnotePagePlan
	PageBottomReserves []float64
	FootnoteTextHeight float64
}

func layoutPDFPagesWithPrintedFootnoteReserves(doc pdfDocumentSpec, fontFace *builtinFontFace) (pdfPrintedFootnoteReservedLayout, error) {
	pages, used, err := layoutPDFPages(doc, fontFace)
	if err != nil {
		return pdfPrintedFootnoteReservedLayout{}, err
	}
	if len(doc.PrintedFootnotes) == 0 || len(pages) == 0 {
		return pdfPrintedFootnoteReservedLayout{Pages: pages, UsedGlyphs: used}, nil
	}

	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	footnoteTextHeight := pdfPrintedFootnoteTextAreaHeight(doc, styles)
	if footnoteTextHeight <= 0 {
		return pdfPrintedFootnoteReservedLayout{Pages: pages, UsedGlyphs: used}, nil
	}

	var plans []pdfPrintedFootnotePagePlan
	var reserves []float64
	for range pdfPrintedFootnoteReserveLayoutIterations {
		probe := doc
		probe.PageBottomReserves = reserves
		probePages, _, err := layoutPDFPages(probe, fontFace)
		if err != nil {
			return pdfPrintedFootnoteReservedLayout{}, err
		}
		nextPlans, nextReserves, err := buildPDFPrintedFootnotePagePlansAndReserves(doc, probePages, footnoteTextHeight)
		if err != nil {
			return pdfPrintedFootnoteReservedLayout{}, err
		}
		if pdfPrintedFootnoteReservesStable(reserves, nextReserves) && pdfPrintedFootnotePlansStable(plans, nextPlans) {
			reserves = nextReserves
			break
		}
		plans = nextPlans
		reserves = nextReserves
	}

	finalDoc := doc
	finalDoc.PageBottomReserves = reserves
	pages, used, err = layoutPDFPages(finalDoc, fontFace)
	if err != nil {
		return pdfPrintedFootnoteReservedLayout{}, err
	}
	plans, reserves, err = buildPDFPrintedFootnotePagePlansAndReserves(doc, pages, footnoteTextHeight)
	if err != nil {
		return pdfPrintedFootnoteReservedLayout{}, err
	}
	return pdfPrintedFootnoteReservedLayout{
		Pages:              pages,
		UsedGlyphs:         used,
		Plans:              plans,
		PageBottomReserves: reserves,
		FootnoteTextHeight: footnoteTextHeight,
	}, nil
}

func pdfPrintedFootnoteReservesStable(left []float64, right []float64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !pdfFloatNearlyEqual(left[i], right[i]) {
			return false
		}
	}
	return true
}

func pdfPrintedFootnotePlansStable(left []pdfPrintedFootnotePagePlan, right []pdfPrintedFootnotePagePlan) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].PageIndex != right[i].PageIndex ||
			!samePDFPrintedFootnoteRefs(left[i].Refs, right[i].Refs) ||
			!samePDFPrintedFootnoteQueue(left[i].Queue, right[i].Queue) ||
			left[i].ContinuationPages != right[i].ContinuationPages {
			return false
		}
	}
	return true
}

func samePDFPrintedFootnoteQueue(left []pdfPrintedFootnoteQueueEntry, right []pdfPrintedFootnoteQueueEntry) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func pdfFloatNearlyEqual(left float64, right float64) bool {
	const epsilon = 0.001
	if left > right {
		return left-right < epsilon
	}
	return right-left < epsilon
}

func samePDFPrintedFootnoteRefs(left []pdfPrintedFootnoteRef, right []pdfPrintedFootnoteRef) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
