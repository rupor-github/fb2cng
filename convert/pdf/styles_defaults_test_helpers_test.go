package pdf

func headingPDFStyle(depth int) pdfBlockResolvedStyle {
	lineHeight := pdfAdjustedLineHeight
	if depth > 1 {
		lineHeight = pdfSectionTitleHeaderLineHeight
	}
	return headingPDFStyleWithLineHeight(pdfHeadingFontSize(depth), lineHeight, pdfHeadingMarginFactor(depth))
}
