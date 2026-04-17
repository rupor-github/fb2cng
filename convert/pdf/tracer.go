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
//     emission, element types as they reach folio).
//   - Anchor registration (id → pageIdx) and the order in which folio
//     visits page pointers — essential for debugging pointer→index
//     drift when pages carry no anchor targets.
//   - Internal-link rewrites (URI="#id" → DestName="id") across
//     paragraph/heading PlacedBlock trees.
//   - Vertical margin tree structure before and after collapse — the
//     PDF equivalent of kfxdump -margins, emitted pre-PDF-write.
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
)

// PDFTracer accumulates pipeline events for a single PDF generation run.
//
// A nil tracer is a valid no-op — every public method is nil-safe, so
// instrumentation sites can call methods unconditionally without
// guarding against the tracer being disabled.
type PDFTracer struct {
	enabled  bool
	workDir  string
	entries  []pdfTraceEntry
	sections map[string]int // per-operation counters for the summary header
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

// TraceAddElement records a top-level element reaching folio via
// renderContext.addElement.  atPageTop indicates whether a zero-height
// sentinel Div was inserted ahead of the element to preserve its
// SpaceBefore against folio's stripLeadingOffset.
func (t *PDFTracer) TraceAddElement(elem layout.Element, atPageTop bool) {
	if !t.IsEnabled() {
		return
	}
	kind := fmt.Sprintf("%T", elem)
	details := fmt.Sprintf("atPageTop=%t", atPageTop)
	if atPageTop {
		details += " (sentinel emitted)"
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "ADD",
		subject:   kind,
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
// index assigned by the anchor tracker; ptr is the pointer address
// (as hex text) so cross-references with TraceAnchorRegister can be
// verified manually.
//
// The order of observations must match folio's physical page order
// for pointer→index mapping to be correct.  A gap in the observed
// sequence (e.g. index 0, 2, 3) is a sign that some pages lack
// instrumented elements and pointer→index drift is likely.
func (t *PDFTracer) TracePageObserve(pageIdx int, ptr string, firstElem string) {
	if !t.IsEnabled() {
		return
	}
	details := fmt.Sprintf("ptr=%s first-elem=%s", ptr, firstElem)
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "PAGE",
		subject:   fmt.Sprintf("index=%d", pageIdx),
		details:   details,
	})
	t.sections["pages_observed"]++
}

// TraceAnchorRegister records a named-destination being added to the
// document.  pageIdx is the final resolved index the id will navigate
// to; ptr is the *PageResult pointer address for cross-referencing
// with TracePageObserve.
func (t *PDFTracer) TraceAnchorRegister(id string, pageIdx int, ptr string) {
	if !t.IsEnabled() {
		return
	}
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "ANCHOR",
		subject:   id,
		details:   fmt.Sprintf("pageIdx=%d ptr=%s", pageIdx, ptr),
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
// distinguishes pre-collapse from post-collapse snapshots.  The tree
// is rendered recursively with one line per node, indented by depth,
// so diffs between label="before" and label="after" highlight the
// effect of the collapse algorithm.
func (t *PDFTracer) TraceMarginTree(label string, tree *margins.ContentTree) {
	if !t.IsEnabled() || tree == nil || tree.Root == nil {
		return
	}
	var sb strings.Builder
	formatMarginNode(&sb, tree.Root, 0)
	t.entries = append(t.entries, pdfTraceEntry{
		operation: "TREE",
		subject:   label,
		details:   sb.String(),
	})
	t.sections["tree_"+label]++
}

// formatMarginNode renders a single ContentNode and its children into
// sb with depth-based indentation.  Leaf nodes show their content type
// and margins; container nodes show their kind/flags and recurse.  The
// root sentinel (Parent==nil AND Index==-1 AND ContainerKind==Root) is
// rendered as a bare "root" marker.
func formatMarginNode(sb *strings.Builder, n *margins.ContentNode, depth int) {
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
		sb.WriteString("]\n")
	}
	for _, c := range n.Children {
		formatMarginNode(sb, c, depth+1)
	}
}

// formatMarginPtr formats a nullable margin value for trace output.
func formatMarginPtr(m *float64) string {
	if m == nil {
		return "nil"
	}
	return fmt.Sprintf("%.3f", *m)
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

	return tracePath
}
