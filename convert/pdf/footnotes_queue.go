package pdf

import (
	"strconv"
	"strings"

	"fbc/content"
)

type pdfPrintedFootnoteRef struct {
	ID    string
	Label string
}

type pdfPrintedFootnoteQueueEntry struct {
	ID        string
	PageLabel string
	Nested    bool
}

func buildPDFPrintedFootnoteQueue(doc pdfDocumentSpec, pageRefs []pdfPrintedFootnoteRef) []pdfPrintedFootnoteQueueEntry {
	if len(doc.PrintedFootnotes) == 0 || len(pageRefs) == 0 {
		return nil
	}
	queued := make(map[string]bool)
	queue := make([]pdfPrintedFootnoteQueueEntry, 0, len(pageRefs))
	for _, ref := range pageRefs {
		id := strings.TrimSpace(ref.ID)
		if id == "" || queued[id] {
			continue
		}
		if _, ok := doc.PrintedFootnotes[id]; !ok {
			continue
		}
		queued[id] = true
		queue = append(queue, pdfPrintedFootnoteQueueEntry{ID: id, PageLabel: pdfPrintedFootnoteQueueLabel(doc.Content, ref, len(queue)+1)})
	}
	for i := 0; i < len(queue); i++ {
		note := doc.PrintedFootnotes[queue[i].ID]
		for _, nestedRef := range pdfPrintedFootnoteNestedRefs(note, doc.PrintedFootnotes) {
			nestedID := strings.TrimSpace(nestedRef.ID)
			if nestedID == "" || queued[nestedID] {
				continue
			}
			queued[nestedID] = true
			label := pdfPrintedFootnoteQueueLabel(doc.Content, nestedRef, len(queue)+1)
			queue = append(queue, pdfPrintedFootnoteQueueEntry{ID: nestedID, PageLabel: label, Nested: true})
		}
	}
	return queue
}

func pdfPrintedFootnoteQueueLabel(c *content.Content, ref pdfPrintedFootnoteRef, fallback int) string {
	fallbackLabel := strconv.Itoa(max(fallback, 1))
	if pdfPrintedFootnoteReferencesRenumbered(c) {
		return fallbackLabel
	}
	label := strings.TrimSpace(ref.Label)
	if label != "" {
		return label
	}
	return fallbackLabel
}

func pdfPrintedFootnotePageRefs(doc pdfDocumentSpec, page pdfPage) []pdfPrintedFootnoteRef {
	if len(doc.PrintedFootnotes) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var refs []pdfPrintedFootnoteRef
	for _, line := range page.Lines {
		for _, fragment := range line.Fragments {
			id := strings.TrimSpace(fragment.FootnoteID)
			if id == "" || strings.TrimSpace(fragment.LinkHref) != "" || seen[id] {
				continue
			}
			if _, ok := doc.PrintedFootnotes[id]; !ok {
				continue
			}
			seen[id] = true
			refs = append(refs, pdfPrintedFootnoteRef{ID: id, Label: strings.TrimSpace(shapedRunes(fragment.Text))})
		}
	}
	return refs
}

func pdfPrintedFootnoteNestedRefs(note pdfPrintedFootnote, footnotes map[string]pdfPrintedFootnote) []pdfPrintedFootnoteRef {
	if len(footnotes) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var refs []pdfPrintedFootnoteRef
	appendRefs := func(blocks []pdfTextBlock) {
		for _, block := range blocks {
			appendPDFPrintedFootnoteNestedRefsFromRuns(&refs, seen, footnotes, block.Runs)
			for _, runs := range block.TableCellRuns {
				appendPDFPrintedFootnoteNestedRefsFromRuns(&refs, seen, footnotes, runs)
			}
		}
	}
	appendRefs(note.TitleBlocks)
	appendRefs(note.BodyBlocks)
	return refs
}

func appendPDFPrintedFootnoteNestedRefsFromRuns(refs *[]pdfPrintedFootnoteRef, seen map[string]bool, footnotes map[string]pdfPrintedFootnote, runs []pdfInlineRun) {
	for i := 0; i < len(runs); i++ {
		id := strings.TrimSpace(runs[i].FootnoteID)
		if id == "" {
			if targetID, ok := strings.CutPrefix(strings.TrimSpace(runs[i].LinkHref), "#"); ok {
				id = strings.TrimSpace(targetID)
			}
		}
		if id == "" {
			continue
		}
		label := runs[i].Text
		for i+1 < len(runs) && strings.TrimSpace(runs[i+1].FootnoteID) == id {
			i++
			label += runs[i].Text
		}
		if seen[id] {
			continue
		}
		if _, ok := footnotes[id]; !ok {
			continue
		}
		seen[id] = true
		*refs = append(*refs, pdfPrintedFootnoteRef{ID: id, Label: strings.TrimSpace(label)})
	}
}

func pdfPrintedFootnoteBlocksForQueueEntry(
	c *content.Content,
	note pdfPrintedFootnote,
	entry pdfPrintedFootnoteQueueEntry,
	continuation bool,
	resolver *pdfStyleResolver,
) []pdfTextBlock {
	return pdfPrintedFootnotePageBlocks(c, note, pdfPrintedFootnoteQueueEntryTitleLabel(c, resolver, entry.PageLabel), continuation)
}

func pdfPrintedFootnoteQueueEntryTitleLabel(c *content.Content, resolver *pdfStyleResolver, label string) string {
	if !pdfPrintedFootnoteReferencesRenumbered(c) {
		return label
	}
	return pdfDecoratedFootnoteReferenceLabel(resolver, pdfStyleLinkFootnote, label)
}
