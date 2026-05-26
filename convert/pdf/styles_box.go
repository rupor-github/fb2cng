package pdf

func (r *pdfStyleResolver) pageStyle() pdfBlockResolvedStyle {
	page := r.defaultStyle(pdfStylePage)
	page = mergePDFStyleOverridesWithFont(page, r.namedStyle(pdfStylePage), r.defaultStyle(pdfStylePage), page.Paragraph.FontSize)
	rootLeft, rootRight := r.rootHorizontalMargins()
	page.MarginLeft += rootLeft
	page.MarginRight += rootRight
	for _, name := range []string{pdfStyleHTML, pdfStyleBody} {
		root := r.namedStyle(name)
		fallback := r.defaultStyle(name)
		if root.HasSpaceBefore || root.SpaceBefore != fallback.SpaceBefore {
			page.SpaceBefore += root.SpaceBefore
		}
		if root.HasSpaceAfter || root.SpaceAfter != fallback.SpaceAfter {
			page.SpaceAfter += root.SpaceAfter
		}
	}
	return page
}

func (r *pdfStyleResolver) rootHorizontalMargins() (float64, float64) {
	var left float64
	var right float64
	for _, name := range []string{pdfStyleHTML, pdfStyleBody} {
		root := r.namedStyle(name)
		fallback := r.defaultStyle(name)
		if root.MarginLeft != fallback.MarginLeft {
			left += root.MarginLeft
		}
		if root.MarginRight != fallback.MarginRight {
			right += root.MarginRight
		}
	}
	return left, right
}

func blockContentWidth(contentWidth float64, style pdfBlockResolvedStyle) float64 {
	available := max(contentWidth-style.MarginLeft-style.MarginRight-style.PaddingLeft-style.PaddingRight, pdfMinBlockWidth)
	width := available
	if style.HasWidth {
		width = style.Width.resolve(available)
	}
	if style.HasMinWidth {
		width = max(width, style.MinWidth.resolve(available))
	}
	if style.HasMaxWidth {
		width = min(width, style.MaxWidth.resolve(available))
	}
	return min(max(width, pdfMinBlockWidth), available)
}

func blockBoxWidth(contentWidth float64, style pdfBlockResolvedStyle) float64 {
	return blockContentWidth(contentWidth, style) + style.PaddingLeft + style.PaddingRight
}

func (l pdfBlockLength) resolve(available float64) float64 {
	if l.Percent {
		return available * l.Value / 100
	}
	return l.Value
}
