package pdf

import (
	"github.com/carlos7ags/folio/layout"

	"fbc/convert/margins"
)

// collapseMargins runs the full CSS margin collapsing pipeline on a flat
// element list.  It builds a margin tree from the elements, collapses the
// tree, and applies the collapsed values back to the folio elements.
//
// The meta map provides container kind/flag annotations for Div wrappers
// (populated by tagContainer during element creation).  The signals map
// provides empty-line margin-absorption annotations (populated by
// handleEmptyLine/consumePendingEmptyLine).
func collapseMargins(elements []layout.Element, meta map[*layout.Div]*marginMeta, signals map[layout.Element]*emptyLineSignal, tracer *PDFTracer) {
	if len(elements) == 0 {
		return
	}
	result := buildMarginTree(elements, meta, signals)
	tracer.TraceMarginTree("before", result.tree, result.nodeElem)
	margins.CollapseTree(result.tree)
	tracer.TraceMarginTree("after", result.tree, result.nodeElem)
	applyCollapsedMargins(result)
}

// applyCollapsedMargins updates folio elements with collapsed margin values
// from the margin tree.  It is called after margins.CollapseTree() has run.
//
// For Div containers: update SpaceBefore/SpaceAfter on the Div.
// For Paragraphs:     update SpaceBefore/SpaceAfter directly.
// For Headings:       no direct margin API; margins are on the wrapper Div.
func applyCollapsedMargins(r *marginTreeResult) {
	if r == nil || r.tree == nil {
		return
	}

	// Update wrapper Div containers.
	for containerNode, div := range r.wrapperDiv {
		applyContainerMargins(containerNode, div)
	}

	// Update leaf content nodes (Paragraphs).
	for node, elem := range r.nodeElem {
		if node.Index == -1 {
			continue // virtual container — handled above
		}
		applyLeafMargins(node, elem)
	}
}

// applyContainerMargins sets the Div's SpaceBefore/SpaceAfter to the
// collapsed margin values from its virtual container node.
func applyContainerMargins(node *margins.ContentNode, div *layout.Div) {
	if div == nil {
		return
	}

	newMT := margins.MarginValue(node.MarginTop)
	newMB := margins.MarginValue(node.MarginBottom)
	oldMT := div.GetSpaceBefore()
	oldMB := div.GetSpaceAfter()

	if !floatEqual(newMT, oldMT) {
		div.SetSpaceBefore(max(newMT, 0))
	}
	if !floatEqual(newMB, oldMB) {
		div.SetSpaceAfter(max(newMB, 0))
	}
}

// applyLeafMargins sets SpaceBefore/SpaceAfter on a Paragraph (or ignores
// element types that lack margin setters, such as Heading and ImageElement).
func applyLeafMargins(node *margins.ContentNode, elem layout.Element) {
	para, ok := elem.(*layout.Paragraph)
	if !ok {
		return // Heading, ImageElement, etc. — no margin API
	}

	newMT := margins.MarginValue(node.MarginTop)
	newMB := margins.MarginValue(node.MarginBottom)
	oldMT := para.GetSpaceBefore()
	oldMB := para.GetSpaceAfter()

	if !floatEqual(newMT, oldMT) {
		para.SetSpaceBefore(max(newMT, 0))
	}
	if !floatEqual(newMB, oldMB) {
		para.SetSpaceAfter(max(newMB, 0))
	}
}

// floatEqual returns true if a and b are equal within epsilon.
func floatEqual(a, b float64) bool {
	const epsilon = 1e-9
	diff := a - b
	return diff >= -epsilon && diff <= epsilon
}
