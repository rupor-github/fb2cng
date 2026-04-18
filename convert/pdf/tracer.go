// Copyright 2026 rupor-github
// SPDX-License-Identifier: Apache-2.0

package pdf

// PDFTracer records pipeline-state events during PDF generation so they
// can be inspected post-mortem from the debug report archive.
//
// The tracer mirrors convert/kfx/style_tracer.go in shape and intent:
// it is enabled only when a non-empty working directory is supplied
// (i.e. when the caller is running with reporting enabled via the -d
// CLI flag), and every recording method is a no-op otherwise so there
// is zero runtime cost in normal operation.
//
// The tracer captures events that are difficult or impossible to
// reconstruct from the final PDF file:
//
//   - addElement decisions (AreaBreak add/suppress, page-top sentinel
//     emission, element types as they reach folio) — each ADD entry
//     includes a short preview of the element's visible content when
//     one was registered via Label() at creation time.
//   - Anchor registration (id → pageIdx) and the order in which folio
//     visits pages — essential for debugging page-index drift when
//     pages carry no anchor targets.
//   - Internal-link rewrites (URI="#id" → DestName="id") across
//     paragraph/heading PlacedBlock trees.
//   - Vertical margin tree structure before and after collapse — the
//     PDF equivalent of kfxdump -margins, emitted pre-PDF-write.  Leaf
//     nodes show the content preview registered via Label() when the
//     corresponding element was created.
//
// On Flush() the accumulated entries are serialised to
// <workDir>/pdf-trace.txt, which is then picked up automatically by
// the reporter when it archives the working directory.  The tracer
// clears its buffer after flush so double-flushing is safe.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/carlos7ags/folio/layout"

	"fbc/convert/margins"
	"fbc/convert/structure"
)

// previewMaxLen caps the length of content previews embedded in trace
// output so long paragraphs do not dominate a line.  The cutoff is
// generous enough for a short sentence or the first clause of a
// paragraph to land intact; beyond that an ellipsis is appended.
const previewMaxLen = 48

// PDFTracer accumulates pipeline events for a single PDF generation run.
//
// A nil tracer is a valid no-op — every public method is nil-safe, so
// instrumentation sites can call methods unconditionally without
// guarding against the tracer being disabled.
type PDFTracer struct {
	enabled  bool
	workDir  string
	entries  []pdfTraceEntry
	sections map[string]int            // per-operation counters for the summary header
	labels   map[layout.Element]string // content previews registered via Label
}

// pdfTraceEntry is one line in the trace file, with an operation label
// and a formatted detail block.  The index in PDFTracer.entries
// becomes the trace line number in the output.
type pdfTraceEntry struct {
	operation string
	subject   string
	details   string
}

// NewPDFTracer creates a new tracer.  If workDir is empty the tracer
// is disabled and every method becomes a no-op.  This matches the
// convention used by KFX's StyleTracer so the two pipelines behave
// identically under -d.
func NewPDFTracer(workDir string) *PDFTracer {
	return &PDFTracer{
		workDir:  workDir,
		enabled:  workDir != "",
		sections: make(map[string]int),
		labels:   make(map[layout.Element]string),
	}
}

// IsEnabled reports whether the tracer is active.  Returns false when
// the receiver is nil so instrumentation sites can safely skip
// expensive formatting work.
func (t *PDFTracer) IsEnabled() bool {
	if t == nil {
		return false
	}
	return t.enabled
}

// Label stores a short human-readable preview string for the given
// element, to be consulted later by describeElement() during ADD and
// margin-tree rendering.  Called at element-creation time from factory
// helpers (paragraph/heading/image) where the source text is still in
// scope.
//
// The preview is truncated to previewMaxLen runes and whitespace is
// collapsed so the trace remains readable even for long paragraphs.
func (t *PDFTracer) Label(elem layout.Element, preview string) {
	if !t.IsEnabled() || elem == nil {
		return
	}
	t.labels[elem] = truncatePreview(preview)
}

// describeElement returns a human-readable single-line description of
// a folio element: its type name plus either a content preview (for
// elements that were previously Label()'d) or a structural hint (child
// count for Divs, "AreaBreak" for breaks, etc.).  Always safe to call
// for any element including nil.
func (t *PDFTracer) describeElement(elem layout.Element) string {
	if elem == nil {
		return "<nil>"
	}
	kind := fmt.Sprintf("%T", elem)
	if t != nil {
		if preview, ok := t.labels[elem]; ok && preview != "" {
			return fmt.Sprintf("%s %q", kind, preview)
		}
	}
	// Structural hints for elements we did not Label.  Div exposes
	// Children(); descend once to find a labelled paragraph/heading so
	// a bare wrapper Div still yields context.
	if div, ok := elem.(*layout.Div); ok {
		children := div.Children()
		if nested := t.firstChildPreview(children); nested != "" {
			return fmt.Sprintf("%s (%d children) %q", kind, len(children), nested)
		}
		return fmt.Sprintf("%s (%d children)", kind, len(children))
	}
	// Internal wrapper types (anchoredElement, internalLinkRewriter)
	// delegate to an inner element; surface its preview so the trace
	// remains readable.  unwrapInnerElement uses a narrow type-switch
	// over pdf-package-private wrappers and is nil-safe.
	if inner := unwrapInnerElement(elem); inner != nil {
		if preview, ok := t.labels[inner]; ok && preview != "" {
			return fmt.Sprintf("%s → %s %q", kind, fmt.Sprintf("%T", inner), preview)
		}
		if div, ok := inner.(*layout.Div); ok {
			if nested := t.firstChildPreview(div.Children()); nested != "" {
				return fmt.Sprintf("%s → *layout.Div %q", kind, nested)
			}
		}
	}
	return kind
}

// unwrapInnerElement returns the inner element wrapped by one of the
// pdf package's decorator types (anchoredElement, internalLinkRewriter).
// Returns nil for elements that are not known wrappers so callers can
// distinguish "unwrappable" from "no preview available".
func unwrapInnerElement(elem layout.Element) layout.Element {
	switch w := elem.(type) {
	case *anchoredElement:
		return w.inner
	case *internalLinkRewriter:
		return w.inner
	}
	return nil
}

// firstChildPreview walks a Div's direct and indirect children looking
// for the first element with a registered preview label.  Returns ""
// if none found within a reasonable depth (to bound recursion on
// deeply nested structures).
func (t *PDFTracer) firstChildPreview(children []layout.Element) string {
	const maxDepth = 4
	return t.firstChildPreviewDepth(children, maxDepth)
}

func (t *PDFTracer) firstChildPreviewDepth(children []layout.Element, depth int) string {
	if t == nil || depth <= 0 {
		return ""
	}
	for _, c := range children {
		if p, ok := t.labels[c]; ok && p != "" {
			return p
		}
		if div, ok := c.(*layout.Div); ok {
			if nested := t.firstChildPreviewDepth(div.Children(), depth-1); nested != "" {
				return nested
			}
		}
	}
	return ""
}

// truncatePreview collapses runs of whitespace into single spaces and
// caps the resulting string at previewMaxLen runes, appending an
// ellipsis when truncated.  Returns "" for inputs that contain only
// whitespace.
func truncatePreview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= previewMaxLen {
		return s
	}
	return string(runes[:previewMaxLen]) + "…"
}

// TraceAddElement records a top-level element reaching folio via
// renderContext.addElement.  atPageTop indicates whether a zero-height
// sentinel Div was inserted ahead of the element to preserve its
// SpaceBefore against folio's stripLeadingOffset.
func (t *PDFTracer) TraceAddElement(elem layout.Element, atPageTop bool) {
	if !t.IsEnabled() {
		return
	}
	details := fmt.Sprintf("atPageTop=%t", atPageTop)
	if atPageTop {
		details += " (sentinel emitted)"
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "ADD",
		subject:   t.describeElement(elem),
		details:   details,
	})
	t.sections["elements_added"]++
}

// TraceAreaBreak records an explicit AreaBreak being forwarded to
// folio or suppressed because the builder is already at page-top.
// Duplicate AreaBreaks are the classic source of blank pages in PDF
// output; counting suppress events makes the class of bug obvious
// in the trace summary.
func (t *PDFTracer) TraceAreaBreak(action string) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "BREAK",
		subject:   action,
	})
	t.sections["break_"+action]++
}

// TracePageObserve records the first Draw-time observation of a
// previously unseen *PageResult pointer.  pageIdx is the sequential
// index assigned by the anchor tracker.
//
// The order of observations must match folio's physical page order
// for pointer→index mapping to be correct.  A gap in the observed
// sequence (e.g. index 0, 2, 3) is a sign that some pages lack
// instrumented elements and pointer→index drift is likely.
func (t *PDFTracer) TracePageObserve(pageIdx int) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "PAGE",
		subject:   fmt.Sprintf("index=%d", pageIdx),
	})
	t.sections["pages_observed"]++
}

// TraceAnchorRegister records a named-destination being added to the
// document.  pageIdx is the final resolved index the id will navigate
// to.
func (t *PDFTracer) TraceAnchorRegister(id string, pageIdx int) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "ANCHOR",
		subject:   id,
		details:   fmt.Sprintf("pageIdx=%d", pageIdx),
	})
	t.sections["anchors_registered"]++
}

// TraceLinkRewrite records an inline LinkURI="#id" being rewritten
// to DestName="id" by internalLinkRewriter.  Counting these is a
// useful sanity check against the total number of internal href
// spans in the source document.
func (t *PDFTracer) TraceLinkRewrite(from, to string) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "REWRITE",
		subject:   from,
		details:   "-> " + to,
	})
	t.sections["links_rewritten"]++
}

// TraceMarginTree records a snapshot of a margin ContentTree.  label
// distinguishes pre-collapse from post-collapse snapshots.  nodeElem
// supplies the leaf-node → element mapping so leaves can be annotated
// with the content preview registered for that element.  The tree is
// rendered recursively with one line per node, indented by depth, so
// diffs between label="before" and label="after" highlight the effect
// of the collapse algorithm.
func (t *PDFTracer) TraceMarginTree(label string, tree *margins.ContentTree, nodeElem map[*margins.ContentNode]layout.Element) {
	if !t.IsEnabled() || tree == nil || tree.Root == nil {
		return
	}
	var sb strings.Builder
	t.formatMarginNode(&sb, tree.Root, 0, nodeElem)
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "TREE",
		subject:   label,
		details:   sb.String(),
	})
	t.sections["tree_"+label]++
}

// formatMarginNode renders a single ContentNode and its children into
// sb with depth-based indentation.  Leaf nodes show their content type
// and margins, plus a content preview when the corresponding element
// was Label()'d at creation time.  Container nodes show kind/flags and
// recurse.  The root sentinel (Parent==nil AND Index==-1 AND
// ContainerKind==Root) is rendered as a bare "root" marker.
func (t *PDFTracer) formatMarginNode(sb *strings.Builder, n *margins.ContentNode, depth int, nodeElem map[*margins.ContentNode]layout.Element) {
	if n == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	switch {
	case n.Parent == nil && n.Index == -1 && n.ContainerKind == margins.ContainerRoot:
		fmt.Fprintf(sb, "%sroot\n", indent)
	case n.Index == -1:
		fmt.Fprintf(sb, "%s[container kind=%s flags=%d mt=%s mb=%s]\n", indent, n.ContainerKind.String(), n.ContainerFlags, formatMarginPtr(n.MarginTop), formatMarginPtr(n.MarginBottom))
	default:
		fmt.Fprintf(sb, "%s[%d %s mt=%s mb=%s", indent, n.Index, n.ContentType, formatMarginPtr(n.MarginTop), formatMarginPtr(n.MarginBottom))
		if n.StripMarginBottom {
			sb.WriteString(" stripMB")
		}
		if n.EmptyLineMarginTop != nil {
			fmt.Fprintf(sb, " emlMT=%.3f", *n.EmptyLineMarginTop)
		}
		if n.EmptyLineMarginBottom != nil {
			fmt.Fprintf(sb, " emlMB=%.3f", *n.EmptyLineMarginBottom)
		}
		if preview := t.previewForNode(n, nodeElem); preview != "" {
			fmt.Fprintf(sb, " %q", preview)
		}
		sb.WriteString("]\n")
	}
	for _, c := range n.Children {
		t.formatMarginNode(sb, c, depth+1, nodeElem)
	}
}

// previewForNode looks up the element associated with a leaf margin
// node and returns its registered preview label, if any.
func (t *PDFTracer) previewForNode(n *margins.ContentNode, nodeElem map[*margins.ContentNode]layout.Element) string {
	if t == nil || nodeElem == nil {
		return ""
	}
	elem, ok := nodeElem[n]
	if !ok {
		return ""
	}
	if preview, ok := t.labels[elem]; ok {
		return preview
	}
	// Fall back to scanning Div children for a labelled descendant.
	if div, ok := elem.(*layout.Div); ok {
		return t.firstChildPreview(div.Children())
	}
	return ""
}

// formatMarginPtr formats a nullable margin value for trace output.
func formatMarginPtr(m *float64) string {
	if m == nil {
		return "nil"
	}
	return fmt.Sprintf("%.3f", *m)
}

// ---------------------------------------------------------------------------
// Outline / TOC tracing
// ---------------------------------------------------------------------------

// TraceTOCInput records the full plan.TOC tree as received by the
// outline finalizer.  This is the "before" snapshot — the logical TOC
// that structure.builder produced after wrong-nesting compensation.
// Comparing it with the OUTLINE_ADD / OUTLINE_HOIST / OUTLINE_SKIP
// events that follow reveals any resolution failures or structural
// discrepancies between the plan and the final PDF outline.
func (t *PDFTracer) TraceTOCInput(entries []*structure.TOCEntry) {
	if !t.IsEnabled() {
		return
	}
	var sb strings.Builder
	t.formatTOCEntries(&sb, entries, 0)
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "TOC_INPUT",
		subject:   fmt.Sprintf("%d root entries", len(entries)),
		details:   sb.String(),
	})
	t.sections["toc_input"]++
}

// formatTOCEntries recursively renders a TOCEntry tree into sb with
// depth-based indentation.  Each entry shows its ID, title (full,
// Go-escaped), and IncludeInTOC flag.
func (t *PDFTracer) formatTOCEntries(sb *strings.Builder, entries []*structure.TOCEntry, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, e := range entries {
		if e == nil {
			continue
		}
		flag := " "
		if !e.IncludeInTOC {
			flag = "!"
		}
		fmt.Fprintf(sb, "%s%s[%s] %q\n", indent, flag, e.ID, e.Title)
		if len(e.Children) > 0 {
			t.formatTOCEntries(sb, e.Children, depth+1)
		}
	}
}

// TraceOutlineAdd records a TOC entry being added to the PDF outline
// at the given depth (0 = root, 1 = child of root, etc.) with its
// resolved page index.  The title is emitted in full with Go escaping.
func (t *PDFTracer) TraceOutlineAdd(id, title string, depth, pageIdx int) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "OUTLINE_ADD",
		subject:   fmt.Sprintf("depth=%d %q", depth, title),
		details:   fmt.Sprintf("id=%s pageIdx=%d", id, pageIdx),
	})
	t.sections["outline_added"]++
}

// TraceOutlineSkip records a TOC entry that could not be resolved to a
// page index.  action describes what happened: "hoist" when the
// entry's children were promoted to the parent level, "drop" when the
// entry has no children and is simply omitted.  The title is emitted
// in full with Go escaping.
func (t *PDFTracer) TraceOutlineSkip(id, title, action string) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "OUTLINE_SKIP",
		subject:   fmt.Sprintf("%s %q", action, title),
		details:   fmt.Sprintf("id=%s", id),
	})
	t.sections["outline_skipped"]++
}

// TraceOutlineDone records the completion of the outline build with a
// count of total entries added.
func (t *PDFTracer) TraceOutlineDone(added, skipped int) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "OUTLINE_DONE",
		subject:   fmt.Sprintf("added=%d skipped=%d", added, skipped),
	})
}

// Flush serialises the accumulated trace to <workDir>/pdf-trace.txt
// and returns the path, or an empty string if the tracer is disabled
// or there is nothing to write.  The internal buffer is cleared after
// a successful flush so a subsequent flush is a no-op.
//
// The caller typically defers Flush() at the start of Generate() so
// the trace file is written even when PDF generation fails partway
// through — invaluable for diagnosing crashes.
func (t *PDFTracer) Flush() string {
	if !t.IsEnabled() || len(t.entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("=== PDF Generation Trace ===\n\n")

	// Summary counters, sorted for stable diffs across runs.
	sb.WriteString("Summary:\n")
	keys := make([]string, 0, len(t.sections))
	for k := range t.sections {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&sb, "  %s: %d\n", k, t.sections[k])
	}
	sb.WriteString("\n")

	sb.WriteString("Detailed Trace:\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for i, entry := range t.entries {
		fmt.Fprintf(&sb, "[%04d] %-8s %s\n", i+1, entry.operation, entry.subject)
		if entry.details != "" {
			for _, line := range strings.Split(strings.TrimRight(entry.details, "\n"), "\n") {
				sb.WriteString("       " + line + "\n")
			}
		}
	}

	tracePath := filepath.Join(t.workDir, "pdf-trace.txt")
	if err := os.WriteFile(tracePath, []byte(sb.String()), 0o644); err != nil {
		return ""
	}

	t.entries = nil
	t.sections = make(map[string]int)
	t.labels = make(map[layout.Element]string)

	return tracePath
}
