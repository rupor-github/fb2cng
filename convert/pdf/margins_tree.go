package pdf

import (
	"github.com/carlos7ags/folio/layout"

	"fbc/convert/margins"
)

// marginMeta stores margin collapsing metadata for a Div container.
// Populated during element creation (by tagContainer) to identify
// container kinds and behavioral flags for the collapse algorithm.
type marginMeta struct {
	kind  margins.ContainerKind
	flags margins.ContainerFlags
}

// marginTreeResult holds the built margin tree together with lookup maps
// that the apply phase uses to map tree nodes back to folio elements.
type marginTreeResult struct {
	tree *margins.ContentTree

	// nodeElem maps leaf content nodes to their folio elements
	// (Paragraph, Heading, ImageElement, etc.).
	nodeElem map[*margins.ContentNode]layout.Element

	// wrapperDiv maps container nodes to the Div that owns them.
	wrapperDiv map[*margins.ContentNode]*layout.Div
}

// buildMarginTree constructs a margins.ContentTree from an element list.
//
// Div elements become container nodes; Paragraphs, Headings, and other
// elements become leaf content nodes.  The meta map provides container
// kind/flag annotations for Div wrappers (populated by tagContainer
// during element creation).
//
// The resulting tree can be passed to margins.CollapseTree() and the
// collapsed values applied back via applyCollapsedMargins().
func buildMarginTree(elements []layout.Element, meta map[*layout.Div]*marginMeta) *marginTreeResult {
	result := &marginTreeResult{
		tree:       margins.NewContentTree(nil),
		nodeElem:   make(map[*margins.ContentNode]layout.Element),
		wrapperDiv: make(map[*margins.ContentNode]*layout.Div),
	}
	counter := 0
	addElements(result, result.tree.Root, elements, meta, &counter)
	return result
}

// addElements walks a slice of elements and adds each to the tree under parent.
func addElements(r *marginTreeResult, parent *margins.ContentNode, elements []layout.Element, meta map[*layout.Div]*marginMeta, counter *int) {
	for _, elem := range elements {
		*counter++
		addElement(r, parent, elem, *counter, meta, counter)
	}
}

// addElement adds a single folio element to the margin tree.
func addElement(r *marginTreeResult, parent *margins.ContentNode, elem layout.Element, order int, meta map[*layout.Div]*marginMeta, counter *int) {
	switch e := elem.(type) {
	case *layout.Paragraph:
		node := &margins.ContentNode{
			Index:        order,
			ContentType:  "text",
			MarginTop:    margins.PtrFloat64(e.GetSpaceBefore()),
			MarginBottom: margins.PtrFloat64(e.GetSpaceAfter()),
			Parent:       parent,
			EntryOrder:   order,
		}
		parent.Children = append(parent.Children, node)
		r.nodeElem[node] = e

	case *layout.Div:
		kind := margins.ContainerSection // default
		var flags margins.ContainerFlags
		if meta != nil {
			if m, ok := meta[e]; ok {
				kind = m.kind
				flags = m.flags
			}
		}
		containerNode := &margins.ContentNode{
			Index:          -1, // virtual container
			ContainerKind:  kind,
			ContainerFlags: flags,
			MarginTop:      margins.PtrFloat64(e.GetSpaceBefore()),
			MarginBottom:   margins.PtrFloat64(e.GetSpaceAfter()),
			Parent:         parent,
			HasWrapper:     true,
			EntryOrder:     order,
		}
		parent.Children = append(parent.Children, containerNode)
		r.wrapperDiv[containerNode] = e

		// Recurse into the Div's children.
		addElements(r, containerNode, e.Children(), meta, counter)

	case *layout.Heading:
		// Headings have no SpaceBefore/SpaceAfter API.  Their internal
		// spacing (headingSize*0.5) is non-removable.  When a heading is
		// inside a wrapper Div, margin collapsing operates on the Div's
		// margins.  A bare heading (no wrapper) participates as a leaf
		// with no margins.
		node := &margins.ContentNode{
			Index:       order,
			ContentType: "heading",
			Parent:      parent,
			EntryOrder:  order,
		}
		parent.Children = append(parent.Children, node)
		r.nodeElem[node] = e

	default:
		// ImageElement, Table, AreaBreak, etc. — leaf nodes with no margins.
		node := &margins.ContentNode{
			Index:       order,
			ContentType: "other",
			Parent:      parent,
			EntryOrder:  order,
		}
		parent.Children = append(parent.Children, node)
		r.nodeElem[node] = elem
	}
}

// tagContainer records margin collapsing metadata for a Div container.
// Called during element creation to associate a Div with its container
// kind and flags.  Border/padding barrier flags are automatically
// computed from the resolved style.
func tagContainer(meta map[*layout.Div]*marginMeta, div *layout.Div, kind margins.ContainerKind, flags margins.ContainerFlags, style resolvedStyle) {
	if meta == nil {
		return
	}
	if style.HasBorder {
		flags |= margins.FlagHasBorderTop | margins.FlagHasBorderBottom
	}
	if style.PaddingTop > 0 {
		flags |= margins.FlagHasPaddingTop
	}
	if style.PaddingBottom > 0 {
		flags |= margins.FlagHasPaddingBottom
	}
	meta[div] = &marginMeta{kind: kind, flags: flags}
}
