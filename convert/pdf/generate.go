package pdf

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/internal/pdfdoc"
	"fbc/convert/structure"
	"fbc/fb2"
)

const (
	pdfVersion = "1.4"
	defaultDPI = 300
)

// Generate writes a native PDF document.
//
// The current native renderer writes fixed-size PDF 1.4 pages with embedded
// Unicode font resources, selectable title/author text, initial FB2 text body
// pagination, and Info dictionary metadata. Later milestones will replace the
// fixed default styles with the KFX-aligned CSS pipeline.
func Generate(ctx context.Context, c *content.Content, outputName string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil {
		return errors.New("content is required")
	}
	if cfg == nil {
		return errors.New("document config is required")
	}

	pageWidth, pageHeight, err := pageSizePoints(cfg.Images.Screen)
	if err != nil {
		return err
	}

	blocks, err := collectTextBlocks(c)
	if err != nil {
		return fmt.Errorf("collect pdf text blocks: %w", err)
	}

	data, err := buildSkeletonPDF(skeletonDocument{
		PageWidth:  pageWidth,
		PageHeight: pageHeight,
		Title:      bookTitle(c),
		Author:     bookAuthors(c),
		Blocks:     blocks,
	})
	if err != nil {
		return fmt.Errorf("build pdf: %w", err)
	}

	if log != nil {
		log.Debug("Writing PDF",
			zap.String("file", outputName),
			zap.Float64("page_width_pt", pageWidth),
			zap.Float64("page_height_pt", pageHeight),
			zap.Int("bytes", len(data)))
	}

	if err := os.WriteFile(outputName, data, 0644); err != nil {
		return fmt.Errorf("write pdf: %w", err)
	}
	return nil
}

type skeletonDocument struct {
	PageWidth  float64
	PageHeight float64
	Title      string
	Author     string
	Blocks     []pdfTextBlock
}

type pdfBlockKind int

const (
	pdfBlockParagraph pdfBlockKind = iota
	pdfBlockHeading
	pdfBlockSubtitle
	pdfBlockPoem
	pdfBlockTextAuthor
	pdfBlockEmptyLine
	pdfBlockPageBreak
)

type pdfTextBlock struct {
	Kind  pdfBlockKind
	Text  string
	Depth int
}

type pdfPageLine struct {
	X                float64
	Y                float64
	FontSize         float64
	Text             shapedText
	ExtraWordSpacing float64
}

type pdfPage struct {
	ObjectID  int
	ContentID int
	Lines     []pdfPageLine
}

func pageSizePoints(screen config.ScreenConfig) (float64, float64, error) {
	dpi := screen.DPI
	if dpi == 0 {
		dpi = defaultDPI
	}
	if screen.Width <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen width: %d", screen.Width)
	}
	if screen.Height <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen height: %d", screen.Height)
	}
	if dpi <= 0 {
		return 0, 0, fmt.Errorf("invalid pdf screen dpi: %d", screen.DPI)
	}

	return float64(screen.Width) * 72.0 / float64(dpi), float64(screen.Height) * 72.0 / float64(dpi), nil
}

func buildSkeletonPDF(doc skeletonDocument) ([]byte, error) {
	writer := pdfdoc.NewWriter(pdfVersion)

	const (
		catalogID        = 1
		pagesID          = 2
		firstPageID      = 3
		firstContentID   = 4
		infoID           = 5
		type0FontID      = 6
		cidFontID        = 7
		fontDescriptorID = 8
		fontFileID       = 9
		toUnicodeID      = 10
	)

	fontFace, err := builtinFont("sans-serif", false, false)
	if err != nil {
		return nil, err
	}
	pages, usedGlyphs, err := layoutPDFPages(doc, fontFace)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, errors.New("pdf document must contain at least one page")
	}

	pages[0].ObjectID = firstPageID
	pages[0].ContentID = firstContentID
	nextObjectID := toUnicodeID + 1
	for i := 1; i < len(pages); i++ {
		pages[i].ObjectID = nextObjectID
		nextObjectID++
		pages[i].ContentID = nextObjectID
		nextObjectID++
	}

	fontObjs, err := fontResourceObjects(fontFace, usedGlyphs, fontObjectIDs{
		Type0Font:      type0FontID,
		CIDFont:        cidFontID,
		FontDescriptor: fontDescriptorID,
		FontFile:       fontFileID,
		ToUnicode:      toUnicodeID,
	})
	if err != nil {
		return nil, err
	}

	if err := writer.Object(catalogID, pdfdoc.Dict{
		"Pages": pdfdoc.Ref{ObjectNumber: pagesID},
		"Type":  pdfdoc.Name("Catalog"),
	}); err != nil {
		return nil, err
	}
	kids := make(pdfdoc.Array, 0, len(pages))
	for _, page := range pages {
		kids = append(kids, pdfdoc.Ref{ObjectNumber: page.ObjectID})
	}
	if err := writer.Object(pagesID, pdfdoc.Dict{
		"Count": pdfdoc.Integer(len(pages)),
		"Kids":  kids,
		"Type":  pdfdoc.Name("Pages"),
	}); err != nil {
		return nil, err
	}
	for _, page := range pages {
		if err := writer.Object(page.ObjectID, pdfdoc.Dict{
			"Contents": pdfdoc.Ref{ObjectNumber: page.ContentID},
			"MediaBox": pdfdoc.Array{
				pdfdoc.Integer(0),
				pdfdoc.Integer(0),
				pdfdoc.Number(doc.PageWidth),
				pdfdoc.Number(doc.PageHeight),
			},
			"Parent": pdfdoc.Ref{ObjectNumber: pagesID},
			"Resources": pdfdoc.Dict{
				"Font": pdfdoc.Dict{
					"F1": pdfdoc.Ref{ObjectNumber: type0FontID},
				},
			},
			"Type": pdfdoc.Name("Page"),
		}); err != nil {
			return nil, err
		}
	}

	for _, page := range pages {
		stream, err := flateStream(pageContent(page))
		if err != nil {
			return nil, err
		}
		if err := writer.StreamObject(page.ContentID, pdfdoc.Dict{
			"Filter": pdfdoc.Name("FlateDecode"),
		}, stream); err != nil {
			return nil, err
		}
	}
	if err := writer.Object(infoID, infoDictionary(doc)); err != nil {
		return nil, err
	}
	if err := writer.Object(type0FontID, fontObjs.Type0Font); err != nil {
		return nil, err
	}
	if err := writer.Object(cidFontID, fontObjs.CIDFont); err != nil {
		return nil, err
	}
	if err := writer.Object(fontDescriptorID, fontObjs.FontDescriptor); err != nil {
		return nil, err
	}
	if err := writer.StreamObject(fontFileID, fontObjs.FontFile, fontObjs.FontFileData); err != nil {
		return nil, err
	}
	if err := writer.StreamObject(toUnicodeID, pdfdoc.Dict{}, fontObjs.ToUnicode); err != nil {
		return nil, err
	}

	infoRef := pdfdoc.Ref{ObjectNumber: infoID}
	return writer.Finish(pdfdoc.Trailer{
		Root: pdfdoc.Ref{ObjectNumber: catalogID},
		Info: &infoRef,
	})
}

func pageContent(page pdfPage) []byte {
	var buf bytes.Buffer
	buf.WriteString("q\nBT\n")
	currentFontSize := -1.0
	for _, line := range page.Lines {
		if len(line.Text.Glyphs) == 0 {
			continue
		}
		if line.FontSize != currentFontSize {
			fmt.Fprintf(&buf, "/F1 %s Tf\n", pdfdoc.FormatNumber(line.FontSize))
			currentFontSize = line.FontSize
		}
		fmt.Fprintf(&buf, "1 0 0 1 %s %s Tm\n", pdfdoc.FormatNumber(line.X), pdfdoc.FormatNumber(line.Y))
		if line.ExtraWordSpacing != 0 {
			fmt.Fprintf(&buf, "%s TJ\n", justifiedGlyphArray(line.Text.Glyphs, line.ExtraWordSpacing, line.FontSize))
			continue
		}
		fmt.Fprintf(&buf, "%s Tj\n", pdfdoc.Format(glyphHex(line.Text.Glyphs)))
	}
	buf.WriteString("ET\nQ\n")
	return buf.Bytes()
}

func layoutPDFPages(doc skeletonDocument, face *builtinFontFace) ([]pdfPage, map[uint16]shapedGlyph, error) {
	const margin = 24.0
	contentWidth := max(doc.PageWidth-margin*2, 12)
	used := make(map[uint16]shapedGlyph)
	pages := make([]pdfPage, 0, 2)

	addPage := func() *pdfPage {
		pages = append(pages, pdfPage{})
		return &pages[len(pages)-1]
	}
	addLine := func(page *pdfPage, line pdfPageLine) {
		page.Lines = append(page.Lines, line)
		for id, glyph := range line.Text.Used {
			used[id] = glyph
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
	title, err := shapeText(face, titleText)
	if err != nil {
		return nil, nil, fmt.Errorf("shape title: %w", err)
	}
	addLine(titlePage, pdfPageLine{
		X:        margin,
		Y:        max(doc.PageHeight-54.0, margin),
		FontSize: 14,
		Text:     title,
	})
	authorLines, err := wrapText(face, authorText, 9, contentWidth)
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
			Text:     line,
		})
	}

	if len(doc.Blocks) == 0 {
		return pages, used, nil
	}

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

	for _, block := range doc.Blocks {
		if block.Kind == pdfBlockPageBreak {
			if pageHasText {
				newTextPage()
			}
			continue
		}

		style := pdfStyleForBlock(block)
		if block.Kind == pdfBlockEmptyLine {
			if y-style.Paragraph.LineHeight < bottom {
				newTextPage()
			}
			y -= style.Paragraph.LineHeight
			continue
		}
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		lines, err := layoutParagraph(face, text, style.Paragraph, contentWidth)
		if err != nil {
			return nil, nil, err
		}
		if len(lines) == 0 {
			continue
		}

		needed := style.SpaceBefore + float64(len(lines))*style.Paragraph.LineHeight
		if style.KeepTogether && pageHasText && y-needed < bottom {
			newTextPage()
		}
		if pageHasText {
			y -= style.SpaceBefore
		}
		for _, line := range lines {
			if y-style.Paragraph.FontSize < bottom {
				newTextPage()
			}
			x := margin + line.Indent
			available := contentWidth - line.Indent
			switch style.Paragraph.Align {
			case textAlignCenter:
				x += max((available-line.Width)/2, 0)
			case textAlignRight:
				x += max(available-line.Width, 0)
			}
			addLine(page, pdfPageLine{
				X:                x,
				Y:                y,
				FontSize:         style.Paragraph.FontSize,
				Text:             line.Text,
				ExtraWordSpacing: line.ExtraWordSpacing,
			})
			y -= style.Paragraph.LineHeight
			pageHasText = true
		}
		y -= style.SpaceAfter
	}

	if len(pages[len(pages)-1].Lines) == 0 {
		pages = pages[:len(pages)-1]
	}
	return pages, used, nil
}

type pdfBlockResolvedStyle struct {
	Paragraph    paragraphStyle
	SpaceBefore  float64
	SpaceAfter   float64
	KeepTogether bool
}

func pdfStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	switch block.Kind {
	case pdfBlockHeading:
		fontSize := max(16-float64(block.Depth-1), 11)
		return pdfBlockResolvedStyle{
			Paragraph:    paragraphStyle{FontSize: fontSize, LineHeight: fontSize * 1.25, Align: textAlignCenter},
			SpaceBefore:  10,
			SpaceAfter:   8,
			KeepTogether: true,
		}
	case pdfBlockSubtitle:
		return pdfBlockResolvedStyle{
			Paragraph:    paragraphStyle{FontSize: 11, LineHeight: 14, Align: textAlignCenter},
			SpaceBefore:  6,
			SpaceAfter:   5,
			KeepTogether: true,
		}
	case pdfBlockPoem:
		return pdfBlockResolvedStyle{
			Paragraph:  paragraphStyle{FontSize: 10.5, LineHeight: 13.2, Align: textAlignLeft},
			SpaceAfter: 2,
		}
	case pdfBlockTextAuthor:
		return pdfBlockResolvedStyle{
			Paragraph:  paragraphStyle{FontSize: 10, LineHeight: 12.5, Align: textAlignRight},
			SpaceAfter: 4,
		}
	default:
		return pdfBlockResolvedStyle{
			Paragraph:  paragraphStyle{FontSize: 10.5, LineHeight: 13.4, FirstLineIndent: 14, Align: textAlignJustify},
			SpaceAfter: 3,
		}
	}
}

func justifiedGlyphArray(glyphs []shapedGlyph, extraWordSpacing, fontSize float64) string {
	adjustment := -extraWordSpacing * 1000 / fontSize
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, glyph := range glyphs {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(pdfdoc.Format(glyphHex([]shapedGlyph{glyph})))
		if glyph.Rune == ' ' && i != len(glyphs)-1 {
			buf.WriteByte(' ')
			buf.WriteString(pdfdoc.FormatNumber(adjustment))
		}
	}
	buf.WriteByte(']')
	return buf.String()
}

func collectTextBlocks(c *content.Content) ([]pdfTextBlock, error) {
	if c == nil || c.Book == nil {
		return nil, nil
	}
	plan, err := structure.BuildPlan(c)
	if err != nil {
		return nil, err
	}

	blocks := make([]pdfTextBlock, 0, 64)
	splitSections := splitSectionIDs(plan)
	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.ForceNewPage {
			blocks = append(blocks, pdfTextBlock{Kind: pdfBlockPageBreak})
		}
		appendUnitBlocks(&blocks, unit, splitSections)
	}
	return blocks, nil
}

func splitSectionIDs(plan *structure.Plan) map[string]bool {
	ids := make(map[string]bool)
	if plan == nil {
		return ids
	}
	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.Kind == structure.UnitSection && unit.Section != nil && unit.ID != "" {
			ids[unit.ID] = true
		}
	}
	return ids
}

func appendUnitBlocks(blocks *[]pdfTextBlock, unit *structure.Unit, splitSections map[string]bool) {
	if unit == nil {
		return
	}
	switch unit.Kind {
	case structure.UnitBodyIntro:
		appendBodyIntroBlocks(blocks, unit.Body)
	case structure.UnitSection:
		appendSectionBlocks(blocks, unit.Section, unit.TitleDepth, splitSections)
	case structure.UnitFootnotesBody:
		appendBodyBlocks(blocks, unit.Body, splitSections)
	}
}

func appendBodyIntroBlocks(blocks *[]pdfTextBlock, body *fb2.Body) {
	if body == nil {
		return
	}
	appendTitleBlocks(blocks, body.Title, 1)
	for i := range body.Epigraphs {
		appendEpigraphBlocks(blocks, &body.Epigraphs[i])
	}
}

func appendBodyBlocks(blocks *[]pdfTextBlock, body *fb2.Body, splitSections map[string]bool) {
	if body == nil {
		return
	}
	appendBodyIntroBlocks(blocks, body)
	for i := range body.Sections {
		appendSectionBlocks(blocks, &body.Sections[i], 1, splitSections)
	}
}

func appendSectionBlocks(blocks *[]pdfTextBlock, section *fb2.Section, depth int, splitSections map[string]bool) {
	if section == nil {
		return
	}
	appendTitleBlocks(blocks, section.Title, depth)
	for i := range section.Epigraphs {
		appendEpigraphBlocks(blocks, &section.Epigraphs[i])
	}
	if section.Annotation != nil {
		appendFlowBlocks(blocks, section.Annotation.Items, depth, splitSections)
	}
	for i := range section.Content {
		appendFlowItemBlock(blocks, &section.Content[i], depth, splitSections)
	}
}

func appendTitleBlocks(blocks *[]pdfTextBlock, title *fb2.Title, depth int) {
	if title == nil {
		return
	}
	for i := range title.Items {
		item := &title.Items[i]
		if item.EmptyLine {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine})
			continue
		}
		if item.Paragraph == nil {
			continue
		}
		if text := paragraphText(item.Paragraph); text != "" {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockHeading, Text: text, Depth: depth})
		}
	}
}

func appendFlowBlocks(blocks *[]pdfTextBlock, items []fb2.FlowItem, depth int, splitSections map[string]bool) {
	for i := range items {
		appendFlowItemBlock(blocks, &items[i], depth, splitSections)
	}
}

func appendFlowItemBlock(blocks *[]pdfTextBlock, item *fb2.FlowItem, depth int, splitSections map[string]bool) {
	if item == nil {
		return
	}
	switch item.Kind {
	case fb2.FlowParagraph:
		appendParagraphBlock(blocks, pdfBlockParagraph, item.Paragraph, depth)
	case fb2.FlowSubtitle:
		appendParagraphBlock(blocks, pdfBlockSubtitle, item.Subtitle, depth)
	case fb2.FlowEmptyLine:
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine})
	case fb2.FlowSection:
		if item.Section != nil && splitSections[item.Section.ID] {
			return
		}
		appendSectionBlocks(blocks, item.Section, depth+1, splitSections)
	case fb2.FlowPoem:
		appendPoemBlocks(blocks, item.Poem, depth, splitSections)
	case fb2.FlowCite:
		appendCiteBlocks(blocks, item.Cite, depth, splitSections)
	case fb2.FlowTable:
		if item.Table != nil {
			text := item.Table.AsPlainText()
			if text != "" {
				*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockParagraph, Text: text, Depth: depth})
			}
		}
	}
}

func appendParagraphBlock(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int) {
	if paragraph == nil {
		return
	}
	if text := paragraphText(paragraph); text != "" {
		*blocks = append(*blocks, pdfTextBlock{Kind: kind, Text: text, Depth: depth})
	}
}

func appendPoemBlocks(blocks *[]pdfTextBlock, poem *fb2.Poem, depth int, splitSections map[string]bool) {
	if poem == nil {
		return
	}
	appendTitleBlocks(blocks, poem.Title, depth+1)
	for i := range poem.Epigraphs {
		appendEpigraphBlocks(blocks, &poem.Epigraphs[i])
	}
	for i := range poem.Subtitles {
		appendParagraphBlock(blocks, pdfBlockSubtitle, &poem.Subtitles[i], depth)
	}
	for i := range poem.Stanzas {
		stanza := &poem.Stanzas[i]
		appendTitleBlocks(blocks, stanza.Title, depth+1)
		appendParagraphBlock(blocks, pdfBlockSubtitle, stanza.Subtitle, depth)
		for j := range stanza.Verses {
			appendParagraphBlock(blocks, pdfBlockPoem, &stanza.Verses[j], depth)
		}
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine})
	}
	for i := range poem.TextAuthors {
		appendParagraphBlock(blocks, pdfBlockTextAuthor, &poem.TextAuthors[i], depth)
	}
}

func appendCiteBlocks(blocks *[]pdfTextBlock, cite *fb2.Cite, depth int, splitSections map[string]bool) {
	if cite == nil {
		return
	}
	appendFlowBlocks(blocks, cite.Items, depth, splitSections)
	for i := range cite.TextAuthors {
		appendParagraphBlock(blocks, pdfBlockTextAuthor, &cite.TextAuthors[i], depth)
	}
}

func appendEpigraphBlocks(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph) {
	if epigraph == nil {
		return
	}
	appendFlowBlocks(blocks, epigraph.Flow.Items, 1, nil)
	for i := range epigraph.TextAuthors {
		appendParagraphBlock(blocks, pdfBlockTextAuthor, &epigraph.TextAuthors[i], 1)
	}
}

func paragraphText(paragraph *fb2.Paragraph) string {
	if paragraph == nil {
		return ""
	}
	return strings.TrimSpace(inlineSegmentsText(paragraph.Text))
}

func inlineSegmentsText(segments []fb2.InlineSegment) string {
	var b strings.Builder
	for i := range segments {
		appendInlineSegmentText(&b, &segments[i])
	}
	return b.String()
}

func appendInlineSegmentText(b *strings.Builder, seg *fb2.InlineSegment) {
	if seg == nil {
		return
	}
	if seg.Kind == fb2.InlineImageSegment {
		if seg.Image != nil && seg.Image.Alt != "" {
			if b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(seg.Image.Alt)
		}
		return
	}
	b.WriteString(seg.Text)
	for i := range seg.Children {
		appendInlineSegmentText(b, &seg.Children[i])
	}
}

func infoDictionary(doc skeletonDocument) pdfdoc.Dict {
	info := pdfdoc.Dict{
		"Creator":  pdfdoc.UTF16TextString("fbc"),
		"Producer": pdfdoc.UTF16TextString("fbc"),
	}
	if doc.Title != "" {
		info["Title"] = pdfdoc.UTF16TextString(doc.Title)
	}
	if doc.Author != "" {
		info["Author"] = pdfdoc.UTF16TextString(doc.Author)
	}
	return info
}

func flateStream(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, fmt.Errorf("compress content stream: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finish content stream compression: %w", err)
	}
	return buf.Bytes(), nil
}

func bookTitle(c *content.Content) string {
	if c.Book == nil {
		return strings.TrimSuffix(c.SrcName, ".fb2")
	}
	if title := strings.TrimSpace(c.Book.Description.TitleInfo.BookTitle.Value); title != "" {
		return title
	}
	return strings.TrimSuffix(c.SrcName, ".fb2")
}

func bookAuthors(c *content.Content) string {
	if c.Book == nil {
		return ""
	}

	authors := make([]string, 0, len(c.Book.Description.TitleInfo.Authors))
	for i := range c.Book.Description.TitleInfo.Authors {
		name := authorName(&c.Book.Description.TitleInfo.Authors[i])
		if name != "" {
			authors = append(authors, name)
		}
	}
	return strings.Join(authors, ", ")
}

func authorName(author *fb2.Author) string {
	if author == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	for _, part := range []string{author.FirstName, author.MiddleName, author.LastName} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) != 0 {
		return strings.Join(parts, " ")
	}
	return strings.TrimSpace(author.Nickname)
}
