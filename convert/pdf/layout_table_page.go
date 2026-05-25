package pdf

import "strings"

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
