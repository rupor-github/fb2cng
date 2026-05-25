package pdf

func pdfPageContentMargins(doc pdfDocumentSpec, styles *pdfStyleResolver, baseMargin float64) (float64, float64, float64, float64) {
	return pdfPageContentMarginsWithOptions(doc, styles, baseMargin, false)
}

func pdfPageContentMarginsWithoutRootHorizontal(doc pdfDocumentSpec, styles *pdfStyleResolver, baseMargin float64) (float64, float64, float64, float64) {
	return pdfPageContentMarginsWithOptions(doc, styles, baseMargin, true)
}

func pdfPageContentMarginsWithOptions(doc pdfDocumentSpec, styles *pdfStyleResolver, baseMargin float64, stripRootHorizontal bool) (float64, float64, float64, float64) {
	left := baseMargin
	right := baseMargin
	top := baseMargin
	bottom := baseMargin
	if styles != nil {
		pageStyle := styles.pageStyle()
		left += pageStyle.MarginLeft
		right += pageStyle.MarginRight
		top += pageStyle.SpaceBefore
		bottom += pageStyle.SpaceAfter
		if stripRootHorizontal {
			rootLeft, rootRight := styles.rootHorizontalMargins()
			left -= rootLeft
			right -= rootRight
		}
	}
	left = max(left, 0)
	right = max(right, 0)
	top = max(top, 0)
	bottom = max(bottom, 0)
	if left+right > doc.PageWidth-pdfMinBlockWidth {
		overflow := left + right - (doc.PageWidth - pdfMinBlockWidth)
		left = max(left-overflow/2, 0)
		right = max(right-overflow/2, 0)
	}
	if top+bottom > doc.PageHeight-pdfMinBlockWidth {
		overflow := top + bottom - (doc.PageHeight - pdfMinBlockWidth)
		top = max(top-overflow/2, 0)
		bottom = max(bottom-overflow/2, 0)
	}
	return left, right, top, bottom
}
