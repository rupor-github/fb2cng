package pdf

import (
	"strings"

	"fbc/fb2"
)

type pdfTableLayout struct {
	Width     float64
	ColWidths []float64
	Rows      []pdfTableRowLayout
	Cells     []pdfTableCellLayout
	Groups    []pdfTableRowGroup
	Height    float64
}

type pdfTableRowLayout struct {
	Height float64
}

type pdfTableRowGroup struct {
	Start  int
	End    int
	Height float64
}

type pdfTableCellLayout struct {
	Row     int
	Col     int
	RowSpan int
	ColSpan int
	X       float64
	Width   float64
	Style   pdfBlockResolvedStyle
	Text    string
	Runs    []pdfInlineRun
	Links   []pdfTextLink
	Lines   []paragraphLine
	VAlign  string
}

func layoutPDFTable(doc pdfDocumentSpec, resolver *pdfStyleResolver, block pdfTextBlock, style pdfBlockResolvedStyle, tableWidth float64) (pdfTableLayout, error) {
	if resolver == nil {
		resolver = newPDFStyleResolver(nil, nil)
	}
	if block.Table == nil || len(block.Table.Rows) == 0 || tableWidth <= 0 {
		return pdfTableLayout{}, nil
	}

	cells, colCount := pdfPlacedTableCells(block.Table)
	if colCount == 0 {
		return pdfTableLayout{}, nil
	}
	colWidths, tableScale, err := pdfTableColumnWidths(doc, resolver, cells, colCount, tableWidth, block.ContextClasses)
	if err != nil {
		return pdfTableLayout{}, err
	}

	tableResolver := resolver.scaled(tableScale)
	rows := make([]pdfTableRowLayout, len(block.Table.Rows))
	layoutCells := make([]pdfTableCellLayout, 0, len(cells))
	for _, placed := range cells {
		cellStyle := tableResolver.styleForTableCell(placed.RowStyle, placed.Cell, block.ContextClasses)
		cellStyle.Paragraph.Hyphenator = doc.Hyphenator
		cellWidth := pdfTableColumnsWidth(colWidths, placed.Col, placed.ColSpan)
		innerWidth := max(cellWidth-cellStyle.PaddingLeft-cellStyle.PaddingRight-2*cellStyle.BorderWidth, pdfMinBlockWidth)
		paragraph := fb2.Paragraph{Text: placed.Cell.Content}
		text, links := paragraphTextAndLinks(&paragraph)
		runs := paragraphInlineRuns(&paragraph, doc.Content)
		if block.TableCellRuns != nil {
			if cellRuns, ok := block.TableCellRuns[pdfTableCellKey{placed.Row, placed.Col}]; ok {
				runs = cellRuns
			}
		}
		runs, links = pdfDisablePrintedFootnoteLinks(doc.Content, block.StyleClasses, block.ContextClasses, runs, links)
		runs = applyPDFPseudoContentToInlineRuns(runs, tableResolver)
		runs = inlineRunsWithContext(runs, joinStyleClasses(block.ContextClasses, pdfStyleTable))
		cellDoc := doc
		cellDoc.Styles = tableResolver
		face, _, err := fontForStyle(doc.Fonts, cellStyle.Paragraph)
		if err != nil {
			return pdfTableLayout{}, err
		}
		lines, err := layoutInlineWithShape(
			cellDoc,
			doc.Fonts,
			tableResolver,
			face,
			text,
			runs,
			cellStyle.Paragraph,
			innerWidth,
			paragraphLineShape{},
		)
		if err != nil {
			return pdfTableLayout{}, err
		}
		cellHeight := pdfTableCellContentHeight(cellStyle, lines)
		layoutCells = append(layoutCells, pdfTableCellLayout{
			Row:     placed.Row,
			Col:     placed.Col,
			RowSpan: placed.RowSpan,
			ColSpan: placed.ColSpan,
			X:       pdfTableColumnsWidth(colWidths, 0, placed.Col),
			Width:   cellWidth,
			Style:   cellStyle,
			Text:    text,
			Runs:    runs,
			Links:   links,
			Lines:   lines,
			VAlign:  placed.Cell.VAlign,
		})
		if placed.RowSpan <= 1 {
			rows[placed.Row].Height = max(rows[placed.Row].Height, cellHeight)
		}
	}

	for _, cell := range layoutCells {
		if cell.RowSpan <= 1 {
			continue
		}
		cellHeight := pdfTableCellContentHeight(cell.Style, cell.Lines)
		spanEnd := min(cell.Row+cell.RowSpan, len(rows))
		current := 0.0
		for r := cell.Row; r < spanEnd; r++ {
			current += rows[r].Height
		}
		if cellHeight > current {
			extra := (cellHeight - current) / float64(spanEnd-cell.Row)
			for r := cell.Row; r < spanEnd; r++ {
				rows[r].Height += extra
			}
		}
	}
	for i := range rows {
		if rows[i].Height <= 0 {
			rows[i].Height = style.Paragraph.LineHeight
		}
	}

	layout := pdfTableLayout{Width: tableWidth, ColWidths: colWidths, Rows: rows, Cells: layoutCells}
	layout.Groups = pdfTableRowGroups(layout)
	for _, row := range rows {
		layout.Height += row.Height
	}
	return layout, nil
}

func pdfTableColumnWidths(doc pdfDocumentSpec, resolver *pdfStyleResolver, cells []pdfPlacedTableCell, colCount int, tableWidth float64, contextClasses string) ([]float64, float64, error) {
	minWidths := make([]float64, colCount)
	for _, placed := range cells {
		style := resolver.styleForTableCell(placed.RowStyle, placed.Cell, contextClasses)
		needed, err := pdfTableCellMinWidth(doc, style, placed.Cell)
		if err != nil {
			return nil, 1, err
		}
		current := pdfTableColumnsWidth(minWidths, placed.Col, placed.ColSpan)
		if needed > current {
			add := (needed - current) / float64(placed.ColSpan)
			for c := placed.Col; c < min(placed.Col+placed.ColSpan, len(minWidths)); c++ {
				minWidths[c] += add
			}
		}
	}
	for i := range minWidths {
		if minWidths[i] <= 0 {
			minWidths[i] = tableWidth / float64(colCount)
		}
	}
	total := pdfTableColumnsWidth(minWidths, 0, len(minWidths))
	if total <= 0 {
		widths := make([]float64, colCount)
		for i := range widths {
			widths[i] = tableWidth / float64(colCount)
		}
		return widths, 1, nil
	}
	scale := min(tableWidth/total, 1)
	widths := make([]float64, colCount)
	if scale < 1 {
		for i := range widths {
			widths[i] = minWidths[i] * scale
		}
		return widths, scale, nil
	}
	extra := (tableWidth - total) / float64(colCount)
	for i := range widths {
		widths[i] = minWidths[i] + extra
	}
	return widths, 1, nil
}

func pdfTableCellMinWidth(doc pdfDocumentSpec, style pdfBlockResolvedStyle, cell fb2.TableCell) (float64, error) {
	paragraph := fb2.Paragraph{Text: cell.Content}
	text, _ := paragraphTextAndLinks(&paragraph)
	face, _, err := fontForStyle(doc.Fonts, style.Paragraph)
	if err != nil {
		return 0, err
	}
	maxWord := 0.0
	if style.Paragraph.NoWrap {
		text = strings.Join(strings.Fields(text), " ")
		if text != "" {
			shaped, err := shapeTextWithCache(doc.TextShapers, face, text)
			if err != nil {
				return 0, err
			}
			maxWord = shapedWidthPointsWithSpacing(shaped, style.Paragraph.FontSize, style.Paragraph.LetterSpacing)
		}
	} else {
		for _, word := range strings.Fields(text) {
			shaped, err := shapeTextWithCache(doc.TextShapers, face, word)
			if err != nil {
				return 0, err
			}
			maxWord = max(maxWord, shapedWidthPointsWithSpacing(shaped, style.Paragraph.FontSize, style.Paragraph.LetterSpacing))
		}
	}
	if maxWord <= 0 {
		maxWord = style.Paragraph.FontSize
	}
	return maxWord + style.PaddingLeft + style.PaddingRight + 2*style.BorderWidth, nil
}

func (r *pdfStyleResolver) scaled(scale float64) *pdfStyleResolver {
	if r == nil {
		r = newPDFStyleResolver(nil, nil)
	}
	if scale <= 0 || scale == 1 {
		return r
	}
	out := &pdfStyleResolver{
		styles:        make(map[string]pdfBlockResolvedStyle, len(r.styles)),
		defaults:      make(map[string]pdfBlockResolvedStyle, len(r.defaults)),
		dropcaps:      r.dropcaps,
		pseudoContent: r.pseudoContent,
		log:           r.log,
		tracer:        r.tracer,
	}
	for name, style := range r.styles {
		out.styles[name] = pdfScaleResolvedStyle(style, scale)
	}
	for name, style := range r.defaults {
		out.defaults[name] = pdfScaleResolvedStyle(style, scale)
	}
	return out
}

func pdfScaleResolvedStyle(style pdfBlockResolvedStyle, scale float64) pdfBlockResolvedStyle {
	style.Paragraph.FontSize *= scale
	style.Paragraph.FontSizeSpec = pdfCSSLengthSpec{}
	style.Paragraph.LineHeight *= scale
	style.Paragraph.LineHeightSpec = pdfCSSLengthSpec{}
	style.Paragraph.LetterSpacing *= scale
	style.Paragraph.LetterSpacingSpec = pdfCSSLengthSpec{}
	style.Paragraph.FirstLineIndent *= scale
	style.Paragraph.FirstLineIndentSpec = pdfCSSLengthSpec{}
	style.SpaceBefore *= scale
	style.SpaceBeforeSpec = pdfCSSLengthSpec{}
	style.SpaceAfter *= scale
	style.SpaceAfterSpec = pdfCSSLengthSpec{}
	style.MarginLeft *= scale
	style.MarginLeftSpec = pdfCSSLengthSpec{}
	style.MarginRight *= scale
	style.MarginRightSpec = pdfCSSLengthSpec{}
	style.PaddingTop *= scale
	style.PaddingTopSpec = pdfCSSLengthSpec{}
	style.PaddingRight *= scale
	style.PaddingRightSpec = pdfCSSLengthSpec{}
	style.PaddingBottom *= scale
	style.PaddingBottomSpec = pdfCSSLengthSpec{}
	style.PaddingLeft *= scale
	style.PaddingLeftSpec = pdfCSSLengthSpec{}
	style.BorderWidth *= scale
	return style
}

func (r *pdfStyleResolver) styleForTableCell(row fb2.TableRow, cell fb2.TableCell, contextClasses string) pdfBlockResolvedStyle {
	styleName := pdfStyleTableCell
	if cell.Header {
		styleName = pdfStyleTableHeaderCell
	}
	style := r.styleForBlock(pdfTextBlock{
		Kind:           pdfBlockTableCell,
		StyleName:      styleName,
		StyleClasses:   joinStyleClasses(row.Style, cell.Style),
		ContextClasses: joinStyleClasses(contextClasses, pdfStyleTable),
	})
	if align := strings.TrimSpace(cell.Align); align != "" {
		if textAlign, ok := pdfTextAlignKeyword(align); ok {
			style.Paragraph.Align = textAlign
			style.Paragraph.HasAlign = true
		}
	} else if align := strings.TrimSpace(row.Align); align != "" {
		if textAlign, ok := pdfTextAlignKeyword(align); ok {
			style.Paragraph.Align = textAlign
			style.Paragraph.HasAlign = true
		}
	}
	style.Paragraph.FirstLineIndent = 0
	style.Paragraph.HasFirstLineIndent = true
	return style
}

type pdfPlacedTableCell struct {
	Row      int
	Col      int
	RowSpan  int
	ColSpan  int
	RowStyle fb2.TableRow
	Cell     fb2.TableCell
}

func pdfPlacedTableCells(table *fb2.Table) ([]pdfPlacedTableCell, int) {
	var cells []pdfPlacedTableCell
	occupied := make(map[[2]int]bool)
	colCount := 0
	for rowIndex, row := range table.Rows {
		col := 0
		for _, cell := range row.Cells {
			for occupied[[2]int{rowIndex, col}] {
				col++
			}
			colSpan := max(cell.ColSpan, 1)
			rowSpan := max(cell.RowSpan, 1)
			cells = append(cells, pdfPlacedTableCell{Row: rowIndex, Col: col, RowSpan: rowSpan, ColSpan: colSpan, RowStyle: row, Cell: cell})
			for r := rowIndex; r < min(rowIndex+rowSpan, len(table.Rows)); r++ {
				for c := col; c < col+colSpan; c++ {
					occupied[[2]int{r, c}] = true
				}
			}
			col += colSpan
			colCount = max(colCount, col)
		}
	}
	return cells, colCount
}

func pdfTableColumnsWidth(widths []float64, start int, span int) float64 {
	if span <= 0 || start >= len(widths) {
		return 0
	}
	end := min(start+span, len(widths))
	width := 0.0
	for i := start; i < end; i++ {
		width += widths[i]
	}
	return width
}

func pdfTableCellContentHeight(style pdfBlockResolvedStyle, lines []paragraphLine) float64 {
	lineCount := max(len(lines), 1)
	return style.PaddingTop + float64(lineCount)*style.Paragraph.LineHeight + style.PaddingBottom + 2*style.BorderWidth
}

func pdfTableCellTextHeight(style pdfBlockResolvedStyle, lines []paragraphLine) float64 {
	return float64(max(len(lines), 1)) * style.Paragraph.LineHeight
}

func pdfTableRowGroups(layout pdfTableLayout) []pdfTableRowGroup {
	var groups []pdfTableRowGroup
	for start := 0; start < len(layout.Rows); {
		end := start
		changed := true
		for changed {
			changed = false
			for _, cell := range layout.Cells {
				if cell.Row < start || cell.Row > end {
					continue
				}
				cellEnd := min(cell.Row+cell.RowSpan-1, len(layout.Rows)-1)
				if cellEnd > end {
					end = cellEnd
					changed = true
				}
			}
		}
		height := 0.0
		for r := start; r <= end; r++ {
			height += layout.Rows[r].Height
		}
		groups = append(groups, pdfTableRowGroup{Start: start, End: end, Height: height})
		start = end + 1
	}
	return groups
}

func pdfTableRowsHeight(rows []pdfTableRowLayout, start int, end int) float64 {
	height := 0.0
	for r := start; r <= end && r < len(rows); r++ {
		height += rows[r].Height
	}
	return height
}

func pdfTextAlignKeyword(value string) (textAlign, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "left", "start":
		return textAlignLeft, true
	case "right", "end":
		return textAlignRight, true
	case "center", "middle":
		return textAlignCenter, true
	case "justify":
		return textAlignJustify, true
	default:
		return textAlignLeft, false
	}
}
