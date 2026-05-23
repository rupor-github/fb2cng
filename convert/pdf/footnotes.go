package pdf

import (
	"strings"

	"fbc/common"
	"fbc/content"
)

func pdfPrintedFootnotesEnabled(c *content.Content) bool {
	return c != nil && c.OutputFormat == common.OutputFmtPdf && c.FootnotesMode.IsFloat()
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
