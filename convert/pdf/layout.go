package pdf

type pdfPageLayout struct {
	doc pdfDocumentSpec

	styles      *pdfStyleResolver
	blockStyles []pdfBlockResolvedStyle
	used        map[pdfFontKey]map[uint16]shapedGlyph
	pages       []pdfPage
	page        *pdfPage

	contentLeft   float64
	contentRight  float64
	contentTop    float64
	contentBottom float64

	rootlessContentLeft  float64
	rootlessContentRight float64
	contentWidth         float64
	rootlessContentWidth float64

	printedFootnoteReserve pdfDynamicPrintedFootnoteReserveTracker

	top    float64
	bottom float64
	y      float64

	pageHasText           bool
	previousRenderedImage bool
	activeDropcap         *pdfActiveDropcap
	titleGroup            pdfTitleVignetteContentGroup
}

func layoutPDFPages(doc pdfDocumentSpec) ([]pdfPage, map[pdfFontKey]map[uint16]shapedGlyph, error) {
	layout := newPDFPageLayout(doc)
	if err := layout.layout(); err != nil {
		return nil, nil, err
	}
	return layout.pages, layout.used, nil
}

func newPDFPageLayout(doc pdfDocumentSpec) *pdfPageLayout {
	return &pdfPageLayout{
		doc:   doc,
		used:  make(map[pdfFontKey]map[uint16]shapedGlyph),
		pages: make([]pdfPage, 0, 2),
	}
}

func (l *pdfPageLayout) layout() error {
	l.addCoverPage()
	if len(l.doc.Blocks) == 0 {
		if len(l.pages) == 0 {
			l.addPage()
		}
		return nil
	}

	l.initTextLayout()
	if err := l.layoutBlocks(); err != nil {
		return err
	}
	return l.finish()
}

func (l *pdfPageLayout) addCoverPage() {
	cover := l.doc.Images[l.doc.CoverID]
	if cover == nil {
		return
	}
	rect, ok := fitPDFImageInBox(l.doc, cover, 0, 0, l.doc.PageWidth, l.doc.PageHeight)
	if !ok {
		return
	}
	coverPage := l.addPage()
	addPDFPageAnchor(coverPage, l.doc.CoverID)
	coverPage.Images = append(coverPage.Images, pdfPageImage{
		ImageID: l.doc.CoverID,
		X:       rect.X1,
		Y:       rect.Y1,
		Width:   rect.X2 - rect.X1,
		Height:  rect.Y2 - rect.Y1,
	})
}

func (l *pdfPageLayout) initTextLayout() {
	l.styles = l.doc.Styles
	if l.styles == nil {
		l.styles = newPDFStyleResolver(nil, nil)
	}
	l.blockStyles = l.styles.collapsedBlockStylesWithImages(l.doc.Blocks, l.doc.Images)
	l.contentLeft, l.contentRight, l.contentTop, l.contentBottom = pdfPageContentMargins(l.doc, l.styles, pdfDefaultPageMargin)
	l.rootlessContentLeft, l.rootlessContentRight, _, _ = pdfPageContentMarginsWithoutRootHorizontal(l.doc, l.styles, pdfDefaultPageMargin)
	l.contentWidth = max(l.doc.PageWidth-l.contentLeft-l.contentRight, 12)
	l.rootlessContentWidth = max(l.doc.PageWidth-l.rootlessContentLeft-l.rootlessContentRight, 12)
	l.printedFootnoteReserve = newPDFDynamicPrintedFootnoteReserveTracker(l.doc, l.styles, l.contentLeft, l.contentWidth, l.contentBottom)
	l.top = l.doc.PageHeight - l.contentTop
	l.page = l.addPage()
	l.bottom = l.pageBottom(len(l.pages) - 1)
	l.y = l.top
}

func (l *pdfPageLayout) addPage() *pdfPage {
	l.pages = append(l.pages, pdfPage{})
	return &l.pages[len(l.pages)-1]
}

func (l *pdfPageLayout) pageBottom(pageIndex int) float64 {
	reserve := 0.0
	if pageIndex >= 0 && pageIndex < len(l.doc.PageBottomReserves) {
		reserve = l.doc.PageBottomReserves[pageIndex]
	}
	return pdfReservedContentBottom(l.contentBottom, l.doc.PageHeight-l.contentTop, reserve)
}

func (l *pdfPageLayout) newTextPage() {
	l.titleGroup.reset()
	l.page = l.addPage()
	l.bottom = l.pageBottom(len(l.pages) - 1)
	l.y = l.top
	l.pageHasText = false
	l.previousRenderedImage = false
	l.activeDropcap = nil
	l.printedFootnoteReserve.ResetPage()
}

func (l *pdfPageLayout) finish() error {
	if pdfPrintedFootnoteReferencesRenumbered(l.doc.Content) && len(l.doc.PrintedFootnotes) > 0 {
		if err := applyPDFPageLocalFootnoteReferenceLabels(l.pages, l.doc.Fonts, l.used, l.styles); err != nil {
			return err
		}
	}

	if len(l.pages[len(l.pages)-1].Lines) == 0 && len(l.pages[len(l.pages)-1].Images) == 0 {
		l.pages = l.pages[:len(l.pages)-1]
	}
	return nil
}

func (l *pdfPageLayout) layoutBlocks() error {
	for blockIndex, block := range l.doc.Blocks {
		if block.Kind == pdfBlockPageBreak {
			if l.pageHasText {
				l.newTextPage()
			}
			addPDFPageAnchor(l.page, block.ID)
			continue
		}

		if l.activeDropcap != nil && block.Kind != pdfBlockParagraph {
			if !pdfDropcapExpired(l.activeDropcap, l.page, l.y) {
				l.y = l.activeDropcap.BottomY
			}
			l.activeDropcap = nil
		}

		style := l.blockStyles[blockIndex]
		if style.Hidden {
			continue
		}
		if pdfStyleForcesPageBreakBefore(style) && l.pageHasText {
			l.newTextPage()
		}

		blockLeft := l.contentLeft
		blockWidthLimit := l.contentWidth
		if block.StripRootHorizontalMargins {
			blockLeft = l.rootlessContentLeft
			blockWidthLimit = l.rootlessContentWidth
		}

		if block.Kind == pdfBlockTable {
			if err := l.layoutTableBlock(block, style, blockLeft, blockWidthLimit); err != nil {
				return err
			}
			continue
		}

		if block.Kind == pdfBlockImage {
			if err := l.layoutImageBlock(blockIndex, block, style, blockLeft, blockWidthLimit); err != nil {
				return err
			}
			continue
		}

		if err := l.layoutTextBlock(blockIndex, block, style, blockLeft, blockWidthLimit); err != nil {
			return err
		}

	}

	return nil
}

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
