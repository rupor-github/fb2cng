package pdf

import (
	"strings"

	"fbc/fb2"
)

func (r *pdfStyleResolver) collapsedBlockStylesWithImages(blocks []pdfTextBlock, images fb2.BookImages) []pdfBlockResolvedStyle {
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

	adjustKP3FullBlockImageVerticalMargins(blocks, resolved, images)
	adjustKP3TitleBlockVerticalMargins(blocks, resolved)

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
			if pdfPreservesPreviousMarginBeforeImage(blocks, resolved, previous, i) {
				previous = i
				continue
			}
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

func adjustKP3FullBlockImageVerticalMargins(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle, images fb2.BookImages) {
	// KFX injects fixed 2.6lh margins for ordinary full-width block images
	// preceded by an empty-line, unless the image is followed directly by a
	// caption-like paragraph/subtitle. Mirror that KP3 quirk for native PDF.
	for i := range blocks {
		if resolved[i].Hidden || !pdfBlockImageNeedsKP3FullMargins(blocks, resolved, images, i) {
			continue
		}
		margin := pdfKP3FullBlockImageMargin(resolved[i])
		resolved[i].SpaceBefore = max(resolved[i].SpaceBefore, margin)
		resolved[i].SpaceAfter = max(resolved[i].SpaceAfter, margin)
	}
}

func pdfBlockImageNeedsKP3FullMargins(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle, images fb2.BookImages, index int) bool {
	if index <= 0 || index >= len(blocks) || images == nil {
		return false
	}
	block := blocks[index]
	if block.Kind != pdfBlockImage || isVignetteBlock(block) || isHeadingImageBlock(block) {
		return false
	}
	img := images[block.ImageID]
	if img == nil || !pdfBlockImageUsesFullWidthPercent(img) {
		return false
	}
	if blocks[index-1].Kind != pdfBlockEmptyLine {
		return false
	}
	previous := pdfPreviousContentBlock(blocks, resolved, index-1)
	if previous >= 0 && blocks[previous].Kind == pdfBlockImage {
		return false
	}
	if index+1 < len(blocks) && (blocks[index+1].Kind == pdfBlockParagraph || blocks[index+1].Kind == pdfBlockSubtitle) {
		return false
	}
	return true
}

func pdfKP3FullBlockImageMargin(style pdfBlockResolvedStyle) float64 {
	lineHeight := style.Paragraph.LineHeight
	if lineHeight <= 0 {
		lineHeight = pdfBaseLineHeight
	}
	return lineHeight * pdfFullBlockImageMarginLH
}

func adjustKP3TitleBlockVerticalMargins(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle) {
	for i, block := range blocks {
		if resolved[i].Hidden {
			continue
		}
		if pdfContainerMarginClass(block) != "" && isTitleBottomVignetteBlock(block) {
			resolved[i].SpaceBefore = max(resolved[i].SpaceBefore, pdfTitleVignetteMarginTop)
		}
		if block.Kind == pdfBlockSubtitle {
			previous := pdfPreviousContentBlock(blocks, resolved, i-1)
			if previous >= 0 && isTitleBottomVignetteBlock(blocks[previous]) {
				resolved[i].SpaceBefore = max(resolved[i].SpaceBefore, pdfTitleFollowingSubtitleSpaceBefore)
			}
		}
	}
}

func pdfPreviousContentBlock(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle, start int) int {
	for i := start; i >= 0; i-- {
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

func pdfPreservesPreviousMarginBeforeImage(blocks []pdfTextBlock, resolved []pdfBlockResolvedStyle, previous int, current int) bool {
	if previous < 0 || current <= previous || current >= len(blocks) || blocks[current].Kind != pdfBlockImage {
		return false
	}
	if blocks[previous].Kind == pdfBlockImage || resolved[previous].SpaceAfter <= 0 || resolved[current].SpaceBefore != 0 {
		return false
	}
	for i := previous + 1; i < current; i++ {
		if blocks[i].Kind == pdfBlockEmptyLine && resolved[i].Hidden {
			return true
		}
		if !resolved[i].Hidden {
			return false
		}
	}
	return false
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
