package pdf

import "strings"

type pdfDynamicPrintedFootnoteReserveTracker struct {
	doc                pdfDocumentSpec
	styles             *pdfStyleResolver
	enabled            bool
	contentLeft        float64
	contentWidth       float64
	contentBottom      float64
	footnoteTextHeight float64
	refs               []pdfPrintedFootnoteRef
	seen               map[string]bool
	reserve            float64
	cache              map[string]float64
}

func newPDFDynamicPrintedFootnoteReserveTracker(
	doc pdfDocumentSpec,
	styles *pdfStyleResolver,
	contentLeft float64,
	contentWidth float64,
	contentBottom float64,
) pdfDynamicPrintedFootnoteReserveTracker {
	if styles == nil {
		styles = newPDFStyleResolver(nil, nil)
	}
	tracker := pdfDynamicPrintedFootnoteReserveTracker{
		doc:           doc,
		styles:        styles,
		contentLeft:   contentLeft,
		contentWidth:  contentWidth,
		contentBottom: contentBottom,
		seen:          make(map[string]bool),
		cache:         make(map[string]float64),
	}
	if !doc.DynamicPrintedFootnoteReserves || !pdfPrintedFootnotesEnabled(doc.Content) || len(doc.PrintedFootnotes) == 0 {
		return tracker
	}
	tracker.footnoteTextHeight = pdfPrintedFootnoteTextAreaHeight(doc, styles)
	tracker.enabled = tracker.footnoteTextHeight > 0
	return tracker
}

func (t *pdfDynamicPrintedFootnoteReserveTracker) ResetPage() {
	t.refs = nil
	clear(t.seen)
	t.reserve = 0
}

func (t *pdfDynamicPrintedFootnoteReserveTracker) Enabled() bool {
	return t != nil && t.enabled
}

func (t *pdfDynamicPrintedFootnoteReserveTracker) LineRefs(line paragraphLine) []pdfPrintedFootnoteRef {
	if !t.Enabled() {
		return nil
	}
	return pdfPrintedFootnoteParagraphLineRefs(t.doc, line)
}

func (t *pdfDynamicPrintedFootnoteReserveTracker) ReserveWithAdditionalRefs(additional []pdfPrintedFootnoteRef) (float64, error) {
	if !t.Enabled() || len(additional) == 0 {
		return t.reserve, nil
	}
	candidate := append([]pdfPrintedFootnoteRef(nil), t.refs...)
	candidateSeen := make(map[string]bool, len(t.seen)+len(additional))
	for id := range t.seen {
		candidateSeen[id] = true
	}
	for _, ref := range additional {
		id := strings.TrimSpace(ref.ID)
		if id == "" || candidateSeen[id] {
			continue
		}
		if _, ok := t.doc.PrintedFootnotes[id]; !ok {
			continue
		}
		candidateSeen[id] = true
		candidate = append(candidate, pdfPrintedFootnoteRef{ID: id, Label: strings.TrimSpace(ref.Label)})
	}
	if len(candidate) == len(t.refs) {
		return t.reserve, nil
	}
	key := pdfPrintedFootnoteRefsCacheKey(candidate)
	if reserve, ok := t.cache[key]; ok {
		return reserve, nil
	}
	reserve, err := t.computeReserve(candidate)
	if err != nil {
		return 0, err
	}
	t.cache[key] = reserve
	return reserve, nil
}

func (t *pdfDynamicPrintedFootnoteReserveTracker) CommitAdditionalRefs(additional []pdfPrintedFootnoteRef, reserve float64) {
	if !t.Enabled() || len(additional) == 0 {
		return
	}
	for _, ref := range additional {
		id := strings.TrimSpace(ref.ID)
		if id == "" || t.seen[id] {
			continue
		}
		if _, ok := t.doc.PrintedFootnotes[id]; !ok {
			continue
		}
		t.seen[id] = true
		t.refs = append(t.refs, pdfPrintedFootnoteRef{ID: id, Label: strings.TrimSpace(ref.Label)})
	}
	t.reserve = max(t.reserve, reserve)
}

func (t *pdfDynamicPrintedFootnoteReserveTracker) computeReserve(refs []pdfPrintedFootnoteRef) (float64, error) {
	queue := buildPDFPrintedFootnoteQueue(t.doc, refs)
	if len(queue) == 0 {
		return 0, nil
	}
	queuePages, _, err := layoutPDFPrintedFootnoteQueue(t.doc, queue, t.footnoteTextHeight)
	if err != nil {
		return 0, err
	}
	if len(queuePages) == 0 {
		return 0, nil
	}
	separator := pdfPrintedFootnoteSeparatorMetricsForArea(
		t.doc,
		t.styles,
		t.contentLeft,
		t.contentWidth,
		t.contentBottom,
		t.footnoteTextHeight,
	)
	reserve := pdfPrintedFootnotePagePlanReserve(
		pdfPrintedFootnotePagePlan{QueuePages: queuePages},
		t.footnoteTextHeight,
		separator,
	)
	return reserve, nil
}

func pdfPrintedFootnoteParagraphLineRefs(doc pdfDocumentSpec, line paragraphLine) []pdfPrintedFootnoteRef {
	if len(doc.PrintedFootnotes) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var refs []pdfPrintedFootnoteRef
	for _, fragment := range line.Fragments {
		appendPDFPrintedFootnoteFragmentRef(
			&refs,
			seen,
			doc.PrintedFootnotes,
			fragment.FootnoteID,
			fragment.LinkHref,
			shapedRunes(fragment.Text),
		)
	}
	return refs
}

func pdfPrintedFootnoteRefsCacheKey(refs []pdfPrintedFootnoteRef) string {
	var b strings.Builder
	for _, ref := range refs {
		b.WriteString(strings.TrimSpace(ref.ID))
		b.WriteByte(0)
		b.WriteString(strings.TrimSpace(ref.Label))
		b.WriteByte(0)
	}
	return b.String()
}
