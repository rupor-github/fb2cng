package pdf

import "strings"

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

func pdfBlockImageOverflowsBottom(candidateBottom float64, pageBottom float64) bool {
	return candidateBottom < pageBottom-pdfBlockImageBottomFitOverflow
}

func nextBlockKeepHeight(
	doc pdfDocumentSpec,
	blockStyles []pdfBlockResolvedStyle,
	start int,
	contentWidth float64,
	rootlessContentWidth float64,
	contentHeight float64,
	minLines int,
) (float64, error) {
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
			_, height, ok := fitPDFBlockImageSize(
				doc,
				img,
				blockContentWidth(availableWidth, style),
				maxImageHeight,
				widthReference,
				forceContentWidth,
			)
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
		lines, err := layoutInlineWithShape(
			doc,
			doc.Fonts,
			styles,
			face,
			block.Text,
			runs,
			style.Paragraph,
			blockContentWidth(availableWidth, style),
			paragraphLineShape{},
		)
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
