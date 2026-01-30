package kfx

import (
	"fmt"
)

// ContainerKind identifies the type of container for margin collapsing.
// Different container types may have different collapsing behaviors.
type ContainerKind int

const (
	// ContainerRoot is the virtual root container (the storyline itself).
	ContainerRoot ContainerKind = iota

	// ContainerSection is a regular section containing paragraphs and other content.
	ContainerSection

	// ContainerPoem is a poem container with stanzas and optional title/textauthor.
	ContainerPoem

	// ContainerStanza is a stanza within a poem, uses title-block margin mode.
	ContainerStanza

	// ContainerCite is a citation/quote container.
	ContainerCite

	// ContainerEpigraph is an epigraph container.
	ContainerEpigraph

	// ContainerFootnote is a footnote container.
	ContainerFootnote

	// ContainerTitleBlock is a title wrapper (body-title, chapter-title, etc.).
	// Uses title-block margin mode.
	ContainerTitleBlock

	// ContainerAnnotation is an annotation container.
	ContainerAnnotation
)

// String returns a human-readable name for the container kind.
func (k ContainerKind) String() string {
	switch k {
	case ContainerRoot:
		return "root"
	case ContainerSection:
		return "section"
	case ContainerPoem:
		return "poem"
	case ContainerStanza:
		return "stanza"
	case ContainerCite:
		return "cite"
	case ContainerEpigraph:
		return "epigraph"
	case ContainerFootnote:
		return "footnote"
	case ContainerTitleBlock:
		return "title-block"
	case ContainerAnnotation:
		return "annotation"
	default:
		return "unknown"
	}
}

// ContainerFlags control margin collapsing behavior for a container.
// These flags can be combined using bitwise OR.
type ContainerFlags uint32

const (
	// FlagTitleBlockMode enables title-block margin mode:
	//   - First element LOSES margin-top
	//   - Non-first/non-last: KEEPS margin-top, LOSES margin-bottom
	//   - Last element: KEEPS margin-top, GETS container's margin-bottom
	// This is used for stanzas and title wrappers.
	FlagTitleBlockMode ContainerFlags = 1 << iota

	// FlagHasBorderTop indicates the container has a top border.
	// Prevents first-child margin collapsing with parent.
	FlagHasBorderTop

	// FlagHasBorderBottom indicates the container has a bottom border.
	// Prevents last-child margin collapsing with parent.
	FlagHasBorderBottom

	// FlagHasPaddingTop indicates the container has top padding.
	// Prevents first-child margin collapsing with parent.
	FlagHasPaddingTop

	// FlagHasPaddingBottom indicates the container has bottom padding.
	// Prevents last-child margin collapsing with parent.
	FlagHasPaddingBottom

	// FlagPreventCollapseTop prevents first-child ↔ parent margin-top collapsing.
	// Used when the container has explicit positioning, float, or overflow:hidden.
	FlagPreventCollapseTop

	// FlagPreventCollapseBottom prevents last-child ↔ parent margin-bottom collapsing.
	// Used when the container has explicit height, min-height, or overflow:hidden.
	FlagPreventCollapseBottom

	// FlagStripMiddleMarginBottom removes margin-bottom from all non-last children.
	// This is used for stanzas where KP3 spacing between verses comes entirely from
	// margin-top, not margin-bottom. The last child keeps its mb (or gets accumulated mb
	// from last-child collapsing).
	FlagStripMiddleMarginBottom

	// FlagTransferMBToLastChild changes last-child collapsing direction:
	// Instead of transferring last child's mb TO container, this transfers
	// container's mb TO last child (collapsed with last child's mb).
	// Used for stanzas where the last verse should carry the accumulated margin.
	// NOTE: This flag only transfers when the container is NOT the last child
	// of its parent (to allow proper bubble-up for sibling collapsing).
	FlagTransferMBToLastChild

	// FlagForceTransferMBToLastChild is like FlagTransferMBToLastChild but always
	// transfers the container's margin-bottom to the last child, even when the
	// container is the last child of its parent. Used for annotation containers
	// where we always want the margin-bottom on the last paragraph.
	FlagForceTransferMBToLastChild
)

// HasFlag returns true if the flag is set.
func (f ContainerFlags) HasFlag(flag ContainerFlags) bool {
	return f&flag != 0
}

// IsTitleBlockMode returns true if title-block margin mode is enabled.
func (f ContainerFlags) IsTitleBlockMode() bool {
	return f.HasFlag(FlagTitleBlockMode)
}

// PreventsCollapseTop returns true if first-child collapsing is prevented.
// This is true if any of the following flags are set:
// - FlagPreventCollapseTop
// - FlagHasBorderTop
// - FlagHasPaddingTop
func (f ContainerFlags) PreventsCollapseTop() bool {
	return f.HasFlag(FlagPreventCollapseTop) ||
		f.HasFlag(FlagHasBorderTop) ||
		f.HasFlag(FlagHasPaddingTop)
}

// PreventsCollapseBottom returns true if last-child collapsing is prevented.
// This is true if any of the following flags are set:
// - FlagPreventCollapseBottom
// - FlagHasBorderBottom
// - FlagHasPaddingBottom
func (f ContainerFlags) PreventsCollapseBottom() bool {
	return f.HasFlag(FlagPreventCollapseBottom) ||
		f.HasFlag(FlagHasBorderBottom) ||
		f.HasFlag(FlagHasPaddingBottom)
}

// StripsMiddleMarginBottom returns true if margin-bottom should be stripped
// from all non-last children in this container.
func (f ContainerFlags) StripsMiddleMarginBottom() bool {
	return f.HasFlag(FlagStripMiddleMarginBottom)
}

// TransfersMBToLastChild returns true if the container's margin-bottom should be
// transferred to the last child (instead of the normal direction).
func (f ContainerFlags) TransfersMBToLastChild() bool {
	return f.HasFlag(FlagTransferMBToLastChild)
}

// ForceTransfersMBToLastChild returns true if the container's margin-bottom should
// always be transferred to the last child, regardless of the container's position.
func (f ContainerFlags) ForceTransfersMBToLastChild() bool {
	return f.HasFlag(FlagForceTransferMBToLastChild)
}

// ContentNode represents a content entry in the margin collapsing tree.
// Each node corresponds to a ContentRef and holds margin information
// that will be modified during the collapsing phases.
type ContentNode struct {
	// Index is the index into StorylineBuilder.contentEntries.
	// A value of -1 indicates a virtual container node (no corresponding ContentRef).
	Index int

	// EID is the element ID from the ContentRef (for debugging/tracing).
	EID int

	// ContentType is a description of the content type (e.g., "text", "image").
	ContentType string

	// Style is the current style name from ContentRef (may be updated after collapsing).
	Style string

	// MarginTop is the margin-top value in lh units.
	// nil means no margin (will be removed from style).
	// 0.0 means explicit zero margin (also removed - KP3 doesn't output zero margins).
	MarginTop *float64

	// MarginBottom is the margin-bottom value in lh units.
	// nil means no margin (will be removed from style).
	MarginBottom *float64

	// HasBreakAfterAvoid is true if the element has page-break-after: avoid (yj-break-after: avoid).
	// Elements with this property keep their margin-bottom and don't collapse with next sibling.
	HasBreakAfterAvoid bool

	// StripMarginBottom is true if this element's margin-bottom should be stripped.
	// This is set when an empty-line follows this element, matching KP3 behavior.
	StripMarginBottom bool

	// EmptyLineMarginBottom stores the empty-line margin to apply as this element's margin-bottom.
	// This is set when an empty-line is followed by an image - KP3 puts the empty-line margin
	// on the PREVIOUS element (as mb) rather than the image (as mt).
	EmptyLineMarginBottom *float64

	// EmptyLineMarginTop stores the empty-line margin to apply as this element's margin-top.
	// This is set when an empty-line precedes a text element. The margin is applied during
	// post-processing to avoid font-size scaling that would occur if baked into the CSS style.
	EmptyLineMarginTop *float64

	// IsFloatImage is true for full-width standalone block images (width >= 512px).
	// These images have fixed 2.6lh margins and act as barriers to sibling margin collapsing.
	IsFloatImage bool

	// ContainerKind identifies the type of container this node represents.
	// For leaf content nodes, this is ContainerRoot (not a container).
	ContainerKind ContainerKind

	// ContainerFlags control collapsing behavior for container nodes.
	ContainerFlags ContainerFlags

	// Parent is the parent container node (nil for root).
	Parent *ContentNode

	// Children are the child nodes within this container.
	Children []*ContentNode

	// EntryOrder is the order in which this entry was added (for sibling ordering).
	// Lower values come first. Used to correctly order siblings when containers
	// and content entries are interleaved.
	EntryOrder int

	// HasWrapper is true when this container corresponds to an actual wrapper entry
	// in the storyline (a content entry with content_list).
	//
	// For such containers, KP3 keeps container margins on the wrapper itself and
	// collapses first/last-child margins into the wrapper (not by transferring the
	// wrapper's margins down to children). Purely virtual containers (no wrapper
	// entry) still use transfer semantics.
	HasWrapper bool
}

// IsContainer returns true if this node is a container (has children or is explicitly marked).
func (n *ContentNode) IsContainer() bool {
	return len(n.Children) > 0 || n.ContainerKind != ContainerRoot
}

// IsEmpty returns true if this node is considered "empty" for margin collapsing.
// A node is empty if it has no content (text type with no actual content).
// Images are NOT empty even if they have no text.
// Empty containers with children are NOT empty.
func (n *ContentNode) IsEmpty() bool {
	// Container nodes with children are not empty
	if len(n.Children) > 0 {
		return false
	}
	// Images are never empty for margin purposes
	if n.ContentType == "image" {
		return false
	}
	// Virtual container nodes (Index == -1) with no children are empty
	if n.Index == -1 {
		return true
	}
	// Text nodes would need content check, but we'll assume non-empty for now
	// since FB2 doesn't generate empty text entries
	return false
}

// IsFirst returns true if this node is the first child in its parent.
func (n *ContentNode) IsFirst() bool {
	if n.Parent == nil {
		return true
	}
	return len(n.Parent.Children) > 0 && n.Parent.Children[0] == n
}

// IsLast returns true if this node is the last child in its parent.
func (n *ContentNode) IsLast() bool {
	if n.Parent == nil {
		return true
	}
	children := n.Parent.Children
	return len(children) > 0 && children[len(children)-1] == n
}

// IsOnly returns true if this node is the only child in its parent.
func (n *ContentNode) IsOnly() bool {
	return n.IsFirst() && n.IsLast()
}

// TraceID returns a string identifier for this node suitable for trace output.
func (n *ContentNode) TraceID() string {
	if n.Index == -1 {
		return fmt.Sprintf("container(%s)", n.ContainerKind.String())
	}
	if n.EID > 0 {
		return fmt.Sprintf("eid=%d", n.EID)
	}
	return fmt.Sprintf("idx=%d", n.Index)
}

// ContentTree holds the tree structure for margin collapsing.
// It's built from the flat contentRefs slice using container IDs.
type ContentTree struct {
	// Root is the virtual root node representing the storyline.
	Root *ContentNode

	// NodeMap provides O(1) lookup from content index to node.
	// Only leaf content nodes are in this map (not virtual container nodes).
	NodeMap map[int]*ContentNode

	// WrapperMap tracks which wrapper entry (by index in contentEntries) corresponds
	// to which virtual container node. This is needed to update wrapper styles after
	// margin collapsing modifies the virtual container's margins.
	WrapperMap map[int]*ContentNode

	// tracer is used for logging collapse operations (may be nil).
	tracer *StyleTracer
}

// AllContentNodes returns all leaf content nodes (nodes with actual content)
// in tree traversal order (depth-first, pre-order).
// This includes both direct entries (Index >= 0) and wrapper children (Index < -1).
// Virtual container nodes (Index == -1) are excluded.
func (t *ContentTree) AllContentNodes() []*ContentNode {
	var nodes []*ContentNode
	t.collectContentNodes(t.Root, &nodes)
	return nodes
}

// collectContentNodes recursively collects leaf content nodes.
func (t *ContentTree) collectContentNodes(node *ContentNode, nodes *[]*ContentNode) {
	// If this is a leaf content node (not a virtual container), add it
	// Index == -1 is reserved for virtual containers; all other indices are content
	if node.Index != -1 {
		*nodes = append(*nodes, node)
	}
	// Recurse into children
	for _, child := range node.Children {
		t.collectContentNodes(child, nodes)
	}
}
