package pdf

import (
	"strings"

	"fbc/common"
	"fbc/config"
	"fbc/content"
	"fbc/convert/pdf/structure"
	"fbc/convert/tocnav"
	"fbc/fb2"
)

func collectPDFContent(c *content.Content, cfg *config.DocumentConfig) (pdfContentPlan, error) {
	if c == nil || c.Book == nil {
		return pdfContentPlan{}, nil
	}
	plan, err := structure.BuildPlan(c)
	if err != nil {
		return pdfContentPlan{}, err
	}
	if pdfDefaultFootnoteBacklinksEnabled(c) {
		c.BackLinkIndex = make(map[string][]content.BackLinkRef)
	}

	debugPlan := pdfDebugStructurePlanFromPlan(plan, cfg)
	blocks := make([]pdfTextBlock, 0, 64)
	splitSections := splitSectionIDs(plan)
	splitBodies := splitBodyImageBodies(plan)
	endVignettes := pdfSectionEndVignetteTransfersForPlan(c.Book, plan)
	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.ForceNewPage && unit.Kind != structure.UnitCover {
			blocks = append(blocks, pdfTextBlock{Kind: pdfBlockPageBreak, ID: unit.ID, Text: unit.Title})
		}
		appendUnitBlocks(&blocks, c, unit, splitSections, splitBodies, endVignettes)
	}
	toc := plan.TOC
	blocks, toc = insertAnnotationPageBlocks(blocks, toc, c, cfg)
	blocks = insertTOCPageBlocks(blocks, c, toc, cfg)
	debugPlan.TOC = pdfDebugStructureTOCEntries(toc)
	return pdfContentPlan{
		Blocks:           blocks,
		TOC:              toc,
		PrintedFootnotes: buildPDFPrintedFootnoteBlocks(c),
		DebugPlan:        debugPlan,
	}, nil
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

func insertAnnotationPageBlocks(
	blocks []pdfTextBlock,
	toc []*structure.TOCEntry,
	c *content.Content,
	cfg *config.DocumentConfig,
) ([]pdfTextBlock, []*structure.TOCEntry) {
	if cfg == nil || !cfg.Annotation.Enable || c == nil || c.Book == nil || c.Book.Description.TitleInfo.Annotation == nil ||
		len(c.Book.Description.TitleInfo.Annotation.Items) == 0 {
		return blocks, toc
	}
	title := strings.TrimSpace(cfg.Annotation.Title)
	if title == "" {
		title = "Annotation"
	}
	annotationBlocks := []pdfTextBlock{{Kind: pdfBlockPageBreak, ID: "annotation-page", Text: title}}
	appendTitleBlocksWithOptions(&annotationBlocks, pdfTitleBlockOptions{
		Content:         c,
		Title:           pdfTitleFromStrings(title),
		Depth:           1,
		ID:              "annotation-page-title",
		HeaderStyleName: pdfStyleAnnotationTitle,
		ContextClasses:  pdfStyleAnnotationTitle,
	})
	appendFlowBlocks(&annotationBlocks, c, c.Book.Description.TitleInfo.Annotation.Items, 1, nil, pdfStyleAnnotation, pdfStyleAnnotation, false)
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

func insertTOCPageBlocks(blocks []pdfTextBlock, c *content.Content, entries []*structure.TOCEntry, cfg *config.DocumentConfig) []pdfTextBlock {
	if cfg == nil || cfg.TOCPage.Placement == common.TOCPagePlacementNone || len(entries) == 0 {
		return blocks
	}
	tocBlocks := buildTOCPageBlocksWithTitle(pdfTOCPageTitle(c, cfg), entries, cfg.TOCPage.ChaptersWithoutTitle, cfg.TOCType)
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

func buildTOCPageBlocksWithTitle(title *fb2.Title, entries []*structure.TOCEntry, includeUntitled bool, tocType common.TOCType) []pdfTextBlock {
	items := flattenPDFTOCEntries(entries, includeUntitled, 1)
	if len(items) == 0 {
		return nil
	}
	blocks := []pdfTextBlock{{Kind: pdfBlockPageBreak, ID: "toc-page", Text: "Contents"}}
	appendTitleBlocksWithOptions(&blocks, pdfTitleBlockOptions{
		Title:           title,
		Depth:           1,
		ID:              "toc-page-title",
		HeaderStyleName: pdfStyleTOCTitle,
		ContextClasses:  pdfStyleTOCTitle,
	})
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
	if len(blocks) == 1 {
		return nil
	}
	return blocks
}

func pdfTOCPageTitle(c *content.Content, cfg *config.DocumentConfig) *fb2.Title {
	title := "Contents"
	if c != nil && c.Book != nil {
		if bookTitle := strings.TrimSpace(c.Book.Description.TitleInfo.BookTitle.Value); bookTitle != "" {
			title = bookTitle
		}
	}
	var authors string
	if c != nil && c.Book != nil && cfg != nil && strings.TrimSpace(cfg.TOCPage.AuthorsTemplate) != "" {
		if expanded, err := c.Book.ExpandTemplateMetainfo(
			config.AuthorsTemplateFieldName,
			cfg.TOCPage.AuthorsTemplate,
			c.SrcName,
			c.OutputFormat,
		); err == nil {
			authors = strings.TrimSpace(expanded)
		}
	}
	return pdfTitleFromStrings(title, authors)
}

func pdfTitleFromStrings(lines ...string) *fb2.Title {
	items := make([]fb2.TitleItem, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		items = append(items, fb2.TitleItem{Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Text: strings.TrimSpace(line)}}}})
	}
	if len(items) == 0 {
		return nil
	}
	return &fb2.Title{Items: items}
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

func appendUnitBlocks(
	blocks *[]pdfTextBlock,
	c *content.Content,
	unit *structure.Unit,
	splitSections map[string]bool,
	splitBodies map[*fb2.Body]bool,
	endVignettes pdfSectionEndVignetteTransfers,
) {
	if unit == nil {
		return
	}
	switch unit.Kind {
	case structure.UnitBodyImage:
		if unit.Body != nil {
			appendImageBlockWithOptions(blocks, pdfImageBlockOptions{Image: unit.Body.Image, FallbackID: unit.ID})
		}
	case structure.UnitBodyIntro:
		appendBodyIntroBlocks(blocks, c, unit.Body, !splitBodies[unit.Body])
	case structure.UnitSection:
		appendSectionBlocksWithOptions(blocks, pdfSectionBlockOptions{
			Content:       c,
			Section:       unit.Section,
			Depth:         unit.TitleDepth,
			SplitSections: splitSections,
			EndVignettes:  endVignettes,
		})
	case structure.UnitFootnotesBody:
		appendFootnoteBodyBlocks(blocks, c, unit.Body, splitSections)
	}
}
