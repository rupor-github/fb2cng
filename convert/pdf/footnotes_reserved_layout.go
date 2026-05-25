package pdf

type pdfPrintedFootnoteReservedLayout struct {
	Pages              []pdfPage
	UsedGlyphs         map[pdfFontKey]map[uint16]shapedGlyph
	Plans              []pdfPrintedFootnotePagePlan
	PageBottomReserves []float64
	FootnoteTextHeight float64
}

func layoutPDFPagesWithPrintedFootnoteReserves(doc pdfDocumentSpec) (pdfPrintedFootnoteReservedLayout, error) {
	doc.DynamicPrintedFootnoteReserves = true
	pages, used, err := layoutPDFPages(doc)
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

	plans, reserves, err := buildPDFPrintedFootnotePagePlansAndReserves(doc, pages, footnoteTextHeight)
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
