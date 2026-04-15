package pdf

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"
	"go.uber.org/zap"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/structure"
	"fbc/fb2"
)

type splitSection struct {
	section    *fb2.Section
	depth      int
	titleDepth int
	remaining  []fb2.FlowItem
	parentUnit *structure.Unit
}

type renderContext struct {
	c             *content.Content
	doc           *document.Document
	styles        *styleResolver
	fonts         *fontRegistry
	log           *zap.Logger
	contentHeight float64 // usable page content height in points (page height minus vertical margins)
}

type flowBuilder struct {
	ctx       *renderContext
	elements  *[]layout.Element
	ancestors []styleScope
	parent    resolvedStyle
}

type textContext struct {
	ancestors []styleScope
	parent    resolvedStyle
	linkURI   string
	hyphenate bool
}

// Generate creates a PDF using the PDF-local style and rendering pipeline.
func Generate(ctx context.Context, c *content.Content, outputPath string, cfg *config.DocumentConfig, log *zap.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.Book == nil {
		return fmt.Errorf("nil content or book")
	}
	if log == nil {
		log = zap.NewNop()
	}

	plan, err := structure.BuildPlan(c)
	if err != nil {
		return fmt.Errorf("build structure plan: %w", err)
	}

	parsed := buildCombinedStylesheet(c.Book.Stylesheets, log)
	styles := newStyleResolverFromParsed(parsed, log)
	bodyStyle := defaultResolvedStyle()
	if styles != nil {
		bodyStyle = styles.Resolve("body", "", nil, defaultResolvedStyle())
	}

	geom := GeometryFromStyles(cfg, bodyStyle)
	doc := document.NewDocument(geom.PageSize)
	doc.SetMargins(geom.Margins)
	doc.SetAutoBookmarks(true)
	doc.SetTagged(true)
	applyMetadata(doc, c)

	rc := &renderContext{
		c:             c,
		doc:           doc,
		styles:        styles,
		fonts:         newFontRegistry(c.Book.Stylesheets, parsed, log),
		log:           log.Named("pdf"),
		contentHeight: geom.PageSize.Height - geom.Margins.Top - geom.Margins.Bottom,
	}

	if err := addPlan(rc, plan); err != nil {
		return fmt.Errorf("render structure plan: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := savePDF(doc, outputPath); err != nil {
		return err
	}
	log.Info("PDF generation completed", zap.String("output", outputPath), zap.Int("units", len(plan.Units)))
	return nil
}

// savePDF writes the document through a buffered writer so folio's many
// small writes are coalesced into large OS-level writes.  This avoids
// thousands of individual write(2) syscalls that are particularly expensive
// on cross-boundary filesystems (e.g. WSL2 drvfs mounts).
func savePDF(doc *document.Document, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("save pdf: %w", err)
	}
	bw := bufio.NewWriterSize(f, 256*1024)
	if _, err := doc.WriteTo(bw); err != nil {
		_ = f.Close()
		return fmt.Errorf("save pdf: %w", err)
	}
	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return fmt.Errorf("save pdf: %w", err)
	}
	return f.Close()
}

func applyMetadata(doc *document.Document, c *content.Content) {
	if doc == nil || c == nil || c.Book == nil {
		return
	}

	book := c.Book
	doc.Info.Title = book.Description.TitleInfo.BookTitle.Value

	var authors []string
	for _, a := range book.Description.TitleInfo.Authors {
		name := strings.TrimSpace(strings.Join([]string{a.FirstName, a.MiddleName, a.LastName}, " "))
		if name != "" {
			authors = append(authors, name)
		}
	}
	if len(authors) > 0 {
		doc.Info.Author = strings.Join(authors, ", ")
	}

	if book.Description.TitleInfo.Annotation != nil {
		doc.Info.Subject = book.Description.TitleInfo.Annotation.AsPlainText()
	}
	if c.SrcName != "" {
		doc.Info.Creator = "fbc"
	}
}

func addPlan(rc *renderContext, plan *structure.Plan) error {
	if rc == nil || rc.doc == nil || plan == nil {
		return nil
	}

	if len(plan.Units) == 0 {
		rc.doc.Add(newParagraphElement(rc, "p", "", nil, defaultResolvedStyle(), "Empty document"))
		return nil
	}

	for i := range plan.Units {
		unit := &plan.Units[i]
		if i > 0 && unit.ForceNewPage {
			rc.doc.Add(layout.NewAreaBreak())
		}
		if err := addUnit(rc, unit); err != nil {
			return err
		}
	}

	return nil
}

func addUnit(rc *renderContext, unit *structure.Unit) error {
	if rc == nil || unit == nil {
		return nil
	}

	switch unit.Kind {
	case structure.UnitCover:
		return addCoverUnit(rc)
	case structure.UnitBodyImage:
		return addBodyImageUnit(rc, unit.Body)
	case structure.UnitBodyIntro:
		return addBodyIntroUnit(rc, unit.Body)
	case structure.UnitFootnotesBody:
		return addFootnotesBodyUnit(rc, unit.Body)
	case structure.UnitSection:
		return addSectionUnit(rc, unit)
	default:
		return nil
	}
}

func addCoverUnit(rc *renderContext) error {
	if rc == nil || rc.c == nil || rc.c.CoverID == "" {
		return nil
	}
	img := rc.c.ImagesIndex[rc.c.CoverID]
	if img == nil {
		return nil
	}
	elem, err := newWrappedImageElement(rc, img, img.Filename, "image", "image-block", nil, "", img.Filename)
	if err != nil {
		return err
	}
	rc.doc.Add(elem)
	return nil
}

func addBodyImageUnit(rc *renderContext, body *fb2.Body) error {
	if body == nil || body.Image == nil {
		return nil
	}
	elem, err := newBlockBookImageElement(rc, body.Image, "image", "image-block", nil)
	if err != nil {
		return err
	}
	rc.doc.Add(elem)
	return nil
}

func addBodyIntroUnit(rc *renderContext, body *fb2.Body) error {
	if rc == nil || body == nil {
		return nil
	}
	var elements []layout.Element
	parent := defaultResolvedStyle()
	b := flowBuilder{ctx: rc, elements: &elements, parent: parent}
	b.renderBodyIntro(body)
	for _, elem := range elements {
		rc.doc.Add(elem)
	}
	return nil
}

func addFootnotesBodyUnit(rc *renderContext, body *fb2.Body) error {
	if rc == nil || body == nil {
		return nil
	}
	var elements []layout.Element
	b := flowBuilder{ctx: rc, elements: &elements, parent: defaultResolvedStyle()}
	if body.Title != nil {
		b.renderTitleBlock(body.Title, "footnote-title", 1, false)
	}
	b.renderEpigraphs(body.Epigraphs, 1)
	for i := range body.Sections {
		b.renderFootnoteSection(&body.Sections[i])
	}
	for _, elem := range elements {
		rc.doc.Add(elem)
	}
	return nil
}

func addSectionUnit(rc *renderContext, unit *structure.Unit) error {
	if rc == nil || unit == nil || unit.Section == nil {
		return nil
	}
	queue := []splitSection{{
		section:    unit.Section,
		depth:      unit.Depth,
		titleDepth: unit.TitleDepth,
		parentUnit: unit,
	}}
	first := true
	for len(queue) > 0 {
		work := queue[0]
		queue = queue[1:]
		if !first {
			rc.doc.Add(layout.NewAreaBreak())
		}
		first = false

		elements, splits, err := renderSplitSection(rc, &work)
		if err != nil {
			return err
		}
		for _, elem := range elements {
			rc.doc.Add(elem)
		}
		next := append([]splitSection{}, splits...)
		for _, rem := range work.remaining {
			if rem.Kind != fb2.FlowSection || rem.Section == nil {
				continue
			}
			next = append(next, splitSection{
				section:    rem.Section,
				depth:      work.depth,
				titleDepth: work.titleDepth,
				parentUnit: work.parentUnit,
			})
		}
		queue = append(next, queue...)
	}
	return nil
}

func renderSplitSection(rc *renderContext, work *splitSection) ([]layout.Element, []splitSection, error) {
	if rc == nil || work == nil || work.section == nil {
		return nil, nil, nil
	}
	var elements []layout.Element
	b := flowBuilder{ctx: rc, elements: &elements, parent: defaultResolvedStyle()}
	splits := b.renderSection(work.section, work.depth, work.titleDepth)
	for i := range splits {
		splits[i].parentUnit = work.parentUnit
	}
	return elements, splits, nil
}

func (b *flowBuilder) renderBodyIntro(body *fb2.Body) {
	if body == nil {
		return
	}
	if !b.ctx.c.Book.BodyTitleNeedsBreak() && body.Image != nil {
		b.renderImage(body.Image, "image image-block", nil)
	}
	if body.Title != nil {
		b.renderBodyTitle(body)
	}
	b.renderEpigraphs(body.Epigraphs, 1)
}

func (b *flowBuilder) renderBodyTitle(body *fb2.Body) {
	if body == nil || body.Title == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "body-title", b.resolve("div", "body-title"), &elems)
	if body.Main() {
		child.renderVignette(common.VignettePosBookTitleTop, "vignette vignette-book-title-top")
	}
	child.renderTitleHeading(body.Title, 1, "body-title-header")
	if body.Main() {
		child.renderVignette(common.VignettePosBookTitleBottom, "vignette vignette-book-title-bottom")
	}
	b.pushWrapped("div", "body-title", elems)
}

func (b *flowBuilder) renderFootnoteSection(section *fb2.Section) {
	if section == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "footnote", b.resolve("div", "footnote"), &elems)
	if section.Title != nil {
		child.renderTitleBlock(section.Title, "footnote-title", 1, false)
	}
	child.renderEpigraphs(section.Epigraphs, 1)
	if section.Image != nil {
		child.renderImage(section.Image, "image image-block", nil)
	}
	if section.Annotation != nil {
		child.renderAnnotation(section.Annotation, 1, 1)
	}
	child.renderFlowItems(section.Content, 1, 1, "section")
	b.pushWrapped("div", "footnote", elems)
}

func (b *flowBuilder) renderSection(section *fb2.Section, depth int, titleDepth int) []splitSection {
	if section == nil {
		return nil
	}
	if section.Title != nil {
		b.renderSectionTitle(section, titleDepth)
	}
	b.renderEpigraphs(section.Epigraphs, depth)
	if section.Image != nil {
		b.renderImage(section.Image, "image image-block", nil)
	}

	contentTitleDepth := titleDepth
	if section.HasTitle() {
		contentTitleDepth = titleDepth + 1
	}
	if section.Annotation != nil {
		b.renderAnnotation(section.Annotation, depth, contentTitleDepth)
	}
	splits := b.renderFlowItems(section.Content, depth, contentTitleDepth, "section")
	if len(splits) == 0 && section.Title != nil {
		if titleDepth == 1 {
			b.renderVignette(common.VignettePosChapterEnd, "vignette vignette-chapter-end")
		} else {
			b.renderVignette(common.VignettePosSectionEnd, "vignette vignette-section-end")
		}
	}
	return splits
}

func (b *flowBuilder) renderSectionTitle(section *fb2.Section, titleDepth int) {
	if section == nil || section.Title == nil {
		return
	}
	var wrapperClass string
	var headerClass string
	var topPos, bottomPos common.VignettePos
	if titleDepth == 1 {
		wrapperClass = "chapter-title"
		headerClass = "chapter-title-header"
		topPos = common.VignettePosChapterTitleTop
		bottomPos = common.VignettePosChapterTitleBottom
	} else {
		wrapperClass = fmt.Sprintf("section-title section-title-h%d", min(titleDepth, 6))
		headerClass = "section-title-header"
		topPos = common.VignettePosSectionTitleTop
		bottomPos = common.VignettePosSectionTitleBottom
	}

	var elems []layout.Element
	child := b.descend("div", wrapperClass, b.resolve("div", wrapperClass), &elems)
	child.renderVignette(topPos, "vignette vignette-"+topPos.String())
	child.renderTitleHeading(section.Title, titleDepth, headerClass)
	child.renderVignette(bottomPos, "vignette vignette-"+bottomPos.String())
	b.pushWrapped("div", wrapperClass, elems)
}

func (b *flowBuilder) renderTitleBlock(title *fb2.Title, classPrefix string, headingLevel int, asHeading bool) {
	if title == nil {
		return
	}
	if asHeading {
		b.renderTitleHeading(title, headingLevel, classPrefix)
		return
	}
	var elems []layout.Element
	child := b.descend("div", classPrefix, b.resolve("div", classPrefix), &elems)
	firstParagraph := true
	for _, item := range title.Items {
		if item.Paragraph != nil {
			class := classPrefix + "-next"
			if firstParagraph {
				class = classPrefix + "-first"
				firstParagraph = false
			}
			if item.Paragraph.Style != "" {
				class += " " + item.Paragraph.Style
			}
			child.renderParagraph(item.Paragraph, "p", class)
			continue
		}
		if item.EmptyLine {
			child.renderEmptyLine(classPrefix + "-emptyline")
		}
	}
	b.pushWrapped("div", classPrefix, elems)
}

func (b *flowBuilder) renderTitleHeading(title *fb2.Title, headingLevel int, classPrefix string) {
	if title == nil {
		return
	}
	var runs []layout.TextRun
	firstParagraph := true
	prevWasEmptyLine := false
	for i, item := range title.Items {
		if item.Paragraph != nil {
			if i > 0 && !prevWasEmptyLine {
				runs = append(runs, b.textRunForClass("\n", classPrefix+"-break", textContext{ancestors: b.ancestors, parent: b.parent, hyphenate: false}))
			}
			class := classPrefix + "-next"
			if firstParagraph {
				class = classPrefix + "-first"
				firstParagraph = false
			}
			if item.Paragraph.Style != "" {
				class += " " + item.Paragraph.Style
			}
			runs = append(runs, b.paragraphRuns(item.Paragraph, class, textContext{ancestors: b.ancestors, parent: b.parent, hyphenate: !item.Paragraph.Special && b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil})...)
			prevWasEmptyLine = false
			continue
		}
		if item.EmptyLine {
			runs = append(runs, b.textRunForClass("\n\n", classPrefix+"-emptyline", textContext{ancestors: b.ancestors, parent: b.parent, hyphenate: false}))
			prevWasEmptyLine = true
		}
	}
	if len(runs) == 0 {
		return
	}
	headingClass := classPrefix
	heading := newHeadingElement(b.ctx, headingTag(headingLevel), headingClass, b.ancestors, b.parent, headingLevel, runs)
	*b.elements = append(*b.elements, heading)
}

func headingTag(level int) string {
	switch min(max(level, 1), 6) {
	case 1:
		return "h1"
	case 2:
		return "h2"
	case 3:
		return "h3"
	case 4:
		return "h4"
	case 5:
		return "h5"
	default:
		return "h6"
	}
}

func (b *flowBuilder) renderEpigraphs(epigraphs []fb2.Epigraph, depth int) {
	for i := range epigraphs {
		epigraph := &epigraphs[i]
		var elems []layout.Element
		child := b.descend("div", "epigraph", b.resolve("div", "epigraph"), &elems)
		child.renderFlowItems(epigraph.Flow.Items, depth, depth, "epigraph")
		for j := range epigraph.TextAuthors {
			child.renderParagraph(&epigraph.TextAuthors[j], "p", "text-author")
		}
		b.pushWrapped("div", "epigraph", elems)
	}
}

func (b *flowBuilder) renderAnnotation(flow *fb2.Flow, depth int, titleDepth int) {
	if flow == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "annotation", b.resolve("div", "annotation"), &elems)
	child.renderFlowItems(flow.Items, depth, titleDepth, "annotation")
	b.pushWrapped("div", "annotation", elems)
}

func (b *flowBuilder) renderFlowItems(items []fb2.FlowItem, depth int, titleDepth int, context string) []splitSection {
	var splits []splitSection
	for i := range items {
		item := &items[i]
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				b.renderParagraph(item.Paragraph, "p", item.Paragraph.Style)
			}
		case fb2.FlowImage:
			if item.Image != nil {
				b.renderImage(item.Image, "image image-block", nil)
			}
		case fb2.FlowEmptyLine:
			b.renderEmptyLine("emptyline")
		case fb2.FlowSubtitle:
			if item.Subtitle != nil {
				class := context + "-subtitle"
				if item.Subtitle.Style != "" {
					class += " " + item.Subtitle.Style
				}
				b.renderParagraph(item.Subtitle, "p", class)
			}
		case fb2.FlowPoem:
			if item.Poem != nil {
				b.renderPoem(item.Poem, depth)
			}
		case fb2.FlowCite:
			if item.Cite != nil {
				b.renderCite(item.Cite, depth)
			}
		case fb2.FlowTable:
			if item.Table != nil {
				b.renderTable(item.Table)
			}
		case fb2.FlowSection:
			if item.Section == nil {
				continue
			}
			sectionDepth := depth + 1
			childTitleDepth := titleDepth
			if item.Section.HasTitle() && b.ctx.c.Book.SectionNeedsBreak(sectionDepth) {
				return []splitSection{{
					section:    item.Section,
					depth:      sectionDepth,
					titleDepth: childTitleDepth,
					remaining:  items[i+1:],
				}}
			}
			var elems []layout.Element
			child := b.descend("div", "section", b.resolve("div", "section"), &elems)
			childSplits := child.renderSection(item.Section, sectionDepth, childTitleDepth)
			if len(childSplits) > 0 {
				last := &childSplits[len(childSplits)-1]
				last.remaining = append(last.remaining, items[i+1:]...)
				b.pushWrapped("div", "section", elems)
				return childSplits
			}
			b.pushWrapped("div", "section", elems)
		}
	}
	return splits
}

func (b *flowBuilder) renderPoem(poem *fb2.Poem, depth int) {
	if poem == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "poem", b.resolve("div", "poem"), &elems)
	if poem.Title != nil {
		child.renderTitleBlock(poem.Title, "poem-title", depth, false)
	}
	child.renderEpigraphs(poem.Epigraphs, depth)
	for i := range poem.Subtitles {
		child.renderParagraph(&poem.Subtitles[i], "p", "poem-subtitle")
	}
	for i := range poem.Stanzas {
		child.renderStanza(&poem.Stanzas[i], depth)
	}
	for i := range poem.TextAuthors {
		child.renderParagraph(&poem.TextAuthors[i], "p", "text-author")
	}
	if poem.Date != nil {
		dateText := poem.Date.Display
		if dateText == "" && !poem.Date.Value.IsZero() {
			dateText = poem.Date.Value.Format("2006-01-02")
		}
		if dateText != "" {
			child.renderPlainParagraph("p", "date", dateText)
		}
	}
	b.pushWrapped("div", "poem", elems)
}

func (b *flowBuilder) renderStanza(stanza *fb2.Stanza, depth int) {
	if stanza == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("div", "stanza", b.resolve("div", "stanza"), &elems)
	if stanza.Title != nil {
		child.renderTitleBlock(stanza.Title, "stanza-title", depth, false)
	}
	if stanza.Subtitle != nil {
		child.renderParagraph(stanza.Subtitle, "p", "stanza-subtitle")
	}
	for i := range stanza.Verses {
		child.renderParagraph(&stanza.Verses[i], "p", "verse")
	}
	b.pushWrapped("div", "stanza", elems)
}

func (b *flowBuilder) renderCite(cite *fb2.Cite, depth int) {
	if cite == nil {
		return
	}
	var elems []layout.Element
	child := b.descend("blockquote", "cite", b.resolve("blockquote", "cite"), &elems)
	child.renderFlowItems(cite.Items, depth, depth, "cite")
	for i := range cite.TextAuthors {
		child.renderParagraph(&cite.TextAuthors[i], "p", "text-author")
	}
	b.pushWrapped("blockquote", "cite", elems)
}

func (b *flowBuilder) renderTable(table *fb2.Table) {
	if table == nil {
		return
	}
	style := b.resolve("table", table.Style)
	tbl := layout.NewTable().SetAutoColumnWidths().SetBorderCollapse(true)
	if style.WidthPercent > 0 {
		tbl.SetMinWidthUnit(layout.Pct(style.WidthPercent))
	} else {
		tbl.SetMinWidthUnit(layout.Pct(100))
	}
	for i := range table.Rows {
		rowData := &table.Rows[i]
		row := tbl.AddRow()
		for j := range rowData.Cells {
			cellData := &rowData.Cells[j]
			cellElem := b.newTableCellElement(cellData)
			cell := row.AddCellElement(cellElem)
			if cellData.ColSpan > 1 {
				cell.SetColspan(cellData.ColSpan)
			}
			if cellData.RowSpan > 1 {
				cell.SetRowspan(cellData.RowSpan)
			}
			cellStyle := b.resolve(tableCellTag(cellData), cellData.Style)
			applyCellStyle(cell, cellStyle)
		}
	}
	*b.elements = append(*b.elements, wrapBlockElement(style, tbl))
}

func tableCellTag(cell *fb2.TableCell) string {
	if cell != nil && cell.Header {
		return "th"
	}
	return "td"
}

func applyCellStyle(cell *layout.Cell, style resolvedStyle) {
	if cell == nil {
		return
	}
	if style.HasBorder {
		cell.SetBorders(layout.AllBorders(style.Border))
	}
	if style.Background != nil {
		cell.SetBackground(*style.Background)
	}
	if style.PaddingTop != 0 || style.PaddingRight != 0 || style.PaddingBottom != 0 || style.PaddingLeft != 0 {
		cell.SetPaddingSides(layout.Padding{
			Top:    max(style.PaddingTop, 0),
			Right:  max(style.PaddingRight, 0),
			Bottom: max(style.PaddingBottom, 0),
			Left:   max(style.PaddingLeft, 0),
		})
	}
	cell.SetAlign(style.Align)
}

func (b *flowBuilder) newTableCellElement(cell *fb2.TableCell) layout.Element {
	class := ""
	if cell != nil {
		class = cell.Style
	}
	style := b.resolve(tableCellTag(cell), class)
	ancestors := append(append([]styleScope{}, b.ancestors...), styleScope{Tag: tableCellTag(cell), Classes: splitClasses(class)})
	var runs []layout.TextRun
	if cell != nil {
		for i := range cell.Content {
			runs = append(runs, b.inlineRuns(&cell.Content[i], textContext{ancestors: ancestors, parent: style, hyphenate: false})...)
		}
	}
	if len(runs) == 0 {
		para := newParagraphElement(b.ctx, tableCellTag(cell), class, b.ancestors, b.parent, strings.TrimSpace(cell.AsPlainText()))
		applyParagraphStyle(para, style)
		return para
	}
	para := layout.NewStyledParagraph(runs...)
	applyParagraphStyle(para, style)
	return para
}

func (b *flowBuilder) renderParagraph(p *fb2.Paragraph, tag string, class string) {
	if p == nil {
		return
	}
	runs := b.paragraphRuns(p, class, textContext{ancestors: b.ancestors, parent: b.parent, hyphenate: !p.Special && b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil})
	if len(runs) == 0 {
		plain := strings.TrimSpace(p.AsPlainText())
		if plain == "" {
			style := b.resolve(tag, class)
			spacer := layout.NewDiv().SetSpaceBefore(style.MarginTop).SetSpaceAfter(style.MarginBottom)
			*b.elements = append(*b.elements, spacer)
			return
		}
		runs = []layout.TextRun{b.textRunForStyle(plain, b.resolve(tag, class))}
	}
	para := newStyledParagraphElement(b.ctx, tag, class, b.ancestors, b.parent, runs)
	*b.elements = append(*b.elements, para)
}

func (b *flowBuilder) renderPlainParagraph(tag string, class string, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	para := newParagraphElement(b.ctx, tag, class, b.ancestors, b.parent, text)
	*b.elements = append(*b.elements, para)
}

func (b *flowBuilder) renderEmptyLine(class string) {
	style := b.resolve("div", class)
	spacer := layout.NewDiv().SetSpaceBefore(style.MarginTop).SetSpaceAfter(style.MarginBottom)
	*b.elements = append(*b.elements, wrapIfNeeded("div", class, style, spacer))
}

func (b *flowBuilder) renderImage(img *fb2.Image, class string, extraAncestors []styleScope) {
	if img == nil {
		return
	}
	elem, err := newBlockBookImageElement(b.ctx, img, "image", class, extraAncestors)
	if err != nil {
		b.ctx.log.Warn("Skipping image", zap.String("href", img.Href), zap.Error(err))
		return
	}
	*b.elements = append(*b.elements, elem)
}

func (b *flowBuilder) renderVignette(pos common.VignettePos, class string) {
	if b.ctx == nil || b.ctx.c == nil || b.ctx.c.Book == nil || !b.ctx.c.Book.IsVignetteEnabled(pos) {
		return
	}
	id := b.ctx.c.Book.VignetteIDs[pos]
	img := b.ctx.c.ImagesIndex[id]
	if img == nil {
		return
	}
	elem, err := newWrappedImageElement(b.ctx, img, id, class, "image-vignette", nil, "", img.Filename)
	if err != nil {
		b.ctx.log.Warn("Skipping vignette", zap.String("id", id), zap.Error(err))
		return
	}
	*b.elements = append(*b.elements, elem)
}

func (b *flowBuilder) resolve(tag, classes string) resolvedStyle {
	if b.ctx == nil || b.ctx.styles == nil {
		return defaultResolvedStyle()
	}
	return b.ctx.styles.Resolve(tag, classes, b.ancestors, b.parent)
}

func (b *flowBuilder) descend(tag, classes string, style resolvedStyle, out *[]layout.Element) flowBuilder {
	ancestors := append([]styleScope{}, b.ancestors...)
	ancestors = append(ancestors, styleScope{Tag: tag, Classes: splitClasses(classes)})
	return flowBuilder{
		ctx:       b.ctx,
		elements:  out,
		ancestors: ancestors,
		parent:    style,
	}
}

func (b *flowBuilder) pushWrapped(tag, classes string, elems []layout.Element) {
	if len(elems) == 0 {
		return
	}
	style := b.resolve(tag, classes)
	container := layout.NewDiv()
	for _, elem := range elems {
		container.Add(elem)
	}
	applyDivStyle(container, style)
	if tag == "blockquote" {
		container.SetTag("BlockQuote")
	} else if tag == "div" && strings.Contains(classes, "section") {
		container.SetTag("Sect")
	}
	*b.elements = append(*b.elements, container)
}

func wrapIfNeeded(tag, classes string, style resolvedStyle, elem layout.Element) layout.Element {
	if style.PaddingTop == 0 && style.PaddingRight == 0 && style.PaddingBottom == 0 && style.PaddingLeft == 0 &&
		style.Background == nil && !style.HasBorder && !style.KeepTogether && style.WidthPercent == 0 {
		return elem
	}
	div := layout.NewDiv().Add(elem)
	applyDivStyle(div, style)
	if tag == "blockquote" {
		div.SetTag("BlockQuote")
	}
	return div
}

func newBlockBookImageElement(rc *renderContext, img *fb2.Image, wrapperClass string, imageClass string, extraAncestors []styleScope) (layout.Element, error) {
	if rc == nil || rc.c == nil || img == nil {
		return nil, fmt.Errorf("nil image")
	}
	bookImg, id := imageByID(rc.c, img.Href)
	if bookImg == nil {
		return nil, fmt.Errorf("image %q not found", img.Href)
	}
	return newWrappedImageElement(rc, bookImg, id, wrapperClass, imageClass, extraAncestors, img.Alt, img.Title)
}

func newWrappedImageElement(rc *renderContext, img *fb2.BookImage, imageID string, wrapperClass string, imageClass string, extraAncestors []styleScope, alt string, title string) (layout.Element, error) {
	if rc == nil || img == nil {
		return nil, fmt.Errorf("nil image")
	}
	var wrappedAncestors []styleScope
	style := defaultResolvedStyle()
	if rc.styles != nil {
		style = rc.styles.Resolve("div", wrapperClass, extraAncestors, defaultResolvedStyle())
		wrappedAncestors = append(append([]styleScope{}, extraAncestors...), styleScope{Tag: "div", Classes: splitClasses(wrapperClass)})
	} else {
		wrappedAncestors = append([]styleScope{}, extraAncestors...)
	}
	imageElem, err := newImageElement(rc, img, imageID, imageClass, wrappedAncestors, alt, title)
	if err != nil {
		return nil, err
	}
	return wrapBlockElement(style, imageElem), nil
}

func newImageElement(rc *renderContext, img *fb2.BookImage, imageID string, class string, extraAncestors []styleScope, alt string, title string) (layout.Element, error) {
	if rc == nil || img == nil {
		return nil, fmt.Errorf("nil image")
	}
	pdfImg, err := newPDFImage(img)
	if err != nil {
		return nil, err
	}
	style := defaultResolvedStyle()
	if rc.styles != nil {
		style = rc.styles.Resolve("img", class, extraAncestors, defaultResolvedStyle())
	}
	imageElem := layout.NewImageElement(pdfImg)
	applyImageStyle(imageElem, style, img, rc.contentHeight)
	altText := strings.TrimSpace(alt)
	if altText == "" {
		altText = strings.TrimSpace(title)
	}
	if altText == "" {
		altText = imageID
	}
	imageElem.SetAltText(altText)
	return wrapIfNeeded("img", class, style, imageElem), nil
}

func wrapBlockElement(style resolvedStyle, elem layout.Element) layout.Element {
	if elem == nil {
		return nil
	}
	div := layout.NewDiv().Add(elem)
	applyDivStyle(div, style)
	return div
}

func newParagraphElement(rc *renderContext, tag, classes string, ancestors []styleScope, parent resolvedStyle, text string) *layout.Paragraph {
	style := defaultResolvedStyle()
	if rc != nil && rc.styles != nil {
		style = rc.styles.Resolve(tag, classes, ancestors, parent)
	}
	para := newParagraphWithStyle(rc, style, text)
	applyParagraphStyle(para, style)
	return para
}

func newStyledParagraphElement(rc *renderContext, tag, classes string, ancestors []styleScope, parent resolvedStyle, runs []layout.TextRun) layout.Element {
	style := defaultResolvedStyle()
	if rc != nil && rc.styles != nil {
		style = rc.styles.Resolve(tag, classes, ancestors, parent)
	}
	para := layout.NewStyledParagraph(runs...)
	applyParagraphStyle(para, style)
	return wrapIfNeeded(tag, classes, style, para)
}

func newHeadingElement(rc *renderContext, tag, classes string, ancestors []styleScope, parent resolvedStyle, level int, runs []layout.TextRun) layout.Element {
	style := defaultResolvedStyle()
	if rc != nil && rc.styles != nil {
		style = rc.styles.Resolve(tag, classes, ancestors, parent)
	}
	headingLevel := layout.H1
	switch min(max(level, 1), 6) {
	case 1:
		headingLevel = layout.H1
	case 2:
		headingLevel = layout.H2
	case 3:
		headingLevel = layout.H3
	case 4:
		headingLevel = layout.H4
	case 5:
		headingLevel = layout.H5
	case 6:
		headingLevel = layout.H6
	}
	std, emb := rc.fonts.resolve(style, plainTextRuns(runs))
	var heading *layout.Heading
	if emb != nil {
		heading = layout.NewHeadingEmbedded(plainTextRuns(runs), headingLevel, emb)
	} else {
		heading = layout.NewHeadingWithFont(plainTextRuns(runs), headingLevel, std, style.FontSize)
	}
	heading.SetRuns(runs).SetAlign(style.Align)
	return wrapIfNeeded(tag, classes, style, heading)
}

func newParagraphWithStyle(rc *renderContext, style resolvedStyle, text string) *layout.Paragraph {
	std, emb := rc.fonts.resolve(style, text)
	if emb != nil {
		return layout.NewParagraphEmbedded(text, emb, style.FontSize)
	}
	return layout.NewParagraph(text, std, style.FontSize)
}

func applyParagraphStyle(para *layout.Paragraph, style resolvedStyle) {
	if para == nil {
		return
	}
	para.SetAlign(style.Align)
	para.SetLeading(style.LineHeight)
	para.SetSpaceBefore(style.MarginTop)
	para.SetSpaceAfter(style.MarginBottom)
	para.SetFirstLineIndent(style.TextIndent)
	if style.Background != nil {
		para.SetBackground(*style.Background)
	}
	if style.WhiteSpace == "pre-wrap" {
		para.SetHyphens("manual")
	}
}

func applyDivStyle(div *layout.Div, style resolvedStyle) {
	if div == nil {
		return
	}
	div.SetSpaceBefore(style.MarginTop)
	div.SetSpaceAfter(style.MarginBottom)
	if style.PaddingTop != 0 || style.PaddingRight != 0 || style.PaddingBottom != 0 || style.PaddingLeft != 0 {
		div.SetPaddingAll(layout.Padding{
			Top:    max(style.PaddingTop, 0),
			Right:  max(style.PaddingRight, 0),
			Bottom: max(style.PaddingBottom, 0),
			Left:   max(style.PaddingLeft, 0),
		})
	}
	if style.Background != nil {
		div.SetBackground(*style.Background)
	}
	if style.HasBorder {
		div.SetBorder(style.Border)
	}
	if style.KeepTogether {
		div.SetKeepTogether(true)
	}
	if style.WidthPercent > 0 {
		div.SetWidthPercent(style.WidthPercent / 100.0)
	}
}

func applyImageStyle(elem *layout.ImageElement, style resolvedStyle, img *fb2.BookImage, maxContentHeight float64) {
	if elem == nil {
		return
	}
	elem.SetAlign(style.Align)
	if style.WidthPercent > 0 {
		return
	}
	if img == nil || img.Dim.Width <= 0 || img.Dim.Height <= 0 {
		return
	}
	w := PxToPt(img.Dim.Width, 1)
	h := PxToPt(img.Dim.Height, 1)
	// Scale down proportionally so the image fits within the usable page
	// content height. Without this, images taller than the page cause the
	// folio layout engine to loop infinitely (Div returns LayoutPartial
	// with zero progress when its child image returns LayoutNothing).
	if maxContentHeight > 0 && h > maxContentHeight {
		scale := maxContentHeight / h
		w *= scale
		h = maxContentHeight
	}
	if w > 0 || h > 0 {
		elem.SetSize(w, h)
	}
}

func (b *flowBuilder) paragraphRuns(p *fb2.Paragraph, class string, tc textContext) []layout.TextRun {
	if p == nil {
		return nil
	}
	tag := "p"
	style := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		style = b.ctx.styles.Resolve(tag, class, tc.ancestors, tc.parent)
	}
	segments := p.Text
	if hasStyle("has-dropcap", p.Style) {
		segments = splitDropcapSegments(segments)
	}
	var runs []layout.TextRun
	for i := range segments {
		runs = append(runs, b.inlineRuns(&segments[i], textContext{
			ancestors: append(append([]styleScope{}, tc.ancestors...), styleScope{Tag: tag, Classes: splitClasses(class)}),
			parent:    style,
			linkURI:   tc.linkURI,
			hyphenate: tc.hyphenate,
		})...)
	}
	return runs
}

func (b *flowBuilder) inlineRuns(seg *fb2.InlineSegment, tc textContext) []layout.TextRun {
	if seg == nil {
		return nil
	}
	classes := inlineSegmentClass(seg, b.ctx.c)
	tag := inlineSegmentTag(seg)
	style := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		style = b.ctx.styles.Resolve(tag, classes, tc.ancestors, tc.parent)
	}
	pseudo := pseudoContent{}
	if b.ctx != nil && b.ctx.styles != nil {
		pseudo = b.ctx.styles.ResolvePseudo(tag, classes, tc.ancestors, tc.parent)
	}
	linkURI := tc.linkURI
	if seg.Kind == fb2.InlineLink && seg.Href != "" && !strings.HasPrefix(seg.Href, "#") {
		linkURI = seg.Href
	}

	var runs []layout.TextRun
	appendText := func(text string) {
		if text == "" {
			return
		}
		run := b.textRunForStyle(text, style)
		if linkURI != "" {
			run.LinkURI = linkURI
		}
		runs = append(runs, run)
	}
	appendText(pseudo.Before)
	if seg.Text != "" {
		text := seg.Text
		if tc.hyphenate && !isCodeSegment(seg) && b.ctx != nil && b.ctx.c != nil && b.ctx.c.Hyphen != nil {
			text = b.ctx.c.Hyphen.Hyphenate(text)
		}
		appendText(text)
	}
	if seg.Kind == fb2.InlineImageSegment && seg.Image != nil {
		if run := b.inlineImageRun(seg.Image, tc); run != nil {
			runs = append(runs, *run)
		}
	}
	childHyphenate := tc.hyphenate && !isCodeSegment(seg)
	childAncestors := append(append([]styleScope{}, tc.ancestors...), styleScope{Tag: tag, Classes: splitClasses(classes)})
	for i := range seg.Children {
		runs = append(runs, b.inlineRuns(&seg.Children[i], textContext{ancestors: childAncestors, parent: style, linkURI: linkURI, hyphenate: childHyphenate})...)
	}
	appendText(pseudo.After)
	return runs
}

func isCodeSegment(seg *fb2.InlineSegment) bool {
	return seg != nil && seg.Kind == fb2.InlineCode
}

func (b *flowBuilder) inlineImageRun(img *fb2.InlineImage, tc textContext) *layout.TextRun {
	if img == nil {
		return nil
	}
	bookImg, id := imageByID(b.ctx.c, img.Href)
	if bookImg == nil {
		if img.Alt == "" {
			return nil
		}
		run := b.textRunForStyle(img.Alt, tc.parent)
		return &run
	}
	elem, err := newImageElement(b.ctx, bookImg, id, "image-inline", tc.ancestors, img.Alt, "")
	if err != nil {
		if img.Alt == "" {
			return nil
		}
		run := b.textRunForStyle(img.Alt, tc.parent)
		return &run
	}
	run := layout.RunInline(elem)
	return &run
}

func (b *flowBuilder) textRunForClass(text string, classes string, tc textContext) layout.TextRun {
	style := defaultResolvedStyle()
	if b.ctx != nil && b.ctx.styles != nil {
		style = b.ctx.styles.Resolve("span", classes, tc.ancestors, tc.parent)
	}
	run := b.textRunForStyle(text, style)
	if tc.linkURI != "" {
		run.LinkURI = tc.linkURI
	}
	return run
}

func (b *flowBuilder) textRunForStyle(text string, style resolvedStyle) layout.TextRun {
	std, emb := b.ctx.fonts.resolve(style, text)
	var run layout.TextRun
	if emb != nil {
		run = layout.NewRunEmbedded(text, emb, style.FontSize)
	} else {
		run = layout.NewRun(text, std, style.FontSize)
	}
	if style.HasColor {
		run.Color = style.Color
	}
	if style.Underline {
		run.Decoration |= layout.DecorationUnderline
	}
	if style.Strike {
		run.Decoration |= layout.DecorationStrikethrough
	}
	run.BaselineShift = style.BaselineShift
	if style.Background != nil {
		run.BackgroundColor = style.Background
	}
	return run
}

func inlineSegmentTag(seg *fb2.InlineSegment) string {
	if seg == nil {
		return "span"
	}
	switch seg.Kind {
	case fb2.InlineStrong:
		return "strong"
	case fb2.InlineEmphasis:
		return "em"
	case fb2.InlineStrikethrough:
		return "del"
	case fb2.InlineSub:
		return "sub"
	case fb2.InlineSup:
		return "sup"
	case fb2.InlineCode:
		return "code"
	case fb2.InlineLink:
		return "a"
	default:
		return "span"
	}
}

func inlineSegmentClass(seg *fb2.InlineSegment, c *content.Content) string {
	if seg == nil {
		return ""
	}
	switch seg.Kind {
	case fb2.InlineStrong:
		return "strong"
	case fb2.InlineEmphasis:
		return "emphasis"
	case fb2.InlineStrikethrough:
		return "strikethrough"
	case fb2.InlineSub:
		return "sub"
	case fb2.InlineSup:
		return "sup"
	case fb2.InlineCode:
		return "code"
	case fb2.InlineNamedStyle:
		return seg.Style
	case fb2.InlineLink:
		if after, ok := strings.CutPrefix(seg.Href, "#"); ok {
			if c != nil {
				if _, isFootnote := c.FootnotesIndex[after]; isFootnote {
					return "link-footnote"
				}
			}
			return "link-internal"
		}
		return "link-external"
	default:
		return ""
	}
}

func plainTextRuns(runs []layout.TextRun) string {
	var sb strings.Builder
	for _, run := range runs {
		sb.WriteString(run.Text)
	}
	return strings.TrimSpace(sb.String())
}

func splitDropcapSegments(segments []fb2.InlineSegment) []fb2.InlineSegment {
	clone := append([]fb2.InlineSegment(nil), segments...)
	for i := range clone {
		seg := &clone[i]
		if seg.Kind != fb2.InlineText || seg.Text == "" {
			continue
		}
		r, size := utf8.DecodeRuneInString(seg.Text)
		if r == utf8.RuneError && size == 0 {
			continue
		}
		drop := fb2.InlineSegment{
			Kind:  fb2.InlineNamedStyle,
			Style: "dropcap",
			Children: []fb2.InlineSegment{{
				Kind: fb2.InlineText,
				Text: string(r),
			}},
		}
		rest := seg.Text[size:]
		clone = append(clone[:i], append([]fb2.InlineSegment{drop}, clone[i:]...)...)
		clone[i+1].Text = rest
		if clone[i+1].Text == "" && len(clone[i+1].Children) == 0 {
			clone = append(clone[:i+1], clone[i+2:]...)
		}
		return clone
	}
	return clone
}

func hasStyle(style, styles string) bool {
	if styles == "" {
		return false
	}
	if style == "" {
		return true
	}
	for part := range strings.FieldsSeq(styles) {
		if part == style {
			return true
		}
	}
	return false
}
