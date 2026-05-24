package pdf

import (
	"strings"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

func pdfPrintedFootnotesEnabled(c *content.Content) bool {
	return c != nil && c.OutputFormat == common.OutputFmtPdf && c.FootnotesMode.IsFloat()
}

func pdfPrintedFootnoteReferencesRenumbered(c *content.Content) bool {
	return pdfPrintedFootnotesEnabled(c) && c.FootnotesMode == common.FootnotesModeFloatRenumbered
}

func buildPDFPrintedFootnoteBlocks(c *content.Content) map[string]pdfPrintedFootnote {
	if !pdfPrintedFootnotesEnabled(c) || c.Book == nil || len(c.FootnotesIndex) == 0 {
		return nil
	}

	out := make(map[string]pdfPrintedFootnote, len(c.FootnotesIndex))
	for id, ref := range c.FootnotesIndex {
		section := pdfFootnoteSectionByRef(c.Book, ref)
		if section == nil {
			continue
		}

		titleBlocks := pdfPrintedFootnoteActualTitleBlocks(c, section, false)
		continuationTitleBlocks := pdfPrintedFootnoteActualTitleBlocks(c, section, true)
		bodyBlocks := pdfPrintedFootnoteBodyBlocks(c, section)
		out[id] = pdfPrintedFootnote{
			ID:                      id,
			TitleBlocks:             titleBlocks,
			BodyBlocks:              bodyBlocks,
			ContinuationTitleBlocks: continuationTitleBlocks,
			Blocks:                  append(clonePDFTextBlocks(titleBlocks), bodyBlocks...),
			ContinuationBlocks:      append(clonePDFTextBlocks(continuationTitleBlocks), bodyBlocks...),
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func pdfFootnoteSectionByRef(book *fb2.FictionBook, ref fb2.FootnoteRef) *fb2.Section {
	if book == nil || ref.BodyIdx < 0 || ref.BodyIdx >= len(book.Bodies) {
		return nil
	}
	body := &book.Bodies[ref.BodyIdx]
	if ref.SectionIdx < 0 || ref.SectionIdx >= len(body.Sections) {
		return nil
	}
	return &body.Sections[ref.SectionIdx]
}

func pdfPrintedFootnoteActualTitleBlocks(c *content.Content, section *fb2.Section, continuation bool) []pdfTextBlock {
	if section == nil || pdfTitleEmpty(section.Title) {
		return nil
	}
	title := clonePDFTitle(section.Title)
	if continuation {
		appendPDFContinuationMarker(title, c)
	}
	var blocks []pdfTextBlock
	appendTitleParagraphBlocks(&blocks, c, title, section.ID, pdfStyleFootnoteTitle, pdfStyleFootnoteTitle, false)
	return blocks
}

func pdfPrintedFootnoteBodyBlocks(c *content.Content, section *fb2.Section) []pdfTextBlock {
	if section == nil {
		return nil
	}
	copySection := *section
	copySection.Title = nil
	var blocks []pdfTextBlock
	appendFootnoteSectionContentBlocks(&blocks, c, &copySection, nil)
	return applyPDFFootnoteContextToBlocks(blocks)
}

func applyPDFFootnoteContextToBlocks(blocks []pdfTextBlock) []pdfTextBlock {
	for i := range blocks {
		blocks[i].ContextClasses = joinStyleClasses(blocks[i].ContextClasses, pdfStyleFootnote)
	}
	return blocks
}

func pdfPrintedFootnotePageBlocks(c *content.Content, note pdfPrintedFootnote, pageLabel string, continuation bool) []pdfTextBlock {
	label := strings.TrimSpace(pageLabel)
	if label == "" {
		label = "?"
	}

	titleBlocks := note.TitleBlocks
	if continuation {
		titleBlocks = note.ContinuationTitleBlocks
	}

	var blocks []pdfTextBlock
	if len(titleBlocks) > 0 {
		blocks = append(blocks, pdfPrefixFootnoteTitleBlocks(label, titleBlocks)...)
	} else {
		labelBlock := pdfPageLabelFootnoteTitleBlock(note.ID, label, titleBlocks)
		if continuation {
			appendPDFContinuationMarkerToTitleBlock(&labelBlock, c)
		}
		blocks = append(blocks, labelBlock)
	}
	blocks = append(blocks, clonePDFTextBlocks(note.BodyBlocks)...)
	return blocks
}

func pdfPageLabelFootnoteTitleBlock(id string, label string, titleBlocks []pdfTextBlock) pdfTextBlock {
	block := pdfTextBlock{
		Kind:           pdfBlockParagraph,
		ID:             strings.TrimSpace(id),
		Text:           label,
		Runs:           []pdfInlineRun{{Text: label}},
		StyleClasses:   joinStyleClasses(pdfStyleFootnoteTitle, pdfStyleFootnoteTitle+"-first"),
		ContextClasses: pdfStyleFootnoteTitle,
	}
	if len(titleBlocks) > 0 {
		block = clonePDFTextBlock(titleBlocks[0])
		block.Text = label
		block.Runs = []pdfInlineRun{{Text: label}}
	}
	return block
}

func appendPDFContinuationMarkerToTitleBlock(block *pdfTextBlock, c *content.Content) {
	marker := ""
	if c != nil {
		marker = strings.TrimSpace(c.FootnoteContinuationStr)
	}
	if block == nil || marker == "" {
		return
	}
	text := " " + marker
	block.Text += text
	block.Runs = append(block.Runs, pdfInlineRun{
		Text:         text,
		StyleClasses: pdfStyleFootnoteContinuation,
	})
}

func pdfPrefixFootnoteTitleBlocks(label string, titleBlocks []pdfTextBlock) []pdfTextBlock {
	blocks := clonePDFTextBlocks(titleBlocks)
	if len(blocks) == 0 {
		return []pdfTextBlock{{
			Kind:           pdfBlockParagraph,
			Text:           label,
			StyleClasses:   joinStyleClasses(pdfStyleFootnoteTitle, pdfStyleFootnoteTitle+"-first"),
			ContextClasses: pdfStyleFootnoteTitle,
		}}
	}
	prefix := label + " "
	blocks[0].Text = prefix + blocks[0].Text
	if len(blocks[0].Runs) == 0 {
		blocks[0].Runs = []pdfInlineRun{{Text: blocks[0].Text}}
	} else {
		blocks[0].Runs = append([]pdfInlineRun{{Text: prefix}}, blocks[0].Runs...)
	}
	return blocks
}

func appendPDFContinuationMarker(title *fb2.Title, c *content.Content) {
	marker := ""
	if c != nil {
		marker = strings.TrimSpace(c.FootnoteContinuationStr)
	}
	if title == nil || marker == "" {
		return
	}
	paragraph := lastPDFTitleParagraph(title)
	if paragraph == nil {
		title.Items = append(title.Items, fb2.TitleItem{Paragraph: &fb2.Paragraph{}})
		paragraph = title.Items[len(title.Items)-1].Paragraph
	}
	paragraph.Text = append(paragraph.Text, fb2.InlineSegment{
		Kind:  fb2.InlineNamedStyle,
		Style: pdfStyleFootnoteContinuation,
		Children: []fb2.InlineSegment{{
			Kind: fb2.InlineText,
			Text: " " + marker,
		}},
	})
}

func lastPDFTitleParagraph(title *fb2.Title) *fb2.Paragraph {
	if title == nil {
		return nil
	}
	for i := len(title.Items) - 1; i >= 0; i-- {
		if title.Items[i].Paragraph != nil {
			return title.Items[i].Paragraph
		}
	}
	return nil
}

func pdfTitleEmpty(title *fb2.Title) bool {
	if title == nil {
		return true
	}
	for i := range title.Items {
		paragraph := title.Items[i].Paragraph
		if paragraph == nil {
			continue
		}
		if paragraph.AsPlainText() != "" || paragraph.AsImageAlt() != "" {
			return false
		}
	}
	return true
}

func clonePDFTextBlocks(blocks []pdfTextBlock) []pdfTextBlock {
	if len(blocks) == 0 {
		return nil
	}
	out := make([]pdfTextBlock, len(blocks))
	for i := range blocks {
		out[i] = clonePDFTextBlock(blocks[i])
	}
	return out
}

func clonePDFTextBlock(block pdfTextBlock) pdfTextBlock {
	clone := block
	clone.Runs = clonePDFInlineRuns(block.Runs)
	if len(block.Links) > 0 {
		clone.Links = append([]pdfTextLink(nil), block.Links...)
	}
	if len(block.BacklinkRefIDs) > 0 {
		clone.BacklinkRefIDs = append([]string(nil), block.BacklinkRefIDs...)
	}
	if len(block.TableCellRuns) > 0 {
		clone.TableCellRuns = make(map[pdfTableCellKey][]pdfInlineRun, len(block.TableCellRuns))
		for key, runs := range block.TableCellRuns {
			clone.TableCellRuns[key] = clonePDFInlineRuns(runs)
		}
	}
	return clone
}

func clonePDFInlineRuns(runs []pdfInlineRun) []pdfInlineRun {
	if len(runs) == 0 {
		return nil
	}
	return append([]pdfInlineRun(nil), runs...)
}

func clonePDFTitle(title *fb2.Title) *fb2.Title {
	if title == nil {
		return nil
	}
	clone := &fb2.Title{Lang: title.Lang, Items: make([]fb2.TitleItem, len(title.Items))}
	for i := range title.Items {
		clone.Items[i] = fb2.TitleItem{EmptyLine: title.Items[i].EmptyLine}
		if title.Items[i].Paragraph != nil {
			clone.Items[i].Paragraph = clonePDFParagraph(title.Items[i].Paragraph)
		}
	}
	return clone
}

func clonePDFParagraph(paragraph *fb2.Paragraph) *fb2.Paragraph {
	if paragraph == nil {
		return nil
	}
	clone := *paragraph
	clone.Text = clonePDFInlineSegments(paragraph.Text)
	return &clone
}

func clonePDFInlineSegments(segments []fb2.InlineSegment) []fb2.InlineSegment {
	if segments == nil {
		return nil
	}
	clone := make([]fb2.InlineSegment, len(segments))
	for i := range segments {
		clone[i] = segments[i]
		clone[i].Children = clonePDFInlineSegments(segments[i].Children)
	}
	return clone
}

func pdfPrintedFootnoteRefsClickable(c *content.Content, styleClasses string, contextClasses string) bool {
	return !pdfPrintedFootnotesEnabled(c)
}

func pdfFootnoteTargetIDFromHref(c *content.Content, href string) (string, bool) {
	if c == nil {
		return "", false
	}
	targetID, ok := strings.CutPrefix(strings.TrimSpace(href), "#")
	if !ok || targetID == "" {
		return "", false
	}
	_, ok = c.FootnotesIndex[targetID]
	return targetID, ok
}

func pdfDisablePrintedFootnoteLinks(
	c *content.Content,
	styleClasses string,
	contextClasses string,
	runs []pdfInlineRun,
	links []pdfTextLink,
) ([]pdfInlineRun, []pdfTextLink) {
	if pdfPrintedFootnoteRefsClickable(c, styleClasses, contextClasses) {
		return runs, links
	}

	var changedRuns bool
	for i := range runs {
		targetID, ok := pdfFootnoteTargetIDFromHref(c, runs[i].LinkHref)
		if !ok {
			continue
		}
		runs[i].FootnoteID = targetID
		runs[i].LinkHref = ""
		runs[i].AnchorID = ""
		changedRuns = true
	}
	if changedRuns {
		runs = trimInlineRuns(runs)
	}

	if len(links) == 0 {
		return runs, links
	}
	filtered := links[:0]
	for _, link := range links {
		if _, ok := pdfFootnoteTargetIDFromHref(c, link.Href); ok {
			continue
		}
		filtered = append(filtered, link)
	}
	return runs, filtered
}
