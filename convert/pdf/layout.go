package pdf

import "strings"

func layoutPDFPages(doc skeletonDocument, _ *builtinFontFace) ([]pdfPage, map[pdfFontKey]map[uint16]shapedGlyph, error) {
	const margin = 24.0
	used := make(map[pdfFontKey]map[uint16]shapedGlyph)
	pages := make([]pdfPage, 0, 2)

	addPage := func() *pdfPage {
		pages = append(pages, pdfPage{})
		return &pages[len(pages)-1]
	}
	addLine := func(page *pdfPage, line pdfPageLine) {
		if len(line.Fragments) != 0 {
			page.Lines = append(page.Lines, line)
			for _, fragment := range line.Fragments {
				key := fragment.FontKey
				if key.Family == "" {
					key = pdfFontKey{Family: "serif"}
				}
				fontUsed := used[key]
				if fontUsed == nil {
					fontUsed = make(map[uint16]shapedGlyph)
					used[key] = fontUsed
				}
				for id, glyph := range fragment.Text.Used {
					fontUsed[id] = glyph
				}
			}
			return
		}
		if line.FontKey.Family == "" {
			line.FontKey = pdfFontKey{Family: "serif"}
		}
		page.Lines = append(page.Lines, line)
		fontUsed := used[line.FontKey]
		if fontUsed == nil {
			fontUsed = make(map[uint16]shapedGlyph)
			used[line.FontKey] = fontUsed
		}
		for id, glyph := range line.Text.Used {
			fontUsed[id] = glyph
		}
	}
	addAnchor := func(page *pdfPage, id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		for _, existing := range page.Anchors {
			if existing == id {
				return
			}
		}
		page.Anchors = append(page.Anchors, id)
	}
	addInlineImages := func(page *pdfPage, line paragraphLine, x float64, y float64) {
		currentX := x
		for i, fragment := range line.Fragments {
			if fragment.ImageID != "" && fragment.Width > 0 && fragment.ImageHeight > 0 {
				page.Images = append(page.Images, pdfPageImage{
					ImageID: fragment.ImageID,
					X:       currentX,
					Y:       y + fragment.BaselineShift,
					Width:   fragment.Width,
					Height:  fragment.ImageHeight,
				})
			}
			currentX += fragment.Width + line.ExtraCharSpacing*float64(max(len(fragment.Text.Glyphs)-1, 0))
			if i != len(line.Fragments)-1 {
				currentX += line.ExtraCharSpacing
			}
			if line.ExtraWordSpacing != 0 && i != len(line.Fragments)-1 && paragraphFragmentEndsWithSpace(fragment) {
				currentX += line.ExtraWordSpacing
			}
		}
	}
	addBlockDecoration := func(page *pdfPage, style pdfBlockResolvedStyle, x, topY, width, bottomY float64) {
		if page == nil || width <= 0 || topY <= bottomY {
			return
		}
		height := topY - bottomY
		if style.HasBackground {
			page.Backgrounds = append(page.Backgrounds, pdfPageRect{
				X:      x,
				Y:      bottomY,
				Width:  width,
				Height: height,
				Color:  style.BackgroundColor,
			})
		}
		if style.HasBorder && style.BorderWidth > 0 {
			page.Borders = append(page.Borders, pdfPageBorder{
				X:         x,
				Y:         bottomY,
				Width:     width,
				Height:    height,
				LineWidth: style.BorderWidth,
				Color:     style.BorderColor,
			})
		}
	}

	if cover := doc.Images[doc.CoverID]; cover != nil {
		if rect, ok := fitPDFImageInBox(doc, cover, 0, 0, doc.PageWidth, doc.PageHeight); ok {
			coverPage := addPage()
			addAnchor(coverPage, doc.CoverID)
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
	blockStyles := styles.collapsedBlockStyles(doc.Blocks)
	contentLeft, contentRight, contentTop, contentBottom := pdfPageContentMargins(doc, styles, margin)
	rootlessContentLeft, rootlessContentRight, _, _ := pdfPageContentMarginsWithoutRootHorizontal(doc, styles, margin)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	rootlessContentWidth := max(doc.PageWidth-rootlessContentLeft-rootlessContentRight, 12)
	page := addPage()
	top := doc.PageHeight - contentTop
	bottom := contentBottom
	y := top
	pageHasText := false
	previousRenderedImage := false
	titleGroup := pdfTitleVignetteContentGroup{}
	newTextPage := func() {
		titleGroup.reset()
		page = addPage()
		y = top
		pageHasText = false
		previousRenderedImage = false
	}

	for blockIndex, block := range doc.Blocks {
		if block.Kind == pdfBlockPageBreak {
			if pageHasText {
				newTextPage()
			}
			addAnchor(page, block.ID)
			continue
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
			tableWidth := blockContentWidth(blockWidthLimit, style)
			table, err := layoutPDFTable(doc, styles, block, style, tableWidth)
			if err != nil {
				return nil, nil, err
			}
			if table.Width <= 0 || len(table.Groups) == 0 {
				continue
			}
			needed := style.SpaceBefore + style.PaddingTop + table.Height + style.PaddingBottom + style.SpaceAfter
			if style.KeepTogether && pageHasText && y-needed < bottom && needed <= top-bottom {
				newTextPage()
			}
			addAnchor(page, block.ID)
			y -= style.SpaceBefore + style.PaddingTop
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
					addBlockDecoration(page, cell.Style, cellX, cellTop, cell.Width, cellBottom)
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
						addInlineImages(page, line, x, lineY)
						addLinkAnnotations(page, linkBlock, line, lineSearchStart, x, lineY, cell.Style.Paragraph.FontSize)
						lineSearchStart = nextLineSearchStart(cell.Text, line, lineSearchStart)
						addLine(page, pdfPageLine{
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
						})
						lineY -= cell.Style.Paragraph.LineHeight
					}
				}
				y -= group.Height
				pageHasText = true
				previousRenderedImage = false
			}
			y -= style.PaddingBottom + style.SpaceAfter
			if pdfStyleForcesPageBreakAfter(style) && pageHasText {
				newTextPage()
			}
			continue
		}

		if block.Kind == pdfBlockImage {
			backgroundX := blockLeft + style.MarginLeft
			backgroundWidth := blockBoxWidth(blockWidthLimit, style)
			blockWidth := blockContentWidth(blockWidthLimit, style)
			img := doc.Images[block.ImageID]
			if img == nil {
				continue
			}
			maxImageHeight := top - bottom - style.SpaceBefore - style.PaddingTop - style.PaddingBottom - style.SpaceAfter
			if maxImageHeight <= 0 {
				continue
			}
			forceContentWidth := isVignetteBlock(block) || isHeadingImageBlock(block)
			widthReference := pdfBlockImageWidthReference(block, style, blockWidthLimit, rootlessContentWidth, img, forceContentWidth)
			width, height, ok := fitPDFBlockImageSizeWithReference(doc, img, blockWidth, maxImageHeight, widthReference, forceContentWidth)
			if !ok {
				continue
			}
			needed := style.SpaceBefore + style.PaddingTop + height + style.PaddingBottom + style.SpaceAfter
			if pageHasText {
				keepWithNext, err := nextBlockKeepHeight(doc, blockStyles, blockIndex+1, contentWidth, rootlessContentWidth, top-bottom, pdfKeepWithNextLines(doc.Blocks, blockStyles, blockIndex))
				if err != nil {
					return nil, nil, err
				}
				if keepWithNext > 0 && y-needed-keepWithNext < bottom {
					newTextPage()
				} else if y-needed < bottom {
					newTextPage()
				}
			}
			y -= style.SpaceBefore
			y -= style.PaddingTop
			if y-height < bottom {
				newTextPage()
				y -= style.SpaceBefore
				y -= style.PaddingTop
			}
			addAnchor(page, block.ID)
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
			addBlockDecoration(page, style, backgroundX, backgroundTop, backgroundWidth, backgroundBottom)
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
		lines, err := layoutInlineParagraph(doc, doc.Fonts, styles, face, block.Text, runs, style.Paragraph, blockWidth)
		if err != nil {
			return nil, nil, err
		}
		if len(lines) == 0 {
			continue
		}

		needed := style.SpaceBefore + style.PaddingTop + float64(len(lines))*style.Paragraph.LineHeight + style.PaddingBottom
		if style.KeepTogether && pageHasText && y-needed < bottom {
			newTextPage()
		}
		if keepLines := pdfKeepWithNextLines(doc.Blocks, blockStyles, blockIndex); keepLines > 0 && pageHasText {
			keepWithNext, err := nextBlockKeepHeight(doc, blockStyles, blockIndex+1, contentWidth, rootlessContentWidth, top-bottom, keepLines)
			if err != nil {
				return nil, nil, err
			}
			if keepWithNext > 0 && y-needed-style.SpaceAfter-keepWithNext < bottom {
				newTextPage()
			}
		}
		if !style.KeepTogether && pageHasText {
			linesFit := countFittingLines(y-style.SpaceBefore-style.PaddingTop, bottom, style.Paragraph.FontSize, style.Paragraph.LineHeight)
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
		addAnchor(page, block.ID)
		y -= style.SpaceBefore
		backgroundX := blockLeft + style.MarginLeft
		backgroundWidth := blockBoxWidth(blockWidthLimit, style)
		y -= style.PaddingTop
		fragmentPage := page
		fragmentTop := y + style.PaddingTop
		lineSearchStart := 0
		for lineIndex, line := range lines {
			if !pageHasText || previousRenderedImage {
				y -= style.Paragraph.FontSize
				previousRenderedImage = false
			}
			if y-style.Paragraph.FontSize < bottom {
				if pageHasText {
					addBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, y)
				}
				newTextPage()
				fragmentPage = page
				fragmentTop = y + style.Paragraph.FontSize
				y -= style.Paragraph.FontSize
			}
			remainingAfterLine := len(lines) - lineIndex - 1
			if remainingAfterLine > 0 && remainingAfterLine < style.Widows && y-style.Paragraph.LineHeight-style.Paragraph.FontSize < bottom {
				addBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, y)
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
			addInlineImages(page, line, x, y)
			addLinkAnnotations(page, block, line, lineSearchStart, x, y, style.Paragraph.FontSize)
			lineSearchStart = nextLineSearchStart(block.Text, line, lineSearchStart)
			addLine(page, pdfPageLine{
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
			})
			y -= style.Paragraph.LineHeight
			pageHasText = true
			previousRenderedImage = false
		}
		backgroundBottom := y - style.PaddingBottom
		addBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, backgroundBottom)
		y -= style.PaddingBottom + style.SpaceAfter
		if pdfStyleForcesPageBreakAfter(style) && pageHasText {
			newTextPage()
		}
	}

	if len(pages[len(pages)-1].Lines) == 0 && len(pages[len(pages)-1].Images) == 0 {
		pages = pages[:len(pages)-1]
	}
	return pages, used, nil
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
		face, err := fontForKey(fonts, line.FontKey)
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
		face, err := fontForKey(fonts, fragment.FontKey)
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

func pdfPageContentMargins(doc skeletonDocument, styles *pdfStyleResolver, baseMargin float64) (float64, float64, float64, float64) {
	return pdfPageContentMarginsWithOptions(doc, styles, baseMargin, false)
}

func pdfPageContentMarginsWithoutRootHorizontal(doc skeletonDocument, styles *pdfStyleResolver, baseMargin float64) (float64, float64, float64, float64) {
	return pdfPageContentMarginsWithOptions(doc, styles, baseMargin, true)
}

func pdfPageContentMarginsWithOptions(doc skeletonDocument, styles *pdfStyleResolver, baseMargin float64, stripRootHorizontal bool) (float64, float64, float64, float64) {
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

func nextBlockKeepHeight(doc skeletonDocument, blockStyles []pdfBlockResolvedStyle, start int, contentWidth float64, rootlessContentWidth float64, contentHeight float64, minLines int) (float64, error) {
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
			widthReference := pdfBlockImageWidthReference(block, style, availableWidth, rootlessContentWidth, img, forceContentWidth)
			_, height, ok := fitPDFBlockImageSizeWithReference(doc, img, blockContentWidth(availableWidth, style), maxImageHeight, widthReference, forceContentWidth)
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
		return style.SpaceBefore + style.PaddingTop + float64(min(minLines, len(lines)))*style.Paragraph.LineHeight + style.PaddingBottom, nil
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
