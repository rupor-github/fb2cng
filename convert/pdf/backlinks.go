package pdf

import "slices"

func resolvePDFBacklinkPagesAndText(doc *pdfDocumentSpec, pages []pdfPage) bool {
	if doc == nil || doc.Content == nil {
		return false
	}
	changed := false
	anchorPages := pdfAnchorPages(pages)
	for _, refs := range doc.Content.BackLinkIndex {
		for _, ref := range refs {
			if pageNumber := anchorPages[ref.RefID]; pageNumber > 0 {
				changed = doc.Content.SetBackLinkRefPage(ref.RefID, pageNumber) || changed
			}
		}
	}
	backlinkPages := pdfBacklinkAnnotationPages(pages)
	for i := range doc.Blocks {
		block := &doc.Blocks[i]
		if len(block.BacklinkRefIDs) == 0 {
			continue
		}
		visibleRefIDs := pdfVisibleBacklinkRefIDs(block.BacklinkRefIDs, anchorPages, backlinkPages)
		if !slices.Equal(block.BacklinkRefIDs, visibleRefIDs) {
			block.BacklinkRefIDs = visibleRefIDs
			changed = true
		}
		text, runs := pdfBacklinkBlockContent(doc.Content, visibleRefIDs)
		if block.Text == text && slices.Equal(block.Runs, runs) {
			continue
		}
		block.Text = text
		block.Runs = runs
		changed = true
	}
	return changed
}

func pdfVisibleBacklinkRefIDs(refIDs []string, anchorPages map[string]int, backlinkPages map[string]int) []string {
	if len(refIDs) == 0 || len(anchorPages) == 0 || len(backlinkPages) == 0 {
		return refIDs
	}
	visible := make([]string, 0, len(refIDs))
	for _, refID := range refIDs {
		refPage := anchorPages[refID]
		backlinkPage := backlinkPages["#"+refID]
		if refPage > 0 && backlinkPage > 0 && refPage == backlinkPage {
			continue
		}
		visible = append(visible, refID)
	}
	return visible
}

func pdfAnchorPages(pages []pdfPage) map[string]int {
	out := make(map[string]int)
	for i := range pages {
		pageNumber := i + 1
		for _, anchor := range pages[i].Anchors {
			if _, exists := out[anchor]; !exists {
				out[anchor] = pageNumber
			}
		}
	}
	return out
}

func pdfBacklinkAnnotationPages(pages []pdfPage) map[string]int {
	out := make(map[string]int)
	for i := range pages {
		pageNumber := i + 1
		for _, annotation := range pages[i].Annotations {
			if _, exists := out[annotation.Href]; !exists {
				out[annotation.Href] = pageNumber
			}
		}
	}
	return out
}
