package pdf

import "strings"

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

func (l *pdfPageLayout) layoutTableBlock(block pdfTextBlock, style pdfBlockResolvedStyle, blockLeft, blockWidthLimit float64) error {
	blockSpaceBefore := func() float64 { return pdfEffectiveBlockSpaceBefore(style, l.pageHasText, l.y, l.top) }
	tableWidth := blockContentWidth(blockWidthLimit, style)
	table, err := layoutPDFTable(l.doc, l.styles, block, style, tableWidth)
	if err != nil {
		return err
	}
	if table.Width <= 0 || len(table.Groups) == 0 {
		return nil
	}
	needed := blockSpaceBefore() + style.PaddingTop + table.Height + style.PaddingBottom + style.SpaceAfter
	if style.KeepTogether && l.pageHasText && l.y-needed < l.bottom && needed <= l.top-l.bottom {
		l.newTextPage()
	}
	addPDFPageAnchor(l.page, block.ID)
	l.y -= blockSpaceBefore() + style.PaddingTop
	tableX := blockLeft + style.MarginLeft + style.PaddingLeft
	for groupIndex, group := range table.Groups {
		if l.pageHasText && l.y-group.Height < l.bottom && (!style.KeepTogether || groupIndex > 0 || needed > l.top-l.bottom) {
			l.newTextPage()
		}
		groupTop := l.y
		for _, cell := range table.Cells {
			if cell.Row < group.Start || cell.Row > group.End {
				continue
			}
			if err := l.renderTableCell(table, group, cell, tableX, groupTop); err != nil {
				return err
			}
		}
		l.y -= group.Height
		l.pageHasText = true
		l.previousRenderedImage = true
	}
	l.y -= style.PaddingBottom + style.SpaceAfter
	if pdfStyleForcesPageBreakAfter(style) && l.pageHasText {
		l.newTextPage()
	}
	return nil
}

func (l *pdfPageLayout) renderTableCell(table pdfTableLayout, group pdfTableRowGroup, cell pdfTableCellLayout, tableX, groupTop float64) error {
	cellTop := groupTop - pdfTableRowsHeight(table.Rows, group.Start, cell.Row-1)
	cellBottom := cellTop - pdfTableRowsHeight(table.Rows, cell.Row, min(cell.Row+cell.RowSpan-1, group.End))
	cellX := tableX + cell.X
	addPDFBlockDecoration(l.page, cell.Style, cellX, cellTop, cell.Width, cellBottom)
	if len(cell.Lines) == 0 {
		return nil
	}
	innerWidth := max(cell.Width-cell.Style.PaddingLeft-cell.Style.PaddingRight-2*cell.Style.BorderWidth, 1)
	textHeight := pdfTableCellTextHeight(cell.Style, cell.Lines)
	availableHeight := max(cellTop-cellBottom-cell.Style.PaddingTop-cell.Style.PaddingBottom-2*cell.Style.BorderWidth, 0)
	verticalOffset := 0.0
	switch strings.ToLower(strings.TrimSpace(cell.VAlign)) {
	case "top":
	case "bottom":
		verticalOffset = max(availableHeight-textHeight, 0)
	default:
		verticalOffset = max((availableHeight-textHeight)/2, 0)
	}
	_, fontKey, err := fontForStyle(l.doc.Fonts, cell.Style.Paragraph)
	if err != nil {
		return err
	}
	lineY := cellTop - cell.Style.BorderWidth - cell.Style.PaddingTop - verticalOffset - cell.Style.Paragraph.FontSize
	lineSearchStart := 0
	linkBlock := pdfTextBlock{Text: cell.Text, Links: cell.Links}
	for _, line := range cell.Lines {
		x := cellX + cell.Style.BorderWidth + cell.Style.PaddingLeft + line.Indent
		available := innerWidth - line.Indent
		switch cell.Style.Paragraph.Align {
		case textAlignCenter:
			x += max((available-line.Width)/2, 0)
		case textAlignRight:
			x += max(available-line.Width, 0)
		}
		pageLine := pdfPageLine{
			X:                x,
			Y:                lineY,
			FontSize:         cell.Style.Paragraph.FontSize,
			LetterSpacing:    cell.Style.Paragraph.LetterSpacing,
			FontKey:          fontKey,
			Color:            cell.Style.Paragraph.Color,
			Text:             line.Text,
			Underline:        cell.Style.Paragraph.Underline,
			Strikethrough:    cell.Style.Paragraph.Strikethrough,
			Fragments:        pageLineFragments(line.Fragments),
			ExtraWordSpacing: line.ExtraWordSpacing,
			ExtraCharSpacing: line.ExtraCharSpacing,
			BreakStats:       line.BreakStats,
		}
		pageLine.X = pdfPageLineXAdjustedForVisualRight(pageLine, available)
		x = pageLine.X
		addPDFInlineImages(l.page, line, x, lineY)
		addLinkAnnotations(l.page, linkBlock, line, lineSearchStart, x, lineY, cell.Style.Paragraph.FontSize)
		addPDFParagraphFragmentAnchors(l.page, line)
		lineSearchStart = nextLineSearchStart(cell.Text, line, lineSearchStart)
		addPDFPageLine(l.page, l.used, pageLine)
		lineY -= cell.Style.Paragraph.LineHeight
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
