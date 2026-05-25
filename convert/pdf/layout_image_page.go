package pdf

func (l *pdfPageLayout) layoutImageBlock(blockIndex int, block pdfTextBlock, style pdfBlockResolvedStyle, blockLeft, blockWidthLimit float64) error {
	blockSpaceBefore := func() float64 { return pdfEffectiveBlockSpaceBefore(style, l.pageHasText, l.y, l.top) }
	backgroundX := blockLeft + style.MarginLeft
	backgroundWidth := blockBoxWidth(blockWidthLimit, style)
	blockWidth := blockContentWidth(blockWidthLimit, style)
	img := l.doc.Images[block.ImageID]
	if img == nil {
		return nil
	}
	maxImageHeight := l.top - l.bottom - blockSpaceBefore() - style.PaddingTop - style.PaddingBottom - style.SpaceAfter
	if maxImageHeight <= 0 {
		return nil
	}
	forceContentWidth := isVignetteBlock(block) || isHeadingImageBlock(block)
	widthReference := pdfBlockImageReferenceWidth(block, style, blockWidthLimit, l.rootlessContentWidth, img, forceContentWidth)
	width, height, ok := fitPDFBlockImageSize(l.doc, img, blockWidth, maxImageHeight, widthReference, forceContentWidth)
	if !ok {
		return nil
	}
	needed := blockSpaceBefore() + style.PaddingTop + height + style.PaddingBottom + style.SpaceAfter
	if l.pageHasText {
		keepWithNext, err := nextBlockKeepHeight(l.doc, l.blockStyles, blockIndex+1, l.contentWidth, l.rootlessContentWidth, l.top-l.bottom, pdfKeepWithNextLines(l.doc.Blocks, l.blockStyles, blockIndex))
		if err != nil {
			return err
		}
		if keepWithNext > 0 && l.y-needed-keepWithNext < l.bottom && needed+keepWithNext <= l.top-l.bottom {
			l.newTextPage()
		} else if pdfBlockImageOverflowsBottom(l.y-needed, l.bottom) {
			l.newTextPage()
		}
	}
	l.y -= blockSpaceBefore()
	l.y -= style.PaddingTop
	if pdfBlockImageOverflowsBottom(l.y-height, l.bottom) {
		l.newTextPage()
		l.y -= blockSpaceBefore()
		l.y -= style.PaddingTop
	}
	addPDFPageAnchor(l.page, block.ID)
	backgroundTop := l.y + style.PaddingTop
	l.y -= height
	imageX := blockLeft + style.MarginLeft + style.PaddingLeft
	switch style.Paragraph.Align {
	case textAlignCenter:
		imageX += max((blockWidth-width)/2, 0)
	case textAlignRight:
		imageX += max(blockWidth-width, 0)
	default:
	}
	l.page.Images = append(l.page.Images, pdfPageImage{
		ImageID: block.ImageID,
		X:       imageX,
		Y:       l.y,
		Width:   width,
		Height:  height,
	})
	if isTitleTopVignetteBlock(block) {
		l.titleGroup.start(l.page, l.y)
	} else if isTitleBottomVignetteBlock(block) {
		l.titleGroup.finish(l.page, l.y+height, l.doc.Fonts)
	} else if l.titleGroup.active && !isTitleHeaderImageBlock(block) {
		l.titleGroup.reset()
	}
	l.pageHasText = true
	l.previousRenderedImage = true
	backgroundBottom := l.y - style.PaddingBottom
	addPDFBlockDecoration(l.page, style, backgroundX, backgroundTop, backgroundWidth, backgroundBottom)
	l.y -= style.PaddingBottom + style.SpaceAfter
	if pdfStyleForcesPageBreakAfter(style) && l.pageHasText {
		l.newTextPage()
	}
	return nil
}
