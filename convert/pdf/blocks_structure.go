package pdf

import (
	"strings"

	"fbc/common"
	"fbc/fb2"
)

func appendBodyIntroBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, body *fb2.Body, includeImage bool) {
	if body == nil {
		return
	}
	if includeImage {
		appendImageBlock(blocks, body.Image, "")
	}
	if body.Title != nil && body.Main() {
		appendVignette(blocks, book, common.VignettePosBookTitleTop, pdfStyleBodyTitle, pdfStyleBodyTitle, true)
	}
	appendTitleBlocksWithIDHeaderAndClasses(blocks, body.Title, 1, "", pdfStyleBodyTitleHeader, pdfStyleBodyTitle)
	if body.Title != nil && body.Main() {
		appendVignette(blocks, book, common.VignettePosBookTitleBottom, pdfStyleBodyTitle, pdfStyleBodyTitle, true)
	}
	for i := range body.Epigraphs {
		appendEpigraphBlocks(blocks, &body.Epigraphs[i])
	}
}

func appendFootnoteBodyBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, body *fb2.Body, splitSections map[string]bool) {
	if body == nil {
		return
	}
	appendBodyIntroBlocks(blocks, book, body, true)
	for i := range body.Sections {
		appendFootnoteSectionBlocks(blocks, book, &body.Sections[i], splitSections)
	}
}

func appendFootnoteSectionBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, section *fb2.Section, splitSections map[string]bool) {
	if section == nil {
		return
	}
	appendTitleParagraphBlocksWithIDAndClasses(blocks, section.Title, section.ID, pdfStyleFootnoteTitle)
	for i := range section.Epigraphs {
		appendEpigraphBlocks(blocks, &section.Epigraphs[i])
	}
	appendImageBlock(blocks, section.Image, section.ID)
	if section.Annotation != nil {
		appendFlowBlocks(blocks, book, section.Annotation.Items, 1, splitSections, pdfStyleAnnotation, pdfStyleAnnotation, false)
	}
	appendFlowBlocks(blocks, book, section.Content, 1, splitSections, pdfStyleFootnote, pdfStyleFootnote, false)
}

func appendSectionBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, section *fb2.Section, depth int, splitSections map[string]bool, contextClasses string, stripRootHorizontalMargins bool) {
	if section == nil {
		return
	}
	titleClasses := sectionTitleContainerClasses(depth)
	titleContextClasses := joinStyleClasses(contextClasses, titleClasses)
	headerStyle := sectionTitleHeaderStyleName(depth)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, true, titleClasses, titleContextClasses)
	}
	appendTitleBlocksFull(blocks, section.Title, depth, section.ID, headerStyle, titleClasses, titleContextClasses, stripRootHorizontalMargins)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, false, titleClasses, titleContextClasses)
	}
	for i := range section.Epigraphs {
		appendEpigraphBlocksFull(blocks, &section.Epigraphs[i], contextClasses, stripRootHorizontalMargins)
	}
	appendImageBlockFull(blocks, section.Image, section.ID, "", contextClasses, stripRootHorizontalMargins)
	if section.Annotation != nil {
		annotationContextClasses := joinStyleClasses(contextClasses, pdfStyleAnnotation)
		appendFlowBlocks(blocks, book, section.Annotation.Items, depth, splitSections, pdfStyleAnnotation, annotationContextClasses, stripRootHorizontalMargins || len(section.Annotation.Items) > 1)
	}
	for i := range section.Content {
		appendFlowItem(blocks, book, &section.Content[i], depth, splitSections, "", contextClasses, stripRootHorizontalMargins)
	}
	if section.Title != nil {
		appendEndVignette(blocks, book, depth, contextClasses, stripRootHorizontalMargins)
	}
}

func appendTitleBlocks(blocks *[]pdfTextBlock, title *fb2.Title, depth int) {
	appendTitleBlocksFull(blocks, title, depth, "", pdfHeadingStyleName(depth), "", "", false)
}

func appendTitleParagraphBlocksWithIDAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, id string, styleClass string) {
	appendTitleParagraphBlocks(blocks, title, id, styleClass, "", false)
}

func appendTitleParagraphBlocks(blocks *[]pdfTextBlock, title *fb2.Title, id string, styleClass string, contextClasses string, stripRootHorizontalMargins bool) {
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
		appendParagraphBlockFull(blocks, pdfBlockParagraph, &paragraph, 1, joinStyleClasses(styleClass, positionClass), contextClasses, stripRootHorizontalMargins)
		if len(*blocks) > before {
			anchorID = ""
		}
	}
}

func appendTitleBlocksWithID(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string) {
	appendTitleBlocksFull(blocks, title, depth, id, pdfHeadingStyleName(depth), "", "", false)
}

func appendTitleBlocksWithIDAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, styleClasses string) {
	appendTitleBlocksFull(blocks, title, depth, id, pdfHeadingStyleName(depth), styleClasses, strings.TrimSpace(styleClasses), false)
}

func appendTitleBlocksWithIDHeaderAndClasses(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string) {
	appendTitleBlocksFull(blocks, title, depth, id, headerStyleName, styleClasses, strings.TrimSpace(styleClasses), false)
}

func appendTitleBlocksFull(blocks *[]pdfTextBlock, title *fb2.Title, depth int, id string, headerStyleName string, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
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
		runs := paragraphInlineRuns(item.Paragraph)
		classes := joinStyleClasses(item.Paragraph.Style, styleClasses, positionClass)
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

func appendFlowBlocks(blocks *[]pdfTextBlock, book *fb2.FictionBook, items []fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
	for i := range items {
		appendFlowItem(blocks, book, &items[i], depth, splitSections, styleClasses, contextClasses, stripRootHorizontalMargins)
	}
}

func appendFlowItem(blocks *[]pdfTextBlock, book *fb2.FictionBook, item *fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
	if item == nil {
		return
	}
	switch item.Kind {
	case fb2.FlowParagraph:
		appendParagraphBlockFull(blocks, pdfBlockParagraph, item.Paragraph, depth, styleClasses, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowSubtitle:
		appendParagraphBlockFull(blocks, pdfBlockSubtitle, item.Subtitle, depth, subtitleStyleClasses(styleClasses), contextClasses, stripRootHorizontalMargins)
	case fb2.FlowEmptyLine:
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: strings.TrimSpace(styleClasses), ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
	case fb2.FlowImage:
		appendImageBlockFull(blocks, item.Image, "", styleClasses, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowSection:
		if item.Section != nil && splitSections[item.Section.ID] {
			return
		}
		appendSectionBlocks(blocks, book, item.Section, depth+1, splitSections, "", stripRootHorizontalMargins)
	case fb2.FlowPoem:
		appendPoemBlocks(blocks, item.Poem, depth, splitSections, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowCite:
		appendCiteBlocks(blocks, item.Cite, depth, splitSections, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowTable:
		if item.Table != nil {
			text := item.Table.AsPlainText()
			if text != "" {
				*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockParagraph, Text: text, Depth: depth, StyleName: pdfStyleParagraph, StyleClasses: joinStyleClasses(styleClasses, pdfStyleTable), ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
			}
		}
	}
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

func blockHasStyleClass(block pdfTextBlock, className string) bool {
	for _, class := range strings.Fields(block.StyleClasses) {
		if class == className {
			return true
		}
	}
	return false
}

func appendParagraphBlockWithClasses(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string) {
	appendParagraphBlockFull(blocks, kind, paragraph, depth, styleClasses, strings.TrimSpace(styleClasses), false)
}

func appendParagraphBlockFull(blocks *[]pdfTextBlock, kind pdfBlockKind, paragraph *fb2.Paragraph, depth int, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
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
	runs := paragraphInlineRuns(paragraph)
	if paragraphIsCodeBlock(paragraph) {
		styleClasses = joinStyleClasses(styleClasses, pdfStyleCode)
	}
	if text != "" || inlineRunsRenderable(runs) {
		*blocks = append(*blocks, pdfTextBlock{Kind: kind, ID: paragraph.ID, Text: text, Runs: runs, Depth: depth, StyleName: styleName, StyleClasses: joinStyleClasses(paragraph.Style, styleClasses), ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins, Links: links})
	}
}

func appendPoemBlocks(blocks *[]pdfTextBlock, poem *fb2.Poem, depth int, splitSections map[string]bool, contextClasses string, stripRootHorizontalMargins bool) {
	if poem == nil {
		return
	}
	poemContextClasses := joinStyleClasses(contextClasses, pdfStylePoem)
	appendTitleBlocksFull(blocks, poem.Title, depth+1, "", pdfHeadingStyleName(depth+1), "", poemContextClasses, stripRootHorizontalMargins)
	for i := range poem.Epigraphs {
		appendEpigraphBlocksFull(blocks, &poem.Epigraphs[i], poemContextClasses, stripRootHorizontalMargins)
	}
	for i := range poem.Subtitles {
		appendParagraphBlockFull(blocks, pdfBlockSubtitle, &poem.Subtitles[i], depth, pdfStylePoemSubtitle, poemContextClasses, stripRootHorizontalMargins)
	}
	for i := range poem.Stanzas {
		stanza := &poem.Stanzas[i]
		appendTitleBlocksFull(blocks, stanza.Title, depth+1, "", pdfHeadingStyleName(depth+1), "", poemContextClasses, stripRootHorizontalMargins)
		appendParagraphBlockFull(blocks, pdfBlockSubtitle, stanza.Subtitle, depth, pdfStyleStanzaSubtitle, poemContextClasses, stripRootHorizontalMargins)
		for j := range stanza.Verses {
			appendParagraphBlockFull(blocks, pdfBlockPoem, &stanza.Verses[j], depth, pdfStylePoem, poemContextClasses, stripRootHorizontalMargins)
		}
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, ContextClasses: strings.TrimSpace(poemContextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
	}
	for i := range poem.TextAuthors {
		appendParagraphBlockFull(blocks, pdfBlockTextAuthor, &poem.TextAuthors[i], depth, "", poemContextClasses, stripRootHorizontalMargins)
	}
}

func appendCiteBlocks(blocks *[]pdfTextBlock, cite *fb2.Cite, depth int, splitSections map[string]bool, contextClasses string, stripRootHorizontalMargins bool) {
	if cite == nil {
		return
	}
	citeContextClasses := joinStyleClasses(contextClasses, pdfStyleCite)
	appendFlowBlocks(blocks, nil, cite.Items, depth, splitSections, pdfStyleCite, citeContextClasses, stripRootHorizontalMargins)
	for i := range cite.TextAuthors {
		appendParagraphBlockFull(blocks, pdfBlockTextAuthor, &cite.TextAuthors[i], depth, "", citeContextClasses, stripRootHorizontalMargins)
	}
}

func appendEpigraphBlocks(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph) {
	appendEpigraphBlocksFull(blocks, epigraph, "", false)
}

func appendEpigraphBlocksFull(blocks *[]pdfTextBlock, epigraph *fb2.Epigraph, contextClasses string, stripRootHorizontalMargins bool) {
	if epigraph == nil {
		return
	}
	epigraphContextClasses := joinStyleClasses(contextClasses, pdfStyleEpigraph)
	appendFlowBlocks(blocks, nil, epigraph.Flow.Items, 1, nil, pdfStyleEpigraph, epigraphContextClasses, stripRootHorizontalMargins)
	for i := range epigraph.TextAuthors {
		appendParagraphBlockFull(blocks, pdfBlockTextAuthor, &epigraph.TextAuthors[i], 1, "", epigraphContextClasses, stripRootHorizontalMargins)
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
