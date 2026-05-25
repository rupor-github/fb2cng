package pdf

import "strings"

func layoutPDFPages(doc pdfDocumentSpec) ([]pdfPage, map[pdfFontKey]map[uint16]shapedGlyph, error) {
	used := make(map[pdfFontKey]map[uint16]shapedGlyph)
	pages := make([]pdfPage, 0, 2)

	addPage := func() *pdfPage {
		pages = append(pages, pdfPage{})
		return &pages[len(pages)-1]
	}
	if cover := doc.Images[doc.CoverID]; cover != nil {
		if rect, ok := fitPDFImageInBox(doc, cover, 0, 0, doc.PageWidth, doc.PageHeight); ok {
			coverPage := addPage()
			addPDFPageAnchor(coverPage, doc.CoverID)
			coverPage.Images = append(coverPage.Images, pdfPageImage{
				ImageID: doc.CoverID,
				X:       rect.X1,
				Y:       rect.Y1,
				Width:   rect.X2 - rect.X1,
				Height:  rect.Y2 - rect.Y1,
			})
		}
	}

	if len(doc.Blocks) == 0 {
		if len(pages) == 0 {
			addPage()
		}
		return pages, used, nil
	}

	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	blockStyles := styles.collapsedBlockStylesWithImages(doc.Blocks, doc.Images)
	contentLeft, contentRight, contentTop, contentBottom := pdfPageContentMargins(doc, styles, pdfDefaultPageMargin)
	rootlessContentLeft, rootlessContentRight, _, _ := pdfPageContentMarginsWithoutRootHorizontal(doc, styles, pdfDefaultPageMargin)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	rootlessContentWidth := max(doc.PageWidth-rootlessContentLeft-rootlessContentRight, 12)
	printedFootnoteReserve := newPDFDynamicPrintedFootnoteReserveTracker(doc, styles, contentLeft, contentWidth, contentBottom)
	pageBottom := func(pageIndex int) float64 {
		reserve := 0.0
		if pageIndex >= 0 && pageIndex < len(doc.PageBottomReserves) {
			reserve = doc.PageBottomReserves[pageIndex]
		}
		return pdfReservedContentBottom(contentBottom, doc.PageHeight-contentTop, reserve)
	}
	page := addPage()
	top := doc.PageHeight - contentTop
	bottom := pageBottom(len(pages) - 1)
	y := top
	pageHasText := false
	previousRenderedImage := false
	var activeDropcap *pdfActiveDropcap
	titleGroup := pdfTitleVignetteContentGroup{}
	newTextPage := func() {
		titleGroup.reset()
		page = addPage()
		bottom = pageBottom(len(pages) - 1)
		y = top
		pageHasText = false
		previousRenderedImage = false
		activeDropcap = nil
		printedFootnoteReserve.ResetPage()
	}

	for blockIndex, block := range doc.Blocks {
		if block.Kind == pdfBlockPageBreak {
			if pageHasText {
				newTextPage()
			}
			addPDFPageAnchor(page, block.ID)
			continue
		}

		if activeDropcap != nil && block.Kind != pdfBlockParagraph {
			if !pdfDropcapExpired(activeDropcap, page, y) {
				y = activeDropcap.BottomY
			}
			activeDropcap = nil
		}

		style := blockStyles[blockIndex]
		if style.Hidden {
			continue
		}
		if pdfStyleForcesPageBreakBefore(style) && pageHasText {
			newTextPage()
		}

		blockLeft := contentLeft
		blockWidthLimit := contentWidth
		if block.StripRootHorizontalMargins {
			blockLeft = rootlessContentLeft
			blockWidthLimit = rootlessContentWidth
		}

		if block.Kind == pdfBlockTable {
			blockSpaceBefore := func() float64 { return pdfEffectiveBlockSpaceBefore(style, pageHasText, y, top) }
			tableWidth := blockContentWidth(blockWidthLimit, style)
			table, err := layoutPDFTable(doc, styles, block, style, tableWidth)
			if err != nil {
				return nil, nil, err
			}
			if table.Width <= 0 || len(table.Groups) == 0 {
				continue
			}
			needed := blockSpaceBefore() + style.PaddingTop + table.Height + style.PaddingBottom + style.SpaceAfter
			if style.KeepTogether && pageHasText && y-needed < bottom && needed <= top-bottom {
				newTextPage()
			}
			addPDFPageAnchor(page, block.ID)
			y -= blockSpaceBefore() + style.PaddingTop
			tableX := blockLeft + style.MarginLeft + style.PaddingLeft
			for groupIndex, group := range table.Groups {
				if pageHasText && y-group.Height < bottom && (!style.KeepTogether || groupIndex > 0 || needed > top-bottom) {
					newTextPage()
				}
				groupTop := y
				for _, cell := range table.Cells {
					if cell.Row < group.Start || cell.Row > group.End {
						continue
					}
					cellTop := groupTop - pdfTableRowsHeight(table.Rows, group.Start, cell.Row-1)
					cellBottom := cellTop - pdfTableRowsHeight(table.Rows, cell.Row, min(cell.Row+cell.RowSpan-1, group.End))
					cellX := tableX + cell.X
					addPDFBlockDecoration(page, cell.Style, cellX, cellTop, cell.Width, cellBottom)
					if len(cell.Lines) == 0 {
						continue
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
					_, fontKey, err := fontForStyle(doc.Fonts, cell.Style.Paragraph)
					if err != nil {
						return nil, nil, err
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
						addPDFInlineImages(page, line, x, lineY)
						addLinkAnnotations(page, linkBlock, line, lineSearchStart, x, lineY, cell.Style.Paragraph.FontSize)
						addPDFParagraphFragmentAnchors(page, line)
						lineSearchStart = nextLineSearchStart(cell.Text, line, lineSearchStart)
						addPDFPageLine(page, used, pageLine)
						lineY -= cell.Style.Paragraph.LineHeight
					}
				}
				y -= group.Height
				pageHasText = true
				previousRenderedImage = true
			}
			y -= style.PaddingBottom + style.SpaceAfter
			if pdfStyleForcesPageBreakAfter(style) && pageHasText {
				newTextPage()
			}
			continue
		}

		if block.Kind == pdfBlockImage {
			blockSpaceBefore := func() float64 { return pdfEffectiveBlockSpaceBefore(style, pageHasText, y, top) }
			backgroundX := blockLeft + style.MarginLeft
			backgroundWidth := blockBoxWidth(blockWidthLimit, style)
			blockWidth := blockContentWidth(blockWidthLimit, style)
			img := doc.Images[block.ImageID]
			if img == nil {
				continue
			}
			maxImageHeight := top - bottom - blockSpaceBefore() - style.PaddingTop - style.PaddingBottom - style.SpaceAfter
			if maxImageHeight <= 0 {
				continue
			}
			forceContentWidth := isVignetteBlock(block) || isHeadingImageBlock(block)
			widthReference := pdfBlockImageReferenceWidth(block, style, blockWidthLimit, rootlessContentWidth, img, forceContentWidth)
			width, height, ok := fitPDFBlockImageSize(doc, img, blockWidth, maxImageHeight, widthReference, forceContentWidth)
			if !ok {
				continue
			}
			needed := blockSpaceBefore() + style.PaddingTop + height + style.PaddingBottom + style.SpaceAfter
			if pageHasText {
				keepWithNext, err := nextBlockKeepHeight(doc, blockStyles, blockIndex+1, contentWidth, rootlessContentWidth, top-bottom, pdfKeepWithNextLines(doc.Blocks, blockStyles, blockIndex))
				if err != nil {
					return nil, nil, err
				}
				if keepWithNext > 0 && y-needed-keepWithNext < bottom && needed+keepWithNext <= top-bottom {
					newTextPage()
				} else if pdfBlockImageOverflowsBottom(y-needed, bottom) {
					newTextPage()
				}
			}
			y -= blockSpaceBefore()
			y -= style.PaddingTop
			if pdfBlockImageOverflowsBottom(y-height, bottom) {
				newTextPage()
				y -= blockSpaceBefore()
				y -= style.PaddingTop
			}
			addPDFPageAnchor(page, block.ID)
			backgroundTop := y + style.PaddingTop
			y -= height
			imageX := blockLeft + style.MarginLeft + style.PaddingLeft
			switch style.Paragraph.Align {
			case textAlignCenter:
				imageX += max((blockWidth-width)/2, 0)
			case textAlignRight:
				imageX += max(blockWidth-width, 0)
			default:
			}
			page.Images = append(page.Images, pdfPageImage{
				ImageID: block.ImageID,
				X:       imageX,
				Y:       y,
				Width:   width,
				Height:  height,
			})
			if isTitleTopVignetteBlock(block) {
				titleGroup.start(page, y)
			} else if isTitleBottomVignetteBlock(block) {
				titleGroup.finish(page, y+height, doc.Fonts)
			} else if titleGroup.active && !isTitleHeaderImageBlock(block) {
				titleGroup.reset()
			}
			pageHasText = true
			previousRenderedImage = true
			backgroundBottom := y - style.PaddingBottom
			addPDFBlockDecoration(page, style, backgroundX, backgroundTop, backgroundWidth, backgroundBottom)
			y -= style.PaddingBottom + style.SpaceAfter
			if pdfStyleForcesPageBreakAfter(style) && pageHasText {
				newTextPage()
			}
			continue
		}

		if titleGroup.active && !isTitleHeaderBlock(block) {
			titleGroup.reset()
		}
		style.Paragraph.Hyphenator = doc.Hyphenator
		blockWidth := blockContentWidth(blockWidthLimit, style)
		if block.Kind == pdfBlockEmptyLine {
			continue
		}
		text := strings.TrimSpace(block.Text)
		if text == "" && !inlineRunsRenderable(block.Runs) {
			continue
		}
		face, fontKey, err := fontForStyle(doc.Fonts, style.Paragraph)
		if err != nil {
			return nil, nil, err
		}
		runs := inlineRunsWithContext(block.Runs, inlineRunContextClassesForBlock(block))
		lineHeight := pdfEffectiveParagraphLineHeight(style.Paragraph)
		blockSpaceBefore := func() float64 { return pdfEffectiveBlockSpaceBefore(style, pageHasText, y, top) }
		firstBaselineY := func() float64 {
			baseline := y - blockSpaceBefore() - style.PaddingTop
			if !pageHasText || previousRenderedImage {
				baseline -= style.Paragraph.FontSize
			}
			return baseline
		}
		layoutTextBlock := func() ([]paragraphLine, pdfDropcapLayout, bool, error) {
			baseline := firstBaselineY()
			if pdfDropcapExpiredForLine(activeDropcap, page, baseline, lineHeight, style.Paragraph.FontSize) {
				activeDropcap = nil
			}
			shape := pdfActiveDropcapShape(activeDropcap, page, block, baseline, style.Paragraph)
			layoutText := block.Text
			layoutRuns := runs
			var dropcap pdfDropcapLayout
			dropcapOK := false
			if pdfBlockStartsDropcap(block) {
				var err error
				dropcap, dropcapOK, err = buildPDFDropcapLayout(doc, styles, block, style.Paragraph, face, runs, blockWidth, firstBaselineY())
				if err != nil {
					return nil, pdfDropcapLayout{}, false, err
				}
				if dropcapOK {
					shape = repeatPDFDropcapInset(dropcap.ExclusionWidth, dropcap.Lines)
					layoutText = dropcap.BodyText
					layoutRuns = dropcap.BodyRuns
				}
			}
			lines, err := layoutInlineWithShape(doc, doc.Fonts, styles, face, layoutText, layoutRuns, style.Paragraph, blockWidth, shape)
			return lines, dropcap, dropcapOK, err
		}
		lines, dropcap, dropcapOK, err := layoutTextBlock()
		if err != nil {
			return nil, nil, err
		}
		if len(lines) == 0 && !dropcapOK {
			continue
		}

		textHeight := float64(len(lines)) * lineHeight
		if dropcapOK {
			textHeight = max(textHeight, dropcap.ReservedHeight)
		}
		needed := blockSpaceBefore() + style.PaddingTop + textHeight + style.PaddingBottom
		if dropcapOK && pageHasText {
			requiredDropcapLines := max(dropcap.Lines, 1)
			if countFittingLines(firstBaselineY(), bottom, style.Paragraph.FontSize, lineHeight) < requiredDropcapLines {
				newTextPage()
			}
		}
		if style.KeepTogether && pageHasText && y-needed < bottom {
			newTextPage()
		}
		if keepLines := pdfKeepWithNextLines(doc.Blocks, blockStyles, blockIndex); keepLines > 0 && pageHasText {
			keepWithNext, err := nextBlockKeepHeight(doc, blockStyles, blockIndex+1, contentWidth, rootlessContentWidth, top-bottom, keepLines)
			if err != nil {
				return nil, nil, err
			}
			if keepWithNext > 0 && y-needed-style.SpaceAfter-keepWithNext < bottom && needed+style.SpaceAfter+keepWithNext <= top-bottom {
				newTextPage()
			}
		}
		if !style.KeepTogether && pageHasText {
			linesFit := countFittingLines(y-blockSpaceBefore()-style.PaddingTop, bottom, style.Paragraph.FontSize, lineHeight)
			if linesFit > 0 && linesFit < len(lines) {
				firstFragmentLines := linesFit
				if remaining := len(lines) - firstFragmentLines; remaining < style.Widows {
					firstFragmentLines = len(lines) - style.Widows
				}
				if firstFragmentLines < min(style.Orphans, len(lines)) {
					newTextPage()
				}
			}
		}
		lines, dropcap, dropcapOK, err = layoutTextBlock()
		if err != nil {
			return nil, nil, err
		}
		if len(lines) == 0 && !dropcapOK {
			continue
		}
		if dropcapOK && pageHasText && printedFootnoteReserve.Enabled() {
			var dropcapFootnoteRefs []pdfPrintedFootnoteRef
			for _, line := range lines {
				dropcapFootnoteRefs = append(dropcapFootnoteRefs, printedFootnoteReserve.LineRefs(line)...)
			}
			if len(dropcapFootnoteRefs) > 0 {
				reserve, err := printedFootnoteReserve.ReserveWithAdditionalRefs(dropcapFootnoteRefs)
				if err != nil {
					return nil, nil, err
				}
				reservedBottom := pdfReservedContentBottom(contentBottom, top, reserve)
				if dropcap.BottomY < reservedBottom {
					newTextPage()
					lines, dropcap, dropcapOK, err = layoutTextBlock()
					if err != nil {
						return nil, nil, err
					}
					if len(lines) == 0 && !dropcapOK {
						continue
					}
				}
			}
		}
		if dropcapOK {
			pdfTraceResolvedDropcap(styles, blockIndex, block, dropcap, style.Paragraph)
		}
		addPDFPageAnchor(page, block.ID)
		y -= blockSpaceBefore()
		backgroundX := blockLeft + style.MarginLeft
		backgroundWidth := blockBoxWidth(blockWidthLimit, style)
		y -= style.PaddingTop
		fragmentPage := page
		fragmentTop := y + style.PaddingTop
		lineSearchStart := 0
		if dropcapOK {
			dropcapX := blockLeft + style.MarginLeft + style.PaddingLeft
			dropLine := paragraphLine{Text: dropcap.Fragment.Text, Width: dropcap.Fragment.Width, Fragments: []paragraphLineFragment{dropcap.Fragment}}
			if dropcap.Fragment.LinkHref != "" {
				addFragmentLinkAnnotations(page, dropLine, dropcapX, dropcap.BaselineY)
			}
			addPDFParagraphFragmentAnchors(page, dropLine)
			addPDFPageLine(page, used, pdfPageLine{
				X:             dropcapX,
				Y:             dropcap.BaselineY,
				FontSize:      dropcap.Fragment.FontSize,
				LetterSpacing: dropcap.Fragment.LetterSpacing,
				FontKey:       dropcap.Fragment.FontKey,
				Color:         dropcap.Fragment.Color,
				Text:          dropcap.Fragment.Text,
				Underline:     dropcap.Fragment.Underline,
				Strikethrough: dropcap.Fragment.Strikethrough,
				Fragments:     pageLineFragments([]paragraphLineFragment{dropcap.Fragment}),
			})
			activeDropcap = &pdfActiveDropcap{Page: page, X: dropcapX, TopY: dropcap.TopY, BottomY: dropcap.BottomY, ExclusionWidth: dropcap.ExclusionWidth, Lines: dropcap.Lines, Char: dropcap.Run.Text, BodySearchOffset: dropcap.BodySearchOffset}
			lineSearchStart = dropcap.BodySearchOffset
		}
		for lineIndex, line := range lines {
			if !pageHasText || previousRenderedImage {
				y -= style.Paragraph.FontSize
				previousRenderedImage = false
			}
			if printedFootnoteReserve.Enabled() {
				lineRefs := printedFootnoteReserve.LineRefs(line)
				if len(lineRefs) > 0 {
					reserve, err := printedFootnoteReserve.ReserveWithAdditionalRefs(lineRefs)
					if err != nil {
						return nil, nil, err
					}
					reservedBottom := pdfReservedContentBottom(contentBottom, top, reserve)
					if pageHasText && y-style.Paragraph.FontSize < reservedBottom {
						addPDFBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, y)
						newTextPage()
						fragmentPage = page
						fragmentTop = y + style.Paragraph.FontSize
						y -= style.Paragraph.FontSize
						reserve, err = printedFootnoteReserve.ReserveWithAdditionalRefs(lineRefs)
						if err != nil {
							return nil, nil, err
						}
						reservedBottom = pdfReservedContentBottom(contentBottom, top, reserve)
					}
					printedFootnoteReserve.CommitAdditionalRefs(lineRefs, reserve)
					bottom = max(bottom, reservedBottom)
				}
			}
			if y-style.Paragraph.FontSize < bottom {
				if pageHasText {
					addPDFBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, y)
				}
				newTextPage()
				fragmentPage = page
				fragmentTop = y + style.Paragraph.FontSize
				y -= style.Paragraph.FontSize
			}
			remainingAfterLine := len(lines) - lineIndex - 1
			if remainingAfterLine > 0 && remainingAfterLine < style.Widows && y-lineHeight-style.Paragraph.FontSize < bottom {
				addPDFBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, y)
				newTextPage()
				fragmentPage = page
				fragmentTop = y
			}
			x := blockLeft + style.MarginLeft + style.PaddingLeft + line.Indent
			available := blockWidth - line.Indent
			switch style.Paragraph.Align {
			case textAlignCenter:
				x += max((available-line.Width)/2, 0)
			case textAlignRight:
				x += max(available-line.Width, 0)
			}
			pageLine := pdfPageLine{
				X:                x,
				Y:                y,
				FontSize:         style.Paragraph.FontSize,
				LetterSpacing:    style.Paragraph.LetterSpacing,
				FontKey:          fontKey,
				Color:            style.Paragraph.Color,
				Underline:        style.Paragraph.Underline,
				Strikethrough:    style.Paragraph.Strikethrough,
				Text:             line.Text,
				Fragments:        pageLineFragments(line.Fragments),
				ExtraWordSpacing: line.ExtraWordSpacing,
				ExtraCharSpacing: line.ExtraCharSpacing,
				BreakStats:       line.BreakStats,
			}
			pageLine.X = pdfPageLineXAdjustedForVisualRight(pageLine, available)
			x = pageLine.X
			addPDFInlineImages(page, line, x, y)
			addLinkAnnotations(page, block, line, lineSearchStart, x, y, style.Paragraph.FontSize)
			addPDFParagraphFragmentAnchors(page, line)
			lineSearchStart = nextLineSearchStart(block.Text, line, lineSearchStart)
			addPDFPageLine(page, used, pageLine)
			y -= lineHeight
			pageHasText = true
			previousRenderedImage = false
		}
		if dropcapOK && len(lines) == 0 {
			pageHasText = true
			previousRenderedImage = false
		}
		backgroundBottom := y - style.PaddingBottom
		addPDFBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, backgroundBottom)
		y -= style.PaddingBottom + style.SpaceAfter
		if activeDropcap != nil && activeDropcap.Page != page {
			activeDropcap = nil
		}
		if pdfStyleForcesPageBreakAfter(style) && pageHasText {
			newTextPage()
		}
	}

	if pdfPrintedFootnoteReferencesRenumbered(doc.Content) && len(doc.PrintedFootnotes) > 0 {
		if err := applyPDFPageLocalFootnoteReferenceLabels(pages, doc.Fonts, used, styles); err != nil {
			return nil, nil, err
		}
	}

	if len(pages[len(pages)-1].Lines) == 0 && len(pages[len(pages)-1].Images) == 0 {
		pages = pages[:len(pages)-1]
	}
	return pages, used, nil
}

func pdfReservedContentBottom(contentBottom float64, top float64, reserve float64) float64 {
	if reserve <= 0 {
		return contentBottom
	}
	maxBottom := top - pdfBaseLineHeight
	if maxBottom <= contentBottom {
		return contentBottom
	}
	return min(contentBottom+reserve, maxBottom)
}

func pdfEffectiveParagraphLineHeight(style paragraphStyle) float64 {
	lineHeight := style.LineHeight
	if lineHeight <= 0 {
		lineHeight = pdfBaseLineHeight
	}
	fontSize := style.FontSize
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	return max(lineHeight, fontSize)
}

func pdfEffectiveBlockSpaceBefore(style pdfBlockResolvedStyle, pageHasText bool, y float64, top float64) float64 {
	if style.SpaceBefore < 0 && !pageHasText && y >= top-0.001 {
		return 0
	}
	return style.SpaceBefore
}

type pdfTitleVignetteContentGroup struct {
	active          bool
	page            *pdfPage
	topBoundary     float64
	lineStart       int
	imageStart      int
	annotationStart int
}

func (g *pdfTitleVignetteContentGroup) start(page *pdfPage, topBoundary float64) {
	if page == nil {
		g.reset()
		return
	}
	g.active = true
	g.page = page
	g.topBoundary = topBoundary
	g.lineStart = len(page.Lines)
	g.imageStart = len(page.Images)
	g.annotationStart = len(page.Annotations)
}

func (g *pdfTitleVignetteContentGroup) finish(page *pdfPage, bottomBoundary float64, fonts *pdfFontRegistry) {
	if !g.active || g.page != page || page == nil {
		g.reset()
		return
	}
	lineEnd := len(page.Lines)
	imageEnd := len(page.Images) - 1
	annotationEnd := len(page.Annotations)
	visualTop, visualBottom, ok := pdfPageTitleContentVisualBounds(page, g.lineStart, lineEnd, g.imageStart, imageEnd, fonts)
	if ok && g.topBoundary > bottomBoundary {
		shift := (g.topBoundary+bottomBoundary)/2 - (visualTop+visualBottom)/2
		shiftPDFTitleContent(page, g.lineStart, lineEnd, g.imageStart, imageEnd, g.annotationStart, annotationEnd, shift)
	}
	g.reset()
}

func (g *pdfTitleVignetteContentGroup) reset() {
	*g = pdfTitleVignetteContentGroup{}
}

func pdfPageTitleContentVisualBounds(page *pdfPage, lineStart, lineEnd int, imageStart, imageEnd int, fonts *pdfFontRegistry) (float64, float64, bool) {
	if page == nil {
		return 0, 0, false
	}
	var top float64
	var bottom float64
	ok := false
	include := func(itemTop float64, itemBottom float64) {
		if itemTop <= itemBottom {
			return
		}
		if !ok || itemTop > top {
			top = itemTop
		}
		if !ok || itemBottom < bottom {
			bottom = itemBottom
		}
		ok = true
	}
	for i := max(lineStart, 0); i < lineEnd && i < len(page.Lines); i++ {
		includePDFLineVisualBounds(include, page.Lines[i], fonts)
	}
	for i := max(imageStart, 0); i < imageEnd && i < len(page.Images); i++ {
		image := page.Images[i]
		include(image.Y+image.Height, image.Y)
	}
	return top, bottom, ok
}

func includePDFLineVisualBounds(include func(float64, float64), line pdfPageLine, fonts *pdfFontRegistry) {
	if len(line.Fragments) == 0 {
		face, err := resolvePDFFontFace(fonts, line.FontKey)
		if err != nil {
			face = nil
		}
		ascent, descent := pdfFontVisualMetrics(face, line.FontSize)
		include(line.Y+ascent, line.Y-descent)
		return
	}
	for _, fragment := range line.Fragments {
		baseline := line.Y + fragment.BaselineShift
		if fragment.ImageID != "" && fragment.ImageHeight > 0 {
			include(baseline+fragment.ImageHeight, baseline)
			continue
		}
		face, err := resolvePDFFontFace(fonts, fragment.FontKey)
		if err != nil {
			face = nil
		}
		ascent, descent := pdfFontVisualMetrics(face, fragment.FontSize)
		include(baseline+ascent, baseline-descent)
	}
}

func pdfFontVisualMetrics(face *builtinFontFace, fontSize float64) (float64, float64) {
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	if face == nil || face.UnitsPerEm <= 0 {
		return fontSize * 0.8, fontSize * 0.2
	}
	ascent := float64(face.Ascent) * fontSize / float64(face.UnitsPerEm)
	descent := -float64(face.Descent) * fontSize / float64(face.UnitsPerEm)
	if ascent <= 0 || descent < 0 {
		return fontSize * 0.8, fontSize * 0.2
	}
	return ascent, descent
}

func shiftPDFTitleContent(page *pdfPage, lineStart, lineEnd int, imageStart, imageEnd int, annotationStart, annotationEnd int, shift float64) {
	if page == nil || shift == 0 {
		return
	}
	for i := max(lineStart, 0); i < lineEnd && i < len(page.Lines); i++ {
		page.Lines[i].Y += shift
	}
	for i := max(imageStart, 0); i < imageEnd && i < len(page.Images); i++ {
		page.Images[i].Y += shift
	}
	for i := max(annotationStart, 0); i < annotationEnd && i < len(page.Annotations); i++ {
		page.Annotations[i].Rect.Y1 += shift
		page.Annotations[i].Rect.Y2 += shift
	}
}

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

func pdfBlockImageOverflowsBottom(candidateBottom float64, pageBottom float64) bool {
	return candidateBottom < pageBottom-pdfBlockImageBottomFitOverflow
}

func nextBlockKeepHeight(doc pdfDocumentSpec, blockStyles []pdfBlockResolvedStyle, start int, contentWidth float64, rootlessContentWidth float64, contentHeight float64, minLines int) (float64, error) {
	if minLines <= 0 {
		return 0, nil
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	for i := start; i < len(doc.Blocks); i++ {
		block := doc.Blocks[i]
		switch block.Kind {
		case pdfBlockPageBreak:
			return 0, nil
		case pdfBlockEmptyLine:
			continue
		}
		style := blockStyles[i]
		if style.Hidden || pdfStyleForcesPageBreakBefore(style) {
			return 0, nil
		}
		availableWidth := contentWidth
		if block.StripRootHorizontalMargins {
			availableWidth = rootlessContentWidth
		}
		if block.Kind == pdfBlockImage {
			img := doc.Images[block.ImageID]
			if img == nil {
				continue
			}
			maxImageHeight := contentHeight - style.SpaceBefore - style.PaddingTop - style.PaddingBottom - style.SpaceAfter
			if maxImageHeight <= 0 {
				return 0, nil
			}
			forceContentWidth := isVignetteBlock(block) || isHeadingImageBlock(block)
			widthReference := pdfBlockImageReferenceWidth(block, style, availableWidth, rootlessContentWidth, img, forceContentWidth)
			_, height, ok := fitPDFBlockImageSize(doc, img, blockContentWidth(availableWidth, style), maxImageHeight, widthReference, forceContentWidth)
			if !ok {
				return 0, nil
			}
			return style.SpaceBefore + style.PaddingTop + height + style.PaddingBottom, nil
		}
		if block.Kind == pdfBlockTable {
			table, err := layoutPDFTable(doc, styles, block, style, blockContentWidth(availableWidth, style))
			if err != nil {
				return 0, err
			}
			if len(table.Groups) == 0 {
				continue
			}
			return style.SpaceBefore + style.PaddingTop + table.Groups[0].Height + style.PaddingBottom, nil
		}
		text := strings.TrimSpace(block.Text)
		if text == "" && !inlineRunsRenderable(block.Runs) {
			continue
		}
		style.Paragraph.Hyphenator = doc.Hyphenator
		face, _, err := fontForStyle(doc.Fonts, style.Paragraph)
		if err != nil {
			return 0, err
		}
		runs := inlineRunsWithContext(block.Runs, inlineRunContextClassesForBlock(block))
		lines, err := layoutInlineParagraph(doc, doc.Fonts, styles, face, block.Text, runs, style.Paragraph, blockContentWidth(availableWidth, style))
		if err != nil {
			return 0, err
		}
		if len(lines) == 0 {
			continue
		}
		lineHeight := pdfEffectiveParagraphLineHeight(style.Paragraph)
		return style.SpaceBefore + style.PaddingTop + float64(min(minLines, len(lines)))*lineHeight + style.PaddingBottom, nil
	}
	return 0, nil
}

func pdfKeepWithNextLines(blocks []pdfTextBlock, styles []pdfBlockResolvedStyle, index int) int {
	lines := styles[index].KeepWithNextLines
	if styles[index].PageBreakAfterMode == pdfPageBreakAvoid {
		lines = max(lines, pdfSingleKeepLine)
	}
	next := pdfNextKeepBlockIndex(blocks, styles, index+1)
	if next >= 0 && styles[next].PageBreakBeforeMode == pdfPageBreakAvoid {
		lines = max(lines, pdfSingleKeepLine)
	}
	return lines
}

func pdfNextKeepBlockIndex(blocks []pdfTextBlock, styles []pdfBlockResolvedStyle, start int) int {
	for i := start; i < len(blocks); i++ {
		if blocks[i].Kind == pdfBlockPageBreak {
			return -1
		}
		if styles[i].Hidden || blocks[i].Kind == pdfBlockEmptyLine {
			continue
		}
		return i
	}
	return -1
}

func pdfStyleForcesPageBreakBefore(style pdfBlockResolvedStyle) bool {
	return style.PageBreakBefore || style.PageBreakBeforeMode == pdfPageBreakAlways
}

func pdfStyleForcesPageBreakAfter(style pdfBlockResolvedStyle) bool {
	return style.PageBreakAfter || style.PageBreakAfterMode == pdfPageBreakAlways
}

func countFittingLines(y float64, bottom float64, fontSize float64, lineHeight float64) int {
	count := 0
	for y-fontSize >= bottom {
		count++
		y -= lineHeight
	}
	return count
}
