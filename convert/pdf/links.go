// Copyright 2026 rupor-github
// SPDX-License-Identifier: Apache-2.0

package pdf

// This file implements internal (intra-document) link support for the PDF
// pipeline.  Folio's public API has two mechanisms for links:
//
//   - layout.Link / NewInternalLink — BLOCK-level elements that can carry
//     a DestName (internal named destination).  Unsuitable for inline
//     footnote refs embedded in prose because splitting a paragraph at
//     every link introduces visible wrap points.
//
//   - TextRun.LinkURI — INLINE links that flow through paragraph layout
//     and produce precise per-line LinkArea spans.  The URI field becomes
//     a /URI action in the resulting PDF annotation — external-style only.
//     There is no TextRun.LinkDest counterpart.
//
// Folio's own HTML-to-PDF pipeline has the same limitation (see
// html/converter_paragraph.go:331 — internal hrefs become LinkURI with
// a leading "#").  As a result most PDF viewers cannot navigate those
// #fragment URIs to a real location.
//
// Our workaround exploits two facts:
//
//  1. layout.PlacedBlock.Links is a slice of LinkArea with both URI
//     and DestName fields, and the renderer passes both to the Document
//     layer unchanged.  The Document resolves DestName against a list
//     of named destinations registered via doc.AddNamedDest(...).
//
//  2. Folio calls each PlacedBlock.Draw closure once per page during
//     layout.  Draw closures run inside buildAllPages(), which is the
//     first thing WriteTo() does — well before named-destination
//     resolution happens later in the same WriteTo() call.  So a Draw
//     closure can mutate doc.namedDests and the mutation is visible to
//     the annotation writer.
//
// Target side (where links point to):
//
//   anchoredElement — a decorator that wraps the inner target element
//   and chains anchor registration into the Draw closure of the inner
//   element's FIRST PlacedBlock.  This guarantees:
//
//     * Zero contribution to layout (no extra blocks, no extra height).
//     * The anchor registers on the same page the inner element's first
//       block is drawn on — which is by definition where the link
//       target visually begins.
//     * No free-standing zero-height blocks that could trigger extra
//       page flushes in folio's flushPage loop.
//
// Source side (inline link spans):
//
//   internalLinkRewriter — a wrapper Element that delegates PlanLayout
//   to an inner Paragraph, then walks every PlacedBlock.Links[*] and,
//   for each LinkArea whose URI begins with "#", moves URI → DestName
//   and strips the leading "#".  This preserves folio's precise
//   per-span link geometry while producing internal GoTo annotations
//   instead of external /URI actions.

import (
	"strings"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"
)

// anchorTracker correlates layout.PageResult pointers with sequential
// page indices and registers FB2 anchor IDs as named destinations.
//
// A single tracker instance is shared across all anchoredElements in a
// document.  The tracker is populated during layout (PlacedBlock.Draw
// closures) and consulted indirectly through doc.namedDests during the
// annotation-writing phase of the same WriteTo() call.
//
// When a non-nil tracer is supplied, the tracker emits PAGE/ANCHOR
// events for every new pointer observation and every id→page mapping;
// these are invaluable for diagnosing page-index drift that would
// otherwise require env-gated printf instrumentation.
type anchorTracker struct {
	doc        *document.Document
	pageIndex  map[*layout.PageResult]int // allocated on first Draw per page
	nextIndex  int
	registered map[string]bool // id -> already added to doc.namedDests
	tracer     *PDFTracer      // nil-safe debug-report tracer
}

func newAnchorTracker(doc *document.Document, tracer *PDFTracer) *anchorTracker {
	return &anchorTracker{
		doc:        doc,
		pageIndex:  make(map[*layout.PageResult]int),
		registered: make(map[string]bool),
		tracer:     tracer,
	}
}

// resolvePageIndex returns a 0-based sequential index for the given
// PageResult pointer, assigning a new index on first observation.
//
// Pages are processed in order by folio's flushPage loop (each call
// allocates a fresh *PageResult), so first-sight assignment matches
// the physical page order for normal-flow content.
func (t *anchorTracker) resolvePageIndex(page *layout.PageResult) int {
	if idx, ok := t.pageIndex[page]; ok {
		return idx
	}
	idx := t.nextIndex
	t.pageIndex[page] = idx
	t.nextIndex++
	if t.tracer.IsEnabled() {
		t.tracer.TracePageObserve(idx)
	}
	return idx
}

// register adds a NamedDest for id pointing to the given page index.
// Duplicates are silently ignored (the first observation wins).
func (t *anchorTracker) register(id string, pageIdx int) {
	if id == "" || t.registered[id] {
		return
	}
	t.registered[id] = true
	t.doc.AddNamedDest(document.NamedDest{
		Name:      id,
		PageIndex: pageIdx,
		FitType:   "Fit",
	})
	if t.tracer.IsEnabled() {
		t.tracer.TraceAnchorRegister(id, pageIdx)
	}
}

// anchoredElement decorates an inner layout.Element by chaining one or
// more anchor-id registrations into the inner element's first placed
// block's Draw closure.
//
// The decorator delegates PlanLayout entirely to the inner element.
// It never emits extra blocks, contributes no height, and performs no
// drawing itself.  The only observable effect is that the first block
// in the returned plan carries a wrapped Draw closure that — in
// addition to its original behavior — records the current page for
// each anchor id in the shared tracker.
//
// If the inner element returns LayoutNothing (zero placed blocks) on
// first call, the anchors are forwarded to the overflow element so
// they still register when the inner element eventually places content
// on a later page.
type anchoredElement struct {
	inner   layout.Element
	ids     []string
	tracker *anchorTracker
}

// newAnchoredElement wraps inner with anchor-id registrations.  If ids
// is empty or tracker is nil, inner is returned unchanged.
func newAnchoredElement(inner layout.Element, ids []string, tracker *anchorTracker) layout.Element {
	if inner == nil {
		return nil
	}
	if len(ids) == 0 || tracker == nil {
		return inner
	}
	return &anchoredElement{inner: inner, ids: ids, tracker: tracker}
}

// newPageProbe wraps inner with a tracker-only probe that observes the
// PageResult pointer on every Draw invocation but registers no anchor
// ids.  This is how the anchor tracker learns about pages that don't
// themselves carry any id targets: folio processes pages sequentially,
// so every page must be observed in order for resolvePageIndex to
// return the correct 0-based index.
//
// Wrapping every element emitted through addElement with a probe (and
// having the probe forward through overflow) guarantees that every
// page gets at least one Draw-time observation, so the pointer→index
// map in anchorTracker reflects the true physical page order.
func newPageProbe(inner layout.Element, tracker *anchorTracker) layout.Element {
	if inner == nil {
		return nil
	}
	if tracker == nil {
		return inner
	}
	return &anchoredElement{inner: inner, ids: nil, tracker: tracker}
}

// PlanLayout implements layout.Element.  It delegates to the inner
// element and then chains anchor registration into the first
// PlacedBlock's Draw closure, or forwards to the overflow when no
// blocks are placed on this page.
func (a *anchoredElement) PlanLayout(area layout.LayoutArea) layout.LayoutPlan {
	plan := a.inner.PlanLayout(area)
	if len(plan.Blocks) > 0 {
		chainAnchorDraw(&plan.Blocks[0], a.ids, a.tracker)
		return plan
	}
	// No blocks placed on this page (LayoutNothing / pure overflow).
	// Forward the anchors to the overflow so they register on the page
	// where the inner element finally places content.
	if plan.Overflow != nil {
		plan.Overflow = &anchoredElement{inner: plan.Overflow, ids: a.ids, tracker: a.tracker}
	}
	return plan
}

// MinWidth / MaxWidth: forward the Measurable interface when the inner
// element provides it, so table auto-sizing and flex layout see the
// inner element's natural dimensions.
func (a *anchoredElement) MinWidth() float64 {
	if m, ok := a.inner.(layout.Measurable); ok {
		return m.MinWidth()
	}
	return 0
}

func (a *anchoredElement) MaxWidth() float64 {
	if m, ok := a.inner.(layout.Measurable); ok {
		return m.MaxWidth()
	}
	return 0
}

// chainAnchorDraw wraps block.Draw so that the original Draw still runs
// and then (or before) the anchor ids are registered.  The registration
// runs AFTER the original Draw so that any page-state mutations the
// original performs (e.g. updating ctx.Page fields) are already in
// effect.
func chainAnchorDraw(block *layout.PlacedBlock, ids []string, tracker *anchorTracker) {
	orig := block.Draw
	block.Draw = func(ctx layout.DrawContext, x, topY float64) {
		if orig != nil {
			orig(ctx, x, topY)
		}
		if ctx.Page == nil || tracker == nil {
			return
		}
		pageIdx := tracker.resolvePageIndex(ctx.Page)
		for _, id := range ids {
			tracker.register(id, pageIdx)
		}
	}
}

// internalLinkRewriter wraps an inner layout.Element (typically a
// Paragraph) and post-processes its layout plan to convert inline
// internal-link URIs (#fragment) into proper DestName annotations.
//
// The wrapper delegates PlanLayout entirely to the inner element and
// then walks the resulting PlacedBlock tree, rewriting every LinkArea:
//
//	URI = "#foo" → URI = "", DestName = "foo"
//
// This must be done BOTH for the fitted Blocks slice and recursively
// for the Overflow element (via another wrapper), so page-split
// paragraphs carry the rewrite on every continuation.
type internalLinkRewriter struct {
	inner  layout.Element
	tracer *PDFTracer // nil-safe; emits REWRITE events when enabled
}

func newInternalLinkRewriter(inner layout.Element, tracer *PDFTracer) layout.Element {
	if inner == nil {
		return nil
	}
	return &internalLinkRewriter{inner: inner, tracer: tracer}
}

// PlanLayout implements layout.Element.
func (w *internalLinkRewriter) PlanLayout(area layout.LayoutArea) layout.LayoutPlan {
	plan := w.inner.PlanLayout(area)
	rewriteInternalLinks(plan.Blocks, w.tracer)
	if plan.Overflow != nil {
		plan.Overflow = &internalLinkRewriter{inner: plan.Overflow, tracer: w.tracer}
	}
	return plan
}

// MinWidth / MaxWidth: forward the Measurable interface when the inner
// element provides it, so table auto-sizing and flex layout behave
// identically to the unwrapped paragraph.
func (w *internalLinkRewriter) MinWidth() float64 {
	if m, ok := w.inner.(layout.Measurable); ok {
		return m.MinWidth()
	}
	return 0
}

func (w *internalLinkRewriter) MaxWidth() float64 {
	if m, ok := w.inner.(layout.Measurable); ok {
		return m.MaxWidth()
	}
	return 0
}

// rewriteInternalLinks walks a block tree and rewrites every LinkArea
// whose URI begins with "#" into a DestName link.  Mutates in place.
// Emits REWRITE events into tracer when it is enabled; the tracer is
// nil-safe so the call site is unconditional.
func rewriteInternalLinks(blocks []layout.PlacedBlock, tracer *PDFTracer) {
	for i := range blocks {
		for j := range blocks[i].Links {
			link := &blocks[i].Links[j]
			if strings.HasPrefix(link.URI, "#") {
				from := link.URI
				link.DestName = link.URI[1:]
				link.URI = ""
				tracer.TraceLinkRewrite(from, link.DestName)
			}
		}
		if len(blocks[i].Children) > 0 {
			rewriteInternalLinks(blocks[i].Children, tracer)
		}
	}
}
