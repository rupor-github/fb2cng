package pdf

func (r *pdfStyleResolver) collapsedBlockStyles(blocks []pdfTextBlock) []pdfBlockResolvedStyle {
	if r == nil {
		r = newPDFStyleResolver(nil, nil)
	}
	resolved := make([]pdfBlockResolvedStyle, len(blocks))
	for i, block := range blocks {
		resolved[i] = r.styleForBlock(block)
	}

	previous := -1
	for i, block := range blocks {
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
