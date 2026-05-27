package pdf

func pdfPrintedFootnoteQueueBlocks(doc pdfDocumentSpec, queue []pdfPrintedFootnoteQueueEntry) []pdfTextBlock {
	if len(queue) == 0 || len(doc.PrintedFootnotes) == 0 {
		return nil
	}
	var blocks []pdfTextBlock
	labels := pdfPrintedFootnoteQueueLabels(queue)
	for _, entry := range queue {
		note, ok := doc.PrintedFootnotes[entry.ID]
		if !ok {
			continue
		}
		entryBlocks := pdfFootnoteQueueBlocks(doc.Content, note, entry, false, doc.Styles)
		blocks = append(blocks, applyPDFPrintedFootnoteQueueReferenceLabels(entryBlocks, doc.Content, labels, doc.Styles)...)
	}
	return blocks
}

func layoutPDFPrintedFootnoteQueue(
	doc pdfDocumentSpec,
	queue []pdfPrintedFootnoteQueueEntry,
	areaHeight float64,
) ([]pdfPage, map[pdfFontKey]map[uint16]shapedGlyph, error) {
	blocks := pdfPrintedFootnoteQueueBlocks(doc, queue)
	if len(blocks) == 0 {
		return nil, nil, nil
	}
	if areaHeight <= 0 {
		areaHeight = 1
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	_, _, contentTop, contentBottom := pdfContentMargins(doc, styles, pdfDefaultPageMargin, false)
	subDoc := doc
	subDoc.PageHeight = contentTop + contentBottom + areaHeight
	subDoc.Blocks = blocks
	subDoc.TOC = nil
	subDoc.PrintedFootnotes = nil
	subDoc.CoverID = ""
	return layoutPDFPages(subDoc)
}
