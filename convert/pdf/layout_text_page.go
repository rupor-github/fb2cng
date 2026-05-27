package pdf

import "strings"

type pdfShapeTextBlockFunc func() ([]paragraphLine, pdfDropcapLayout, bool, error)

type pdfTextBlockRender struct {
	block   pdfTextBlock
	style   pdfBlockResolvedStyle
	lines   []paragraphLine
	dropcap pdfDropcapLayout

	fontKey   pdfFontKey
	dropcapOK bool

	lineHeight      float64
	blockLeft       float64
	blockWidth      float64
	blockWidthLimit float64
	backgroundX     float64
	backgroundWidth float64

	blockSpaceBefore func() float64
	fragmentPage     *pdfPage
	fragmentTop      float64
	lineSearchStart  int
}

func (l *pdfPageLayout) layoutTextBlock(blockIndex int, block pdfTextBlock, style pdfBlockResolvedStyle, blockLeft, blockWidthLimit float64) error {
	if l.titleGroup.active && !isTitleHeaderBlock(block) {
		l.titleGroup.reset()
	}
	style.Paragraph.Hyphenator = l.doc.Hyphenator
	blockWidth := blockContentWidth(blockWidthLimit, style)
	if block.Kind == pdfBlockEmptyLine {
		return nil
	}
	text := strings.TrimSpace(block.Text)
	if text == "" && !inlineRunsRenderable(block.Runs) {
		return nil
	}
	face, fontKey, err := fontForStyle(l.doc.Fonts, style.Paragraph)
	if err != nil {
		return err
	}
	runs := contextInlineRuns(block.Runs, inlineRunContextClassesForBlock(block))
	lineHeight := pdfEffectiveParagraphLineHeight(style.Paragraph)
	blockSpaceBefore := func() float64 { return pdfEffectiveBlockSpaceBefore(style, l.pageHasText, l.y, l.top) }
	firstBaselineY := func() float64 { return l.textBlockFirstBaselineY(style, blockSpaceBefore) }
	shapeTextBlock := func() ([]paragraphLine, pdfDropcapLayout, bool, error) {
		return l.shapeTextBlock(block, style, face, runs, blockWidth, lineHeight, firstBaselineY)
	}
	lines, dropcap, dropcapOK, err := shapeTextBlock()
	if err != nil {
		return err
	}
	if len(lines) == 0 && !dropcapOK {
		return nil
	}

	if err := l.paginateTextBlock(blockIndex, style, lines, dropcap, dropcapOK, lineHeight, blockSpaceBefore, firstBaselineY); err != nil {
		return err
	}
	lines, dropcap, dropcapOK, err = shapeTextBlock()
	if err != nil {
		return err
	}
	if len(lines) == 0 && !dropcapOK {
		return nil
	}
	lines, dropcap, dropcapOK, err = l.reserveTextDropcapFootnotes(lines, dropcap, dropcapOK, shapeTextBlock)
	if err != nil {
		return err
	}
	if len(lines) == 0 && !dropcapOK {
		return nil
	}
	if dropcapOK {
		pdfTraceResolvedDropcap(l.styles, blockIndex, block, dropcap, style.Paragraph)
	}
	render := pdfTextBlockRender{
		block:            block,
		style:            style,
		lines:            lines,
		dropcap:          dropcap,
		fontKey:          fontKey,
		dropcapOK:        dropcapOK,
		lineHeight:       lineHeight,
		blockLeft:        blockLeft,
		blockWidth:       blockWidth,
		blockWidthLimit:  blockWidthLimit,
		blockSpaceBefore: blockSpaceBefore,
	}
	return l.renderTextBlock(&render)
}

func (l *pdfPageLayout) reserveTextDropcapFootnotes(
	lines []paragraphLine,
	dropcap pdfDropcapLayout,
	dropcapOK bool,
	shapeTextBlock pdfShapeTextBlockFunc,
) ([]paragraphLine, pdfDropcapLayout, bool, error) {
	if !dropcapOK || !l.pageHasText || !l.printedFootnoteReserve.Enabled() {
		return lines, dropcap, dropcapOK, nil
	}
	var dropcapFootnoteRefs []pdfPrintedFootnoteRef
	for _, line := range lines {
		dropcapFootnoteRefs = append(dropcapFootnoteRefs, l.printedFootnoteReserve.LineRefs(line)...)
	}
	if len(dropcapFootnoteRefs) == 0 {
		return lines, dropcap, dropcapOK, nil
	}
	reserve, err := l.printedFootnoteReserve.ReserveWithAdditionalRefs(dropcapFootnoteRefs)
	if err != nil {
		return nil, pdfDropcapLayout{}, false, err
	}
	reservedBottom := pdfReservedContentBottom(l.contentBottom, l.top, reserve)
	if dropcap.BottomY >= reservedBottom {
		return lines, dropcap, dropcapOK, nil
	}
	l.newTextPage()
	return shapeTextBlock()
}

func (l *pdfPageLayout) renderTextBlock(r *pdfTextBlockRender) error {
	addPDFPageAnchor(l.page, r.block.ID)
	l.y -= r.blockSpaceBefore()
	r.backgroundX = r.blockLeft + r.style.MarginLeft
	r.backgroundWidth = blockBoxWidth(r.blockWidthLimit, r.style)
	l.y -= r.style.PaddingTop
	r.fragmentPage = l.page
	r.fragmentTop = l.y + r.style.PaddingTop
	if r.dropcapOK {
		r.lineSearchStart = l.renderTextDropcap(r.block, r.style, r.blockLeft, r.dropcap)
	}
	if err := l.renderTextLines(r); err != nil {
		return err
	}
	if r.dropcapOK && len(r.lines) == 0 {
		l.pageHasText = true
		l.previousRenderedImage = false
	}
	backgroundBottom := l.y - r.style.PaddingBottom
	addPDFBlockDecoration(r.fragmentPage, r.style, r.backgroundX, r.fragmentTop, r.backgroundWidth, backgroundBottom)
	l.y -= r.style.PaddingBottom + r.style.SpaceAfter
	if l.activeDropcap != nil && l.activeDropcap.Page != l.page {
		l.activeDropcap = nil
	}
	if pdfStyleForcesPageBreakAfter(r.style) && l.pageHasText {
		l.newTextPage()
	}
	return nil
}

func (l *pdfPageLayout) prepareTextLinePage(r *pdfTextBlockRender, line paragraphLine, lineIndex int) error {
	if !l.pageHasText || l.previousRenderedImage {
		l.y -= r.style.Paragraph.FontSize
		l.previousRenderedImage = false
	}
	if l.printedFootnoteReserve.Enabled() {
		lineRefs := l.printedFootnoteReserve.LineRefs(line)
		if len(lineRefs) > 0 {
			reserve, err := l.printedFootnoteReserve.ReserveWithAdditionalRefs(lineRefs)
			if err != nil {
				return err
			}
			reservedBottom := pdfReservedContentBottom(l.contentBottom, l.top, reserve)
			if l.pageHasText && l.y-r.style.Paragraph.FontSize < reservedBottom {
				addPDFBlockDecoration(r.fragmentPage, r.style, r.backgroundX, r.fragmentTop, r.backgroundWidth, l.y)
				l.newTextPage()
				r.fragmentPage = l.page
				r.fragmentTop = l.y + r.style.Paragraph.FontSize
				l.y -= r.style.Paragraph.FontSize
				reserve, err = l.printedFootnoteReserve.ReserveWithAdditionalRefs(lineRefs)
				if err != nil {
					return err
				}
				reservedBottom = pdfReservedContentBottom(l.contentBottom, l.top, reserve)
			}
			l.printedFootnoteReserve.CommitAdditionalRefs(lineRefs, reserve)
			l.bottom = max(l.bottom, reservedBottom)
		}
	}
	if l.y-r.style.Paragraph.FontSize < l.bottom {
		if l.pageHasText {
			addPDFBlockDecoration(r.fragmentPage, r.style, r.backgroundX, r.fragmentTop, r.backgroundWidth, l.y)
		}
		l.newTextPage()
		r.fragmentPage = l.page
		r.fragmentTop = l.y + r.style.Paragraph.FontSize
		l.y -= r.style.Paragraph.FontSize
	}
	remainingAfterLine := len(r.lines) - lineIndex - 1
	if remainingAfterLine > 0 && remainingAfterLine < r.style.Widows && l.y-r.lineHeight-r.style.Paragraph.FontSize < l.bottom {
		addPDFBlockDecoration(r.fragmentPage, r.style, r.backgroundX, r.fragmentTop, r.backgroundWidth, l.y)
		l.newTextPage()
		r.fragmentPage = l.page
		r.fragmentTop = l.y
	}
	return nil
}

func (l *pdfPageLayout) renderTextLines(r *pdfTextBlockRender) error {
	for lineIndex, line := range r.lines {
		if err := l.prepareTextLinePage(r, line, lineIndex); err != nil {
			return err
		}
		x := r.blockLeft + r.style.MarginLeft + r.style.PaddingLeft + line.Indent
		available := r.blockWidth - line.Indent
		switch r.style.Paragraph.Align {
		case textAlignCenter:
			x += max((available-line.Width)/2, 0)
		case textAlignRight:
			x += max(available-line.Width, 0)
		}
		pageLine := pdfPageLine{
			X:                x,
			Y:                l.y,
			FontSize:         r.style.Paragraph.FontSize,
			LetterSpacing:    r.style.Paragraph.LetterSpacing,
			FontKey:          r.fontKey,
			Color:            r.style.Paragraph.Color,
			Underline:        r.style.Paragraph.Underline,
			Strikethrough:    r.style.Paragraph.Strikethrough,
			Text:             line.Text,
			Fragments:        pageLineFragments(line.Fragments),
			ExtraWordSpacing: line.ExtraWordSpacing,
			ExtraCharSpacing: line.ExtraCharSpacing,
			BreakStats:       line.BreakStats,
		}
		pageLine.X = pdfPageLineXAdjustedForVisualRight(pageLine, available)
		x = pageLine.X
		addPDFInlineImages(l.page, line, x, l.y)
		addLinkAnnotations(l.page, r.block, line, r.lineSearchStart, x, l.y, r.style.Paragraph.FontSize)
		addPDFParagraphFragmentAnchors(l.page, line)
		r.lineSearchStart = nextLineSearchStart(r.block.Text, line, r.lineSearchStart)
		addPDFPageLine(l.page, l.used, pageLine)
		l.y -= r.lineHeight
		l.pageHasText = true
		l.previousRenderedImage = false
	}
	return nil
}

func (l *pdfPageLayout) renderTextDropcap(block pdfTextBlock, style pdfBlockResolvedStyle, blockLeft float64, dropcap pdfDropcapLayout) int {
	dropcapX := blockLeft + style.MarginLeft + style.PaddingLeft
	dropLine := paragraphLine{Text: dropcap.Fragment.Text, Width: dropcap.Fragment.Width, Fragments: []paragraphLineFragment{dropcap.Fragment}}
	if dropcap.Fragment.LinkHref != "" {
		addFragmentLinkAnnotations(l.page, dropLine, dropcapX, dropcap.BaselineY)
	}
	addPDFParagraphFragmentAnchors(l.page, dropLine)
	addPDFPageLine(l.page, l.used, pdfPageLine{
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
	l.activeDropcap = &pdfActiveDropcap{
		Page:             l.page,
		X:                dropcapX,
		TopY:             dropcap.TopY,
		BottomY:          dropcap.BottomY,
		ExclusionWidth:   dropcap.ExclusionWidth,
		Lines:            dropcap.Lines,
		Char:             dropcap.Run.Text,
		BodySearchOffset: dropcap.BodySearchOffset,
	}
	return dropcap.BodySearchOffset
}

func (l *pdfPageLayout) paginateTextBlock(
	blockIndex int,
	style pdfBlockResolvedStyle,
	lines []paragraphLine,
	dropcap pdfDropcapLayout,
	dropcapOK bool,
	lineHeight float64,
	blockSpaceBefore func() float64,
	firstBaselineY func() float64,
) error {
	textHeight := float64(len(lines)) * lineHeight
	if dropcapOK {
		textHeight = max(textHeight, dropcap.ReservedHeight)
	}
	needed := blockSpaceBefore() + style.PaddingTop + textHeight + style.PaddingBottom
	if dropcapOK && l.pageHasText {
		requiredDropcapLines := max(dropcap.Lines, 1)
		if countFittingLines(firstBaselineY(), l.bottom, style.Paragraph.FontSize, lineHeight) < requiredDropcapLines {
			l.newTextPage()
		}
	}
	if style.KeepTogether && l.pageHasText && l.y-needed < l.bottom {
		l.newTextPage()
	}
	if keepLines := pdfKeepWithNextLines(l.doc.Blocks, l.blockStyles, blockIndex); keepLines > 0 && l.pageHasText {
		keepWithNext, err := nextBlockKeepHeight(
			l.doc,
			l.blockStyles,
			blockIndex+1,
			l.contentWidth,
			l.rootlessContentWidth,
			l.top-l.bottom,
			keepLines,
		)
		if err != nil {
			return err
		}
		if keepWithNext > 0 && l.y-needed-style.SpaceAfter-keepWithNext < l.bottom && needed+style.SpaceAfter+keepWithNext <= l.top-l.bottom {
			l.newTextPage()
		}
	}
	if !style.KeepTogether && l.pageHasText {
		linesFit := countFittingLines(l.y-blockSpaceBefore()-style.PaddingTop, l.bottom, style.Paragraph.FontSize, lineHeight)
		if linesFit > 0 && linesFit < len(lines) {
			firstFragmentLines := linesFit
			if remaining := len(lines) - firstFragmentLines; remaining < style.Widows {
				firstFragmentLines = len(lines) - style.Widows
			}
			if firstFragmentLines < min(style.Orphans, len(lines)) {
				l.newTextPage()
			}
		}
	}
	return nil
}

func (l *pdfPageLayout) textBlockFirstBaselineY(style pdfBlockResolvedStyle, blockSpaceBefore func() float64) float64 {
	baseline := l.y - blockSpaceBefore() - style.PaddingTop
	if !l.pageHasText || l.previousRenderedImage {
		baseline -= style.Paragraph.FontSize
	}
	return baseline
}

func (l *pdfPageLayout) shapeTextBlock(
	block pdfTextBlock,
	style pdfBlockResolvedStyle,
	face *builtinFontFace,
	runs []pdfInlineRun,
	blockWidth float64,
	lineHeight float64,
	firstBaselineY func() float64,
) ([]paragraphLine, pdfDropcapLayout, bool, error) {
	baseline := firstBaselineY()
	if pdfDropcapExpiredForLine(l.activeDropcap, l.page, baseline, lineHeight, style.Paragraph.FontSize) {
		l.activeDropcap = nil
	}
	shape := pdfActiveDropcapShape(l.activeDropcap, l.page, block, baseline, style.Paragraph)
	layoutText := block.Text
	layoutRuns := runs
	var dropcap pdfDropcapLayout
	dropcapOK := false
	if pdfBlockStartsDropcap(block) {
		var err error
		dropcap, dropcapOK, err = buildPDFDropcapLayout(l.doc, l.styles, block, style.Paragraph, face, runs, blockWidth, firstBaselineY())
		if err != nil {
			return nil, pdfDropcapLayout{}, false, err
		}
		if dropcapOK {
			shape = repeatPDFDropcapInset(dropcap.ExclusionWidth, dropcap.Lines)
			layoutText = dropcap.BodyText
			layoutRuns = dropcap.BodyRuns
		}
	}
	lines, err := layoutInline(l.doc, l.doc.Fonts, l.styles, face, layoutText, layoutRuns, style.Paragraph, blockWidth, shape)
	return lines, dropcap, dropcapOK, err
}
