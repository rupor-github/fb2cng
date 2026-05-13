package pdf

func (r *pdfStyleResolver) collapsedBlockStyles(blocks []pdfTextBlock) []pdfBlockResolvedStyle {
	if r == nil {
		r = newPDFStyleResolver(nil, nil)
	}
	resolved := make([]pdfBlockResolvedStyle, len(blocks))
	for i, block := range blocks {
		resolved[i] = r.styleForBlock(block)
	}

	previousContent := -1
	for i, block := range blocks {
		if resolved[i].Hidden {
			continue
		}
		if block.Kind != pdfBlockEmptyLine {
			if block.Kind != pdfBlockPageBreak {
				previousContent = i
			}
			continue
		}

		resolved[i].Hidden = true
		margin := pdfEmptyLineMargin(resolved[i])
		if previousContent >= 0 && blocks[previousContent].Kind != pdfBlockImage {
			resolved[previousContent].SpaceAfter = 0
		}
		nextContent := pdfNextContentBlock(blocks, resolved, i+1)
		if nextContent < 0 || margin <= 0 {
			continue
		}
		if previousContent >= 0 && blocks[nextContent].Kind == pdfBlockImage && blocks[previousContent].Kind != pdfBlockImage {
			resolved[previousContent].SpaceAfter = max(resolved[previousContent].SpaceAfter, margin)
			continue
		}
		resolved[nextContent].SpaceBefore = max(resolved[nextContent].SpaceBefore, margin)
	}

	previous := -1
	for i, block := range blocks {
		if resolved[i].Hidden {
			continue
		}
		if !pdfBlockParticipatesInMarginCollapse(block) {
			previous = -1
			continue
		}
		if previous >= 0 {
			previousMargin := resolved[previous].SpaceAfter
			currentMargin := resolved[i].SpaceBefore
			collapsed := pdfCollapseVerticalMargins(previousMargin, currentMargin)
			resolved[previous].SpaceAfter = 0
			resolved[i].SpaceBefore = collapsed
			r.tracer.traceMarginCollapse(previous, i, blocks[previous], block, previousMargin, currentMargin, collapsed)
		}
		previous = i
	}
	return resolved
}

func pdfNextContentBlock(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle, start int) int {
	for i := start; i < len(blocks); i++ {
		if resolved[i].Hidden || blocks[i].Kind == pdfBlockEmptyLine {
			continue
		}
		if blocks[i].Kind == pdfBlockPageBreak {
			return -1
		}
		return i
	}
	return -1
}

func pdfEmptyLineMargin(style pdfBlockResolvedStyle) float64 {
	fontSize := style.Paragraph.FontSize
	if fontSize <= 0 {
		fontSize = pdfBaseFontSize
	}
	lineHeight := style.Paragraph.LineHeight
	if lineHeight <= 0 {
		lineHeight = fontSize * 1.2
	}
	space := style.SpaceBefore
	if space == 0 {
		space = fontSize
	}
	return max(space/fontSize, 0) * lineHeight / 2
}

func pdfBlockParticipatesInMarginCollapse(block pdfTextBlock) bool {
	switch block.Kind {
	case pdfBlockPageBreak, pdfBlockEmptyLine:
		return false
	default:
		return true
	}
}

func pdfCollapseVerticalMargins(a, b float64) float64 {
	switch {
	case a >= 0 && b >= 0:
		return max(a, b)
	case a <= 0 && b <= 0:
		return min(a, b)
	default:
		return a + b
	}
}
