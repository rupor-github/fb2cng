package pdf

import (
	"strings"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/structure"
	"fbc/convert/tocnav"
	"fbc/fb2"
)

func collectTextBlocks(c *content.Content) ([]pdfTextBlock, error) {
	plan, err := collectPDFContent(c, nil)
	if err != nil {
		return nil, err
	}
	return plan.Blocks, nil
}

func collectPDFContent(c *content.Content, cfg *config.DocumentConfig) (pdfContentPlan, error) {
	if c == nil || c.Book == nil {
		return pdfContentPlan{}, nil
	}
	plan, err := structure.BuildPlan(c)
	if err != nil {
		return pdfContentPlan{}, err
	}

	debugPlan := pdfDebugStructurePlanFromPlan(plan, cfg)
	blocks := make([]pdfTextBlock, 0, 64)
	splitSections := splitSectionIDs(plan)
	splitBodies := splitBodyImageBodies(plan)
	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.ForceNewPage && unit.Kind != structure.UnitCover {
			blocks = append(blocks, pdfTextBlock{Kind: pdfBlockPageBreak, ID: unit.ID, Text: unit.Title})
		}
		appendUnitBlocks(&blocks, c.Book, unit, splitSections, splitBodies)
	}
	toc := plan.TOC
	blocks, toc = insertAnnotationPageBlocks(blocks, toc, c.Book.Description.TitleInfo.Annotation, cfg)
	blocks = insertTOCPageBlocks(blocks, toc, cfg)
	debugPlan.TOC = pdfDebugStructureTOCEntries(toc)
	return pdfContentPlan{Blocks: blocks, TOC: toc, DebugPlan: debugPlan}, nil
}

func pdfDebugStructurePlanFromPlan(plan *structure.Plan, cfg *config.DocumentConfig) pdfDebugStructurePlan {
	if plan == nil {
		return pdfDebugStructurePlan{}
	}
	debugPlan := pdfDebugStructurePlan{
		Units:     make([]pdfDebugStructureUnit, 0, len(plan.Units)),
		Landmarks: plan.Landmarks,
	}
	for i := range plan.Units {
		unit := &plan.Units[i]
		debugUnit := pdfDebugStructureUnit{
			Index:        i,
			Kind:         structureUnitKindString(unit.Kind),
			ID:           unit.ID,
			Title:        unit.Title,
			Depth:        unit.Depth,
			TitleDepth:   unit.TitleDepth,
			ForceNewPage: unit.ForceNewPage,
			IsTopLevel:   unit.IsTopLevel,
		}
		if unit.Body != nil {
			debugUnit.BodyName = unit.Body.Name
		}
		if unit.Section != nil {
			debugUnit.SectionID = unit.Section.ID
		}
		debugPlan.Units = append(debugPlan.Units, debugUnit)
	}
	if cfg != nil {
		debugPlan.Generated = pdfDebugStructureGenerated{
			AnnotationPage:                 cfg.Annotation.Enable,
			AnnotationInTOC:                cfg.Annotation.InTOC,
			TOCPagePlacement:               cfg.TOCPage.Placement.String(),
			TOCIncludeChaptersWithoutTitle: cfg.TOCPage.ChaptersWithoutTitle,
			TOCType:                        cfg.TOCType.String(),
		}
	}
	return debugPlan
}

func structureUnitKindString(kind structure.UnitKind) string {
	switch kind {
	case structure.UnitCover:
		return "cover"
	case structure.UnitBodyImage:
		return "body-image"
	case structure.UnitBodyIntro:
		return "body-intro"
	case structure.UnitSection:
		return "section"
	case structure.UnitFootnotesBody:
		return "footnotes-body"
	case structure.UnitAnnotation:
		return "annotation"
	case structure.UnitTOC:
		return "toc"
	default:
		return "unknown"
	}
}

func pdfDebugStructureTOCEntries(entries []*structure.TOCEntry) []pdfDebugStructureTOCEntry {
	out := make([]pdfDebugStructureTOCEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		out = append(out, pdfDebugStructureTOCEntry{
			ID:           entry.ID,
			Title:        entry.Title,
			IncludeInTOC: entry.IncludeInTOC,
			Children:     pdfDebugStructureTOCEntries(entry.Children),
		})
	}
	return out
}

func insertAnnotationPageBlocks(blocks []pdfTextBlock, toc []*structure.TOCEntry, annotation *fb2.Flow, cfg *config.DocumentConfig) ([]pdfTextBlock, []*structure.TOCEntry) {
	if cfg == nil || !cfg.Annotation.Enable || annotation == nil || len(annotation.Items) == 0 {
		return blocks, toc
	}
	title := strings.TrimSpace(cfg.Annotation.Title)
	if title == "" {
		title = "Annotation"
	}
	annotationBlocks := []pdfTextBlock{
		{Kind: pdfBlockPageBreak, ID: "annotation-page", Text: title},
		{Kind: pdfBlockHeading, ID: "annotation-page-title", Text: title, Depth: 1, StyleName: pdfStyleAnnotationTitle},
	}
	appendFlowBlocks(&annotationBlocks, nil, annotation.Items, 1, nil, pdfStyleAnnotation)
	out := make([]pdfTextBlock, 0, len(annotationBlocks)+len(blocks))
	out = append(out, annotationBlocks...)
	out = append(out, blocks...)
	if !cfg.Annotation.InTOC {
		return out, toc
	}
	tocOut := make([]*structure.TOCEntry, 0, len(toc)+1)
	tocOut = append(tocOut, &structure.TOCEntry{ID: "annotation-page", Title: title, IncludeInTOC: true})
	tocOut = append(tocOut, toc...)
	return out, tocOut
}

func insertTOCPageBlocks(blocks []pdfTextBlock, entries []*structure.TOCEntry, cfg *config.DocumentConfig) []pdfTextBlock {
	if cfg == nil || cfg.TOCPage.Placement == common.TOCPagePlacementNone || len(entries) == 0 {
		return blocks
	}
	tocBlocks := buildTOCPageBlocks(entries, cfg.TOCPage.ChaptersWithoutTitle, cfg.TOCType)
	if len(tocBlocks) == 0 {
		return blocks
	}
	switch cfg.TOCPage.Placement {
	case common.TOCPagePlacementBefore:
		out := make([]pdfTextBlock, 0, len(tocBlocks)+len(blocks))
		out = append(out, tocBlocks...)
		out = append(out, blocks...)
		return out
	case common.TOCPagePlacementAfter:
		out := make([]pdfTextBlock, 0, len(blocks)+len(tocBlocks))
		out = append(out, blocks...)
		out = append(out, tocBlocks...)
		return out
	default:
		return blocks
	}
}

func buildTOCPageBlocks(entries []*structure.TOCEntry, includeUntitled bool, tocType common.TOCType) []pdfTextBlock {
	items := flattenPDFTOCEntries(entries, includeUntitled, 1)
	if len(items) == 0 {
		return nil
	}
	blocks := []pdfTextBlock{
		{Kind: pdfBlockPageBreak, ID: "toc-page", Text: "Contents"},
		{Kind: pdfBlockHeading, ID: "toc-page-title", Text: "Contents", Depth: 1, StyleName: pdfStyleTOCTitle},
	}
	var appendTOCNodeBlocks func(nodes []*tocnav.Node)
	appendTOCNodeBlocks = func(nodes []*tocnav.Node) {
		for _, node := range nodes {
			if node == nil || strings.TrimSpace(node.Item.Title) == "" || node.Item.ID == "" {
				continue
			}
			title := strings.TrimSpace(node.Item.Title)
			blocks = append(blocks, pdfTextBlock{
				Kind:      pdfBlockTOCEntry,
				Text:      title,
				Runs:      []pdfInlineRun{{Text: title, StyleClasses: pdfStyleLinkTOC}},
				Depth:     max(node.Item.Level, 1),
				StyleName: pdfStyleTOCItem,
				Links:     []pdfTextLink{{Start: 0, End: runeLenString(title), Href: "#" + node.Item.ID}},
			})
			appendTOCNodeBlocks(node.Children)
		}
	}
	appendTOCNodeBlocks(tocnav.Shape(items, tocType))
	if len(blocks) == 2 {
		return nil
	}
	return blocks
}

func flattenPDFTOCEntries(entries []*structure.TOCEntry, includeUntitled bool, level int) []tocnav.Item {
	items := make([]tocnav.Item, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		title := strings.TrimSpace(entry.Title)
		include := entry.IncludeInTOC || includeUntitled
		if include && title != "" && entry.ID != "" {
			items = append(items, tocnav.Item{ID: entry.ID, Title: title, Href: "#" + entry.ID, Level: level})
			items = append(items, flattenPDFTOCEntries(entry.Children, includeUntitled, level+1)...)
			continue
		}
		items = append(items, flattenPDFTOCEntries(entry.Children, includeUntitled, level)...)
	}
	return items
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

func splitBodyImageBodies(plan *structure.Plan) map[*fb2.Body]bool {
	bodies := make(map[*fb2.Body]bool)
	if plan == nil {
		return bodies
	}
	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.Kind == structure.UnitBodyImage && unit.Body != nil {
			bodies[unit.Body] = true
		}
	}
	return bodies
}

func appendUnitBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, unit *structure.Unit, splitSections map[string]bool, splitBodies map[*fb2.Body]bool) {
	if unit == nil {
		return
	}
	switch unit.Kind {
	case structure.UnitBodyImage:
		if unit.Body != nil {
			appendImageBlock(blocks, unit.Body.Image, unit.ID)
		}
	case structure.UnitBodyIntro:
		appendBodyIntroBlocks(blocks, book, unit.Body, !splitBodies[unit.Body])
	case structure.UnitSection:
		appendSectionBlocks(blocks, book, unit.Section, unit.TitleDepth, splitSections)
	case structure.UnitFootnotesBody:
		appendBodyBlocks(blocks, book, unit.Body, splitSections)
	}
}

func appendBodyIntroBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, body *fb2.Body, includeImage bool) {
	if body == nil {
		return
	}
	if includeImage {
		appendImageBlock(blocks, body.Image, "")
	}
	if body.Title != nil && body.Main() {
		appendVignetteBlock(blocks, book, common.VignettePosBookTitleTop)
	}
	appendTitleBlocks(blocks, body.Title, 1)
	if body.Title != nil && body.Main() {
		appendVignetteBlock(blocks, book, common.VignettePosBookTitleBottom)
	}
	for i := range body.Epigraphs {
		appendEpigraphBlocks(blocks, &body.Epigraphs[i])
	}
}

func appendBodyBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, body *fb2.Body, splitSections map[string]bool) {
	if body == nil {
		return
	}
	appendBodyIntroBlocks(blocks, book, body, true)
	for i := range body.Sections {
		appendSectionBlocks(blocks, book, &body.Sections[i], 1, splitSections)
	}
}

func appendSectionBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, section *fb2.Section, depth int, splitSections map[string]bool) {
	if section == nil {
		return
	}
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, true)
	}
	appendTitleBlocksWithID(blocks, section.Title, depth, section.ID)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, false)
	}
	for i := range section.Epigraphs {
		appendEpigraphBlocks(blocks, &section.Epigraphs[i])
	}
	appendImageBlock(blocks, section.Image, section.ID)
	if section.Annotation != nil {
		appendFlowBlocks(blocks, book, section.Annotation.Items, depth, splitSections, pdfStyleAnnotation)
	}
	for i := range section.Content {
		appendFlowItemBlock(blocks, book, &section.Content[i], depth, splitSections, "")
	}
	if section.Title != nil {
		appendEndVignetteBlock(blocks, book, depth)
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
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine})
			continue
		}
		if item.Paragraph == nil {
			continue
		}
		text, links := paragraphTextAndLinks(item.Paragraph)
		runs := paragraphInlineRuns(item.Paragraph)
		classes := ""
		if paragraphIsCodeBlock(item.Paragraph) {
			classes = pdfStyleCode
		}
		if text != "" || inlineRunsRenderable(runs) {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockHeading, ID: anchorID, Text: text, Runs: runs, Depth: depth, StyleName: pdfHeadingStyleName(depth), StyleClasses: classes, Links: links})
			anchorID = ""
		}
	}
}

func appendFlowBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, items []fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string) {
	for i := range items {
		appendFlowItemBlock(blocks, book, &items[i], depth, splitSections, styleClasses)
	}
}

func appendFlowItemBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, item *fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string) {
	if item == nil {
		return
	}
	switch item.Kind {
	case fb2.FlowParagraph:
		appendParagraphBlockWithClasses(blocks, pdfBlockParagraph, item.Paragraph, depth, styleClasses)
	case fb2.FlowSubtitle:
		appendParagraphBlockWithClasses(blocks, pdfBlockSubtitle, item.Subtitle, depth, subtitleStyleClasses(styleClasses))
	case fb2.FlowEmptyLine:
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine})
	case fb2.FlowImage:
		appendImageBlock(blocks, item.Image, "")
	case fb2.FlowSection:
		if item.Section != nil && splitSections[item.Section.ID] {
			return
		}
		appendSectionBlocks(blocks, book, item.Section, depth+1, splitSections)
	case fb2.FlowPoem:
		appendPoemBlocks(blocks, item.Poem, depth, splitSections)
	case fb2.FlowCite:
		appendCiteBlocks(blocks, item.Cite, depth, splitSections)
	case fb2.FlowTable:
		if item.Table != nil {
			text := item.Table.AsPlainText()
			if text != "" {
				*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockParagraph, Text: text, Depth: depth, StyleName: pdfStyleParagraph, StyleClasses: joinStyleClasses(styleClasses, pdfStyleTable)})
			}
		}
	}
}

func appendImageBlock(blocks *[]pdfTextBlock, image *fb2.Image, fallbackID string) {
	if image == nil {
		return
	}
	imageID := imageRefID(image.Href)
	if imageID == "" {
		return
	}
	anchorID := image.ID
	if anchorID == "" {
		anchorID = fallbackID
	}
	*blocks = append(*blocks, pdfTextBlock{
		Kind:      pdfBlockImage,
		ID:        anchorID,
		Text:      strings.TrimSpace(image.Alt),
		StyleName: pdfStyleImage,
		ImageID:   imageID,
	})
}

func appendTitleVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int, top bool) {
	if depth == 1 {
		if top {
			appendVignetteBlock(blocks, book, common.VignettePosChapterTitleTop)
			return
		}
		appendVignetteBlock(blocks, book, common.VignettePosChapterTitleBottom)
		return
	}
	if top {
		appendVignetteBlock(blocks, book, common.VignettePosSectionTitleTop)
		return
	}
	appendVignetteBlock(blocks, book, common.VignettePosSectionTitleBottom)
}

func appendEndVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int) {
	if depth == 1 {
		appendVignetteBlock(blocks, book, common.VignettePosChapterEnd)
		return
	}
	appendVignetteBlock(blocks, book, common.VignettePosSectionEnd)
}

func appendVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, position common.VignettePos) {
	if book == nil || !book.IsVignetteEnabled(position) {
		return
	}
	imageID := strings.TrimSpace(book.VignetteIDs[position])
	if imageID == "" {
		return
	}
	*blocks = append(*blocks, pdfTextBlock{
		Kind:         pdfBlockImage,
		StyleName:    pdfStyleImage,
		StyleClasses: joinStyleClasses("vignette", "vignette-"+position.String()),
		ImageID:      imageID,
	})
}

func isVignetteBlock(block pdfTextBlock) bool {
	for _, class := range strings.Fields(block.StyleClasses) {
		if class == "vignette" {
			return true
		}
	}
	return false
}

func appendParagraphBlock(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int) {
	appendParagraphBlockWithClasses(blocks, kind, paragraph, depth, "")
}

func appendParagraphBlockWithClasses(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string) {
	if paragraph == nil {
		return
	}
	text, links := paragraphTextAndLinks(paragraph)
	runs := paragraphInlineRuns(paragraph)
	if paragraphIsCodeBlock(paragraph) {
		styleClasses = joinStyleClasses(styleClasses, pdfStyleCode)
	}
	if text != "" || inlineRunsRenderable(runs) {
		*blocks = append(*blocks, pdfTextBlock{Kind: kind, ID: paragraph.ID, Text: text, Runs: runs, Depth: depth, StyleName: pdfStyleNameForKind(kind), StyleClasses: joinStyleClasses(paragraph.Style, styleClasses), Links: links})
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
		appendParagraphBlockWithClasses(blocks, pdfBlockSubtitle, &poem.Subtitles[i], depth, pdfStylePoemSubtitle)
	}
	for i := range poem.Stanzas {
		stanza := &poem.Stanzas[i]
		appendTitleBlocks(blocks, stanza.Title, depth+1)
		appendParagraphBlockWithClasses(blocks, pdfBlockSubtitle, stanza.Subtitle, depth, pdfStyleStanzaSubtitle)
		for j := range stanza.Verses {
			appendParagraphBlockWithClasses(blocks, pdfBlockPoem, &stanza.Verses[j], depth, pdfStylePoem)
		}
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine})
	}
	for i := range poem.TextAuthors {
		appendParagraphBlock(blocks, pdfBlockTextAuthor, &poem.TextAuthors[i], depth)
	}
}

func appendCiteBlocks(blocks *[]pdfTextBlock, cite *fb2.Cite, depth int, splitSections map[string]bool) {
	if cite == nil {
		return
	}
	appendFlowBlocks(blocks, nil, cite.Items, depth, splitSections, pdfStyleCite)
	for i := range cite.TextAuthors {
		appendParagraphBlock(blocks, pdfBlockTextAuthor, &cite.TextAuthors[i], depth)
	}
}

func appendEpigraphBlocks(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph) {
	if epigraph == nil {
		return
	}
	appendFlowBlocks(blocks, nil, epigraph.Flow.Items, 1, nil, pdfStyleEpigraph)
	for i := range epigraph.TextAuthors {
		appendParagraphBlock(blocks, pdfBlockTextAuthor, &epigraph.TextAuthors[i], 1)
	}
}

func paragraphTextAndLinks(paragraph *fb2.Paragraph) (string, []pdfTextLink) {
	if paragraph == nil {
		return "", nil
	}
	return inlineSegmentsTextAndLinks(paragraph.Text)
}

func inlineRunsRenderable(runs []pdfInlineRun) bool {
	for _, run := range runs {
		if run.Text != "" || run.ImageID != "" {
			return true
		}
	}
	return false
}

func paragraphInlineRuns(paragraph *fb2.Paragraph) []pdfInlineRun {
	if paragraph == nil {
		return nil
	}
	var runs []pdfInlineRun
	for i := range paragraph.Text {
		appendInlineSegmentRun(&runs, &paragraph.Text[i], pdfInlineRun{})
	}
	if paragraphIsCodeBlock(paragraph) {
		return trimCodeBlockInlineRuns(runs)
	}
	return trimInlineRuns(runs)
}

func paragraphIsCodeBlock(paragraph *fb2.Paragraph) bool {
	if paragraph == nil || len(paragraph.Text) == 0 {
		return false
	}
	seenCode := false
	for i := range paragraph.Text {
		if !inlineSegmentIsCodeBlockContent(&paragraph.Text[i], false, &seenCode) {
			return false
		}
	}
	return seenCode
}

func inlineSegmentIsCodeBlockContent(seg *fb2.InlineSegment, inCode bool, seenCode *bool) bool {
	if seg == nil {
		return true
	}
	if seg.Kind == fb2.InlineCode {
		inCode = true
		*seenCode = true
	}
	if seg.Text != "" && !inCode && strings.TrimSpace(seg.Text) != "" {
		return false
	}
	if seg.Kind == fb2.InlineImageSegment && !inCode {
		return false
	}
	for i := range seg.Children {
		if !inlineSegmentIsCodeBlockContent(&seg.Children[i], inCode, seenCode) {
			return false
		}
	}
	return true
}

func inlineSegmentsTextAndLinks(segments []fb2.InlineSegment) (string, []pdfTextLink) {
	var b strings.Builder
	var links []pdfTextLink
	for i := range segments {
		appendInlineSegmentText(&b, &links, &segments[i])
	}
	return normalizeInlineTextAndLinks(b.String(), links)
}

func imageRefID(href string) string {
	return strings.TrimPrefix(strings.TrimSpace(href), "#")
}

func normalizeInlineTextAndLinks(raw string, links []pdfTextLink) (string, []pdfTextLink) {
	runes := []rune(raw)
	boundary := make([]int, len(runes)+1)
	var b strings.Builder
	normalizedLen := 0
	pendingSpace := false
	for i, r := range runes {
		if isBreakableSpace(r) {
			boundary[i] = normalizedLen
			if normalizedLen > 0 {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace && normalizedLen > 0 {
			b.WriteByte(' ')
			normalizedLen++
		}
		pendingSpace = false
		boundary[i] = normalizedLen
		b.WriteRune(r)
		normalizedLen++
	}
	boundary[len(runes)] = normalizedLen

	normalizedLinks := make([]pdfTextLink, 0, len(links))
	for _, link := range links {
		start, end := trimRawLinkRange(runes, link.Start, link.End)
		if start >= end || strings.TrimSpace(link.Href) == "" {
			continue
		}
		normalizedStart := boundary[start]
		normalizedEnd := boundary[end]
		if normalizedStart >= normalizedEnd {
			continue
		}
		normalizedLinks = append(normalizedLinks, pdfTextLink{Start: normalizedStart, End: normalizedEnd, Href: link.Href})
	}
	return b.String(), normalizedLinks
}

func trimRawLinkRange(runes []rune, start int, end int) (int, int) {
	start = min(max(start, 0), len(runes))
	end = min(max(end, start), len(runes))
	for start < end && isBreakableSpace(runes[start]) {
		start++
	}
	for end > start && isBreakableSpace(runes[end-1]) {
		end--
	}
	return start, end
}

func appendInlineSegmentText(b *strings.Builder, links *[]pdfTextLink, seg *fb2.InlineSegment) {
	if seg == nil {
		return
	}
	if seg.Kind == fb2.InlineImageSegment {
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

func appendInlineSegmentRun(runs *[]pdfInlineRun, seg *fb2.InlineSegment, inherited pdfInlineRun) {
	if seg == nil {
		return
	}
	style := inherited
	style.Text = ""
	style = applyInlineSegmentStyle(style, seg)
	if seg.Kind == fb2.InlineImageSegment {
		if seg.Image != nil {
			style.ImageID = imageRefID(seg.Image.Href)
			appendInlineRun(runs, style)
		}
		return
	}
	if seg.Text != "" {
		style.Text = seg.Text
		appendInlineRun(runs, style)
	}
	style.Text = ""
	for i := range seg.Children {
		appendInlineSegmentRun(runs, &seg.Children[i], style)
	}
}

func pdfLinkStyleClass(seg *fb2.InlineSegment) string {
	if seg == nil || strings.TrimSpace(seg.Href) == "" {
		return ""
	}
	if strings.EqualFold(seg.LinkType, "note") {
		return pdfStyleLinkFootnote
	}
	if strings.HasPrefix(strings.TrimSpace(seg.Href), "#") {
		return pdfStyleLinkInternal
	}
	return pdfStyleLinkExternal
}

func applyInlineSegmentStyle(style pdfInlineRun, seg *fb2.InlineSegment) pdfInlineRun {
	switch seg.Kind {
	case fb2.InlineStrong:
		style.Bold = true
	case fb2.InlineEmphasis:
		style.Italic = true
	case fb2.InlineNamedStyle:
		style.StyleClasses = joinStyleClasses(style.StyleClasses, seg.Style)
	case fb2.InlineStrikethrough:
		style.Strikethrough = true
	case fb2.InlineSub:
		style.Subscript = true
		style.Superscript = false
	case fb2.InlineSup:
		style.Superscript = true
		style.Subscript = false
	case fb2.InlineCode:
		style.Code = true
		style.StyleClasses = joinStyleClasses(style.StyleClasses, pdfStyleCode)
	case fb2.InlineLink:
		style.StyleClasses = joinStyleClasses(style.StyleClasses, pdfLinkStyleClass(seg))
		style.LinkHref = strings.TrimSpace(seg.Href)
	}
	return style
}

func appendInlineRun(runs *[]pdfInlineRun, run pdfInlineRun) {
	if run.Superscript || run.Subscript {
		run.Text = strings.TrimSpace(run.Text)
	}
	if run.Text == "" && run.ImageID == "" {
		return
	}
	last := len(*runs) - 1
	if last >= 0 && sameInlineStyle((*runs)[last], run) {
		(*runs)[last].Text += run.Text
		return
	}
	*runs = append(*runs, run)
}

func sameInlineStyle(a, b pdfInlineRun) bool {
	return a.StyleClasses == b.StyleClasses && a.LinkHref == b.LinkHref && a.ImageID == b.ImageID && a.Bold == b.Bold && a.Italic == b.Italic && a.Underline == b.Underline && a.Strikethrough == b.Strikethrough && a.Subscript == b.Subscript && a.Superscript == b.Superscript && a.Code == b.Code
}

func trimCodeBlockInlineRuns(runs []pdfInlineRun) []pdfInlineRun {
	for len(runs) > 0 {
		trimmed := strings.TrimLeft(runs[0].Text, "\r\n")
		if trimmed != "" || runs[0].ImageID != "" {
			runs[0].Text = trimmed
			break
		}
		runs = runs[1:]
	}
	for len(runs) > 0 {
		last := len(runs) - 1
		trimmed := strings.TrimRight(runs[last].Text, " \t\n\r")
		if trimmed != "" || runs[last].ImageID != "" {
			runs[last].Text = trimmed
			break
		}
		runs = runs[:last]
	}
	return runs
}

func trimInlineRuns(runs []pdfInlineRun) []pdfInlineRun {
	for len(runs) > 0 {
		trimmed := strings.TrimLeft(runs[0].Text, " \t\n\r")
		if trimmed != "" || runs[0].ImageID != "" {
			runs[0].Text = trimmed
			break
		}
		runs = runs[1:]
	}
	for len(runs) > 0 {
		last := len(runs) - 1
		trimmed := strings.TrimRight(runs[last].Text, " \t\n\r")
		if trimmed != "" || runs[last].ImageID != "" {
			runs[last].Text = trimmed
			break
		}
		runs = runs[:last]
	}
	return runs
}

func subtitleStyleClasses(containerClasses string) string {
	classes := strings.Fields(containerClasses)
	for i, class := range classes {
		switch class {
		case pdfStyleAnnotation:
			classes[i] = pdfStyleAnnotationSubtitle
		case pdfStylePoem:
			classes[i] = pdfStylePoemSubtitle
		case pdfStyleEpigraph:
			classes[i] = pdfStyleEpigraphSubtitle
		case pdfStyleCite:
			classes[i] = pdfStyleCiteSubtitle
		}
	}
	return strings.Join(classes, " ")
}

func joinStyleClasses(values ...string) string {
	seen := make(map[string]bool)
	classes := make([]string, 0, len(values))
	for _, value := range values {
		for _, class := range strings.Fields(value) {
			if seen[class] {
				continue
			}
			seen[class] = true
			classes = append(classes, class)
		}
	}
	return strings.Join(classes, " ")
}

func runeLenString(s string) int {
	return len([]rune(s))
}
