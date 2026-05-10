package pdf

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/text/language"

	"fbc/config"
	"fbc/content"
	contenttext "fbc/content/text"
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

	contentPlan, err := collectPDFContent(c)
	if err != nil {
		return fmt.Errorf("collect pdf content: %w", err)
	}

	data, err := buildSkeletonPDF(skeletonDocument{
		PageWidth:  pageWidth,
		PageHeight: pageHeight,
		Title:      bookTitle(c),
		Author:     bookAuthors(c),
		Blocks:     contentPlan.Blocks,
		TOC:        contentPlan.TOC,
		Hyphenator: pdfHyphenator(c, log),
		Debug:      c.Debug,
		WorkDir:    c.WorkDir,
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
	TOC        []*structure.TOCEntry
	Hyphenator paragraphHyphenator
	Debug      bool
	WorkDir    string
}

type pdfBlockKind int

func (k pdfBlockKind) String() string {
	switch k {
	case pdfBlockParagraph:
		return "paragraph"
	case pdfBlockHeading:
		return "heading"
	case pdfBlockSubtitle:
		return "subtitle"
	case pdfBlockPoem:
		return "poem"
	case pdfBlockTextAuthor:
		return "text-author"
	case pdfBlockEmptyLine:
		return "empty-line"
	case pdfBlockPageBreak:
		return "page-break"
	default:
		return "unknown"
	}
}

const (
	pdfBlockParagraph pdfBlockKind = iota
	pdfBlockHeading
	pdfBlockSubtitle
	pdfBlockPoem
	pdfBlockTextAuthor
	pdfBlockEmptyLine
	pdfBlockPageBreak
)

type pdfContentPlan struct {
	Blocks []pdfTextBlock
	TOC    []*structure.TOCEntry
}

type pdfTextBlock struct {
	Kind  pdfBlockKind
	ID    string
	Text  string
	Depth int
	Links []pdfTextLink
}

type pdfTextLink struct {
	Start int
	End   int
	Href  string
}

type pdfPageLine struct {
	X                float64
	Y                float64
	FontSize         float64
	Text             shapedText
	ExtraWordSpacing float64
}

type pdfPage struct {
	ObjectID    int
	ContentID   int
	Lines       []pdfPageLine
	Anchors     []string
	Annotations []pdfLinkAnnotation
}

type pdfLinkAnnotation struct {
	ObjectID int
	Rect     pdfRect
	Href     string
}

type pdfRect struct {
	X1 float64
	Y1 float64
	X2 float64
	Y2 float64
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
	if err := writePDFDebugDumps(doc, pages); err != nil {
		return nil, err
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
	outlines := buildOutlines(doc.TOC, pages, &nextObjectID)
	assignAnnotationObjectIDs(pages, &nextObjectID)

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

	catalog := pdfdoc.Dict{
		"Pages": pdfdoc.Ref{ObjectNumber: pagesID},
		"Type":  pdfdoc.Name("Catalog"),
	}
	if outlines.RootID != 0 {
		catalog["Outlines"] = pdfdoc.Ref{ObjectNumber: outlines.RootID}
	}
	if names := namedDestinations(pages); names != nil {
		catalog["Names"] = names
	}
	if err := writer.Object(catalogID, catalog); err != nil {
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
		pageDict := pdfdoc.Dict{
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
		}
		if len(page.Annotations) != 0 {
			annots := make(pdfdoc.Array, 0, len(page.Annotations))
			for _, annot := range page.Annotations {
				annots = append(annots, pdfdoc.Ref{ObjectNumber: annot.ObjectID})
			}
			pageDict["Annots"] = annots
		}
		if err := writer.Object(page.ObjectID, pageDict); err != nil {
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
	if err := writeOutlineObjects(writer, outlines); err != nil {
		return nil, err
	}
	if err := writeAnnotationObjects(writer, pages); err != nil {
		return nil, err
	}

	infoRef := pdfdoc.Ref{ObjectNumber: infoID}
	return writer.Finish(pdfdoc.Trailer{
		Root: pdfdoc.Ref{ObjectNumber: catalogID},
		Info: &infoRef,
	})
}

type pdfOutlines struct {
	RootID int
	Items  []*pdfOutlineItem
}

type pdfOutlineItem struct {
	ObjectID int
	Title    string
	PageID   int
	ParentID int
	PrevID   int
	NextID   int
	FirstID  int
	LastID   int
	Count    int
	Children []*pdfOutlineItem
}

func buildOutlines(entries []*structure.TOCEntry, pages []pdfPage, nextObjectID *int) pdfOutlines {
	anchorPages := make(map[string]int)
	for i := range pages {
		for _, id := range pages[i].Anchors {
			if _, exists := anchorPages[id]; !exists {
				anchorPages[id] = i
			}
		}
	}
	nodes := resolveOutlineItems(entries, pages, anchorPages)
	if len(nodes) == 0 {
		return pdfOutlines{}
	}
	outlines := pdfOutlines{
		RootID: *nextObjectID,
	}
	(*nextObjectID)++
	assignOutlineObjectIDs(nodes, nextObjectID)
	linkOutlineSiblings(outlines.RootID, nodes)
	outlines.Items = flattenOutlineItems(nodes)
	return outlines
}

func resolveOutlineItems(entries []*structure.TOCEntry, pages []pdfPage, anchorPages map[string]int) []*pdfOutlineItem {
	items := make([]*pdfOutlineItem, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		children := resolveOutlineItems(entry.Children, pages, anchorPages)
		pageIndex, ok := anchorPages[entry.ID]
		if !ok || pageIndex < 0 || pageIndex >= len(pages) || strings.TrimSpace(entry.Title) == "" {
			items = append(items, children...)
			continue
		}
		items = append(items, &pdfOutlineItem{
			Title:    entry.Title,
			PageID:   pages[pageIndex].ObjectID,
			Children: children,
		})
	}
	return items
}

func assignOutlineObjectIDs(items []*pdfOutlineItem, nextObjectID *int) {
	for _, item := range items {
		item.ObjectID = *nextObjectID
		(*nextObjectID)++
		assignOutlineObjectIDs(item.Children, nextObjectID)
	}
}

func linkOutlineSiblings(parentID int, items []*pdfOutlineItem) {
	for i, item := range items {
		item.ParentID = parentID
		if i > 0 {
			item.PrevID = items[i-1].ObjectID
		}
		if i+1 < len(items) {
			item.NextID = items[i+1].ObjectID
		}
		if len(item.Children) != 0 {
			item.FirstID = item.Children[0].ObjectID
			item.LastID = item.Children[len(item.Children)-1].ObjectID
			item.Count = countOutlineDescendants(item.Children)
			linkOutlineSiblings(item.ObjectID, item.Children)
		}
	}
}

func countOutlineDescendants(items []*pdfOutlineItem) int {
	count := len(items)
	for _, item := range items {
		count += countOutlineDescendants(item.Children)
	}
	return count
}

func flattenOutlineItems(items []*pdfOutlineItem) []*pdfOutlineItem {
	out := make([]*pdfOutlineItem, 0, countOutlineDescendants(items))
	var walk func([]*pdfOutlineItem)
	walk = func(items []*pdfOutlineItem) {
		for _, item := range items {
			out = append(out, item)
			walk(item.Children)
		}
	}
	walk(items)
	return out
}

func writeOutlineObjects(writer *pdfdoc.Writer, outlines pdfOutlines) error {
	if outlines.RootID == 0 {
		return nil
	}
	root := pdfdoc.Dict{
		"Count": pdfdoc.Integer(len(outlines.Items)),
		"Type":  pdfdoc.Name("Outlines"),
	}
	topLevel := topLevelOutlineItems(outlines)
	if len(topLevel) != 0 {
		root["First"] = pdfdoc.Ref{ObjectNumber: topLevel[0].ObjectID}
		root["Last"] = pdfdoc.Ref{ObjectNumber: topLevel[len(topLevel)-1].ObjectID}
	}
	if err := writer.Object(outlines.RootID, root); err != nil {
		return err
	}
	for _, item := range outlines.Items {
		dict := pdfdoc.Dict{
			"Dest": pdfdoc.Array{
				pdfdoc.Ref{ObjectNumber: item.PageID},
				pdfdoc.Name("Fit"),
			},
			"Parent": pdfdoc.Ref{ObjectNumber: item.ParentID},
			"Title":  pdfdoc.UTF16TextString(item.Title),
		}
		if item.PrevID != 0 {
			dict["Prev"] = pdfdoc.Ref{ObjectNumber: item.PrevID}
		}
		if item.NextID != 0 {
			dict["Next"] = pdfdoc.Ref{ObjectNumber: item.NextID}
		}
		if item.FirstID != 0 {
			dict["First"] = pdfdoc.Ref{ObjectNumber: item.FirstID}
			dict["Last"] = pdfdoc.Ref{ObjectNumber: item.LastID}
			dict["Count"] = pdfdoc.Integer(item.Count)
		}
		if err := writer.Object(item.ObjectID, dict); err != nil {
			return err
		}
	}
	return nil
}

func topLevelOutlineItems(outlines pdfOutlines) []*pdfOutlineItem {
	items := make([]*pdfOutlineItem, 0)
	for _, item := range outlines.Items {
		if item.ParentID == outlines.RootID {
			items = append(items, item)
		}
	}
	return items
}

func addLinkAnnotations(page *pdfPage, block pdfTextBlock, line paragraphLine, searchStart int, x float64, y float64, fontSize float64) {
	if len(block.Links) == 0 || len(line.Text.Glyphs) == 0 {
		return
	}
	lineText := shapedRunes(line.Text)
	lineStart, lineEnd, ok := lineRuneRange(block.Text, lineText, searchStart)
	if !ok {
		return
	}
	for _, link := range block.Links {
		start := max(link.Start, lineStart)
		end := min(link.End, lineEnd)
		if start >= end || strings.TrimSpace(link.Href) == "" {
			continue
		}
		x1 := x + glyphAdvanceRange(line.Text.Glyphs, 0, start-lineStart, fontSize)
		x2 := x + glyphAdvanceRange(line.Text.Glyphs, 0, end-lineStart, fontSize)
		if x2 <= x1 {
			continue
		}
		page.Annotations = append(page.Annotations, pdfLinkAnnotation{
			Rect: pdfRect{
				X1: x1,
				Y1: y - fontSize*0.2,
				X2: x2,
				Y2: y + fontSize,
			},
			Href: link.Href,
		})
	}
}

func nextLineSearchStart(text string, line paragraphLine, searchStart int) int {
	lineText := shapedRunes(line.Text)
	_, end, ok := lineRuneRange(text, lineText, searchStart)
	if !ok {
		return searchStart
	}
	return end
}

func lineRuneRange(text string, lineText string, searchStart int) (int, int, bool) {
	lineText = strings.TrimSuffix(lineText, "-")
	lineText = strings.TrimSpace(lineText)
	if lineText == "" {
		return searchStart, searchStart, false
	}
	runes := []rune(text)
	lineRunes := []rune(lineText)
	for start := max(searchStart, 0); start+len(lineRunes) <= len(runes); start++ {
		if string(runes[start:start+len(lineRunes)]) == lineText {
			return start, start + len(lineRunes), true
		}
	}
	return searchStart, searchStart, false
}

func glyphAdvanceRange(glyphs []shapedGlyph, start int, end int, fontSize float64) float64 {
	start = max(start, 0)
	end = min(max(end, start), len(glyphs))
	width := 0
	for _, glyph := range glyphs[start:end] {
		width += glyph.Width
	}
	return float64(width) * fontSize / 1000.0
}

func assignAnnotationObjectIDs(pages []pdfPage, nextObjectID *int) {
	for i := range pages {
		for j := range pages[i].Annotations {
			pages[i].Annotations[j].ObjectID = *nextObjectID
			(*nextObjectID)++
		}
	}
}

func writeAnnotationObjects(writer *pdfdoc.Writer, pages []pdfPage) error {
	for _, page := range pages {
		for _, annot := range page.Annotations {
			dict := pdfdoc.Dict{
				"Border":  pdfdoc.Array{pdfdoc.Integer(0), pdfdoc.Integer(0), pdfdoc.Integer(0)},
				"Rect":    rectArray(annot.Rect),
				"Subtype": pdfdoc.Name("Link"),
				"Type":    pdfdoc.Name("Annot"),
			}
			if target, ok := strings.CutPrefix(annot.Href, "#"); ok && target != "" {
				dict["Dest"] = pdfdoc.HexString([]byte(target))
			} else {
				dict["A"] = pdfdoc.Dict{
					"S":   pdfdoc.Name("URI"),
					"URI": pdfdoc.HexString([]byte(annot.Href)),
				}
			}
			if err := writer.Object(annot.ObjectID, dict); err != nil {
				return err
			}
		}
	}
	return nil
}

func rectArray(rect pdfRect) pdfdoc.Array {
	return pdfdoc.Array{
		pdfdoc.Number(rect.X1),
		pdfdoc.Number(rect.Y1),
		pdfdoc.Number(rect.X2),
		pdfdoc.Number(rect.Y2),
	}
}

func namedDestinations(pages []pdfPage) pdfdoc.Dict {
	anchors := make(map[string]int)
	for i := range pages {
		for _, id := range pages[i].Anchors {
			if _, exists := anchors[id]; !exists {
				anchors[id] = pages[i].ObjectID
			}
		}
	}
	if len(anchors) == 0 {
		return nil
	}

	ids := make([]string, 0, len(anchors))
	for id := range anchors {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	items := make(pdfdoc.Array, 0, len(ids)*2)
	for _, id := range ids {
		items = append(items,
			pdfdoc.HexString([]byte(id)),
			pdfdoc.Array{pdfdoc.Ref{ObjectNumber: anchors[id]}, pdfdoc.Name("Fit")},
		)
	}
	return pdfdoc.Dict{
		"Dests": pdfdoc.Dict{
			"Names": items,
		},
	}
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

	for blockIndex, block := range doc.Blocks {
		if block.Kind == pdfBlockPageBreak {
			if pageHasText {
				newTextPage()
			}
			addAnchor(page, block.ID)
			continue
		}

		style := pdfStyleForBlock(block)
		style.Paragraph.Hyphenator = doc.Hyphenator
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
		if style.KeepWithNextLines > 0 && pageHasText {
			keepWithNext, err := nextBlockKeepHeight(face, doc.Blocks[blockIndex+1:], doc.Hyphenator, contentWidth, style.KeepWithNextLines)
			if err != nil {
				return nil, nil, err
			}
			if keepWithNext > 0 && y-needed-style.SpaceAfter-keepWithNext < bottom {
				newTextPage()
			}
		}
		if !style.KeepTogether && pageHasText {
			linesFit := countFittingLines(y-style.SpaceBefore, bottom, style.Paragraph.FontSize, style.Paragraph.LineHeight)
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
		lineSearchStart := 0
		for lineIndex, line := range lines {
			if y-style.Paragraph.FontSize < bottom {
				newTextPage()
			}
			remainingAfterLine := len(lines) - lineIndex - 1
			if remainingAfterLine > 0 && remainingAfterLine < style.Widows && y-style.Paragraph.LineHeight-style.Paragraph.FontSize < bottom {
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
			addLinkAnnotations(page, block, line, lineSearchStart, x, y, style.Paragraph.FontSize)
			lineSearchStart = nextLineSearchStart(block.Text, line, lineSearchStart)
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
	Paragraph         paragraphStyle
	SpaceBefore       float64
	SpaceAfter        float64
	KeepTogether      bool
	KeepWithNextLines int
	Orphans           int
	Widows            int
}

func pdfStyleForBlock(block pdfTextBlock) pdfBlockResolvedStyle {
	switch block.Kind {
	case pdfBlockHeading:
		fontSize := max(16-float64(block.Depth-1), 11)
		return pdfBlockResolvedStyle{
			Paragraph:         paragraphStyle{FontSize: fontSize, LineHeight: fontSize * 1.25, Align: textAlignCenter},
			SpaceBefore:       10,
			SpaceAfter:        8,
			KeepTogether:      true,
			KeepWithNextLines: 2,
		}
	case pdfBlockSubtitle:
		return pdfBlockResolvedStyle{
			Paragraph:         paragraphStyle{FontSize: 11, LineHeight: 14, Align: textAlignCenter},
			SpaceBefore:       6,
			SpaceAfter:        5,
			KeepTogether:      true,
			KeepWithNextLines: 1,
		}
	case pdfBlockPoem:
		return pdfBlockResolvedStyle{
			Paragraph:  paragraphStyle{FontSize: 10.5, LineHeight: 13.2, Align: textAlignLeft},
			SpaceAfter: 2,
			Orphans:    2,
			Widows:     2,
		}
	case pdfBlockTextAuthor:
		return pdfBlockResolvedStyle{
			Paragraph:  paragraphStyle{FontSize: 10, LineHeight: 12.5, Align: textAlignRight},
			SpaceAfter: 4,
			Orphans:    2,
			Widows:     2,
		}
	default:
		return pdfBlockResolvedStyle{
			Paragraph:  paragraphStyle{FontSize: 10.5, LineHeight: 13.4, FirstLineIndent: 14, Align: textAlignJustify},
			SpaceAfter: 3,
			Orphans:    2,
			Widows:     2,
		}
	}
}

type pdfDebugBlock struct {
	Index int    `json:"index"`
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Depth int    `json:"depth,omitempty"`
	Text  string `json:"text,omitempty"`
}

type pdfDebugPage struct {
	Number int            `json:"number"`
	Lines  []pdfDebugLine `json:"lines"`
}

type pdfDebugLine struct {
	Text             string  `json:"text"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	FontSize         float64 `json:"font_size"`
	Width            float64 `json:"width"`
	ExtraWordSpacing float64 `json:"extra_word_spacing,omitempty"`
}

func writePDFDebugDumps(doc skeletonDocument, pages []pdfPage) error {
	if !doc.Debug || doc.WorkDir == "" {
		return nil
	}
	blocks := make([]pdfDebugBlock, 0, len(doc.Blocks))
	for i, block := range doc.Blocks {
		blocks = append(blocks, pdfDebugBlock{
			Index: i,
			Kind:  block.Kind.String(),
			ID:    block.ID,
			Depth: block.Depth,
			Text:  block.Text,
		})
	}
	if err := writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-text-blocks.json"), blocks); err != nil {
		return err
	}

	debugPages := make([]pdfDebugPage, 0, len(pages))
	for i, page := range pages {
		debugPage := pdfDebugPage{
			Number: i + 1,
			Lines:  make([]pdfDebugLine, 0, len(page.Lines)),
		}
		for _, line := range page.Lines {
			debugPage.Lines = append(debugPage.Lines, pdfDebugLine{
				Text:             shapedRunes(line.Text),
				X:                line.X,
				Y:                line.Y,
				FontSize:         line.FontSize,
				Width:            shapedWidthPoints(line.Text, line.FontSize),
				ExtraWordSpacing: line.ExtraWordSpacing,
			})
		}
		debugPages = append(debugPages, debugPage)
	}
	return writeJSONDebugDump(filepath.Join(doc.WorkDir, "pdf-layout-pages.json"), debugPages)
}

func writeJSONDebugDump(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

func shapedRunes(text shapedText) string {
	runes := make([]rune, 0, len(text.Glyphs))
	for _, glyph := range text.Glyphs {
		runes = append(runes, glyph.Rune)
	}
	return string(runes)
}

func nextBlockKeepHeight(face *builtinFontFace, blocks []pdfTextBlock, hyphenator paragraphHyphenator, contentWidth float64, minLines int) (float64, error) {
	for _, block := range blocks {
		switch block.Kind {
		case pdfBlockPageBreak:
			return 0, nil
		case pdfBlockEmptyLine:
			continue
		}
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		style := pdfStyleForBlock(block)
		style.Paragraph.Hyphenator = hyphenator
		lines, err := layoutParagraph(face, text, style.Paragraph, contentWidth)
		if err != nil {
			return 0, err
		}
		if len(lines) == 0 {
			continue
		}
		return style.SpaceBefore + float64(min(minLines, len(lines)))*style.Paragraph.LineHeight, nil
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
	plan, err := collectPDFContent(c)
	if err != nil {
		return nil, err
	}
	return plan.Blocks, nil
}

func collectPDFContent(c *content.Content) (pdfContentPlan, error) {
	if c == nil || c.Book == nil {
		return pdfContentPlan{}, nil
	}
	plan, err := structure.BuildPlan(c)
	if err != nil {
		return pdfContentPlan{}, err
	}

	blocks := make([]pdfTextBlock, 0, 64)
	splitSections := splitSectionIDs(plan)
	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.ForceNewPage {
			blocks = append(blocks, pdfTextBlock{Kind: pdfBlockPageBreak, ID: unit.ID, Text: unit.Title})
		}
		appendUnitBlocks(&blocks, unit, splitSections)
	}
	return pdfContentPlan{Blocks: blocks, TOC: plan.TOC}, nil
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
	appendTitleBlocksWithID(blocks, section.Title, depth, section.ID)
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
	appendTitleBlocksWithID(blocks, title, depth, "")
}

func appendTitleBlocksWithID(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string) {
	if title == nil {
		return
	}
	anchorID := id
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
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockHeading, ID: anchorID, Text: text, Depth: depth})
			anchorID = ""
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
	text, links := paragraphTextAndLinks(paragraph)
	if text != "" {
		*blocks = append(*blocks, pdfTextBlock{Kind: kind, ID: paragraph.ID, Text: text, Depth: depth, Links: links})
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
	text, _ := paragraphTextAndLinks(paragraph)
	return text
}

func paragraphTextAndLinks(paragraph *fb2.Paragraph) (string, []pdfTextLink) {
	if paragraph == nil {
		return "", nil
	}
	return inlineSegmentsTextAndLinks(paragraph.Text)
}

func inlineSegmentsTextAndLinks(segments []fb2.InlineSegment) (string, []pdfTextLink) {
	var b strings.Builder
	var links []pdfTextLink
	for i := range segments {
		appendInlineSegmentText(&b, &links, &segments[i])
	}
	return strings.TrimSpace(b.String()), links
}

func appendInlineSegmentText(b *strings.Builder, links *[]pdfTextLink, seg *fb2.InlineSegment) {
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
	start := runeLenString(b.String())
	b.WriteString(seg.Text)
	for i := range seg.Children {
		appendInlineSegmentText(b, links, &seg.Children[i])
	}
	if seg.Kind == fb2.InlineLink && seg.Href != "" {
		end := runeLenString(b.String())
		if end > start {
			*links = append(*links, pdfTextLink{Start: start, End: end, Href: seg.Href})
		}
	}
}

func runeLenString(s string) int {
	return len([]rune(s))
}

func pdfHyphenator(c *content.Content, log *zap.Logger) paragraphHyphenator {
	if c == nil || c.Book == nil || c.Book.Description.TitleInfo.Lang == language.Und {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	return contenttext.NewHyphenator(c.Book.Description.TitleInfo.Lang, log)
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
