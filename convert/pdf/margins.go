package pdf

import "strings"

func (r *pdfStyleResolver) collapsedBlockStyles(blocks []pdfTextBlock) []pdfBlockResolvedStyle {
	if r == nil {
		r = newPDFStyleResolver(nil, nil)
	}
	resolved := make([]pdfBlockResolvedStyle, len(blocks))
	for i, block := range blocks {
		resolved[i] = r.styleForBlock(block)
	}

	r.adjustContainerMargins(blocks, resolved)

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

func (r *pdfStyleResolver) adjustContainerMargins(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle) {
	for i, block := range blocks {
		if resolved[i].Hidden || block.Kind == pdfBlockPageBreak {
			continue
		}
		class := pdfContainerMarginClass(block)
		if class == "" {
			continue
		}
		base := r.styleForBlock(pdfTextBlock{
			Kind:         block.Kind,
			ID:           block.ID,
			Text:         block.Text,
			Runs:         block.Runs,
			Depth:        block.Depth,
			StyleName:    block.StyleName,
			StyleClasses: pdfRemoveContainerControlClasses(block.StyleClasses, class),
			ImageID:      block.ImageID,
			Links:        block.Links,
		})
		if pdfAdjacentBlockHasContainerClass(blocks, resolved, i, -1, class) {
			resolved[i].SpaceBefore = base.SpaceBefore
			if pdfInlineTitleTextBlock(block) {
				resolved[i].SpaceBefore = 0
			}
			resolved[i].PageBreakBefore = base.PageBreakBefore
			resolved[i].PageBreakBeforeMode = base.PageBreakBeforeMode
			resolved[i].HasPageBreakBefore = base.HasPageBreakBefore
		}
		if pdfAdjacentBlockHasContainerClass(blocks, resolved, i, 1, class) {
			resolved[i].SpaceAfter = base.SpaceAfter
			if pdfInlineTitleTextBlock(block) {
				resolved[i].SpaceAfter = 0
			}
			resolved[i].KeepWithNextLines = base.KeepWithNextLines
			resolved[i].PageBreakAfter = base.PageBreakAfter
			resolved[i].PageBreakAfterMode = base.PageBreakAfterMode
			resolved[i].HasPageBreakAfter = base.HasPageBreakAfter
		}
	}
}

func pdfAdjacentBlockHasContainerClass(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle, index int, direction int, class string) bool {
	for i := index + direction; i >= 0 && i < len(blocks); i += direction {
		if blocks[i].Kind == pdfBlockPageBreak {
			return false
		}
		if resolved[i].Hidden {
			continue
		}
		return blockHasStyleClass(blocks[i], class)
	}
	return false
}

func pdfContainerMarginClass(block pdfTextBlock) string {
	for _, class := range []string{pdfStyleBodyTitle, pdfStyleChapterTitle, pdfStyleSectionTitle, pdfStyleAnnotation, pdfStyleEpigraph, pdfStyleCite, pdfStyleStanza} {
		if blockHasStyleClass(block, class) {
			return class
		}
	}
	return ""
}

func pdfRemoveContainerControlClasses(classes string, container string) string {
	out := make([]string, 0, len(classes))
	for _, class := range strings.Fields(classes) {
		if class == container || pdfContainerCompanionClass(container, class) {
			continue
		}
		out = append(out, class)
	}
	return strings.Join(out, " ")
}

func pdfContainerCompanionClass(container string, class string) bool {
	switch container {
	case pdfStyleSectionTitle:
		return strings.HasPrefix(class, pdfStyleSectionTitle+"-h")
	default:
		return false
	}
}

func pdfInlineTitleTextBlock(block pdfTextBlock) bool {
	if block.Kind != pdfBlockHeading || blockHasStyleClass(block, pdfStyleTitleAfterImage) {
		return false
	}
	for _, base := range []string{pdfStyleBodyTitleHeader, pdfStyleChapterTitleHeader, pdfStyleSectionTitleHeader, pdfStyleTOCTitle} {
		if blockHasStyleClass(block, base+"-first") || blockHasStyleClass(block, base+"-next") {
			return true
		}
	}
	return false
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
	case pdfBlockPageBreak, pdfBlockEmptyLine, pdfBlockTable:
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
