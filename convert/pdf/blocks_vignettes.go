package pdf

import (
	"fbc/common"
	"fbc/convert/pdf/structure"
	"fbc/fb2"
)

type pdfSectionEndVignetteTransfers struct {
	suppress  map[*fb2.Section]bool
	inherited map[*fb2.Section][]common.VignettePos
}

func pdfSectionEndVignetteTransfersForPlan(book *fb2.FictionBook, plan *structure.Plan) pdfSectionEndVignetteTransfers {
	transfers := pdfSectionEndVignetteTransfers{}
	if book == nil || plan == nil {
		return transfers
	}

	unitIndexBySection := make(map[*fb2.Section]int)
	for i := range plan.Units {
		unit := &plan.Units[i]
		if unit.Kind == structure.UnitSection && unit.Section != nil {
			unitIndexBySection[unit.Section] = i
		}
	}
	if len(unitIndexBySection) == 0 {
		return transfers
	}

	transfers.suppress = make(map[*fb2.Section]bool)
	transfers.inherited = make(map[*fb2.Section][]common.VignettePos)
	for i := range book.Bodies {
		body := &book.Bodies[i]
		if body.Footnotes() {
			continue
		}
		for j := range body.Sections {
			pdfCollectSectionEndVignetteTransfers(&body.Sections[j], 1, unitIndexBySection, plan, transfers)
		}
	}
	return transfers
}

func pdfCollectSectionEndVignetteTransfers(
	section *fb2.Section,
	titleDepth int,
	unitIndexBySection map[*fb2.Section]int,
	plan *structure.Plan,
	transfers pdfSectionEndVignetteTransfers,
) int {
	if section == nil {
		return -1
	}

	lastDescendantUnit := -1
	childTitleDepth := titleDepth
	if section.HasTitle() {
		childTitleDepth++
	}
	for i := range section.Content {
		item := &section.Content[i]
		if item.Kind != fb2.FlowSection || item.Section == nil {
			continue
		}
		lastDescendantUnit = max(
			lastDescendantUnit,
			pdfCollectSectionEndVignetteTransfers(item.Section, childTitleDepth, unitIndexBySection, plan, transfers),
		)
	}

	if section.HasTitle() && lastDescendantUnit >= 0 {
		receiver := plan.Units[lastDescendantUnit].Section
		if receiver != nil {
			transfers.suppress[section] = true
			transfers.inherited[receiver] = append(transfers.inherited[receiver], pdfEndVignettePosition(titleDepth))
		}
	}

	if unitIndex, ok := unitIndexBySection[section]; ok {
		return max(unitIndex, lastDescendantUnit)
	}
	return lastDescendantUnit
}

func pdfEndVignettePosition(titleDepth int) common.VignettePos {
	if titleDepth <= 1 {
		return common.VignettePosChapterEnd
	}
	return common.VignettePosSectionEnd
}
