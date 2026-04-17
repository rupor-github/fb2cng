// Copyright 2026 rupor-github
// SPDX-License-Identifier: Apache-2.0

package pdf

// This file builds the PDF outline ("bookmark tree") manually from the
// renderer-neutral plan.TOC instead of relying on folio's automatic
// heading-driven outline.
//
// Why not auto-bookmarks?
//
//   Folio's auto-bookmark path (document.buildAutoBookmarks) derives
//   the outline from rendered heading tags (H1..H6) using a level
//   stack.  This caps the outline at six levels — the HTML heading
//   ceiling — which is fine for typical HTML content but is strictly
//   less flexible than KFX (whose Kindle TOC has no depth limit) and
//   EPUB (whose nav.xhtml supports arbitrary <ol> nesting).
//
//   FB2 books with deeply nested wrong-nesting section trees
//   (titled <section>s interleaved with untitled wrapper <section>s)
//   routinely exceed six logical levels.  Matching KFX/EPUB requires
//   unlimited depth.
//
// Where does plan.TOC come from?
//
//   convert/structure/builder.go walks the FB2 body tree and produces
//   a TOC with wrong-nesting compensated (collectSectionChildren
//   promotes titled descendants of untitled wrapper <section>s as
//   children of the last titled sibling).  The same tree drives the
//   EPUB nav.xhtml and KFX navigation, so reusing it guarantees
//   byte-identical outline topology across all three formats.
//
// Timing
//
//   The outline tree is assembled inside a zero-height layout
//   Element ("outlineFinalizer") that is added as the very last
//   document element.  Its Draw closure fires during
//   Document.buildAllPages after every preceding element's
//   PlacedBlock.Draw has run, which is when anchorTracker.register
//   populates tracker.idPage.  By the time Document.WriteTo reaches
//   its outline-serialization phase, doc.outlines has already been
//   populated by the finalizer — folio's auto-bookmark fallback is
//   skipped (doc.autoBookmarks is disabled anyway).
//
// Pointer-safety of doc.AddOutline / Outline.AddChild
//
//   doc.AddOutline appends to doc.outlines and returns a pointer into
//   that slice.  A subsequent AddOutline call may reallocate the
//   slice and invalidate previous pointers.  Outline.AddChild
//   similarly appends to parent.Children.
//
//   The builder works around this with two rules:
//
//   1. Roots are added one at a time; each root's full subtree is
//      built via AddChild before the next root is AddOutline'd.  This
//      keeps the current root pointer valid throughout its subtree
//      construction.
//
//   2. Within a subtree, a node's direct children are all appended
//      first (via AddChild) before recursing into any of them.  We
//      then index back into parent.Children[i] to descend, which is
//      stable because we never mutate parent.Children again during
//      the descent.

import (
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"

	"fbc/convert/structure"
)

// outlineFinalizer is a zero-height layout Element that populates the
// document's outline tree from plan.TOC at the tail end of layout.
//
// It never contributes any visible content.  Its PlanLayout returns a
// single PlacedBlock of height 0 whose Draw closure runs the builder.
// Because the element is added last, its Draw fires after every
// prior element's Draw and therefore after every anchor-id
// registration that would affect page-index resolution.
type outlineFinalizer struct {
	doc     *document.Document
	entries []*structure.TOCEntry
	tracker *anchorTracker
	tracer  *PDFTracer
	done    bool // guard against multiple Draw invocations (overflow, etc.)
}

func newOutlineFinalizer(
	doc *document.Document,
	entries []*structure.TOCEntry,
	tracker *anchorTracker,
	tracer *PDFTracer,
) layout.Element {
	return &outlineFinalizer{
		doc:     doc,
		entries: entries,
		tracker: tracker,
		tracer:  tracer,
	}
}

// PlanLayout implements layout.Element.  Emits a single zero-height
// PlacedBlock whose Draw closure builds the outline tree.  A fatal
// detail: emitting LayoutNothing here would suppress the Draw call
// entirely, so we always emit exactly one block with height 0.
// Folio's renderer tolerates zero-height blocks (they consume no
// space and do not trigger a page break).
func (f *outlineFinalizer) PlanLayout(area layout.LayoutArea) layout.LayoutPlan {
	return layout.LayoutPlan{
		Status:   layout.LayoutFull,
		Consumed: 0,
		Blocks: []layout.PlacedBlock{{
			X:      0,
			Y:      0,
			Width:  0,
			Height: 0,
			Draw: func(ctx layout.DrawContext, x, topY float64) {
				if f.done {
					return
				}
				f.done = true
				// Observe the current page to keep the tracker's
				// physical-page ordering consistent; harmless when
				// the page already has a probe hit.
				if ctx.Page != nil && f.tracker != nil {
					f.tracker.resolvePageIndex(ctx.Page)
				}
				f.build()
			},
		}},
	}
}

// build walks plan.TOC and adds corresponding entries to the PDF
// outline.  Entries whose ID cannot be resolved to a page are
// skipped silently — their descendants are hoisted to the current
// parent so no subtree is lost.  (In practice every titled section
// registers an anchor during layout; this fallback handles
// edge-case plans, e.g. entries pointing at body intros that were
// elided due to filters.)
func (f *outlineFinalizer) build() {
	if f == nil || f.doc == nil {
		return
	}
	for _, root := range f.entries {
		f.addRoot(root)
	}
}

// addRoot adds a single top-level entry (or hoists its children if
// the entry itself lacks a resolvable destination) and completes
// its subtree before returning to the caller.  Completing the
// subtree before the next AddOutline call is essential for pointer
// stability — see the package-level comment above.
func (f *outlineFinalizer) addRoot(entry *structure.TOCEntry) {
	if entry == nil {
		return
	}
	if entry.IncludeInTOC {
		if pageIdx, ok := f.tracker.PageForID(entry.ID); ok {
			o := f.doc.AddOutline(encodePDFTextString(entry.Title), document.FitDest(pageIdx))
			f.addChildren(o, entry.Children)
			return
		}
	}
	// Unresolved root: hoist its children to top level so the
	// subtree is not lost.
	for _, child := range entry.Children {
		f.addRoot(child)
	}
}

// pendingChild records an entry successfully appended to a parent
// outline in Phase 1, along with its stable index into
// parent.Children for the Phase 2 recursive descent.
type pendingChild struct {
	entry *structure.TOCEntry
	idx   int
}

// addChildren appends all direct children of parent first (Phase 1),
// then recurses into each of them by stable index into
// parent.Children (Phase 2).  This two-phase descent avoids
// invalidating previously-returned *Outline pointers when AddChild
// grows the parent's slice.
func (f *outlineFinalizer) addChildren(parent *document.Outline, entries []*structure.TOCEntry) {
	if parent == nil || len(entries) == 0 {
		return
	}

	// Phase 1: append every resolvable child flatly.  Unresolved
	// children are replaced by their own resolvable descendants so
	// deeper branches still surface in the outline.
	var pend []pendingChild
	for _, e := range entries {
		f.appendResolved(parent, e, &pend)
	}

	// Phase 2: recurse using stable indices into parent.Children.
	for _, p := range pend {
		f.addChildren(&parent.Children[p.idx], p.entry.Children)
	}
}

// appendResolved appends entry as a child of parent when it has a
// resolvable destination, or recurses into its children when it
// does not.  The resulting parent.Children indices (for every
// successfully appended entry) are recorded in pend for the Phase 2
// descent.
func (f *outlineFinalizer) appendResolved(
	parent *document.Outline,
	entry *structure.TOCEntry,
	pend *[]pendingChild,
) {
	if entry == nil {
		return
	}
	if entry.IncludeInTOC {
		if pageIdx, ok := f.tracker.PageForID(entry.ID); ok {
			parent.AddChild(encodePDFTextString(entry.Title), document.FitDest(pageIdx))
			*pend = append(*pend, pendingChild{entry: entry, idx: len(parent.Children) - 1})
			return
		}
	}
	// Unresolved: hoist the entry's children to the current parent.
	for _, c := range entry.Children {
		f.appendResolved(parent, c, pend)
	}
}
