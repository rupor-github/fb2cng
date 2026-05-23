package pdf

import (
	"fmt"
	"strings"

	"fbc/common"
	"fbc/content"
	"fbc/fb2"
)

func pdfPrintedFootnotesEnabled(c *content.Content) bool {
	return c != nil && c.OutputFormat == common.OutputFmtPdf && c.FootnotesMode.IsFloat()
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

		out[id] = pdfPrintedFootnote{
			ID:                 id,
			Blocks:             pdfPrintedFootnoteSectionBlocks(c, id, ref, section, false),
			ContinuationBlocks: pdfPrintedFootnoteSectionBlocks(c, id, ref, section, true),
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

func pdfPrintedFootnoteSectionBlocks(
	c *content.Content,
	id string,
	ref fb2.FootnoteRef,
	section *fb2.Section,
	continuation bool,
) []pdfTextBlock {
	if section == nil {
		return nil
	}
	copySection := *section
	copySection.Title = pdfPrintedFootnoteTitle(c, id, ref, section, continuation)
	var blocks []pdfTextBlock
	appendFootnoteSectionContentBlocks(&blocks, c, &copySection, nil)
	return blocks
}

func pdfPrintedFootnoteTitle(
	c *content.Content,
	id string,
	ref fb2.FootnoteRef,
	section *fb2.Section,
	continuation bool,
) *fb2.Title {
	var title *fb2.Title
	if c != nil && c.FootnotesMode == common.FootnotesModeFloatRenumbered && strings.TrimSpace(ref.DisplayText) != "" {
		title = pdfSyntheticFootnoteTitle(id, ref, section)
	} else if section != nil && !pdfTitleEmpty(section.Title) {
		title = clonePDFTitle(section.Title)
	} else {
		title = pdfSyntheticFootnoteTitle(id, ref, section)
	}
	if continuation {
		appendPDFContinuationMarker(title, c)
	}
	return title
}

func pdfSyntheticFootnoteTitle(id string, ref fb2.FootnoteRef, section *fb2.Section) *fb2.Title {
	label := strings.TrimSpace(ref.DisplayText)
	if label == "" && ref.NoteNum > 0 {
		label = fmt.Sprintf("%d", ref.NoteNum)
	}
	if label == "" {
		label = "~ " + strings.TrimSpace(id) + " ~"
	}
	lang := ""
	if section != nil {
		lang = section.Lang
	}
	return &fb2.Title{
		Lang: lang,
		Items: []fb2.TitleItem{{
			Paragraph: &fb2.Paragraph{Text: []fb2.InlineSegment{{Kind: fb2.InlineText, Text: label}}},
		}},
	}
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
	if !pdfPrintedFootnotesEnabled(c) {
		return true
	}
	return hasPDFStyleClass(styleClasses, pdfStyleFootnote) ||
		hasPDFStyleClass(styleClasses, pdfStyleFootnoteTitle) ||
		hasPDFStyleClass(contextClasses, pdfStyleFootnote) ||
		hasPDFStyleClass(contextClasses, pdfStyleFootnoteTitle)
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
