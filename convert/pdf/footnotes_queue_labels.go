package pdf

import (
	"strings"

	"fbc/content"
)

func applyPDFPrintedFootnoteQueueReferenceLabels(
	blocks []pdfTextBlock,
	c *content.Content,
	labels map[string]string,
) []pdfTextBlock {
	if len(blocks) == 0 || len(labels) == 0 {
		return blocks
	}
	out := clonePDFTextBlocks(blocks)
	for i := range out {
		if len(out[i].Runs) > 0 {
			out[i].Runs = pdfPrintedFootnoteQueueReferenceRuns(out[i].Runs, c, labels)
			out[i].Text = plainInlineRunText(out[i].Runs)
		}
		out[i].Links = pdfFilterPrintedFootnoteQueueReferenceLinks(out[i].Links, labels)
		for key, runs := range out[i].TableCellRuns {
			out[i].TableCellRuns[key] = pdfPrintedFootnoteQueueReferenceRuns(runs, c, labels)
		}
	}
	return out
}

func pdfPrintedFootnoteQueueReferenceRuns(runs []pdfInlineRun, c *content.Content, labels map[string]string) []pdfInlineRun {
	if len(runs) == 0 || len(labels) == 0 {
		return runs
	}
	out := clonePDFInlineRuns(runs)
	for i := 0; i < len(out); i++ {
		id, ok := pdfPrintedFootnoteRunTargetID(out[i], labels)
		if !ok {
			continue
		}
		label := labels[id]
		groupText := out[i].Text
		end := i + 1
		for end < len(out) {
			nextID, nextOK := pdfPrintedFootnoteRunTargetID(out[end], labels)
			if !nextOK || nextID != id {
				break
			}
			groupText += out[end].Text
			end++
		}
		for j := i; j < end; j++ {
			out[j].FootnoteID = id
			out[j].LinkHref = ""
			out[j].AnchorID = ""
		}
		if pdfPrintedFootnoteReferencesRenumbered(c) {
			out[i].Text = label
			for j := i + 1; j < end; j++ {
				out[j].Text = ""
			}
		}
		i = end - 1
	}
	return trimInlineRuns(out)
}

func pdfPrintedFootnoteRunTargetID(run pdfInlineRun, labels map[string]string) (string, bool) {
	id := strings.TrimSpace(run.FootnoteID)
	if id != "" {
		_, ok := labels[id]
		return id, ok
	}
	return pdfPrintedFootnoteTargetIDFromLabels(run.LinkHref, labels)
}

func pdfFilterPrintedFootnoteQueueReferenceLinks(links []pdfTextLink, labels map[string]string) []pdfTextLink {
	if len(links) == 0 || len(labels) == 0 {
		return links
	}
	filtered := links[:0]
	for _, link := range links {
		if _, ok := pdfPrintedFootnoteTargetIDFromLabels(link.Href, labels); ok {
			continue
		}
		filtered = append(filtered, link)
	}
	return filtered
}

func pdfPrintedFootnoteTargetIDFromLabels(href string, labels map[string]string) (string, bool) {
	id, ok := strings.CutPrefix(strings.TrimSpace(href), "#")
	if !ok || id == "" {
		return "", false
	}
	_, ok = labels[id]
	return id, ok
}

func pdfPrintedFootnoteQueueLabels(queue []pdfPrintedFootnoteQueueEntry) map[string]string {
	if len(queue) == 0 {
		return nil
	}
	labels := make(map[string]string, len(queue))
	for _, entry := range queue {
		id := strings.TrimSpace(entry.ID)
		label := strings.TrimSpace(entry.PageLabel)
		if id != "" && label != "" {
			labels[id] = label
		}
	}
	return labels
}
