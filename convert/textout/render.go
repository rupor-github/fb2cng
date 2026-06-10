package textout

import (
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/structure"
	"fbc/convert/tocnav"
	"fbc/fb2"
)

type formatKind int

const (
	formatTXT formatKind = iota
	formatMD
)

type renderer struct {
	c        *content.Content
	cfg      *config.DocumentConfig
	format   formatKind
	blocks   []string
	endnotes *endnoteQueue
	options  RenderOptions
}

// RenderOptions controls output details that depend on the final output path.
type RenderOptions struct {
	OutputPath string
}

type endnoteQueue struct {
	ids []string
	set map[string]bool
}

func newEndnoteQueue() *endnoteQueue {
	return &endnoteQueue{set: make(map[string]bool)}
}

func (q *endnoteQueue) add(id string) int {
	if q == nil || id == "" {
		return 0
	}
	if q.set[id] {
		return slices.Index(q.ids, id) + 1
	}
	q.set[id] = true
	q.ids = append(q.ids, id)
	return len(q.ids)
}

// Render converts normalized FB2 content into UTF-8 TXT or Markdown bytes.
func Render(c *content.Content, cfg *config.DocumentConfig) ([]byte, error) {
	return RenderWithOptions(c, cfg, RenderOptions{})
}

// RenderWithOptions converts normalized FB2 content into UTF-8 TXT or Markdown bytes.
func RenderWithOptions(c *content.Content, cfg *config.DocumentConfig, opts RenderOptions) ([]byte, error) {
	if c == nil || c.Book == nil {
		return nil, errors.New("content is required")
	}
	if cfg == nil {
		return nil, errors.New("document config is required")
	}

	var kind formatKind
	switch c.OutputFormat {
	case common.OutputFmtTxt:
		kind = formatTXT
	case common.OutputFmtMd:
		kind = formatMD
	default:
		return nil, fmt.Errorf("unsupported text output format: %s", c.OutputFormat)
	}

	r := &renderer{
		c:        c,
		cfg:      cfg,
		format:   kind,
		endnotes: newEndnoteQueue(),
		options:  opts,
	}
	if c.FootnotesMode.IsFloat() {
		c.BackLinkIndex = make(map[string][]content.BackLinkRef)
	}
	return r.render()
}

func (r *renderer) render() ([]byte, error) {
	plan, err := structure.BuildPlan(r.c)
	if err != nil {
		return nil, fmt.Errorf("build text structure: %w", err)
	}

	r.renderFrontMatter()
	if r.cfg.Annotation.Enable && r.c.Book.Description.TitleInfo.Annotation != nil && r.cfg.Annotation.InTOC {
		plan.TOC = append([]*structure.TOCEntry{{ID: "annotation-page", Title: annotationTitle(r.cfg), IncludeInTOC: true}}, plan.TOC...)
	}
	if r.cfg.TOCPage.Placement == common.TOCPagePlacementBefore {
		r.renderTOC(plan.TOC)
	}
	if r.cfg.Annotation.Enable && r.c.Book.Description.TitleInfo.Annotation != nil {
		r.renderAnnotation()
	}
	for i := range plan.Units {
		r.renderUnit(&plan.Units[i])
	}
	if r.cfg.TOCPage.Placement == common.TOCPagePlacementAfter {
		r.renderTOC(plan.TOC)
	}
	if r.c.FootnotesMode.IsFloat() {
		r.renderEndnotes()
	}

	out := strings.TrimSpace(strings.Join(r.blocks, "\n\n")) + "\n"
	return []byte(out), nil
}

func (r *renderer) renderFrontMatter() {
	info := r.c.Book.Description.TitleInfo
	title := strings.TrimSpace(info.BookTitle.Value)
	if title == "" {
		title = strings.TrimSuffix(r.c.SrcName, ".fb2")
	}
	r.heading(r.plainInline(title), 1)

	lines := make([]string, 0, 6)
	if authors := formatAuthors(info.Authors); authors != "" {
		lines = append(lines, "Authors: "+r.plainInline(authors))
	}
	if series := formatSeries(info.Sequences); series != "" {
		lines = append(lines, "Series: "+r.plainInline(series))
	}
	if lang := strings.TrimSpace(info.Lang.String()); lang != "" && lang != "und" {
		lines = append(lines, "Language: "+r.plainInline(lang))
	}
	if date := formatDate(info.Date); date != "" {
		lines = append(lines, "Date: "+r.plainInline(date))
	}
	if genres := formatGenres(info.Genres); genres != "" {
		lines = append(lines, "Genres: "+r.plainInline(genres))
	}
	r.paragraph(strings.Join(lines, "\n"))
}

func (r *renderer) renderAnnotation() {
	r.heading(r.plainInline(annotationTitle(r.cfg)), 2)
	r.renderFlow(r.c.Book.Description.TitleInfo.Annotation.Items, 0, blockNormal)
}

func (r *renderer) renderTOC(entries []*structure.TOCEntry) {
	items := flattenTOCEntries(entries, r.cfg.TOCPage.ChaptersWithoutTitle, 1)
	if len(items) == 0 {
		return
	}
	r.heading("Table of Contents", 2)
	if r.format == formatMD {
		lines := make([]string, 0, len(items))
		for _, item := range tocnav.Shape(items, r.cfg.TOCType) {
			r.appendMDTOCNode(&lines, item, 0)
		}
		r.block(strings.Join(lines, "\n"))
		return
	}
	lines := make([]string, 0, len(items))
	for _, item := range tocnav.Shape(items, r.cfg.TOCType) {
		r.appendTXTTOCNode(&lines, item, 0)
	}
	r.block(strings.Join(lines, "\n"))
}

func (r *renderer) appendMDTOCNode(lines *[]string, node *tocnav.Node, depth int) {
	if node == nil {
		return
	}
	title := escapeMDInline(strings.TrimSpace(node.Item.Title))
	if title != "" {
		if anchor := markdownAnchor(tocItemAnchor(node.Item)); anchor != "" {
			title = "[" + escapeMDLinkTextRendered(title) + "](#" + escapeMDURL(anchor) + ")"
		}
		*lines = append(*lines, strings.Repeat("  ", depth)+"- "+title)
	}
	for _, child := range node.Children {
		r.appendMDTOCNode(lines, child, depth+1)
	}
}

func (r *renderer) appendTXTTOCNode(lines *[]string, node *tocnav.Node, depth int) {
	if node == nil {
		return
	}
	title := strings.TrimSpace(node.Item.Title)
	if title != "" {
		*lines = append(*lines, strings.Repeat("  ", depth)+"- "+title)
	}
	for _, child := range node.Children {
		r.appendTXTTOCNode(lines, child, depth+1)
	}
}

func (r *renderer) renderUnit(unit *structure.Unit) {
	if unit == nil {
		return
	}
	switch unit.Kind {
	case structure.UnitCover:
		return
	case structure.UnitBodyImage:
		if unit.Body != nil {
			r.image(unit.Body.Image)
		}
	case structure.UnitBodyIntro:
		if unit.Body != nil {
			r.renderBodyIntro(unit.Body, unit.ID)
		}
	case structure.UnitSection:
		if unit.Section != nil {
			r.renderSection(unit.Section, max(1, unit.TitleDepth+1))
		}
	case structure.UnitFootnotesBody:
		if unit.Body != nil {
			r.renderFootnoteBody(unit.Body, unit.ID)
		}
	}
}

func (r *renderer) renderBodyIntro(body *fb2.Body, anchorID string) {
	if body.Image != nil {
		r.image(body.Image)
	}
	if body.Title != nil {
		r.titleWithAnchor(body.Title, 2, anchorID)
	}
	for i := range body.Epigraphs {
		r.renderEpigraph(&body.Epigraphs[i])
	}
}

func (r *renderer) renderFootnoteBody(body *fb2.Body, anchorID string) {
	if body.Title != nil {
		r.titleWithAnchor(body.Title, 2, anchorID)
	} else if title := strings.TrimSpace(body.AsTitleText("Notes")); title != "" {
		r.headingWithAnchor(r.plainInline(title), 2, anchorID)
	}
	for i := range body.Sections {
		r.renderSection(&body.Sections[i], 3)
	}
}

func (r *renderer) renderSection(section *fb2.Section, depth int) {
	if section == nil {
		return
	}
	if section.Title != nil {
		r.titleWithAnchor(section.Title, depth, section.ID)
	}
	for i := range section.Epigraphs {
		r.renderEpigraph(&section.Epigraphs[i])
	}
	r.image(section.Image)
	if section.Annotation != nil {
		r.renderFlow(section.Annotation.Items, depth, blockQuote)
	}
	r.renderFlow(section.Content, depth, blockNormal)
}

type blockStyle int

const (
	blockNormal blockStyle = iota
	blockQuote
)

func (r *renderer) renderFlow(items []fb2.FlowItem, depth int, style blockStyle) {
	for i := range items {
		item := &items[i]
		switch item.Kind {
		case fb2.FlowParagraph:
			if item.Paragraph != nil {
				if r.renderCodeParagraph(item.Paragraph, style) {
					continue
				}
				r.paragraphStyled(r.renderParagraph(item.Paragraph), style)
			}
		case fb2.FlowImage:
			r.image(item.Image)
		case fb2.FlowPoem:
			r.renderPoem(item.Poem, depth)
		case fb2.FlowSubtitle:
			if item.Subtitle != nil {
				r.paragraphStyled(r.renderParagraph(item.Subtitle), style)
			}
		case fb2.FlowCite:
			r.renderCite(item.Cite, depth)
		case fb2.FlowEmptyLine:
			r.block(" ")
		case fb2.FlowTable:
			r.table(item.Table)
		case fb2.FlowSection:
			r.renderSection(item.Section, depth+1)
		}
	}
}

func (r *renderer) renderPoem(poem *fb2.Poem, depth int) {
	if poem == nil {
		return
	}
	if poem.Title != nil {
		r.title(poem.Title, depth+1)
	}
	for i := range poem.Epigraphs {
		r.renderEpigraph(&poem.Epigraphs[i])
	}
	for i := range poem.Subtitles {
		r.paragraph(r.renderParagraph(&poem.Subtitles[i]))
	}
	for i := range poem.Stanzas {
		r.renderStanza(&poem.Stanzas[i], depth+1)
	}
	for i := range poem.TextAuthors {
		r.paragraph(r.textAuthorPrefix() + r.renderParagraph(&poem.TextAuthors[i]))
	}
	if date := formatDate(poem.Date); date != "" {
		r.paragraph(date)
	}
}

func (r *renderer) renderStanza(stanza *fb2.Stanza, depth int) {
	if stanza == nil {
		return
	}
	if stanza.Title != nil {
		r.title(stanza.Title, depth+1)
	}
	if stanza.Subtitle != nil {
		r.paragraph(r.renderParagraph(stanza.Subtitle))
	}
	lines := make([]string, 0, len(stanza.Verses))
	for i := range stanza.Verses {
		line := r.renderParagraph(&stanza.Verses[i])
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return
	}
	if r.format == formatMD {
		r.block(strings.Join(lines, "  \n"))
		return
	}
	r.block(strings.Join(lines, "\n"))
}

func (r *renderer) renderCite(cite *fb2.Cite, depth int) {
	if cite == nil {
		return
	}
	before := len(r.blocks)
	r.renderFlow(cite.Items, depth, blockQuote)
	for i := range cite.TextAuthors {
		r.paragraphStyled(r.textAuthorPrefix()+r.renderParagraph(&cite.TextAuthors[i]), blockQuote)
	}
	if r.format == formatTXT {
		for i := before; i < len(r.blocks); i++ {
			r.blocks[i] = indentBlock(r.blocks[i], "  ")
		}
	}
}

func (r *renderer) renderEpigraph(epigraph *fb2.Epigraph) {
	if epigraph == nil {
		return
	}
	r.renderFlow(epigraph.Flow.Items, 0, blockQuote)
	for i := range epigraph.TextAuthors {
		r.paragraphStyled(r.textAuthorPrefix()+r.renderParagraph(&epigraph.TextAuthors[i]), blockQuote)
	}
}

func (r *renderer) textAuthorPrefix() string {
	if r.format == formatMD {
		return "\\- "
	}
	return "- "
}

func (r *renderer) renderParagraph(p *fb2.Paragraph) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(r.renderSegments(p.Text))
}

func (r *renderer) renderSegments(segments []fb2.InlineSegment) string {
	var b strings.Builder
	for i := range segments {
		b.WriteString(r.renderSegment(&segments[i]))
	}
	return b.String()
}

func (r *renderer) renderSegment(seg *fb2.InlineSegment) string {
	if seg == nil {
		return ""
	}
	text := r.plainInline(seg.Text) + r.renderSegments(seg.Children)
	switch seg.Kind {
	case fb2.InlineText, fb2.InlineNamedStyle:
		return text
	case fb2.InlineStrong:
		return r.wrapInline(text, "**", "**")
	case fb2.InlineEmphasis:
		return r.wrapInline(text, "*", "*")
	case fb2.InlineStrikethrough:
		return r.wrapInline(text, "~~", "~~")
	case fb2.InlineSub:
		return r.subSupInline(text, "sub")
	case fb2.InlineSup:
		return r.subSupInline(text, "sup")
	case fb2.InlineCode:
		raw := seg.Text + rawSegmentsText(seg.Children)
		if r.format == formatMD {
			return codeSpan(normalizeInlineWhitespace(raw))
		}
		return normalizeInlineWhitespace(raw)
	case fb2.InlineLink:
		return r.link(seg, text)
	case fb2.InlineImageSegment:
		return r.inlineImage(seg.Image)
	default:
		return text
	}
}

func (r *renderer) plainInline(text string) string {
	text = normalizeInlineWhitespace(text)
	if r.format == formatMD {
		return escapeMDInline(text)
	}
	return text
}

func (r *renderer) wrapInline(text, before, after string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	if r.format != formatMD {
		return text
	}
	leading, core, trailing := splitSurroundingWhitespace(text)
	return leading + before + core + after + trailing
}

func (r *renderer) subSupInline(text string, tag string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	if r.format != formatMD {
		return text
	}
	leading, core, trailing := splitSurroundingWhitespace(text)
	return leading + "<" + tag + ">" + core + "</" + tag + ">" + trailing
}

func (r *renderer) link(seg *fb2.InlineSegment, text string) string {
	if strings.TrimSpace(text) == "" {
		text = strings.TrimSpace(seg.Href)
	}
	linkID, internal := strings.CutPrefix(seg.Href, "#")
	if internal {
		if _, ok := r.c.FootnotesIndex[linkID]; ok {
			return r.footnoteLink(linkID, normalizeInlineWhitespace(seg.Text+rawSegmentsText(seg.Children)))
		}
		if r.format == formatMD && text != "" {
			return "[" + escapeMDLinkTextRendered(text) + "](" + escapeMDURL(seg.Href) + ")"
		}
		return text
	}
	if r.format == formatMD && seg.Href != "" {
		return "[" + escapeMDLinkTextRendered(text) + "](" + escapeMDURL(seg.Href) + ")"
	}
	if seg.Href != "" && text != seg.Href {
		return text + " (" + seg.Href + ")"
	}
	return text
}

func (r *renderer) footnoteLink(linkID string, text string) string {
	ref := r.c.FootnotesIndex[linkID]
	label := strings.TrimSpace(text)
	if !r.c.FootnotesMode.IsFloat() {
		if r.format == formatMD && label != "" {
			return "[" + escapeMDLinkTextRendered(r.footnoteLabel(label)) + "](#" + escapeMDURL(markdownAnchor(linkID)) + ")"
		}
		return text
	}
	r.c.AddFootnoteBackLinkRef(linkID)
	noteNumber := r.endnotes.add(linkID)
	if r.c.FootnotesMode == common.FootnotesModeFloatRenumbered {
		label = strings.TrimSpace(ref.DisplayText)
	}
	if label == "" {
		label = strings.TrimSpace(ref.DisplayText)
	}
	if label == "" {
		label = fmt.Sprintf("%d", noteNumber)
	}
	if r.format == formatMD {
		return "[" + escapeMDLinkTextRendered(r.footnoteLabel(label)) + "](#" + escapeMDURL(endnoteAnchor(linkID)) + ")"
	}
	if strings.HasPrefix(label, "[") && strings.HasSuffix(label, "]") {
		return label
	}
	return "[" + label + "]"
}

func (r *renderer) footnoteLabel(label string) string {
	if strings.HasPrefix(label, "[") && strings.HasSuffix(label, "]") {
		return escapeMDInline(label)
	}
	return escapeMDInline("[" + label + "]")
}

func (r *renderer) inlineImage(img *fb2.InlineImage) string {
	if img == nil {
		return ""
	}
	label := r.inlineImageLabel(img)
	if r.format == formatMD {
		return r.markdownImage(imgIDFromHref(img.Href), label)
	}
	return "[Image: " + label + "]"
}

func (r *renderer) image(img *fb2.Image) {
	if img == nil {
		return
	}
	label := r.blockImageLabel(img)
	if r.format == formatMD {
		r.block(r.markdownImage(imgIDFromHref(img.Href), label))
		return
	}
	r.block("[Image: " + label + "]")
}

func (r *renderer) inlineImageLabel(img *fb2.InlineImage) string {
	if img == nil {
		return "image"
	}
	label := strings.TrimSpace(img.Alt)
	if label == "" {
		label = imageRefLabel(img.Href, r.c.ImagesIndex)
	}
	if label == "" {
		label = "image"
	}
	return label
}

func (r *renderer) blockImageLabel(img *fb2.Image) string {
	if img == nil {
		return "image"
	}
	label := strings.TrimSpace(img.Alt)
	if label == "" {
		label = strings.TrimSpace(img.Title)
	}
	if label == "" {
		label = imageRefLabel(img.Href, r.c.ImagesIndex)
	}
	if label == "" {
		label = "image"
	}
	return label
}

func (r *renderer) markdownImage(id string, label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "image"
	}
	switch r.cfg.Images.Markdown {
	case common.MarkdownImageModeExternal:
		if target := r.externalImageTarget(id); target != "" {
			return "![" + escapeMDLinkTextRendered(escapeMDInline(label)) + "](" + escapeMDURL(target) + ")"
		}
	case common.MarkdownImageModeEmbedded:
		if target := r.embeddedImageTarget(id); target != "" {
			return "![" + escapeMDLinkTextRendered(escapeMDInline(label)) + "](" + target + ")"
		}
	}
	return "[Image: " + escapeMDInline(label) + "]"
}

func (r *renderer) externalImageTarget(id string) string {
	img := r.bookImage(id)
	if img == nil || len(img.Data) == 0 || strings.TrimSpace(img.Filename) == "" || strings.TrimSpace(r.options.OutputPath) == "" {
		return ""
	}
	dirName := markdownAssetsDirName(r.c)
	if dirName == "" {
		return ""
	}
	assetDir := filepath.Join(filepath.Dir(r.options.OutputPath), dirName)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return ""
	}
	name := filepath.Base(img.Filename)
	if name == "." || name == string(filepath.Separator) || strings.TrimSpace(name) == "" {
		name = id + imageExt(img)
	}
	if err := os.WriteFile(filepath.Join(assetDir, name), img.Data, 0644); err != nil {
		return ""
	}
	return filepath.ToSlash(filepath.Join(dirName, name))
}

func (r *renderer) embeddedImageTarget(id string) string {
	img := r.bookImage(id)
	if img == nil || len(img.Data) == 0 || strings.TrimSpace(img.MimeType) == "" {
		return ""
	}
	return "data:" + img.MimeType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
}

func (r *renderer) bookImage(id string) *fb2.BookImage {
	id = strings.TrimSpace(id)
	if id == "" || r.c == nil || r.c.ImagesIndex == nil {
		return nil
	}
	return r.c.ImagesIndex[id]
}

func markdownAssetsDirName(c *content.Content) string {
	if c == nil || c.Book == nil {
		return ""
	}
	id := strings.TrimSpace(c.Book.Description.DocumentInfo.ID)
	if id == "" {
		id = strings.TrimSuffix(filepath.Base(c.SrcName), filepath.Ext(c.SrcName))
	}
	if id == "" {
		return ""
	}
	return "images-" + safeAssetPathSegment(id)
}

func safeAssetPathSegment(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r == '-' || r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return strings.Trim(b.String(), "._")
}

func imageExt(img *fb2.BookImage) string {
	if img == nil {
		return ""
	}
	if ext := filepath.Ext(img.Filename); ext != "" {
		return ext
	}
	if exts, err := mime.ExtensionsByType(img.MimeType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}

func imgIDFromHref(href string) string {
	return strings.TrimPrefix(strings.TrimSpace(href), "#")
}

func (r *renderer) table(table *fb2.Table) {
	if table == nil || len(table.Rows) == 0 {
		return
	}
	rows := make([][]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		cells := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, strings.TrimSpace(r.renderSegments(cell.Content)))
		}
		rows = append(rows, cells)
	}
	if r.format == formatMD {
		r.block(markdownTable(rows))
		return
	}
	r.block(alignedTable(rows))
}

func (r *renderer) renderEndnotes() {
	if r.endnotes == nil || len(r.endnotes.ids) == 0 {
		return
	}
	r.heading("Notes", 2)
	for i := 0; i < len(r.endnotes.ids); i++ {
		id := r.endnotes.ids[i]
		ref, ok := r.c.FootnotesIndex[id]
		if !ok || ref.BodyIdx < 0 || ref.BodyIdx >= len(r.c.Book.Bodies) {
			continue
		}
		body := &r.c.Book.Bodies[ref.BodyIdx]
		if ref.SectionIdx < 0 || ref.SectionIdx >= len(body.Sections) {
			continue
		}
		section := &body.Sections[ref.SectionIdx]
		label := strings.TrimSpace(ref.DisplayText)
		if label == "" && section.Title != nil {
			label = section.Title.AsTOCText("")
		}
		if label == "" {
			label = fmt.Sprintf("%d", i+1)
		}
		blocks := r.renderFootnoteBlocks(section)
		if len(blocks) == 0 {
			continue
		}
		if r.format == formatMD {
			r.headingWithAnchor(r.plainInline(fmt.Sprintf("%d. %s", i+1, label)), 3, endnoteAnchor(id))
			for _, block := range blocks {
				r.block(block)
			}
			continue
		}
		r.block("[" + label + "]")
		for _, block := range blocks {
			r.block(block)
		}
	}
}

func (r *renderer) renderFootnoteBlocks(section *fb2.Section) []string {
	if section == nil {
		return nil
	}
	child := &renderer{
		c:        r.c,
		cfg:      r.cfg,
		format:   r.format,
		endnotes: r.endnotes,
		options:  r.options,
	}
	for i := range section.Epigraphs {
		child.renderEpigraph(&section.Epigraphs[i])
	}
	child.image(section.Image)
	if section.Annotation != nil {
		child.renderFlow(section.Annotation.Items, 1, blockNormal)
	}
	child.renderFlow(section.Content, 1, blockNormal)
	return slices.DeleteFunc(child.blocks, func(block string) bool { return strings.TrimSpace(block) == "" })
}

func (r *renderer) renderCodeParagraph(p *fb2.Paragraph, style blockStyle) bool {
	code, ok := paragraphCodeText(p)
	if !ok {
		return false
	}
	code = strings.Trim(code, "\n\r")
	if strings.TrimSpace(code) == "" {
		return true
	}
	if r.format == formatMD {
		r.paragraphStyled(fencedCodeBlock(code), style)
		return true
	}
	r.paragraphStyled(code, style)
	return true
}

func paragraphCodeText(p *fb2.Paragraph) (string, bool) {
	if p == nil || len(p.Text) == 0 {
		return "", false
	}
	var b strings.Builder
	foundCode := false
	for i := range p.Text {
		seg := &p.Text[i]
		switch seg.Kind {
		case fb2.InlineCode:
			foundCode = true
			b.WriteString(seg.Text)
			b.WriteString(rawSegmentsText(seg.Children))
		case fb2.InlineText:
			if strings.TrimSpace(seg.Text) != "" || hasMeaningfulSegments(seg.Children) {
				return "", false
			}
			b.WriteString(seg.Text)
		case fb2.InlineImageSegment:
			return "", false
		default:
			if strings.TrimSpace(seg.Text) != "" || hasMeaningfulSegments(seg.Children) {
				return "", false
			}
			b.WriteString(seg.Text)
		}
	}
	return b.String(), foundCode
}

func hasMeaningfulSegments(segments []fb2.InlineSegment) bool {
	for i := range segments {
		seg := &segments[i]
		if seg.Kind == fb2.InlineImageSegment {
			return true
		}
		if strings.TrimSpace(seg.Text) != "" || hasMeaningfulSegments(seg.Children) {
			return true
		}
	}
	return false
}

func rawSegmentsText(segments []fb2.InlineSegment) string {
	var b strings.Builder
	for i := range segments {
		b.WriteString(rawSegmentText(&segments[i]))
	}
	return b.String()
}

func rawSegmentText(seg *fb2.InlineSegment) string {
	if seg == nil {
		return ""
	}
	if seg.Kind == fb2.InlineImageSegment {
		if seg.Image != nil {
			return seg.Image.Alt
		}
		return ""
	}
	return seg.Text + rawSegmentsText(seg.Children)
}

func (r *renderer) title(title *fb2.Title, depth int) {
	r.titleWithAnchor(title, depth, "")
}

func (r *renderer) titleWithAnchor(title *fb2.Title, depth int, anchorID string) {
	if title == nil {
		return
	}
	anchorWritten := false
	for i := range title.Items {
		item := &title.Items[i]
		if item.EmptyLine {
			r.block(" ")
			continue
		}
		if item.Paragraph != nil {
			anchor := ""
			if !anchorWritten {
				anchor = anchorID
				anchorWritten = true
			}
			r.headingWithAnchor(r.renderParagraph(item.Paragraph), depth, anchor)
		}
	}
}

func (r *renderer) heading(text string, depth int) {
	r.headingWithAnchor(text, depth, "")
}

func (r *renderer) headingWithAnchor(text string, depth int, anchorID string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if r.format == formatMD {
		level := min(max(depth, 1), 6)
		heading := strings.Repeat("#", level) + " " + text
		if anchor := markdownAnchor(anchorID); anchor != "" {
			heading = "<a id=\"" + anchor + "\"></a>\n" + heading
		}
		r.block(heading)
		return
	}
	if depth <= 1 {
		r.block(strings.ToUpper(text))
		return
	}
	r.block(text + "\n" + strings.Repeat("-", utf8.RuneCountInString(text)))
}

func markdownAnchor(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	id = strings.TrimPrefix(id, "#")
	return escapeHTMLAttr(id)
}

func tocItemAnchor(item tocnav.Item) string {
	if item.Href != "" {
		return item.Href
	}
	return item.ID
}

func endnoteAnchor(id string) string {
	anchor := markdownAnchor(id)
	if anchor == "" {
		return ""
	}
	return "note-" + anchor
}

func escapeHTMLAttr(text string) string {
	replacer := strings.NewReplacer("&", "&amp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(text)
}

func (r *renderer) paragraph(text string) {
	r.paragraphStyled(text, blockNormal)
}

func (r *renderer) paragraphStyled(text string, style blockStyle) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if style == blockQuote && r.format == formatMD {
		r.block(prefixLines(text, "> "))
		return
	}
	r.block(text)
}

func (r *renderer) block(text string) {
	if text == "" {
		return
	}
	r.blocks = append(r.blocks, text)
}

func annotationTitle(cfg *config.DocumentConfig) string {
	if cfg == nil || strings.TrimSpace(cfg.Annotation.Title) == "" {
		return "Annotation"
	}
	return strings.TrimSpace(cfg.Annotation.Title)
}

func flattenTOCEntries(entries []*structure.TOCEntry, includeUntitled bool, level int) []tocnav.Item {
	items := make([]tocnav.Item, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		title := strings.TrimSpace(entry.Title)
		include := entry.IncludeInTOC && (title != "" || includeUntitled)
		if include {
			if title == "" {
				title = fb2.NoTitleText
			}
			items = append(items, tocnav.Item{ID: entry.ID, Title: title, Href: "#" + entry.ID, Level: level})
		}
		childLevel := level
		if include {
			childLevel++
		}
		items = append(items, flattenTOCEntries(entry.Children, includeUntitled, childLevel)...)
	}
	return items
}

func formatAuthors(authors []fb2.Author) string {
	parts := make([]string, 0, len(authors))
	for _, author := range authors {
		if name := formatAuthor(author); name != "" {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}

func formatAuthor(author fb2.Author) string {
	parts := []string{author.FirstName, author.MiddleName, author.LastName}
	parts = slices.DeleteFunc(parts, func(s string) bool { return strings.TrimSpace(s) == "" })
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	return strings.TrimSpace(author.Nickname)
}

func formatSeries(sequences []fb2.Sequence) string {
	parts := make([]string, 0, len(sequences))
	for _, seq := range sequences {
		name := strings.TrimSpace(seq.Name)
		if name == "" {
			continue
		}
		if seq.Number != nil {
			name = fmt.Sprintf("%s #%d", name, *seq.Number)
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, ", ")
}

func formatGenres(genres []fb2.GenreRef) string {
	parts := make([]string, 0, len(genres))
	for _, genre := range genres {
		if strings.TrimSpace(genre.Value) != "" {
			parts = append(parts, genre.Value)
		}
	}
	return strings.Join(parts, ", ")
}

func formatDate(date *fb2.Date) string {
	if date == nil {
		return ""
	}
	if !date.Value.IsZero() {
		return date.Value.Format("2006-01-02")
	}
	return strings.TrimSpace(date.Display)
}

func imageRefLabel(href string, images fb2.BookImages) string {
	id := strings.TrimPrefix(strings.TrimSpace(href), "#")
	if id == "" {
		return ""
	}
	if img, ok := images[id]; ok && img != nil && strings.TrimSpace(img.Filename) != "" {
		return img.Filename
	}
	return id
}

func markdownTable(rows [][]string) string {
	cols := maxColumns(rows)
	if cols == 0 {
		return ""
	}
	var b strings.Builder
	writeMDTableRow(&b, normalizeRow(rows[0], cols))
	b.WriteByte('\n')
	sep := make([]string, cols)
	for i := range sep {
		sep[i] = "---"
	}
	writeMDTableRow(&b, sep)
	for _, row := range rows[1:] {
		b.WriteByte('\n')
		writeMDTableRow(&b, normalizeRow(row, cols))
	}
	return b.String()
}

func writeMDTableRow(b *strings.Builder, row []string) {
	b.WriteString("| ")
	for i, cell := range row {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(escapeMDTableCell(cell))
	}
	b.WriteString(" |")
}

func alignedTable(rows [][]string) string {
	cols := maxColumns(rows)
	if cols == 0 {
		return ""
	}
	widths := make([]int, cols)
	normalized := make([][]string, len(rows))
	for i, row := range rows {
		normalized[i] = normalizeRow(row, cols)
		for j, cell := range normalized[i] {
			widths[j] = max(widths[j], utf8.RuneCountInString(cell))
		}
	}
	lines := make([]string, 0, len(normalized))
	for _, row := range normalized {
		var b strings.Builder
		for i, cell := range row {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(cell)
			if i < len(row)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell)))
			}
		}
		lines = append(lines, strings.TrimRightFunc(b.String(), unicode.IsSpace))
	}
	return strings.Join(lines, "\n")
}

func normalizeRow(row []string, cols int) []string {
	out := make([]string, cols)
	copy(out, row)
	return out
}

func maxColumns(rows [][]string) int {
	cols := 0
	for _, row := range rows {
		cols = max(cols, len(row))
	}
	return cols
}

func prefixLines(text string, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = prefix
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func indentBlock(text string, prefix string) string {
	return prefixLines(text, prefix)
}

func escapeMDInline(text string) string {
	if text == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range text {
		switch r {
		case '\\', '`', '*', '_', '[', ']', '<', '>':
			b.WriteByte('\\')
		}
		if i == 0 && (r == '#' || r == '-' || r == '+' || r == '=') {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func codeSpan(text string) string {
	if text == "" {
		return "``"
	}
	maxRun := maxConsecutiveBackticks(text)
	fence := strings.Repeat("`", max(1, maxRun+1))
	if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
		return fence + " " + text + " " + fence
	}
	return fence + text + fence
}

func fencedCodeBlock(code string) string {
	fence := strings.Repeat("`", max(3, maxConsecutiveBackticks(code)+1))
	return fence + "\n" + code + "\n" + fence
}

func maxConsecutiveBackticks(text string) int {
	maxRun := 0
	run := 0
	for _, r := range text {
		if r == '`' {
			run++
			maxRun = max(maxRun, run)
			continue
		}
		run = 0
	}
	return maxRun
}

func splitSurroundingWhitespace(text string) (string, string, string) {
	start := 0
	for start < len(text) {
		r, size := utf8.DecodeRuneInString(text[start:])
		if !unicode.IsSpace(r) {
			break
		}
		start += size
	}

	end := len(text)
	for end > start {
		r, size := utf8.DecodeLastRuneInString(text[:end])
		if !unicode.IsSpace(r) {
			break
		}
		end -= size
	}

	return text[:start], text[start:end], text[end:]
}

func normalizeInlineWhitespace(text string) string {
	if text == "" {
		return ""
	}
	var b strings.Builder
	inSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !inSpace {
				b.WriteByte(' ')
				inSpace = true
			}
			continue
		}
		b.WriteRune(r)
		inSpace = false
	}
	return b.String()
}

func escapeMDLinkTextRendered(text string) string {
	return strings.ReplaceAll(text, "|", "\\|")
}

func escapeMDURL(url string) string {
	replacer := strings.NewReplacer(" ", "%20", ")", "%29", "(", "%28")
	return replacer.Replace(url)
}

func escapeMDTableCell(text string) string {
	text = strings.ReplaceAll(text, "\n", "<br>")
	text = strings.ReplaceAll(text, "|", "\\|")
	return text
}
