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
		appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosBookTitleTop, pdfStyleBodyTitle, true)
	}
	appendTitleBlocksWithIDHeaderAndClasses(blocks, body.Title, 1, "", pdfStyleBodyTitleHeader, pdfStyleBodyTitle)
	if body.Title != nil && body.Main() {
		appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosBookTitleBottom, pdfStyleBodyTitle, true)
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
	appendSectionBlocksWithRootHorizontalMargins(blocks, book, section, depth, splitSections, false)
}

func appendSectionBlocksWithRootHorizontalMargins(blocks *[]pdfTextBlock, book *fb2.FictionBook, section *fb2.Section, depth int, splitSections map[string]bool, stripRootHorizontalMargins bool) {
	if section == nil {
		return
	}
	titleClasses := sectionTitleContainerClasses(depth)
	headerStyle := sectionTitleHeaderStyleName(depth)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, true, titleClasses)
	}
	appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks, section.Title, depth, section.ID, headerStyle, titleClasses, stripRootHorizontalMargins)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, false, titleClasses)
	}
	for i := range section.Epigraphs {
		appendEpigraphBlocksWithRootHorizontalMargins(blocks, &section.Epigraphs[i], stripRootHorizontalMargins)
	}
	appendImageBlockWithClassesAndRootHorizontalMargins(blocks, section.Image, section.ID, "", stripRootHorizontalMargins)
	if section.Annotation != nil {
		appendFlowBlocksWithRootHorizontalMargins(blocks, book, section.Annotation.Items, depth, splitSections, pdfStyleAnnotation, stripRootHorizontalMargins || len(section.Annotation.Items) > 1)
	}
	for i := range section.Content {
		appendFlowItemBlockWithRootHorizontalMargins(blocks, book, &section.Content[i], depth, splitSections, "", stripRootHorizontalMargins)
	}
	if section.Title != nil {
		appendEndVignetteBlockWithRootHorizontalMargins(blocks, book, depth, stripRootHorizontalMargins)
	}
}

func appendTitleBlocks(blocks *[]pdfTextBlock, title *fb2.Title, depth int) {
	appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks, title, depth, "", pdfHeadingStyleName(depth), "", false)
}

func appendTitleBlocksWithID(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string) {
	appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks, title, depth, id, pdfHeadingStyleName(depth), "", false)
}

func appendTitleBlocksWithIDAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, styleClasses string) {
	appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks, title, depth, id, pdfHeadingStyleName(depth), styleClasses, false)
}

func appendTitleBlocksWithIDHeaderAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string) {
	appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks, title, depth, id, headerStyleName, styleClasses, false)
}

func appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string, stripRootHorizontalMargins bool) {
	if title == nil {
		return
	}
	blockStripRootHorizontalMargins := stripRootHorizontalMargins || titleWrapperStripRootHorizontalMargins(styleClasses)
	anchorID := id
	firstParagraph := true
	prevWasImageOnlyHeadingParagraph := false
	for i := range title.Items {
		item := &title.Items[i]
		if item.EmptyLine {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: joinStyleClasses(styleClasses, headerStyleName+"-emptyline"), StripRootHorizontalMargins: blockStripRootHorizontalMargins})
			prevWasImageOnlyHeadingParagraph = false
			continue
		}
		if item.Paragraph == nil {
			continue
		}
		positionClass := titleParagraphPositionStyleClass(headerStyleName, firstParagraph)
		firstParagraph = false
		if imageID, alt, ok := paragraphImageOnly(item.Paragraph); ok {
			appendStyledImageIDBlockWithRootHorizontalMargins(blocks, imageID, anchorID, alt, pdfStyleImage, joinStyleClasses(headerStyleName, item.Paragraph.Style, styleClasses, positionClass, pdfStyleHeadingImage), blockStripRootHorizontalMargins)
			anchorID = ""
			prevWasImageOnlyHeadingParagraph = true
			continue
		}
		text, links := paragraphTextAndLinks(item.Paragraph)
		runs := paragraphInlineRuns(item.Paragraph)
		classes := joinStyleClasses(item.Paragraph.Style, styleClasses, positionClass)
		if prevWasImageOnlyHeadingParagraph {
			classes = joinStyleClasses(classes, pdfStyleTitleAfterImage)
		}
		if paragraphIsCodeBlock(item.Paragraph) {
			classes = joinStyleClasses(classes, pdfStyleCode)
		}
		if text != "" || inlineRunsRenderable(runs) {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockHeading, ID: anchorID, Text: text, Runs: runs, Depth: depth, StyleName: headerStyleName, StyleClasses: classes, StripRootHorizontalMargins: blockStripRootHorizontalMargins, Links: links})
			anchorID = ""
			prevWasImageOnlyHeadingParagraph = false
		}
	}
}

func appendFlowBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, items []fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string) {
	appendFlowBlocksWithRootHorizontalMargins(blocks, book, items, depth, splitSections, styleClasses, false)
}

func appendFlowBlocksWithRootHorizontalMargins(blocks *[]pdfTextBlock, book *fb2.FictionBook, items []fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, stripRootHorizontalMargins bool) {
	for i := range items {
		appendFlowItemBlockWithRootHorizontalMargins(blocks, book, &items[i], depth, splitSections, styleClasses, stripRootHorizontalMargins)
	}
}

func appendFlowItemBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, item *fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string) {
	appendFlowItemBlockWithRootHorizontalMargins(blocks, book, item, depth, splitSections, styleClasses, false)
}

func appendFlowItemBlockWithRootHorizontalMargins(blocks *[]pdfTextBlock, book *fb2.FictionBook, item *fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, stripRootHorizontalMargins bool) {
	if item == nil {
		return
	}
	switch item.Kind {
	case fb2.FlowParagraph:
		appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockParagraph, item.Paragraph, depth, styleClasses, stripRootHorizontalMargins)
	case fb2.FlowSubtitle:
		appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockSubtitle, item.Subtitle, depth, subtitleStyleClasses(styleClasses), stripRootHorizontalMargins)
	case fb2.FlowEmptyLine:
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: strings.TrimSpace(styleClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
	case fb2.FlowImage:
		appendImageBlockWithClassesAndRootHorizontalMargins(blocks, item.Image, "", styleClasses, stripRootHorizontalMargins)
	case fb2.FlowSection:
		if item.Section != nil && splitSections[item.Section.ID] {
			return
		}
		appendSectionBlocksWithRootHorizontalMargins(blocks, book, item.Section, depth+1, splitSections, stripRootHorizontalMargins)
	case fb2.FlowPoem:
		appendPoemBlocksWithRootHorizontalMargins(blocks, item.Poem, depth, splitSections, stripRootHorizontalMargins)
	case fb2.FlowCite:
		appendCiteBlocksWithRootHorizontalMargins(blocks, item.Cite, depth, splitSections, stripRootHorizontalMargins)
	case fb2.FlowTable:
		if item.Table != nil {
			text := item.Table.AsPlainText()
			if text != "" {
				*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockParagraph, Text: text, Depth: depth, StyleName: pdfStyleParagraph, StyleClasses: joinStyleClasses(styleClasses, pdfStyleTable), StripRootHorizontalMargins: stripRootHorizontalMargins})
			}
		}
	}
}

func appendImageBlock(blocks *[]pdfTextBlock, image *fb2.Image, fallbackID string) {
	appendImageBlockWithClasses(blocks, image, fallbackID, "")
}

func appendImageBlockWithClasses(blocks *[]pdfTextBlock, image *fb2.Image, fallbackID string, styleClasses string) {
	appendImageBlockWithClassesAndRootHorizontalMargins(blocks, image, fallbackID, styleClasses, false)
}

func appendImageBlockWithClassesAndRootHorizontalMargins(blocks *[]pdfTextBlock, image *fb2.Image, fallbackID string, styleClasses string, stripRootHorizontalMargins bool) {
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
	appendStyledImageIDBlockWithRootHorizontalMargins(blocks, imageID, anchorID, strings.TrimSpace(image.Alt), pdfStyleImage, strings.TrimSpace(styleClasses), stripRootHorizontalMargins)
}

func appendImageIDBlock(blocks *[]pdfTextBlock, imageID string, anchorID string, alt string, styleClasses string) {
	appendStyledImageIDBlock(blocks, imageID, anchorID, alt, pdfStyleImage, styleClasses)
}

func appendStyledImageIDBlock(blocks *[]pdfTextBlock, imageID string, anchorID string, alt string, styleName string, styleClasses string) {
	appendStyledImageIDBlockWithRootHorizontalMargins(blocks, imageID, anchorID, alt, styleName, styleClasses, false)
}

func appendStyledImageIDBlockWithRootHorizontalMargins(blocks *[]pdfTextBlock, imageID string, anchorID string, alt string, styleName string, styleClasses string, stripRootHorizontalMargins bool) {
	if strings.TrimSpace(imageID) == "" {
		return
	}
	if strings.TrimSpace(styleName) == "" {
		styleName = pdfStyleImage
	}
	*blocks = append(*blocks, pdfTextBlock{
		Kind:                       pdfBlockImage,
		ID:                         anchorID,
		Text:                       strings.TrimSpace(alt),
		StyleName:                  styleName,
		StyleClasses:               strings.TrimSpace(styleClasses),
		StripRootHorizontalMargins: stripRootHorizontalMargins,
		ImageID:                    imageID,
	})
}

func appendTitleVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int, top bool, styleClasses string) {
	if depth == 1 {
		if top {
			appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosChapterTitleTop, styleClasses, true)
			return
		}
		appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosChapterTitleBottom, styleClasses, true)
		return
	}
	if top {
		appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosSectionTitleTop, styleClasses, true)
		return
	}
	appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosSectionTitleBottom, styleClasses, true)
}

func appendEndVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int) {
	appendEndVignetteBlockWithRootHorizontalMargins(blocks, book, depth, false)
}

func appendEndVignetteBlockWithRootHorizontalMargins(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int, stripRootHorizontalMargins bool) {
	if depth == 1 {
		appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosChapterEnd, "", stripRootHorizontalMargins)
		return
	}
	appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, common.VignettePosSectionEnd, "", stripRootHorizontalMargins)
}

func sectionTitleContainerClasses(depth int) string {
	if depth <= 1 {
		return pdfStyleChapterTitle
	}
	if depth == 2 {
		return joinStyleClasses(pdfStyleSectionTitle, pdfStyleSectionTitleH2)
	}
	return pdfStyleSectionTitle
}

func sectionTitleHeaderStyleName(depth int) string {
	if depth <= 1 {
		return pdfStyleChapterTitleHeader
	}
	return pdfStyleSectionTitleHeader
}

func titleParagraphPositionStyleClass(headerStyleName string, first bool) string {
	if first {
		return headerStyleName + "-first"
	}
	return headerStyleName + "-next"
}

func appendVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, position common.VignettePos) {
	appendVignetteBlockWithClasses(blocks, book, position, "")
}

func appendVignetteBlockWithClasses(blocks *[]pdfTextBlock, book *fb2.FictionBook, position common.VignettePos, styleClasses string) {
	appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks, book, position, styleClasses, false)
}

func appendVignetteBlockWithClassesAndRootHorizontalMargins(blocks *[]pdfTextBlock, book *fb2.FictionBook, position common.VignettePos, styleClasses string, stripRootHorizontalMargins bool) {
	if book == nil || !book.IsVignetteEnabled(position) {
		return
	}
	imageID := strings.TrimSpace(book.VignetteIDs[position])
	if imageID == "" {
		return
	}
	appendStyledImageIDBlockWithRootHorizontalMargins(blocks, imageID, "", "", pdfStyleImage, joinStyleClasses("vignette", "vignette-"+position.String(), styleClasses), stripRootHorizontalMargins)
}

func isVignetteBlock(block pdfTextBlock) bool {
	return blockHasStyleClass(block, "vignette")
}

func titleWrapperStripRootHorizontalMargins(styleClasses string) bool {
	for _, class := range strings.Fields(styleClasses) {
		switch class {
		case pdfStyleBodyTitle, pdfStyleChapterTitle, pdfStyleSectionTitle:
			return true
		}
	}
	return false
}

func isHeadingImageBlock(block pdfTextBlock) bool {
	return blockHasStyleClass(block, pdfStyleHeadingImage)
}

func blockHasStyleClass(block pdfTextBlock, className string) bool {
	for _, class := range strings.Fields(block.StyleClasses) {
		if class == className {
			return true
		}
	}
	return false
}

func appendParagraphBlock(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int) {
	appendParagraphBlockWithClasses(blocks, kind, paragraph, depth, "")
}

func appendParagraphBlockWithClasses(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string) {
	appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, kind, paragraph, depth, styleClasses, false)
}

func appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string, stripRootHorizontalMargins bool) {
	if paragraph == nil {
		return
	}
	styleName := pdfStyleNameForKind(kind)
	if kind == pdfBlockSubtitle {
		styleName = subtitleStyleName(joinStyleClasses(paragraph.Style, styleClasses))
	}
	if imageID, alt, ok := paragraphImageOnly(paragraph); ok {
		imageStyleName := pdfStyleImage
		imageStyleClasses := strings.TrimSpace(paragraph.Style)
		if kind == pdfBlockHeading {
			imageStyleClasses = joinStyleClasses(pdfHeadingStyleName(depth), imageStyleClasses, styleClasses, pdfStyleHeadingImage)
		} else if kind == pdfBlockSubtitle {
			imageStyleName = styleName
			imageStyleClasses = joinStyleClasses(styleName, imageStyleClasses, styleClasses)
		} else if kind == pdfBlockParagraph {
			imageStyleName = styleName
			imageStyleClasses = joinStyleClasses(imageStyleClasses, styleClasses)
		}
		appendStyledImageIDBlockWithRootHorizontalMargins(blocks, imageID, paragraph.ID, alt, imageStyleName, imageStyleClasses, stripRootHorizontalMargins)
		return
	}
	text, links := paragraphTextAndLinks(paragraph)
	runs := paragraphInlineRuns(paragraph)
	if paragraphIsCodeBlock(paragraph) {
		styleClasses = joinStyleClasses(styleClasses, pdfStyleCode)
	}
	if text != "" || inlineRunsRenderable(runs) {
		*blocks = append(*blocks, pdfTextBlock{Kind: kind, ID: paragraph.ID, Text: text, Runs: runs, Depth: depth, StyleName: styleName, StyleClasses: joinStyleClasses(paragraph.Style, styleClasses), StripRootHorizontalMargins: stripRootHorizontalMargins, Links: links})
	}
}

func appendPoemBlocks(blocks *[]pdfTextBlock, poem *fb2.Poem, depth int, splitSections map[string]bool) {
	appendPoemBlocksWithRootHorizontalMargins(blocks, poem, depth, splitSections, false)
}

func appendPoemBlocksWithRootHorizontalMargins(blocks *[]pdfTextBlock, poem *fb2.Poem, depth int, splitSections map[string]bool, stripRootHorizontalMargins bool) {
	if poem == nil {
		return
	}
	appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks, poem.Title, depth+1, "", pdfHeadingStyleName(depth+1), "", stripRootHorizontalMargins)
	for i := range poem.Epigraphs {
		appendEpigraphBlocksWithRootHorizontalMargins(blocks, &poem.Epigraphs[i], stripRootHorizontalMargins)
	}
	for i := range poem.Subtitles {
		appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockSubtitle, &poem.Subtitles[i], depth, pdfStylePoemSubtitle, stripRootHorizontalMargins)
	}
	for i := range poem.Stanzas {
		stanza := &poem.Stanzas[i]
		appendTitleBlocksWithIDHeaderAndClassesAndRootHorizontalMargins(blocks, stanza.Title, depth+1, "", pdfHeadingStyleName(depth+1), "", stripRootHorizontalMargins)
		appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockSubtitle, stanza.Subtitle, depth, pdfStyleStanzaSubtitle, stripRootHorizontalMargins)
		for j := range stanza.Verses {
			appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockPoem, &stanza.Verses[j], depth, pdfStylePoem, stripRootHorizontalMargins)
		}
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StripRootHorizontalMargins: stripRootHorizontalMargins})
	}
	for i := range poem.TextAuthors {
		appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockTextAuthor, &poem.TextAuthors[i], depth, "", stripRootHorizontalMargins)
	}
}

func appendCiteBlocks(blocks *[]pdfTextBlock, cite *fb2.Cite, depth int, splitSections map[string]bool) {
	appendCiteBlocksWithRootHorizontalMargins(blocks, cite, depth, splitSections, false)
}

func appendCiteBlocksWithRootHorizontalMargins(blocks *[]pdfTextBlock, cite *fb2.Cite, depth int, splitSections map[string]bool, stripRootHorizontalMargins bool) {
	if cite == nil {
		return
	}
	appendFlowBlocksWithRootHorizontalMargins(blocks, nil, cite.Items, depth, splitSections, pdfStyleCite, stripRootHorizontalMargins)
	for i := range cite.TextAuthors {
		appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockTextAuthor, &cite.TextAuthors[i], depth, "", stripRootHorizontalMargins)
	}
}

func appendEpigraphBlocks(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph) {
	appendEpigraphBlocksWithRootHorizontalMargins(blocks, epigraph, false)
}

func appendEpigraphBlocksWithRootHorizontalMargins(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph, stripRootHorizontalMargins bool) {
	if epigraph == nil {
		return
	}
	appendFlowBlocksWithRootHorizontalMargins(blocks, nil, epigraph.Flow.Items, 1, nil, pdfStyleEpigraph, stripRootHorizontalMargins)
	for i := range epigraph.TextAuthors {
		appendParagraphBlockWithClassesAndRootHorizontalMargins(blocks, pdfBlockTextAuthor, &epigraph.TextAuthors[i], 1, "", stripRootHorizontalMargins)
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

func paragraphImageOnly(paragraph *fb2.Paragraph) (string, string, bool) {
	if paragraph == nil || len(paragraph.Text) == 0 {
		return "", "", false
	}
	var imageID string
	var alt string
	for i := range paragraph.Text {
		if !inlineSegmentImageOnly(&paragraph.Text[i], &imageID, &alt) {
			return "", "", false
		}
	}
	return imageID, alt, imageID != ""
}

func inlineSegmentImageOnly(seg *fb2.InlineSegment, imageID *string, alt *string) bool {
	if seg == nil {
		return true
	}
	if seg.Text != "" && strings.TrimSpace(seg.Text) != "" {
		return false
	}
	if seg.Kind == fb2.InlineImageSegment {
		if seg.Image == nil {
			return true
		}
		id := imageRefID(seg.Image.Href)
		if id == "" {
			return true
		}
		if *imageID != "" {
			return false
		}
		*imageID = id
		*alt = strings.TrimSpace(seg.Image.Alt)
	}
	for i := range seg.Children {
		if !inlineSegmentImageOnly(&seg.Children[i], imageID, alt) {
			return false
		}
	}
	return true
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

func subtitleStyleName(classes string) string {
	for _, class := range strings.Fields(classes) {
		switch class {
		case pdfStyleAnnotationSubtitle, pdfStylePoemSubtitle, pdfStyleStanzaSubtitle, pdfStyleEpigraphSubtitle, pdfStyleCiteSubtitle:
			return class
		}
	}
	return pdfStyleSubtitle
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
