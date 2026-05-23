package pdf

import (
	"strconv"
	"strings"

	"fbc/content"
)

type pdfPrintedFootnoteQueueEntry struct {
	ID        string
	PageLabel string
	Nested    bool
}

func buildPDFPrintedFootnoteQueue(doc pdfDocumentSpec, pageRefs []string) []pdfPrintedFootnoteQueueEntry {
	if len(doc.PrintedFootnotes) == 0 || len(pageRefs) == 0 {
		return nil
	}
	queued := make(map[string]bool)
	queue := make([]pdfPrintedFootnoteQueueEntry, 0, len(pageRefs))
	for _, id := range pageRefs {
		id = strings.TrimSpace(id)
		if id == "" || queued[id] {
			continue
		}
		if _, ok := doc.PrintedFootnotes[id]; !ok {
			continue
		}
		queued[id] = true
		queue = append(queue, pdfPrintedFootnoteQueueEntry{ID: id, PageLabel: strconv.Itoa(len(queue) + 1)})
	}
	for i := 0; i < len(queue); i++ {
		note := doc.PrintedFootnotes[queue[i].ID]
		for _, nestedID := range pdfPrintedFootnoteNestedRefs(note, doc.PrintedFootnotes) {
			if queued[nestedID] {
				continue
			}
			queued[nestedID] = true
			queue = append(queue, pdfPrintedFootnoteQueueEntry{ID: nestedID, Nested: true})
		}
	}
	return queue
}

func pdfPrintedFootnotePageRefs(doc pdfDocumentSpec, page pdfPage) []string {
	if len(doc.PrintedFootnotes) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var refs []string
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
			refs = append(refs, id)
		}
	}
	return refs
}

func pdfPrintedFootnoteNestedRefs(note pdfPrintedFootnote, footnotes map[string]pdfPrintedFootnote) []string {
	if len(footnotes) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var refs []string
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

func appendPDFPrintedFootnoteNestedRefsFromRuns(refs *[]string, seen map[string]bool, footnotes map[string]pdfPrintedFootnote, runs []pdfInlineRun) {
	for _, run := range runs {
		id := strings.TrimSpace(run.FootnoteID)
		if id == "" || strings.TrimSpace(run.LinkHref) == "" || seen[id] {
			continue
		}
		if _, ok := footnotes[id]; !ok {
			continue
		}
		seen[id] = true
		*refs = append(*refs, id)
	}
}

func pdfPrintedFootnoteBlocksForQueueEntry(c *content.Content, note pdfPrintedFootnote, entry pdfPrintedFootnoteQueueEntry, continuation bool) []pdfTextBlock {
	if entry.Nested {
		if continuation {
			return clonePDFTextBlocks(note.ContinuationBlocks)
		}
		return clonePDFTextBlocks(note.Blocks)
	}
	return pdfPrintedFootnotePageBlocks(c, note, entry.PageLabel, continuation)
}
