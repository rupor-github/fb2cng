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
		appendImageBlockWithOptions(blocks, pdfImageBlockOptions{Image: body.Image})
	}
	if body.Title != nil && body.Main() {
		appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
			Book:                       book,
			Position:                   common.VignettePosBookTitleTop,
			StyleClasses:               pdfStyleBodyTitle,
			ContextClasses:             pdfStyleBodyTitle,
			StripRootHorizontalMargins: true,
		})
	}
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{
		Content:         c,
		Title:           body.Title,
		Depth:           1,
		HeaderStyleName: pdfStyleBodyTitleHeader,
		StyleClasses:    pdfStyleBodyTitle,
		ContextClasses:  strings.TrimSpace(pdfStyleBodyTitle),
	})
	if body.Title != nil && body.Main() {
		appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
			Book:                       book,
			Position:                   common.VignettePosBookTitleBottom,
			StyleClasses:               pdfStyleBodyTitle,
			ContextClasses:             pdfStyleBodyTitle,
			StripRootHorizontalMargins: true,
		})
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
	appendImageBlockWithOptions(blocks, pdfImageBlockOptions{Image: section.Image, FallbackID: section.ID})
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

type pdfSectionBlockOptions struct {
	Content                    *content.Content
	Section                    *fb2.Section
	Depth                      int
	SplitSections              map[string]bool
	ContextClasses             string
	StripRootHorizontalMargins bool
	EndVignettes               pdfSectionEndVignetteTransfers
}

func appendSectionBlocksWithOptions(blocks *[]pdfTextBlock, opts pdfSectionBlockOptions) {
	section := opts.Section
	if section == nil {
		return
	}
	content := opts.Content
	depth := opts.Depth
	splitSections := opts.SplitSections
	contextClasses := opts.ContextClasses
	stripRootHorizontalMargins := opts.StripRootHorizontalMargins
	endVignettes := opts.EndVignettes
	var book *fb2.FictionBook
	if content != nil {
		book = content.Book
		oldSectionTitle := content.CurrentSectionTitle
		if title := section.AsTitleText(""); title != "" {
			content.CurrentSectionTitle = title
		}
		defer func() { content.CurrentSectionTitle = oldSectionTitle }()
	}
	titleClasses := sectionTitleContainerClasses(depth)
	titleContextClasses := joinStyleClasses(contextClasses, titleClasses)
	headerStyle := sectionTitleHeaderStyleName(depth)
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, true, titleClasses, titleContextClasses)
	}
	appendTitleBlocksWithOptions(blocks, pdfTitleBlockOptions{
		Content:                    content,
		Title:                      section.Title,
		Depth:                      depth,
		ID:                         section.ID,
		HeaderStyleName:            headerStyle,
		StyleClasses:               titleClasses,
		ContextClasses:             titleContextClasses,
		StripRootHorizontalMargins: stripRootHorizontalMargins,
	})
	if section.Title != nil {
		appendTitleVignetteBlock(blocks, book, depth, false, titleClasses, titleContextClasses)
	}
	for i := range section.Epigraphs {
		appendEpigraphBlocksFull(blocks, content, &section.Epigraphs[i], contextClasses, stripRootHorizontalMargins)
	}
	appendImageBlockWithOptions(blocks, pdfImageBlockOptions{
		Image:                      section.Image,
		FallbackID:                 section.ID,
		ContextClasses:             contextClasses,
		StripRootHorizontalMargins: stripRootHorizontalMargins,
	})
	if section.Annotation != nil {
		annotationContextClasses := joinStyleClasses(contextClasses, pdfStyleAnnotation)
		appendFlowBlocks(blocks, content, section.Annotation.Items, depth, splitSections, pdfStyleAnnotation, annotationContextClasses, stripRootHorizontalMargins)
	}
	for i := range section.Content {
		appendFlowItemWithContext(blocks, &section.Content[i], pdfBlockBuildContext{
			Content:                    content,
			Depth:                      depth,
			SplitSections:              splitSections,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
			EndVignettes:               endVignettes,
		})
	}
	if section.Title != nil && !endVignettes.suppress[section] {
		appendEndVignette(blocks, book, depth, contextClasses, stripRootHorizontalMargins)
	}
	for _, position := range endVignettes.inherited[section] {
		appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
			Book:                       book,
			Position:                   position,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
		})
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
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockParagraph, Paragraph: &paragraph, Depth: 1, StyleClasses: joinStyleClasses(styleClass, positionClass), ContextClasses: contextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
		if len(*blocks) > before {
			anchorID = ""
		}
	}
}

type pdfTitleBlockOptions struct {
	Content                    *content.Content
	Title                      *fb2.Title
	Depth                      int
	ID                         string
	HeaderStyleName            string
	StyleClasses               string
	ContextClasses             string
	StripRootHorizontalMargins bool
}

func appendTitleBlocksWithOptions(blocks *[]pdfTextBlock, opts pdfTitleBlockOptions) {
	title := opts.Title
	if title == nil {
		return
	}
	content := opts.Content
	depth := opts.Depth
	styleClasses := opts.StyleClasses
	contextClasses := opts.ContextClasses
	headerStyleName := opts.HeaderStyleName
	if headerStyleName == "" {
		headerStyleName = pdfHeadingStyleName(depth)
	}
	blockStripRootHorizontalMargins := opts.StripRootHorizontalMargins || titleWrapperStripRootHorizontalMargins(styleClasses)
	anchorID := opts.ID
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
			appendStyledImageWithOptions(blocks, pdfStyledImageBlockOptions{
				ImageID:                    imageID,
				AnchorID:                   anchorID,
				Alt:                        alt,
				StyleName:                  pdfStyleImage,
				StyleClasses:               joinStyleClasses(headerStyleName, item.Paragraph.Style, styleClasses, positionClass, pdfStyleHeadingImage),
				ContextClasses:             contextClasses,
				StripRootHorizontalMargins: blockStripRootHorizontalMargins,
			})
			anchorID = ""
			prevWasImageOnlyHeadingParagraph = true
			continue
		}
		text, links := paragraphTextAndLinks(item.Paragraph)
		runs := paragraphInlineRunsWithBacklinks(item.Paragraph, content, pdfRegisterDefaultFootnoteBacklinks(content, styleClasses, contextClasses))
		classes := joinStyleClasses(item.Paragraph.Style, styleClasses, positionClass)
		runs, links = pdfDisablePrintedFootnoteLinks(content, classes, contextClasses, runs, links)
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

type pdfBlockBuildContext struct {
	Content                    *content.Content
	Depth                      int
	SplitSections              map[string]bool
	StyleClasses               string
	ContextClasses             string
	StripRootHorizontalMargins bool
	EndVignettes               pdfSectionEndVignetteTransfers
}

func appendFlowBlocks(blocks *[]pdfTextBlock, c *content.Content, items []fb2.FlowItem, depth int, splitSections map[string]bool, styleClasses string, contextClasses string, stripRootHorizontalMargins bool) {
	appendFlowBlocksWithContext(blocks, items, pdfBlockBuildContext{
		Content:                    c,
		Depth:                      depth,
		SplitSections:              splitSections,
		StyleClasses:               styleClasses,
		ContextClasses:             contextClasses,
		StripRootHorizontalMargins: stripRootHorizontalMargins,
	})
}

func appendFlowBlocksWithContext(blocks *[]pdfTextBlock, items []fb2.FlowItem, ctx pdfBlockBuildContext) {
	for i := range items {
		appendFlowItemWithContext(blocks, &items[i], ctx)
	}
}

func appendFlowItemWithContext(blocks *[]pdfTextBlock, item *fb2.FlowItem, ctx pdfBlockBuildContext) {
	if item == nil {
		return
	}
	content := ctx.Content
	depth := ctx.Depth
	splitSections := ctx.SplitSections
	styleClasses := ctx.StyleClasses
	contextClasses := ctx.ContextClasses
	stripRootHorizontalMargins := ctx.StripRootHorizontalMargins
	switch item.Kind {
	case fb2.FlowParagraph:
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{
			Content:                    content,
			Kind:                       pdfBlockParagraph,
			Paragraph:                  item.Paragraph,
			Depth:                      depth,
			StyleClasses:               styleClasses,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
		})
	case fb2.FlowSubtitle:
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{
			Content:                    content,
			Kind:                       pdfBlockSubtitle,
			Paragraph:                  item.Subtitle,
			Depth:                      depth,
			StyleClasses:               subtitleStyleClasses(styleClasses),
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
		})
	case fb2.FlowEmptyLine:
		*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, StyleClasses: strings.TrimSpace(styleClasses), ContextClasses: strings.TrimSpace(contextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
	case fb2.FlowImage:
		appendImageBlockWithOptions(blocks, pdfImageBlockOptions{
			Image:                      item.Image,
			StyleClasses:               styleClasses,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
		})
	case fb2.FlowSection:
		if item.Section != nil && splitSections[item.Section.ID] {
			return
		}
		appendSectionBlocksWithOptions(blocks, pdfSectionBlockOptions{
			Content:                    content,
			Section:                    item.Section,
			Depth:                      depth + 1,
			SplitSections:              splitSections,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
			EndVignettes:               ctx.EndVignettes,
		})
	case fb2.FlowPoem:
		appendPoemBlocks(blocks, content, item.Poem, depth, splitSections, contextClasses, stripRootHorizontalMargins)
	case fb2.FlowCite:
		appendCiteBlocks(blocks, content, item.Cite, depth, splitSections, contextClasses, stripRootHorizontalMargins)
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
			if pdfRegisterDefaultFootnoteBacklinks(content, styleClasses, contextClasses) {
				block.TableCellRuns = pdfTableCellInlineRuns(item.Table, content, true)
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

type pdfImageBlockOptions struct {
	Image                      *fb2.Image
	FallbackID                 string
	StyleClasses               string
	ContextClasses             string
	StripRootHorizontalMargins bool
}

func appendImageBlockWithOptions(blocks *[]pdfTextBlock, opts pdfImageBlockOptions) {
	image := opts.Image
	if image == nil {
		return
	}
	imageID := imageRefID(image.Href)
	if imageID == "" {
		return
	}
	anchorID := image.ID
	if anchorID == "" {
		anchorID = opts.FallbackID
	}
	appendStyledImageWithOptions(blocks, pdfStyledImageBlockOptions{
		ImageID:                    imageID,
		AnchorID:                   anchorID,
		Alt:                        strings.TrimSpace(image.Alt),
		StyleName:                  pdfStyleImage,
		StyleClasses:               joinStyleClasses("image-block", strings.TrimSpace(opts.StyleClasses)),
		ContextClasses:             opts.ContextClasses,
		StripRootHorizontalMargins: opts.StripRootHorizontalMargins,
	})
}

type pdfStyledImageBlockOptions struct {
	ImageID                    string
	AnchorID                   string
	Alt                        string
	StyleName                  string
	StyleClasses               string
	ContextClasses             string
	StripRootHorizontalMargins bool
}

func appendStyledImageWithOptions(blocks *[]pdfTextBlock, opts pdfStyledImageBlockOptions) {
	if strings.TrimSpace(opts.ImageID) == "" {
		return
	}
	styleName := opts.StyleName
	if strings.TrimSpace(styleName) == "" {
		styleName = pdfStyleImage
	}
	*blocks = append(*blocks, pdfTextBlock{
		Kind:                       pdfBlockImage,
		ID:                         opts.AnchorID,
		Text:                       strings.TrimSpace(opts.Alt),
		StyleName:                  styleName,
		StyleClasses:               strings.TrimSpace(opts.StyleClasses),
		ContextClasses:             strings.TrimSpace(opts.ContextClasses),
		StripRootHorizontalMargins: opts.StripRootHorizontalMargins,
		ImageID:                    opts.ImageID,
	})
}

func appendTitleVignetteBlock(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int, top bool, styleClasses string, contextClasses string) {
	if depth == 1 {
		if top {
			appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
				Book:                       book,
				Position:                   common.VignettePosChapterTitleTop,
				StyleClasses:               styleClasses,
				ContextClasses:             contextClasses,
				StripRootHorizontalMargins: true,
			})
			return
		}
		appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
			Book:                       book,
			Position:                   common.VignettePosChapterTitleBottom,
			StyleClasses:               styleClasses,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: true,
		})
		return
	}
	if top {
		appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
			Book:                       book,
			Position:                   common.VignettePosSectionTitleTop,
			StyleClasses:               styleClasses,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: true,
		})
		return
	}
	appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
		Book:                       book,
		Position:                   common.VignettePosSectionTitleBottom,
		StyleClasses:               styleClasses,
		ContextClasses:             contextClasses,
		StripRootHorizontalMargins: true,
	})
}

func appendEndVignette(blocks *[]pdfTextBlock, book *fb2.FictionBook, depth int, contextClasses string, stripRootHorizontalMargins bool) {
	if depth == 1 {
		appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
			Book:                       book,
			Position:                   common.VignettePosChapterEnd,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
		})
		return
	}
	appendVignetteWithOptions(blocks, pdfVignetteBlockOptions{
		Book:                       book,
		Position:                   common.VignettePosSectionEnd,
		ContextClasses:             contextClasses,
		StripRootHorizontalMargins: stripRootHorizontalMargins,
	})
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

type pdfVignetteBlockOptions struct {
	Book                       *fb2.FictionBook
	Position                   common.VignettePos
	StyleClasses               string
	ContextClasses             string
	StripRootHorizontalMargins bool
}

func appendVignetteWithOptions(blocks *[]pdfTextBlock, opts pdfVignetteBlockOptions) {
	if opts.Book == nil || !opts.Book.IsVignetteEnabled(opts.Position) {
		return
	}
	imageID := strings.TrimSpace(opts.Book.VignetteIDs[opts.Position])
	if imageID == "" {
		return
	}
	appendStyledImageWithOptions(blocks, pdfStyledImageBlockOptions{
		ImageID:                    imageID,
		StyleName:                  pdfStyleImage,
		StyleClasses:               joinStyleClasses("image-vignette", "vignette", "vignette-"+opts.Position.String(), opts.StyleClasses),
		ContextClasses:             opts.ContextClasses,
		StripRootHorizontalMargins: opts.StripRootHorizontalMargins,
	})
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

type pdfParagraphBlockOptions struct {
	Content                    *content.Content
	Kind                       pdfBlockKind
	Paragraph                  *fb2.Paragraph
	Depth                      int
	StyleClasses               string
	ContextClasses             string
	StripRootHorizontalMargins bool
}

func appendParagraphBlockWithOptions(blocks *[]pdfTextBlock, opts pdfParagraphBlockOptions) {
	paragraph := opts.Paragraph
	if paragraph == nil {
		return
	}
	content := opts.Content
	kind := opts.Kind
	depth := opts.Depth
	styleClasses := opts.StyleClasses
	contextClasses := opts.ContextClasses
	stripRootHorizontalMargins := opts.StripRootHorizontalMargins
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
		appendStyledImageWithOptions(blocks, pdfStyledImageBlockOptions{
			ImageID:                    imageID,
			AnchorID:                   paragraph.ID,
			Alt:                        alt,
			StyleName:                  imageStyleName,
			StyleClasses:               imageStyleClasses,
			ContextClasses:             contextClasses,
			StripRootHorizontalMargins: stripRootHorizontalMargins,
		})
		return
	}
	text, links := paragraphTextAndLinks(paragraph)
	runs := paragraphInlineRunsWithBacklinks(paragraph, content, pdfRegisterDefaultFootnoteBacklinks(content, styleClasses, contextClasses))
	if paragraphIsCodeBlock(paragraph) {
		styleClasses = joinStyleClasses(styleClasses, pdfStyleCode)
	}
	finalStyleClasses := joinStyleClasses(paragraph.Style, styleClasses)
	runs, links = pdfDisablePrintedFootnoteLinks(content, finalStyleClasses, contextClasses, runs, links)
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
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockSubtitle, Paragraph: &poem.Subtitles[i], Depth: depth, StyleClasses: pdfStylePoemSubtitle, ContextClasses: poemContextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
	}
	for i := range poem.Stanzas {
		stanza := &poem.Stanzas[i]
		stanzaContextClasses := joinStyleClasses(poemContextClasses, pdfStyleStanza)
		stanzaStart := len(*blocks)
		appendTitleParagraphBlocks(blocks, c, stanza.Title, "", pdfStyleStanzaTitle, stanzaContextClasses, stripRootHorizontalMargins)
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockSubtitle, Paragraph: stanza.Subtitle, Depth: depth, StyleClasses: pdfStyleStanzaSubtitle, ContextClasses: stanzaContextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
		for j := range stanza.Verses {
			appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockPoem, Paragraph: &stanza.Verses[j], Depth: depth, StyleClasses: pdfStylePoem, ContextClasses: stanzaContextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
		}
		applyStyleClassToBlocks((*blocks)[stanzaStart:], pdfStyleStanza)
		if i < len(poem.Stanzas)-1 {
			*blocks = append(*blocks, pdfTextBlock{Kind: pdfBlockEmptyLine, StyleName: pdfStyleEmptyLine, ContextClasses: strings.TrimSpace(stanzaContextClasses), StripRootHorizontalMargins: stripRootHorizontalMargins})
		}
	}
	for i := range poem.TextAuthors {
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockTextAuthor, Paragraph: &poem.TextAuthors[i], Depth: depth, ContextClasses: poemContextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
	}
	if dateText := poemDateText(poem.Date); dateText != "" {
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockParagraph, Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: dateText}}}, Depth: depth, StyleClasses: pdfStyleDate, ContextClasses: poemContextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
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
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockTextAuthor, Paragraph: &cite.TextAuthors[i], Depth: depth, ContextClasses: citeContextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
	}
}

func appendEpigraphBlocksFull(blocks *[]pdfTextBlock, c *content.Content, epigraph *fb2.Epigraph, contextClasses string, stripRootHorizontalMargins bool) {
	if epigraph == nil {
		return
	}
	epigraphContextClasses := joinStyleClasses(contextClasses, pdfStyleEpigraph)
	appendFlowBlocks(blocks, c, epigraph.Flow.Items, 1, nil, pdfStyleEpigraph, epigraphContextClasses, stripRootHorizontalMargins)
	for i := range epigraph.TextAuthors {
		appendParagraphBlockWithOptions(blocks, pdfParagraphBlockOptions{Content: c, Kind: pdfBlockTextAuthor, Paragraph: &epigraph.TextAuthors[i], Depth: 1, ContextClasses: epigraphContextClasses, StripRootHorizontalMargins: stripRootHorizontalMargins})
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
