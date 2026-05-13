package pdf

import (
	"fmt"
	"strings"
)

func layoutPDFPages(doc skeletonDocument, titleFace *builtinFontFace) ([]pdfPage, map[pdfFontKey]map[uint16]shapedGlyph, error) {
	const margin = 24.0
	contentWidth := max(doc.PageWidth-margin*2, 12)
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

	titlePage := addPage()
	titleText := strings.TrimSpace(doc.Title)
	if titleText == "" {
		titleText = "Untitled"
	}
	authorText := strings.TrimSpace(doc.Author)
	if authorText == "" {
		authorText = "fbc"
	}
	titleKey := pdfFontKey{Family: "sans-serif"}
	title, err := shapeText(titleFace, titleText)
	if err != nil {
		return nil, nil, fmt.Errorf("shape title: %w", err)
	}
	addLine(titlePage, pdfPageLine{
		X:        margin,
		Y:        max(doc.PageHeight-54.0, margin),
		FontSize: 14,
		FontKey:  titleKey,
		Text:     title,
	})
	authorLines, err := wrapText(titleFace, authorText, 9, contentWidth)
	if err != nil {
		return nil, nil, fmt.Errorf("shape author: %w", err)
	}
	authorY := max(doc.PageHeight-74.0, margin)
	for i, line := range authorLines {
		y := authorY - float64(i)*11.0
		if y < margin {
			break
		}
		addLine(titlePage, pdfPageLine{
			X:        margin,
			Y:        y,
			FontSize: 9,
			FontKey:  titleKey,
			Text:     line,
		})
	}

	if len(doc.Blocks) == 0 {
		return pages, used, nil
	}

	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	blockStyles := styles.collapsedBlockStyles(doc.Blocks)
	page := addPage()
	top := doc.PageHeight - margin
	bottom := margin
	y := top
	pageHasText := false
	newTextPage := func() {
		page = addPage()
		y = top
		pageHasText = false
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
		if style.PageBreakBefore && pageHasText {
			newTextPage()
		}

		if block.Kind == pdfBlockImage {
			backgroundX := margin + style.MarginLeft
			backgroundWidth := blockBoxWidth(contentWidth, style)
			blockWidth := blockContentWidth(contentWidth, style)
			img := doc.Images[block.ImageID]
			if img == nil {
				continue
			}
			width, height, ok := fitPDFBlockImageSize(img, blockWidth, top-bottom, isVignetteBlock(block) || isHeadingImageBlock(block))
			if !ok {
				continue
			}
			needed := style.SpaceBefore + style.PaddingTop + height + style.PaddingBottom + style.SpaceAfter
			if pageHasText && y-needed < bottom {
				newTextPage()
			}
			addAnchor(page, block.ID)
			if pageHasText {
				y -= style.SpaceBefore
			}
			y -= style.PaddingTop
			if y-height < bottom {
				newTextPage()
			}
			backgroundTop := y + style.PaddingTop
			y -= height
			page.Images = append(page.Images, pdfPageImage{
				ImageID: block.ImageID,
				X:       margin + style.MarginLeft + style.PaddingLeft + max((blockWidth-width)/2, 0),
				Y:       y,
				Width:   width,
				Height:  height,
			})
			pageHasText = true
			backgroundBottom := y - style.PaddingBottom
			addBlockDecoration(page, style, backgroundX, backgroundTop, backgroundWidth, backgroundBottom)
			y -= style.PaddingBottom + style.SpaceAfter
			if style.PageBreakAfter && pageHasText {
				newTextPage()
			}
			continue
		}

		style.Paragraph.Hyphenator = doc.Hyphenator
		blockWidth := blockContentWidth(contentWidth, style)
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
		lines, err := layoutInlineParagraph(doc, doc.Fonts, styles, face, block.Text, block.Runs, style.Paragraph, blockWidth)
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
		if style.KeepWithNextLines > 0 && pageHasText {
			keepWithNext, err := nextBlockKeepHeight(doc.Blocks[blockIndex+1:], doc.Hyphenator, doc.Fonts, styles, contentWidth, style.KeepWithNextLines)
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
		if pageHasText {
			y -= style.SpaceBefore
		}
		backgroundX := margin + style.MarginLeft
		backgroundWidth := blockBoxWidth(contentWidth, style)
		y -= style.PaddingTop
		fragmentPage := page
		fragmentTop := y + style.PaddingTop
		lineSearchStart := 0
		for lineIndex, line := range lines {
			if y-style.Paragraph.FontSize < bottom {
				addBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, y)
				newTextPage()
				fragmentPage = page
				fragmentTop = y
			}
			remainingAfterLine := len(lines) - lineIndex - 1
			if remainingAfterLine > 0 && remainingAfterLine < style.Widows && y-style.Paragraph.LineHeight-style.Paragraph.FontSize < bottom {
				addBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, y)
				newTextPage()
				fragmentPage = page
				fragmentTop = y
			}
			x := margin + style.MarginLeft + style.PaddingLeft + line.Indent
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
		}
		backgroundBottom := y - style.PaddingBottom
		addBlockDecoration(fragmentPage, style, backgroundX, fragmentTop, backgroundWidth, backgroundBottom)
		y -= style.PaddingBottom + style.SpaceAfter
		if style.PageBreakAfter && pageHasText {
			newTextPage()
		}
	}

	if len(pages[len(pages)-1].Lines) == 0 && len(pages[len(pages)-1].Images) == 0 {
		pages = pages[:len(pages)-1]
	}
	return pages, used, nil
}

func nextBlockKeepHeight(blocks []pdfTextBlock, hyphenator paragraphHyphenator, fonts *pdfFontRegistry, styles *pdfStyleResolver, contentWidth float64, minLines int) (float64, error) {
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	for _, block := range blocks {
		switch block.Kind {
		case pdfBlockPageBreak:
			return 0, nil
		case pdfBlockEmptyLine:
			continue
		}
		style := styles.styleForBlock(block)
		if style.Hidden || style.PageBreakBefore {
			return 0, nil
		}
		text := strings.TrimSpace(block.Text)
		if text == "" && !inlineRunsRenderable(block.Runs) {
			continue
		}
		style.Paragraph.Hyphenator = hyphenator
		face, _, err := fontForStyle(fonts, style.Paragraph)
		if err != nil {
			return 0, err
		}
		lines, err := layoutInlineParagraph(skeletonDocument{Images: nil}, fonts, styles, face, block.Text, block.Runs, style.Paragraph, blockContentWidth(contentWidth, style))
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

func countFittingLines(y float64, bottom float64, fontSize float64, lineHeight float64) int {
	count := 0
	for y-fontSize >= bottom {
		count++
		y -= lineHeight
	}
	return count
}
