package pdf

import (
	"strings"

	"fbc/common"
	"fbc/content"
	contentText "fbc/content/text"
	"fbc/fb2"
)

func appendBodyIntroBlocks(blocks *[]pdfTextBlock, c *content.Content, body *fb2.Body, includeImage bool) {
	if body == nil {
		return
	}
	var book *fb2.FictionBook
	if c != nil {
		book = c.Book
	}
	if includeImage {
		appendImageBlock(blocks, body.Image, "")
	}
	if body.Title != nil && body.Main() {
		appendVignette(blocks, book, common.VignettePosBookTitleTop, pdfStyleBodyTitle, pdfStyleBodyTitle, true)
	}
	appendTitleBlocksFull(blocks, c, body.Title, 1, "", pdfStyleBodyTitleHeader, pdfStyleBodyTitle, strings.TrimSpace(pdfStyleBodyTitle), false)
	if body.Title != nil && body.Main() {
		appendVignette(blocks, book, common.VignettePosBookTitleBottom, pdfStyleBodyTitle, pdfStyleBodyTitle, true)
	}
	for i := range body.Epigraphs {
		appendEpigraphBlocksFull(blocks, c, &body.Epigraphs[i], "", false)
	}
}

func appendFootnoteBodyBlocks(blocks *[]pdfTextBlock, c *content.Content, body *fb2.Body, splitSections map[string]bool) {
	if body == nil {
		return
	}
	appendBodyIntroBlocks(blocks, c, body, true)
	sectionBlocks := make([][]pdfTextBlock, len(body.Sections))
	for i := range body.Sections {
		appendFootnoteSectionContentBlocks(&sectionBlocks[i], c, &body.Sections[i], splitSections)
	}
	for i := range body.Sections {
		*blocks = append(*blocks, sectionBlocks[i]...)
		appendPDFDefaultFootnoteBacklinkBlock(blocks, c, body.Sections[i].ID)
	}
}

func appendFootnoteSectionContentBlocks(blocks *[]pdfTextBlock, c *content.Content, section *fb2.Section, splitSections map[string]bool) {
	if section == nil {
		return
	}
	appendTitleParagraphBlocks(blocks, c, section.Title, section.ID, pdfStyleFootnoteTitle, pdfStyleFootnoteTitle, false)
	bodyStart := len(*blocks)
	for i := range section.Epigraphs {
		appendEpigraphBlocksFull(blocks, c, &section.Epigraphs[i], "", false)
	}
	appendImageBlock(blocks, section.Image, section.ID)
	if section.Annotation != nil {
		appendFlowBlocks(blocks, c, section.Annotation.Items, 1, splitSections, pdfStyleAnnotation, pdfStyleAnnotation, false)
	}
	appendFlowBlocks(blocks, c, section.Content, 1, splitSections, pdfStyleFootnote, pdfStyleFootnote, false)
	for i := bodyStart; i < len(*blocks); i++ {
		(*blocks)[i].ContextClasses = joinStyleClasses((*blocks)[i].ContextClasses, pdfStyleFootnote)
	}
}

func appendPDFDefaultFootnoteBacklinkBlock(blocks *[]pdfTextBlock, c *content.Content, sectionID string) {
	if !pdfDefaultFootnoteBacklinksEnabled(c) || strings.TrimSpace(sectionID) == "" {
		return
	}
	refs := c.BackLinkIndex[sectionID]
	if len(refs) == 0 {
		return
	}
	refIDs := make([]string, 0, len(refs))
	for _, ref := range refs {
		refIDs = append(refIDs, ref.RefID)
	}
	text, runs := pdfBacklinkBlockContent(c, refIDs)
	if text == "" || len(runs) == 0 {
		return
	}
	*blocks = append(*blocks, pdfTextBlock{
		Kind:           pdfBlockParagraph,
		Text:           text,
		Runs:           runs,
		StyleName:      pdfStyleParagraph,
		ContextClasses: pdfStyleFootnote,
		BacklinkRefIDs: refIDs,
	})
}

func pdfBacklinkBlockContent(c *content.Content, refIDs []string) (string, []pdfInlineRun) {
	if c == nil || len(refIDs) == 0 {
		return "", nil
	}
	runs := make([]pdfInlineRun, 0, len(refIDs)*2-1)
	var text strings.Builder
	for i, refID := range refIDs {
		ref, ok := c.BackLinkRefByID(refID)
		if !ok {
			continue
		}
		backlinkText := c.BacklinkText(ref)
		if backlinkText == "" {
			continue
		}
		if text.Len() > 0 && i > 0 {
			runs = append(runs, pdfInlineRun{Text: contentText.NBSP})
			text.WriteString(contentText.NBSP)
		}
		runs = append(runs, pdfInlineRun{Text: backlinkText, StyleClasses: pdfStyleLinkBacklink, LinkHref: "#" + ref.RefID})
		text.WriteString(backlinkText)
	}
	return text.String(), runs
}

func appendSectionBlocks(blocks *[]pdfTextBlock, c *content.Content, section *fb2.Section, depth int, splitSections map[string]bool, contextClasses string, stripRootHorizontalMargins bool, endVignettes pdfSectionEndVignetteTransfers) {
	if section == nil {
		return
	}
	var book *fb2.FictionBook
	if c != nil {
		book = c.Book
		oldSectionTitle := c.CurrentSectionTitle
		if title := section.AsTitleText(""); title != "" {
			c.CurrentSectionTitle = title
		}
		defer func() { c.CurrentSectionTitle = oldSectionTitle }()
	}
	titleClasses := sectionTitleContainerClasses(depth)
	titleContextClasses := joinStyleClasses(contextClasses, titleClasses)
	headerStyle := sectionTitleHeaderStyleName(depth)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, true, titleClasses, titleContextClasses)
	}
	appendTitleBlocksFull(blocks, c, section.Title, depth, section.ID, headerStyle, titleClasses, titleContextClasses, stripRootHorizontalMargins)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, false, titleClasses, titleContextClasses)
	}
	for i := range section.Epigraphs {
		appendEpigraphBlocksFull(blocks, c, &section.Epigraphs[i], contextClasses, stripRootHorizontalMargins)
	}
	appendImageBlockFull(blocks, section.Image, section.ID, "", contextClasses, stripRootHorizontalMargins)
	if section.Annotation != nil {
		annotationContextClasses := joinStyleClasses(contextClasses, pdfStyleAnnotation)
		appendFlowBlocks(blocks, c, section.Annotation.Items, depth, splitSections, pdfStyleAnnotation, annotationContextClasses, stripRootHorizontalMargins)
	}
	for i := range section.Content {
		appendFlowItem(blocks, c, &section.Content[i], depth, splitSections, "", contextClasses, stripRootHorizontalMargins, endVignettes)
	}
	if section.Title != nil && !endVignettes.suppress[section] {
		appendEndVignette(blocks, book, depth, contextClasses, stripRootHorizontalMargins)
	}
	for _, position := range endVignettes.inherited[section] {
		appendVignette(blocks, book, position, "", contextClasses, stripRootHorizontalMargins)
	}
}

func appendTitleParagraphBlocks(blocks *[]pdfTextBlock, c *content.Content, title *fb2.Title, id string, styleClass string, contextClasses string, stripRootHorizontalMargins bool) {
	if title == nil {
		return
	}
	anchorID := id
	firstParagraph := true
	for i := range title.Items {
		item := &title.Items[i]
		if item.Paragraph == nil {
			continue
		}
		paragraph := *item.Paragraph
		if anchorID != "" && paragraph.ID == "" {
			paragraph.ID = anchorID
		}
		positionClass := titleParagraphPositionStyleClass(styleClass, firstParagraph)
		firstParagraph = false
		before := len(*blocks)
		appendParagraphBlockFull(blocks, c, pdfBlockParagraph, &paragraph, 1, joinStyleClasses(styleClass, positionClass), contextClasses, stripRootHorizontalMargins)
		if len(*blocks) > before {
			anchorID = ""
		}
	}
}

func appendTitleBlocksFull(blocks *[]pdfTextBlock, c *content.Content, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
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
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: joinStyleClasses(styleClasses, headerStyleName+"-emptyline"), ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: blockStripRootHorizontalMargins})
			prevWasImageOnlyHeadingParagraph = false
			continue
		}
		if item.Paragraph == nil {
			continue
		}
		positionClass := titleParagraphPositionStyleClass(headerStyleName, firstParagraph)
		firstParagraph = false
		if imageID, alt, ok := paragraphImageOnly(item.Paragraph); ok {
			appendStyledImage(blocks, imageID, anchorID, alt, pdfStyleImage, joinStyleClasses(headerStyleName, item.Paragraph.Style, styleClasses, positionClass, pdfStyleHeadingImage), contextClasses, blockStripRootHorizontalMargins)
			anchorID = ""
			prevWasImageOnlyHeadingParagraph = true
			continue
		}
		text, links := paragraphTextAndLinks(item.Paragraph)
		runs := paragraphInlineRunsWithBacklinks(item.Paragraph, c, pdfRegisterDefaultFootnoteBacklinks(c, styleClasses, contextClasses))
		classes := joinStyleClasses(item.Paragraph.Style, styleClasses, positionClass)
		runs, links = pdfDisablePrintedFootnoteLinks(c, classes, contextClasses, runs, links)
		if prevWasImageOnlyHeadingParagraph {
			classes = joinStyleClasses(classes, pdfStyleTitleAfterImage)
		}
		if paragraphIsCodeBlock(item.Paragraph) {
			classes = joinStyleClasses(classes, pdfStyleCode)
		}
		if text != "" || inlineRunsRenderable(runs) {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockHeading, ID: anchorID, Text: text, Runs: runs, Depth: depth, StyleName: headerStyleName, StyleClasses: classes, ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: blockStripRootHorizontalMargins, Links: links})
			anchorID = ""
			prevWasImageOnlyHeadingParagraph = false
		}
	}
}

func appendFlowBlocks(blocks *[]pdfTextBlock, c *content.Content, items []fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
	appendFlowBlocksWithEndVignettes(blocks, c, items, depth, splitSections, styleClasses, contextClasses, stripRootHorizontalMargins, pdfSectionEndVignetteTransfers{})
}

func appendFlowBlocksWithEndVignettes(blocks *[]pdfTextBlock, c *content.Content, items []fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, contextClasses string, stripRootHorizontalMargins bool, endVignettes pdfSectionEndVignetteTransfers) {
	for i := range items {
		appendFlowItem(blocks, c, &items[i], depth, splitSections, styleClasses, contextClasses, stripRootHorizontalMargins, endVignettes)
	}
}

func appendFlowItem(blocks *[]pdfTextBlock, c *content.Content, item *fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, contextClasses string, stripRootHorizontalMargins bool, endVignettes pdfSectionEndVignetteTransfers) {
	if item == nil {
		return
	}
	switch item.Kind {
	case fb2.FlowParagraph:
		appendParagraphBlockFull(blocks, c, pdfBlockParagraph, item.Paragraph, depth, styleClasses, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowSubtitle:
		appendParagraphBlockFull(blocks, c, pdfBlockSubtitle, item.Subtitle, depth, subtitleStyleClasses(styleClasses), contextClasses, stripRootHorizontalMargins)
	case fb2.FlowEmptyLine:
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: strings.TrimSpace(styleClasses), ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
	case fb2.FlowImage:
		appendImageBlockFull(blocks, item.Image, "", styleClasses, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowSection:
		if item.Section != nil && splitSections[item.Section.ID] {
			return
		}
		appendSectionBlocks(blocks, c, item.Section, depth+1, splitSections, "", stripRootHorizontalMargins, endVignettes)
	case fb2.FlowPoem:
		appendPoemBlocks(blocks, c, item.Poem, depth, splitSections, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowCite:
		appendCiteBlocks(blocks, c, item.Cite, depth, splitSections, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowTable:
		if item.Table != nil && len(item.Table.Rows) > 0 {
			block := pdfTextBlock{
				Kind:                       pdfBlockTable,
				ID:                         item.Table.ID,
				Depth:                      depth,
				StyleName:                  pdfStyleTable,
				StyleClasses:               joinStyleClasses(styleClasses, item.Table.Style),
				ContextClasses:             strings.TrimSpace(contextClasses),
				StripRootHorizontalMargins: stripRootHorizontalMargins,
				Table:                      item.Table,
			}
			if pdfRegisterDefaultFootnoteBacklinks(c, styleClasses, contextClasses) {
				block.TableCellRuns = pdfTableCellInlineRuns(item.Table, c, true)
			}
			*blocks = append(*blocks, block)
		}
	}
}

func pdfTableCellInlineRuns(table *fb2.Table, c *content.Content, registerBacklinks bool) map[pdfTableCellKey][]pdfInlineRun {
	placedCells, _ := pdfPlacedTableCells(table)
	if len(placedCells) == 0 {
		return nil
	}
	runs := make(map[pdfTableCellKey][]pdfInlineRun, len(placedCells))
	for _, placed := range placedCells {
		paragraph := fb2.Paragraph{Text: placed.Cell.Content}
		runs[pdfTableCellKey{placed.Row, placed.Col}] = paragraphInlineRunsWithBacklinks(&paragraph, c, registerBacklinks)
	}
	return runs
}

func appendImageBlock(blocks *[]pdfTextBlock, image *fb2.Image, fallbackID string) {
	appendImageBlockWithClasses(blocks, image, fallbackID, "")
}

func appendImageBlockWithClasses(blocks *[]pdfTextBlock, image *fb2.Image, fallbackID string, styleClasses string) {
	appendImageBlockFull(blocks, image, fallbackID, styleClasses, "", false)
}

func appendImageBlockFull(blocks *[]pdfTextBlock, image *fb2.Image, fallbackID string, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
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
	appendStyledImage(blocks, imageID, anchorID, strings.TrimSpace(image.Alt), pdfStyleImage, joinStyleClasses("image-block", strings.TrimSpace(styleClasses)), contextClasses, stripRootHorizontalMargins)
}

func appendStyledImage(blocks *[]pdfTextBlock, imageID string, anchorID string, alt string, styleName string, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
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
		ContextClasses:             strings.TrimSpace(contextClasses),
		StripRootHorizontalMargins: stripRootHorizontalMargins,
		ImageID:                    imageID,
	})
}

func appendTitleVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int, top bool, styleClasses string, contextClasses string) {
	if depth == 1 {
		if top {
			appendVignette(blocks, book, common.VignettePosChapterTitleTop, styleClasses, contextClasses, true)
			return
		}
		appendVignette(blocks, book, common.VignettePosChapterTitleBottom, styleClasses, contextClasses, true)
		return
	}
	if top {
		appendVignette(blocks, book, common.VignettePosSectionTitleTop, styleClasses, contextClasses, true)
		return
	}
	appendVignette(blocks, book, common.VignettePosSectionTitleBottom, styleClasses, contextClasses, true)
}

func appendEndVignette(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int, contextClasses string, stripRootHorizontalMargins bool) {
	if depth == 1 {
		appendVignette(blocks, book, common.VignettePosChapterEnd, "", contextClasses, stripRootHorizontalMargins)
		return
	}
	appendVignette(blocks, book, common.VignettePosSectionEnd, "", contextClasses, stripRootHorizontalMargins)
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

func appendVignette(blocks *[]pdfTextBlock, book *fb2.FictionBook, position common.VignettePos, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
	if book == nil || !book.IsVignetteEnabled(position) {
		return
	}
	imageID := strings.TrimSpace(book.VignetteIDs[position])
	if imageID == "" {
		return
	}
	appendStyledImage(blocks, imageID, "", "", pdfStyleImage, joinStyleClasses("image-vignette", "vignette", "vignette-"+position.String(), styleClasses), contextClasses, stripRootHorizontalMargins)
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

func isTitleTopVignetteBlock(block pdfTextBlock) bool {
	return blockHasStyleClass(block, pdfStyleVignetteBookTop) || blockHasStyleClass(block, pdfStyleVignetteChapterTop) || blockHasStyleClass(block, pdfStyleVignetteSectionTop)
}

func isTitleBottomVignetteBlock(block pdfTextBlock) bool {
	return blockHasStyleClass(block, pdfStyleVignetteBookBottom) || blockHasStyleClass(block, pdfStyleVignetteChapterBot) || blockHasStyleClass(block, pdfStyleVignetteSectionBot)
}

func isTitleHeaderBlock(block pdfTextBlock) bool {
	switch block.StyleName {
	case pdfStyleBodyTitleHeader, pdfStyleChapterTitleHeader, pdfStyleSectionTitleHeader:
		return true
	default:
		return false
	}
}

func isTitleHeaderImageBlock(block pdfTextBlock) bool {
	return block.Kind == pdfBlockImage && isHeadingImageBlock(block)
}

func blockHasStyleClass(block pdfTextBlock, className string) bool {
	for _, class := range strings.Fields(block.StyleClasses) {
		if class == className {
			return true
		}
	}
	return false
}

func appendParagraphBlockFull(blocks *[]pdfTextBlock, c *content.Content, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
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
		appendStyledImage(blocks, imageID, paragraph.ID, alt, imageStyleName, imageStyleClasses, contextClasses, stripRootHorizontalMargins)
		return
	}
	text, links := paragraphTextAndLinks(paragraph)
	runs := paragraphInlineRunsWithBacklinks(paragraph, c, pdfRegisterDefaultFootnoteBacklinks(c, styleClasses, contextClasses))
	if paragraphIsCodeBlock(paragraph) {
		styleClasses = joinStyleClasses(styleClasses, pdfStyleCode)
	}
	finalStyleClasses := joinStyleClasses(paragraph.Style, styleClasses)
	runs, links = pdfDisablePrintedFootnoteLinks(c, finalStyleClasses, contextClasses, runs, links)
	if kind == pdfBlockParagraph && hasPDFStyleClass(finalStyleClasses, "has-dropcap") && !paragraphIsCodeBlock(paragraph) {
		runs = addPDFDropcapInlineRun(runs)
	}
	if text != "" || inlineRunsRenderable(runs) {
		*blocks = append(*blocks, pdfTextBlock{Kind: kind, ID: paragraph.ID, Text: text, Runs: runs, Depth: depth, StyleName: styleName, StyleClasses: finalStyleClasses, ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins, Links: links})
	}
}

func appendPoemBlocks(blocks *[]pdfTextBlock, c *content.Content, poem *fb2.Poem, depth int, splitSections map[string]bool, contextClasses string, stripRootHorizontalMargins bool) {
	if poem == nil {
		return
	}
	poemContextClasses := joinStyleClasses(contextClasses, pdfStylePoem)
	appendTitleParagraphBlocks(blocks, c, poem.Title, "", pdfStylePoemTitle, poemContextClasses, stripRootHorizontalMargins)
	for i := range poem.Epigraphs {
		appendEpigraphBlocksFull(blocks, c, &poem.Epigraphs[i], poemContextClasses, stripRootHorizontalMargins)
	}
	for i := range poem.Subtitles {
		appendParagraphBlockFull(blocks, c, pdfBlockSubtitle, &poem.Subtitles[i], depth, pdfStylePoemSubtitle, poemContextClasses, stripRootHorizontalMargins)
	}
	for i := range poem.Stanzas {
		stanza := &poem.Stanzas[i]
		stanzaContextClasses := joinStyleClasses(poemContextClasses, pdfStyleStanza)
		stanzaStart := len(*blocks)
		appendTitleParagraphBlocks(blocks, c, stanza.Title, "", pdfStyleStanzaTitle, stanzaContextClasses, stripRootHorizontalMargins)
		appendParagraphBlockFull(blocks, c, pdfBlockSubtitle, stanza.Subtitle, depth, pdfStyleStanzaSubtitle, stanzaContextClasses, stripRootHorizontalMargins)
		for j := range stanza.Verses {
			appendParagraphBlockFull(blocks, c, pdfBlockPoem, &stanza.Verses[j], depth, pdfStylePoem, stanzaContextClasses, stripRootHorizontalMargins)
		}
		applyStyleClassToBlocks((*blocks)[stanzaStart:], pdfStyleStanza)
		if i < len(poem.Stanzas)-1 {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, ContextClasses: strings.TrimSpace(stanzaContextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
		}
	}
	for i := range poem.TextAuthors {
		appendParagraphBlockFull(blocks, c, pdfBlockTextAuthor, &poem.TextAuthors[i], depth, "", poemContextClasses, stripRootHorizontalMargins)
	}
	if dateText := poemDateText(poem.Date); dateText != "" {
		appendParagraphBlockFull(blocks, c, pdfBlockParagraph, &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: dateText}}}, depth, pdfStyleDate, poemContextClasses, stripRootHorizontalMargins)
	}
}

func applyStyleClassToBlocks(blocks []pdfTextBlock, class string) {
	for i := range blocks {
		blocks[i].StyleClasses = joinStyleClasses(blocks[i].StyleClasses, class)
	}
}

func poemDateText(date *fb2.Date) string {
	if date == nil {
		return ""
	}
	if date.Display != "" {
		return date.Display
	}
	if !date.Value.IsZero() {
		return date.Value.Format("2006-01-02")
	}
	return ""
}

func appendCiteBlocks(blocks *[]pdfTextBlock, c *content.Content, cite *fb2.Cite, depth int, splitSections map[string]bool, contextClasses string, stripRootHorizontalMargins bool) {
	if cite == nil {
		return
	}
	citeContextClasses := joinStyleClasses(contextClasses, pdfStyleCite)
	appendFlowBlocks(blocks, c, cite.Items, depth, splitSections, pdfStyleCite, citeContextClasses, stripRootHorizontalMargins)
	for i := range cite.TextAuthors {
		appendParagraphBlockFull(blocks, c, pdfBlockTextAuthor, &cite.TextAuthors[i], depth, "", citeContextClasses, stripRootHorizontalMargins)
	}
}

func appendEpigraphBlocksFull(blocks *[]pdfTextBlock, c *content.Content, epigraph *fb2.Epigraph, contextClasses string, stripRootHorizontalMargins bool) {
	if epigraph == nil {
		return
	}
	epigraphContextClasses := joinStyleClasses(contextClasses, pdfStyleEpigraph)
	appendFlowBlocks(blocks, c, epigraph.Flow.Items, 1, nil, pdfStyleEpigraph, epigraphContextClasses, stripRootHorizontalMargins)
	for i := range epigraph.TextAuthors {
		appendParagraphBlockFull(blocks, c, pdfBlockTextAuthor, &epigraph.TextAuthors[i], 1, "", epigraphContextClasses, stripRootHorizontalMargins)
	}
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
