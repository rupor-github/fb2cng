package pdf

func pdfDebugPrintedFootnotesFromReserved(doc pdfDocumentSpec, reserved pdfPrintedFootnoteReservedLayout) pdfDebugPrintedFootnotes {
	summary := pdfDebugPrintedFootnotes{
		Enabled:            pdfPrintedFootnotesEnabled(doc.Content) && len(doc.PrintedFootnotes) > 0,
		SourcePageCount:    len(reserved.Pages),
		FootnoteTextHeight: reserved.FootnoteTextHeight,
	}
	if !summary.Enabled {
		return summary
	}
	summary.Plans = make([]pdfDebugPrintedFootnotePlan, 0, len(reserved.Plans))
	plannedPages := make(map[int]bool, len(reserved.Plans))
	for _, plan := range reserved.Plans {
		debugPlan := pdfDebugPrintedFootnotePlan{
			PageIndex:         plan.PageIndex,
			Refs:              pdfDebugPrintedFootnoteRefs(plan.Refs),
			Queue:             pdfDebugPrintedFootnoteQueue(plan.Queue),
			QueuePageCount:    len(plan.QueuePages),
			ContinuationPages: plan.ContinuationPages,
			QueuePages:        pdfDebugPrintedFootnoteQueuePages(plan.QueuePages),
		}
		if plan.PageIndex >= 0 && plan.PageIndex < len(reserved.PageBottomReserves) {
			debugPlan.Reserve = reserved.PageBottomReserves[plan.PageIndex]
		}
		summary.Plans = append(summary.Plans, debugPlan)
		if plan.PageIndex >= 0 && plan.PageIndex < len(reserved.Pages) {
			plannedPages[plan.PageIndex] = true
		}
		summary.ContinuationPageCount += plan.ContinuationPages
	}
	summary.PlanCount = len(summary.Plans)
	summary.PagesWithoutPrintedRefs = max(len(reserved.Pages)-len(plannedPages), 0)
	for pageIndex, reserve := range reserved.PageBottomReserves {
		if reserve <= 0 {
			continue
		}
		summary.Reserves = append(summary.Reserves, pdfDebugPrintedFootnoteReserve{PageIndex: pageIndex, Reserve: reserve})
	}
	summary.ReserveCount = len(summary.Reserves)
	pdfDebugPrintedFootnotesReserveOverflow(doc, reserved, &summary)
	pdfDebugPrintedFootnotesSyncCounts(&summary)
	return summary
}

func pdfDebugPrintedFootnoteRefs(refs []pdfPrintedFootnoteRef) []pdfDebugPrintedFootnoteRef {
	out := make([]pdfDebugPrintedFootnoteRef, 0, len(refs))
	for _, ref := range refs {
		out = append(out, pdfDebugPrintedFootnoteRef(ref))
	}
	return out
}

func pdfDebugPrintedFootnoteQueue(queue []pdfPrintedFootnoteQueueEntry) []pdfDebugPrintedFootnoteQueueEntry {
	out := make([]pdfDebugPrintedFootnoteQueueEntry, 0, len(queue))
	for _, entry := range queue {
		out = append(out, pdfDebugPrintedFootnoteQueueEntry(entry))
	}
	return out
}

func pdfDebugPrintedFootnoteQueuePages(pages []pdfPage) []pdfDebugPrintedFootnoteQueuePage {
	out := make([]pdfDebugPrintedFootnoteQueuePage, 0, len(pages))
	for i, page := range pages {
		out = append(out, pdfDebugPrintedFootnoteQueuePage{
			Index:     i,
			LineCount: len(page.Lines),
			Bounds:    pdfDebugPageYBounds(page),
		})
	}
	return out
}

func pdfDebugPageYBounds(page pdfPage) *pdfDebugYBounds {
	top, bottom, ok := pdfPageYBounds(page)
	if !ok {
		return nil
	}
	return &pdfDebugYBounds{Top: top, Bottom: bottom, Height: max(top-bottom, 0)}
}

func pdfDebugPrintedFootnotesReserveOverflow(doc pdfDocumentSpec, reserved pdfPrintedFootnoteReservedLayout, summary *pdfDebugPrintedFootnotes) {
	if summary == nil || len(reserved.Plans) == 0 || reserved.FootnoteTextHeight <= 0 {
		return
	}
	styles := doc.Styles
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	contentLeft, contentRight, contentTop, contentBottom := pdfContentMargins(doc, styles, pdfDefaultPageMargin, false)
	contentWidth := max(doc.PageWidth-contentLeft-contentRight, 12)
	separator := pdfPrintedFootnoteSeparatorMetricsForArea(doc, styles, contentLeft, contentWidth, contentBottom, reserved.FootnoteTextHeight)
	maxReserve := max(doc.PageHeight-contentTop-contentBottom-pdfBaseLineHeight, 0)
	if maxReserve <= 0 {
		return
	}
	for _, plan := range reserved.Plans {
		if plan.PageIndex < 0 || plan.PageIndex >= len(reserved.PageBottomReserves) || len(plan.QueuePages) == 0 {
			continue
		}
		requested := pdfPrintedFootnotePagePlanReserve(plan, reserved.FootnoteTextHeight, separator)
		actual := reserved.PageBottomReserves[plan.PageIndex]
		if requested > actual && actual == maxReserve {
			summary.Overflow = append(summary.Overflow, pdfDebugPrintedFootnoteCase{
				Kind:      "reserve",
				Reason:    "clamped_to_main_text_minimum",
				PageIndex: plan.PageIndex,
				Value:     requested,
				Limit:     actual,
			})
		}
	}
}

func pdfDebugPrintedFootnotesSyncCounts(summary *pdfDebugPrintedFootnotes) {
	if summary == nil {
		return
	}
	summary.PlanCount = len(summary.Plans)
	summary.ReserveCount = len(summary.Reserves)
	summary.SkippedCount = len(summary.Skipped)
	summary.OverflowCount = len(summary.Overflow)
}
