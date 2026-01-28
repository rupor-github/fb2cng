package kfx

import (
	"fmt"
	"sort"
)

// This file implements CSS margin collapsing as a post-processing step,
// matching KP3's (Kindle Previewer 3) behavior.
//
// KP3's margin collapser runs multiple phases for each container:
//   Phase 1: Self-collapse for empty nodes
//   Phase 2: First child ↔ Parent margin-top collapsing
//   Phase 3: Last child ↔ Parent margin-bottom collapsing
//   Phase 4: Adjacent sibling collapsing
//
// The collapse algorithm (from KP3 f.java:261-274):
//   - Both positive: use maximum
//   - Both negative: use minimum (most negative)
//   - Mixed signs: add them
//
// This replaces the inline position-tracking approach in style_context.go
// with a unified post-processing step that operates on a ContentTree
// built from the flat contentRefs slice.

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

// NewContentTree creates an empty content tree with a virtual root node.
func NewContentTree(tracer *StyleTracer) *ContentTree {
	return &ContentTree{
		Root: &ContentNode{
			Index:         -1,
			ContainerKind: ContainerRoot,
		},
		NodeMap:    make(map[int]*ContentNode),
		WrapperMap: make(map[int]*ContentNode),
		tracer:     tracer,
	}
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

// collapseValues implements KP3's margin collapse algorithm.
// From KP3 f.java:261-274:
//   - Both positive: use maximum
//   - Both negative: use minimum (most negative)
//   - Mixed signs: add them
func collapseValues(a, b float64) float64 {
	if a >= 0 && b >= 0 {
		return max(a, b)
	} else if a <= 0 && b <= 0 {
		return min(a, b)
	}
	return a + b
}

// ptrFloat64 creates a pointer to a float64 value.
// Returns nil if the value is effectively zero (within epsilon).
func ptrFloat64(v float64) *float64 {
	const epsilon = 1e-9
	if v >= -epsilon && v <= epsilon {
		return nil // Treat near-zero as no margin
	}
	return &v
}

// marginValue returns the float64 value from a margin pointer, or 0 if nil.
func marginValue(m *float64) float64 {
	if m == nil {
		return 0
	}
	return *m
}

// captureMargins extracts margin-top and margin-bottom values from resolved styles
// and stores them in the ContentRef's MarginTop and MarginBottom fields.
// This is called after style resolution to prepare for margin collapsing.
//
// For entries with Children (wrapper blocks), this also recursively captures
// margins for child entries.
func (sb *StorylineBuilder) captureMargins() {
	if sb.styles == nil {
		return
	}

	for i := range sb.contentEntries {
		sb.captureMarginsForRef(&sb.contentEntries[i])
	}
}

// captureMarginsForRef extracts margins for a single ContentRef.
func (sb *StorylineBuilder) captureMarginsForRef(ref *ContentRef) {
	// Get the style name to look up
	styleName := ref.Style
	if styleName == "" {
		return
	}

	// Look up the style definition
	def, ok := sb.styles.Get(styleName)
	if !ok {
		return
	}

	// Resolve inheritance to get full property set (margins may be inherited)
	resolved := sb.styles.resolveInheritance(def)

	// Extract margin values from resolved properties
	ref.MarginTop = extractMarginPtr(resolved.Properties, SymMarginTop)
	ref.MarginBottom = extractMarginPtr(resolved.Properties, SymMarginBottom)

	// Check for yj-break-after: avoid (from page-break-after: avoid)
	// Elements with this property keep their margin-bottom and don't collapse with next sibling
	if isSymbol(resolved.Properties[SymYjBreakAfter], SymAvoid) {
		ref.HasBreakAfterAvoid = true
	}

	// Recursively capture margins for children in wrapper blocks
	for i := range ref.childRefs {
		sb.captureMarginsForRef(&ref.childRefs[i])
	}
}

// extractMarginPtr extracts a margin value from properties and returns a pointer.
// Returns nil if the property doesn't exist or isn't in lh units.
// Returns nil for zero values (KP3 doesn't output zero margins).
func extractMarginPtr(props map[KFXSymbol]any, sym KFXSymbol) *float64 {
	if val, ok := props[sym]; ok {
		if v, unit, ok := measureParts(val); ok && unit == SymUnitLh {
			return ptrFloat64(v) // ptrFloat64 returns nil for near-zero values
		}
	}
	return nil
}

// buildContentTree builds a ContentTree from the flat contentRefs slice.
// The tree structure mirrors the container hierarchy established by EnterContainer/ExitContainer calls.
//
// Algorithm:
// 1. Create a map of container ID -> virtual container node
// 2. For each content entry, create a content node and attach it to its container
// 3. Containers without content entries become virtual nodes (only if they have children)
// 4. Wrapper entries (with childRefs) propagate their margins to virtual container nodes
//
// The resulting tree has:
// - Root node (ContainerRoot) at the top
// - Virtual container nodes for each container ID
// - Leaf content nodes for actual content entries
func (sb *StorylineBuilder) buildContentTree() *ContentTree {
	var tracer *StyleTracer
	if sb.styles != nil {
		tracer = sb.styles.Tracer()
	}
	tree := NewContentTree(tracer)

	// Map from container ID to container node.
	// Container ID 0 is the root.
	containerNodes := make(map[int]*ContentNode)
	containerNodes[0] = tree.Root

	// First pass: collect all unique container IDs and their parent relationships.
	// We use the stored hierarchy info which tracks all containers, even those
	// without direct content (like Poem containers that only have nested Epigraph/Stanza).
	containerParents := make(map[int]int) // containerID -> parentContainerID
	containerKinds := make(map[int]ContainerKind)
	containerFlags := make(map[int]ContainerFlags)
	containerEntryOrders := make(map[int]int) // containerID -> entryOrder

	// Primary source: use the persisted container hierarchy from EnterContainer calls.
	// This correctly tracks all containers including those with no direct content.
	for id, info := range sb.containerHierarchy {
		containerParents[id] = info.parentID
		containerKinds[id] = info.kind
		containerFlags[id] = info.flags
		containerEntryOrders[id] = info.entryOrder
	}

	// Fallback: also check content entries for any containers not in the hierarchy map.
	// This handles edge cases and ensures backward compatibility.
	for i := range sb.contentEntries {
		ref := &sb.contentEntries[i]
		if ref.ContainerID != 0 {
			if _, exists := containerParents[ref.ContainerID]; !exists {
				containerParents[ref.ContainerID] = ref.ParentID
				containerKinds[ref.ContainerID] = ref.ContainerKind
				containerFlags[ref.ContainerID] = ref.ContainerFlags
			}
		}
		// Also check children for container info
		for j := range ref.childRefs {
			child := &ref.childRefs[j]
			if child.ContainerID != 0 {
				if _, exists := containerParents[child.ContainerID]; !exists {
					containerParents[child.ContainerID] = child.ParentID
					containerKinds[child.ContainerID] = child.ContainerKind
					containerFlags[child.ContainerID] = child.ContainerFlags
				}
			}
		}
	}

	// Create virtual container nodes for all containers.
	// Sort container IDs to ensure parent containers are created before children.
	// (Container IDs are assigned sequentially, so lower IDs are parents of higher IDs)
	sortedContainerIDs := make([]int, 0, len(containerParents))
	for id := range containerParents {
		sortedContainerIDs = append(sortedContainerIDs, id)
	}
	// Simple insertion sort (small number of containers)
	for i := 1; i < len(sortedContainerIDs); i++ {
		for j := i; j > 0 && sortedContainerIDs[j] < sortedContainerIDs[j-1]; j-- {
			sortedContainerIDs[j], sortedContainerIDs[j-1] = sortedContainerIDs[j-1], sortedContainerIDs[j]
		}
	}

	for _, containerID := range sortedContainerIDs {
		parentID := containerParents[containerID]

		// Ensure parent container node exists
		parentNode, ok := containerNodes[parentID]
		if !ok {
			// This shouldn't happen if container IDs are properly nested
			parentNode = tree.Root
		}

		// Create virtual container node
		containerNode := &ContentNode{
			Index:          -1, // Virtual node (no ContentRef)
			ContainerKind:  containerKinds[containerID],
			ContainerFlags: containerFlags[containerID],
			Parent:         parentNode,
			EntryOrder:     containerEntryOrders[containerID],
		}

		// Set container margins from the stored map (set via SetContainerMargins)
		if mt, mb := sb.GetContainerMargins(containerID); mt != 0 || mb != 0 {
			containerNode.MarginTop = ptrFloat64(mt)
			containerNode.MarginBottom = ptrFloat64(mb)
		}

		containerNodes[containerID] = containerNode
		parentNode.Children = append(parentNode.Children, containerNode)
	}

	// Second pass: create content nodes and attach them to their containers.
	// Also propagate wrapper entry margins to their virtual containers.
	for i := range sb.contentEntries {
		ref := &sb.contentEntries[i]

		// Handle wrapper entries (those with childRefs) - these represent container wrappers
		// The wrapper's CSS margins should be on the virtual container
		if len(ref.childRefs) > 0 {
			// Find the container that the children belong to
			if len(ref.childRefs) > 0 {
				childContainerID := ref.childRefs[0].ContainerID
				if containerNode, ok := containerNodes[childContainerID]; ok {
					// Propagate wrapper's margins to the virtual container
					containerNode.MarginTop = ref.MarginTop
					containerNode.MarginBottom = ref.MarginBottom
					// Track the mapping from wrapper entry index to container node
					// so we can update the wrapper's style after collapsing
					tree.WrapperMap[i] = containerNode
				}
			}

			// Process children inside the wrapper
			for j := range ref.childRefs {
				child := &ref.childRefs[j]
				sb.addContentNodeToTree(tree, containerNodes, child, i, j)
			}
			continue
		}

		// Regular content entry (not a wrapper)
		sb.addContentNodeToTree(tree, containerNodes, ref, i, -1)
	}

	// Sort children of all nodes by EntryOrder.
	// This ensures correct sibling ordering when containers and content entries
	// are interleaved (e.g., subtitle before poem container).
	sortChildrenByEntryOrder(tree.Root)

	return tree
}

// addContentNodeToTree creates a content node for a ContentRef and adds it to the tree.
// parentIndex is the index in contentEntries, childIndex is the index in childRefs (-1 for direct entries).
func (sb *StorylineBuilder) addContentNodeToTree(tree *ContentTree, containerNodes map[int]*ContentNode, ref *ContentRef, parentIndex, childIndex int) {
	// Determine content type string for the node
	contentType := "text"
	if ref.Type == SymImage {
		contentType = "image"
	}

	// Create content node
	contentNode := &ContentNode{
		Index:                 parentIndex,
		EID:                   ref.EID,
		ContentType:           contentType,
		Style:                 ref.Style,
		MarginTop:             ref.MarginTop,
		MarginBottom:          ref.MarginBottom,
		HasBreakAfterAvoid:    ref.HasBreakAfterAvoid,
		StripMarginBottom:     ref.StripMarginBottom,
		EmptyLineMarginBottom: ref.EmptyLineMarginBottom,
		EmptyLineMarginTop:    ref.EmptyLineMarginTop,
		IsFloatImage:          ref.IsFloatImage,
		EntryOrder:            ref.EntryOrder,
	}

	// For child refs, store both parent and child index for later lookup
	if childIndex >= 0 {
		// Store a composite index that encodes both parent and child position
		// We use negative numbers for child indices: -(parentIndex*1000 + childIndex + 2)
		// The +2 offset ensures the minimum value is -2, avoiding collision with Index=-1
		// which is reserved for virtual container nodes.
		contentNode.Index = -(parentIndex*1000 + childIndex + 2)
	}

	// Find the container node
	containerNode, ok := containerNodes[ref.ContainerID]
	if !ok {
		containerNode = tree.Root
	}

	// Attach content node to container
	contentNode.Parent = containerNode
	containerNode.Children = append(containerNode.Children, contentNode)

	// Add to node map for quick lookup
	tree.NodeMap[contentNode.Index] = contentNode
}

// sortChildrenByEntryOrder recursively sorts children of all nodes by their EntryOrder.
// This ensures correct sibling ordering when containers and content entries
// are interleaved (e.g., subtitle element before poem container).
func sortChildrenByEntryOrder(node *ContentNode) {
	if len(node.Children) > 1 {
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].EntryOrder < node.Children[j].EntryOrder
		})
	}
	// Recursively sort children's children
	for _, child := range node.Children {
		sortChildrenByEntryOrder(child)
	}
}

// CollapseMargins applies CSS margin collapsing as a post-processing step.
// This is called after all content is generated and margins are captured.
//
// KP3's margin collapser runs 4 phases for each container (depth-first, bottom-up):
//
//	Phase 1: Self-collapse for empty nodes
//	Phase 2: First child ↔ Container margin-top
//	Phase 3: Last child ↔ Container margin-bottom
//	Phase 4: Adjacent sibling collapsing
func (sb *StorylineBuilder) CollapseMargins() *ContentTree {
	tree := sb.buildContentTree()
	tree.collapseNode(tree.Root)
	return tree
}

// collapseNode recursively processes a container node and its descendants.
// Process order (based on CSS margin collapsing semantics):
// 1. Empty node collapsing (self-collapse)
// 2. Sibling collapsing at this level (transfers mb from one child to next's mt)
// 3. First-child collapsing (transfers node's mt DOWN to first child)
// 4. Recurse into children (they now have correct mt from steps 2-3)
// 5. Last-child collapsing (pulls up last child's mb to this node)
//
// This order ensures:
// - Sibling margins are transferred before recursion (step 2)
// - Container margins propagate DOWN to children before children are processed (step 3)
// - Children's final mb values bubble UP after they're fully processed (step 5)
func (t *ContentTree) collapseNode(node *ContentNode) {
	t.collapseNodeWithContext(node, false)
}

// collapseNodeWithContext is the internal version that tracks whether this node
// is the last child of its parent. This is needed for FlagTransferMBToLastChild
// containers to decide whether to transfer mb to last child or let it bubble up.
func (t *ContentTree) collapseNodeWithContext(node *ContentNode, isLastChildOfParent bool) {
	if len(node.Children) == 0 {
		return
	}

	// Step 0: Strip margin-bottom for nodes marked for stripping (due to empty-line following)
	// This must happen FIRST before any collapsing to ensure the mb is removed.
	t.stripMarkedMarginBottom(node)

	// Step 1: Empty node collapsing
	t.collapseEmptyNodes(node)

	// Step 2: Sibling collapsing - transfer mb from each child to next sibling's mt
	// This must happen BEFORE first-child collapsing so containers receive sibling margins
	t.collapseSiblings(node)

	// Step 3: First-child collapsing - transfer this node's mt to first child
	// This must happen BEFORE recursion so children have the correct mt
	t.collapseFirstChild(node)

	// Step 4: Recurse into children
	// Children now have correct mt values from steps 2-3
	// Pass isLastChildOfParent context for each child
	for i, child := range node.Children {
		childIsLast := (i == len(node.Children)-1)
		t.collapseNodeWithContext(child, childIsLast)
	}

	// Step 5: Strip middle margin-bottom (for containers like stanza)
	// This must happen AFTER recursion so nested containers have their mb accumulated
	t.stripMiddleMarginBottom(node)

	// Step 6: Last-child collapsing - pull up last child's mb to this node
	// This must happen AFTER recursion so children's mb values are final
	t.collapseLastChildWithContext(node, isLastChildOfParent)
}

// collapseEmptyNodes handles Phase 1: self-collapse for empty nodes.
// If a node is empty (no content), its margin-top and margin-bottom collapse together.
func (t *ContentTree) collapseEmptyNodes(container *ContentNode) {
	for _, child := range container.Children {
		if child.IsEmpty() {
			// Empty node: collapse its own margins
			mt := marginValue(child.MarginTop)
			mb := marginValue(child.MarginBottom)
			if mt != 0 || mb != 0 {
				// Capture before values for tracing
				beforeMT, beforeMB := child.MarginTop, child.MarginBottom

				collapsed := collapseValues(mt, mb)
				child.MarginTop = nil
				child.MarginBottom = ptrFloat64(collapsed)

				// Trace the collapse operation
				if t.tracer != nil && t.tracer.IsEnabled() {
					t.tracer.TraceMarginCollapse("empty", child.TraceID(),
						beforeMT, beforeMB, child.MarginTop, child.MarginBottom,
						container.ContainerKind.String())
				}
			}
		}
	}
}

// stripMarkedMarginBottom removes margin-bottom from nodes marked with StripMarginBottom,
// and applies EmptyLineMarginBottom where set.
//
// This is called when an empty-line follows an element, matching KP3 behavior where the
// preceding element loses its mb and the empty-line's margin goes to the next element's mt.
//
// Additionally, if the stripped margin-bottom is larger than the next sibling's margin-top,
// the next sibling's margin-top is increased to match. This matches KP3 behavior where
// an empty-line after a subtitle (mb=0.833lh) gives the following paragraph mt=0.833lh
// instead of the default empty-line margin (0.5lh).
//
// When EmptyLineMarginBottom is set (for empty-line followed by image), it replaces
// the element's margin-bottom with the empty-line margin value.
//
// When EmptyLineMarginTop is set (for empty-line followed by text), it applies
// the empty-line margin to the element's margin-top using max(emptyline_mt, existing_mt).
// This margin is NOT scaled by font-size since it's applied here, not via CSS style.
func (t *ContentTree) stripMarkedMarginBottom(container *ContentNode) {
	children := container.Children
	for i, child := range children {
		// Handle EmptyLineMarginTop: when empty-line is followed by text,
		// apply the empty-line margin as margin-top using max(emptyline_mt, existing_mt).
		// This margin is stored separately to avoid font-size scaling.
		if child.EmptyLineMarginTop != nil {
			emptyLineMT := *child.EmptyLineMarginTop
			existingMT := 0.0
			if child.MarginTop != nil {
				existingMT = *child.MarginTop
			}
			// Use the larger of the two margins
			if emptyLineMT > existingMT {
				if t.tracer != nil && t.tracer.IsEnabled() {
					beforeMT, beforeMB := child.MarginTop, child.MarginBottom
					child.MarginTop = ptrFloat64(emptyLineMT)
					t.tracer.TraceMarginCollapse("emptyline-mt", child.TraceID(),
						beforeMT, beforeMB, child.MarginTop, child.MarginBottom,
						container.ContainerKind.String())
				} else {
					child.MarginTop = ptrFloat64(emptyLineMT)
				}
			}
			// Continue to check for other flags (StripMarginBottom, etc.)
		}

		// Handle EmptyLineMarginBottom: when empty-line is followed by image,
		// KP3 puts the empty-line margin on the PREVIOUS element as margin-bottom.
		if child.EmptyLineMarginBottom != nil {
			if t.tracer != nil && t.tracer.IsEnabled() {
				beforeMT, beforeMB := child.MarginTop, child.MarginBottom
				child.MarginBottom = child.EmptyLineMarginBottom
				t.tracer.TraceMarginCollapse("emptyline-mb-before-image", child.TraceID(),
					beforeMT, beforeMB, child.MarginTop, child.MarginBottom,
					container.ContainerKind.String())
			} else {
				child.MarginBottom = child.EmptyLineMarginBottom
			}
			// Don't also strip - EmptyLineMarginBottom takes precedence
			continue
		}

		// Float images have fixed margins that don't participate in collapsing.
		// Skip stripping mb from float images - they keep their 2.6lh margins.
		if child.StripMarginBottom && child.MarginBottom != nil && !child.IsFloatImage {
			strippedMB := *child.MarginBottom

			// Trace the strip operation before clearing
			if t.tracer != nil && t.tracer.IsEnabled() {
				beforeMT, beforeMB := child.MarginTop, child.MarginBottom
				child.MarginBottom = nil
				t.tracer.TraceMarginCollapse("strip-emptyline-prev", child.TraceID(),
					beforeMT, beforeMB, child.MarginTop, child.MarginBottom,
					container.ContainerKind.String())
			} else {
				child.MarginBottom = nil
			}

			// Transfer stripped margin to next sibling if it's larger than next's margin-top.
			// This matches KP3 behavior: empty-line after subtitle gives next element
			// max(emptyline_margin, subtitle_mb).
			if i+1 < len(children) {
				next := children[i+1]
				nextMT := 0.0
				if next.MarginTop != nil {
					nextMT = *next.MarginTop
				}
				if strippedMB > nextMT {
					if t.tracer != nil && t.tracer.IsEnabled() {
						beforeMT, beforeMB := next.MarginTop, next.MarginBottom
						next.MarginTop = ptrFloat64(strippedMB)
						t.tracer.TraceMarginCollapse("transfer-stripped-mb", next.TraceID(),
							beforeMT, beforeMB, next.MarginTop, next.MarginBottom,
							container.ContainerKind.String())
					} else {
						next.MarginTop = ptrFloat64(strippedMB)
					}
				}
			}
		}
	}
}

// collapseFirstChild handles Phase 2: first child ↔ container margin-top.
// The first child's margin-top collapses with the container's margin-top.
// The collapsed value goes to the CONTAINER (parent), child's margin-top is REMOVED.
//
// Special cases:
// - If container has FlagTitleBlockMode, first child simply LOSES margin-top
// - If container has FlagPreventCollapseTop (or border/padding-top), no collapsing
// - For virtual containers (Index=-1) with title-block mode: container KEEPS its mt
// - For virtual containers without title-block mode: transfer mt to first child
func (t *ContentTree) collapseFirstChild(container *ContentNode) {
	if len(container.Children) == 0 {
		return
	}
	if container.ContainerFlags.PreventsCollapseTop() {
		return
	}

	first := container.Children[0]
	beforeMT, beforeMB := first.MarginTop, first.MarginBottom

	// For virtual containers: handle based on title-block mode
	if container.Index == -1 {
		if container.ContainerFlags.IsTitleBlockMode() {
			// Title-block mode: container KEEPS its mt (renders on wrapper entry)
			// First child loses its mt (title-block spacing via mt on following elements)
			first.MarginTop = nil
			t.traceFirstChild(first, beforeMT, beforeMB, container)
			return
		}
		// Non-title-block virtual container: transfer mt to first child
		if container.MarginTop != nil {
			if first.MarginTop != nil {
				collapsed := collapseValues(*container.MarginTop, *first.MarginTop)
				first.MarginTop = ptrFloat64(collapsed)
			} else {
				first.MarginTop = container.MarginTop
			}
			container.MarginTop = nil
			t.traceFirstChild(first, beforeMT, beforeMB, container)
		}
		// Don't take margins FROM first child for virtual containers
		return
	}

	// Title-block mode: first child LOSES margin-top (spacing via margin-top on following elements)
	if container.ContainerFlags.IsTitleBlockMode() {
		first.MarginTop = nil
		t.traceFirstChild(first, beforeMT, beforeMB, container)
		return
	}

	// Standard mode: collapse first child's margin-top with container's margin-top
	if container.MarginTop != nil && first.MarginTop != nil {
		collapsed := collapseValues(*container.MarginTop, *first.MarginTop)
		container.MarginTop = ptrFloat64(collapsed)
		first.MarginTop = nil
		t.traceFirstChild(first, beforeMT, beforeMB, container)
	} else if first.MarginTop != nil {
		// Transfer child's margin to container
		container.MarginTop = first.MarginTop
		first.MarginTop = nil
		t.traceFirstChild(first, beforeMT, beforeMB, container)
	}
}

// traceFirstChild logs first-child collapse operation if tracer is enabled.
func (t *ContentTree) traceFirstChild(first *ContentNode, beforeMT, beforeMB *float64, container *ContentNode) {
	if t.tracer != nil && t.tracer.IsEnabled() {
		t.tracer.TraceMarginCollapse("first-child", first.TraceID(),
			beforeMT, beforeMB, first.MarginTop, first.MarginBottom,
			container.ContainerKind.String())
	}
}

// collapseLastChildWithContext handles Phase 3: last child ↔ container margin-bottom.
// The last child's margin-bottom collapses with the container's margin-bottom.
// The collapsed value goes to the CONTAINER (parent), child's margin-bottom is REMOVED.
//
// Special cases:
//   - If container has FlagTitleBlockMode, last child GETS container's margin-bottom
//   - If container has FlagPreventCollapseBottom (or border/padding-bottom), no collapsing
//   - If container is virtual (Index=-1), we still remove the last child's mb but
//     transfer it to the virtual container's mb (for sibling collapsing later)
//   - If container has FlagTransferMBToLastChild AND is NOT the last child of its parent,
//     container's mb goes TO last child (e.g., stanza's mb goes to last verse)
//
// The isLastChildOfParent parameter indicates whether the container itself is the last
// child of its parent. This affects FlagTransferMBToLastChild behavior: if the container
// is the last child, its mb should bubble up to the parent rather than staying on the
// last child (so it can collapse with whatever comes after the parent).
func (t *ContentTree) collapseLastChildWithContext(container *ContentNode, isLastChildOfParent bool) {
	if len(container.Children) == 0 {
		return
	}
	if container.ContainerFlags.PreventsCollapseBottom() {
		return
	}

	last := container.Children[len(container.Children)-1]
	beforeMT, beforeMB := last.MarginTop, last.MarginBottom

	// Title-block mode: container KEEPS its margin-bottom for sibling collapsing
	// (will be transferred to next sibling's margin-top in collapseSiblings phase)
	// The last child doesn't get the container's mb because title-blocks use
	// margin-top based spacing, not margin-bottom.
	if container.ContainerFlags.IsTitleBlockMode() {
		// Don't transfer mb to last child - keep it for sibling collapsing
		return
	}

	// Force-transfer-to-last-child mode: container's mb ALWAYS goes TO last child
	// regardless of position. Used for annotation containers where we want the
	// margin-bottom on the last paragraph even when it's the last element in the storyline.
	if container.ContainerFlags.ForceTransfersMBToLastChild() {
		if container.MarginBottom != nil {
			if last.MarginBottom != nil {
				// Collapse container's mb with last child's mb
				collapsed := collapseValues(*container.MarginBottom, *last.MarginBottom)
				last.MarginBottom = ptrFloat64(collapsed)
			} else {
				// Transfer container's mb to last child
				last.MarginBottom = container.MarginBottom
			}
			container.MarginBottom = nil
			t.traceLastChild(last, beforeMT, beforeMB, container)
		}
		return
	}

	// Transfer-to-last-child mode: container's mb goes TO last child
	// Used for stanzas where the last verse should carry the accumulated margin.
	// BUT: only if this container has siblings after it (is NOT last child of parent).
	// If this container IS the last child, let the mb bubble up to parent for
	// proper collapsing with whatever comes after the parent.
	if container.ContainerFlags.TransfersMBToLastChild() && !isLastChildOfParent {
		if container.MarginBottom != nil {
			if last.MarginBottom != nil {
				// Collapse container's mb with last child's mb
				collapsed := collapseValues(*container.MarginBottom, *last.MarginBottom)
				last.MarginBottom = ptrFloat64(collapsed)
			} else {
				// Transfer container's mb to last child
				last.MarginBottom = container.MarginBottom
			}
			container.MarginBottom = nil
			t.traceLastChild(last, beforeMT, beforeMB, container)
		}
		return
	}

	// Standard mode: collapse last child's margin-bottom with container's margin-bottom
	// This applies to both regular containers (Index >= 0) and virtual containers (Index == -1)
	// For virtual containers, we accumulate the margin-bottom on the container for later
	// sibling collapsing (virtual containers pass their mb to the next sibling)
	//
	// Special case: if the container is ROOT, don't transfer any child's mb to root.
	// Root is never rendered, so the margin would be lost. Instead, let the child
	// keep its mb so it renders on the actual element.
	if container.ContainerKind == ContainerRoot {
		// Don't transfer any child's mb to root - it would be lost
		return
	}
	// Special case: virtual containers (Index == -1) that are the last child of ROOT
	// should not receive the last child's mb. Virtual containers don't have corresponding
	// content entries, so the margin would be lost (only wrapper entries in WrapperMap
	// get their margins applied). When a virtual container is at the end of the storyline
	// (last child of root), the last content element should keep its mb so it renders.
	//
	// This matches KP3 behavior where the last element in a storyline keeps its margin-bottom.
	if container.Index == -1 && isLastChildOfParent && container.Parent != nil && container.Parent.ContainerKind == ContainerRoot {
		// Virtual container at storyline end - last child keeps its mb
		return
	}
	// Float images have fixed margins (2.6lh) that don't participate in collapsing.
	// They keep their margins and act as barriers in the margin flow.
	if last.IsFloatImage {
		return
	}
	if container.MarginBottom != nil && last.MarginBottom != nil {
		collapsed := collapseValues(*container.MarginBottom, *last.MarginBottom)
		container.MarginBottom = ptrFloat64(collapsed)
		last.MarginBottom = nil
		t.traceLastChild(last, beforeMT, beforeMB, container)
	} else if last.MarginBottom != nil {
		// Transfer child's margin to container
		container.MarginBottom = last.MarginBottom
		last.MarginBottom = nil
		t.traceLastChild(last, beforeMT, beforeMB, container)
	}
}

// traceLastChild logs last-child collapse operation if tracer is enabled.
func (t *ContentTree) traceLastChild(last *ContentNode, beforeMT, beforeMB *float64, container *ContentNode) {
	if t.tracer != nil && t.tracer.IsEnabled() {
		t.tracer.TraceMarginCollapse("last-child", last.TraceID(),
			beforeMT, beforeMB, last.MarginTop, last.MarginBottom,
			container.ContainerKind.String())
	}
}

// stripMiddleMarginBottom removes margin-bottom from all non-last children.
// This is used for containers like stanzas where KP3 spacing between elements
// comes entirely from margin-top, not margin-bottom.
//
// Only applies to containers with FlagStripMiddleMarginBottom set.
// The last child keeps its margin-bottom (which may be modified by last-child
// collapsing in the next phase).
func (t *ContentTree) stripMiddleMarginBottom(container *ContentNode) {
	if !container.ContainerFlags.StripsMiddleMarginBottom() {
		return
	}

	children := container.Children
	if len(children) <= 1 {
		return // No middle children
	}

	// Strip margin-bottom from all but the last child
	for i := 0; i < len(children)-1; i++ {
		child := children[i]
		if child.MarginBottom != nil {
			beforeMT, beforeMB := child.MarginTop, child.MarginBottom
			child.MarginBottom = nil
			// Trace the strip operation
			if t.tracer != nil && t.tracer.IsEnabled() {
				t.tracer.TraceMarginCollapse("strip-middle-mb", child.TraceID(),
					beforeMT, beforeMB, child.MarginTop, child.MarginBottom,
					container.ContainerKind.String())
			}
		}
	}
}

// collapseSiblings handles Phase 4: adjacent sibling margin collapsing.
//
// KP3 behavior based on actual output analysis:
//
// For virtual container nodes (Index == -1):
//   - Container's margin-bottom ALWAYS transfers to next sibling's margin-top
//   - This is because containers don't render directly; their margins go to content
//
// For content nodes:
//   - KP3 does NOT transfer margin-bottom to the next sibling's margin-top
//   - Content elements keep their own margin-bottom
//   - The visual spacing between siblings is the SUM of mb + mt (no CSS collapsing)
//   - This matches the doMarginCollapse=false behavior seen in KP3's adapter code
//
// Exception: Elements with page-break-after: avoid followed by a container
// still transfer their margin-bottom into the container's first child.
func (t *ContentTree) collapseSiblings(container *ContentNode) {
	children := container.Children

	for i := 0; i < len(children)-1; i++ {
		curr := children[i]
		next := children[i+1]
		nextBeforeMT, nextBeforeMB := next.MarginTop, next.MarginBottom

		// Only process virtual containers (wrappers) or break-after-avoid elements
		// Content nodes do NOT transfer margins to siblings (except special cases below)
		if !curr.IsContainer() {
			// For content nodes with break-after-avoid AND margin-bottom, AND next sibling
			// is a container: transfer the margin to the container's first child.
			//
			// KP3 behavior: when an element has break-after: avoid AND margin-bottom,
			// AND is followed by a container (poem, epigraph, etc.), the current element's
			// margin-bottom is stripped and transferred to the container's margin-top.
			// The container's first-child collapsing will then propagate it down to the
			// first rendered content.
			//
			// This does NOT apply when the next sibling is a regular content node -
			// in that case, both elements keep their own margins.
			//
			// If the element has break-after: avoid but NO margin-bottom, no transfer happens.
			if curr.HasBreakAfterAvoid && curr.MarginBottom != nil && next.IsContainer() {
				currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
				// Collapse curr's mb with next container's mt (if any) and put result on container
				if next.MarginTop != nil {
					collapsed := collapseValues(*curr.MarginBottom, *next.MarginTop)
					next.MarginTop = ptrFloat64(collapsed)
				} else {
					// Transfer curr's mb to container's mt
					next.MarginTop = curr.MarginBottom
				}
				curr.MarginBottom = nil
				// Trace the transfer on curr (showing mb removal)
				if t.tracer != nil && t.tracer.IsEnabled() {
					t.tracer.TraceMarginCollapse("break-after-avoid-to-container", curr.TraceID(),
						currBeforeMT, currBeforeMB, curr.MarginTop, curr.MarginBottom,
						container.ContainerKind.String())
				}
				t.traceSibling(next, nextBeforeMT, nextBeforeMB, container)
				continue
			}

			// CSS sibling margin collapsing between content node and following container:
			// When a content node is followed by a container (virtual node), we should
			// collapse curr's mb with container's mt using max(), and put the result on
			// the container. The container's first-child collapsing will then propagate
			// the margin to its first child.
			//
			// This is different from content → content collapsing because containers
			// don't render directly - their margins go to their children.
			if curr.MarginBottom != nil && next.IsContainer() {
				currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
				// Collapse curr's mb with container's mt (if any)
				if next.MarginTop != nil {
					collapsed := collapseValues(*curr.MarginBottom, *next.MarginTop)
					next.MarginTop = ptrFloat64(collapsed)
				} else {
					// Transfer curr's mb to container's mt
					next.MarginTop = curr.MarginBottom
				}
				curr.MarginBottom = nil
				if t.tracer != nil && t.tracer.IsEnabled() {
					t.tracer.TraceMarginCollapse("content-to-container-sibling", curr.TraceID(),
						currBeforeMT, currBeforeMB, curr.MarginTop, curr.MarginBottom,
						container.ContainerKind.String())
				}
				t.traceSibling(next, nextBeforeMT, nextBeforeMB, container)
				continue
			}

			// CSS sibling margin collapsing between content nodes:
			// When curr has mb and next has mt, and next's mt >= curr's mb,
			// remove curr's mb (it's absorbed into next's larger mt).
			// This matches standard CSS behavior where adjacent margins collapse to the larger value.
			//
			// KP3 behavior: content elements don't transfer margins to siblings, but when the
			// next sibling has a margin-top that's >= the current element's margin-bottom,
			// the current element's margin-bottom is removed to avoid double-spacing.
			// The visual spacing between elements comes from the larger margin (next's mt).
			//
			// Exception: Float images (full-width standalone images) act as barriers:
			// - A float image's mt does NOT absorb the previous element's mb
			// - The previous element keeps its mb and visual spacing is the SUM of both margins
			if curr.MarginBottom != nil && next.MarginTop != nil && !next.IsFloatImage {
				currMB := *curr.MarginBottom
				nextMT := *next.MarginTop
				if nextMT >= currMB {
					// Next's mt absorbs curr's mb - remove curr's mb
					currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
					curr.MarginBottom = nil
					if t.tracer != nil && t.tracer.IsEnabled() {
						t.tracer.TraceMarginCollapse("sibling-absorb-mb", curr.TraceID(),
							currBeforeMT, currBeforeMB, curr.MarginTop, curr.MarginBottom,
							container.ContainerKind.String())
					}
				}
			}
			// Content nodes keep their margins otherwise
			continue
		}

		// Virtual container (wrapper) nodes: handle margin collapsing with next sibling
		// Containers don't render directly; their margins go to content.
		//
		// For containers with FlagTransferMBToLastChild:
		// - If next sibling's mt >= container's mb, the sibling absorbs it (remove container's mb)
		// - Additionally, if next sibling's mt >= last child's mb, absorb that too
		//   This implements CSS "through-the-container" collapsing where the last child's mb
		//   collapses with whatever comes after the container.
		// - Otherwise, the container keeps its mb to transfer to last child later
		//
		// For containers without the flag:
		// - Transfer container's mb to next sibling's mt (standard container collapsing)
		if curr.ContainerFlags.TransfersMBToLastChild() {
			// Check if next sibling's mt can absorb the container's mb
			if curr.MarginBottom != nil && next.MarginTop != nil {
				currMB := *curr.MarginBottom
				nextMT := *next.MarginTop
				if nextMT >= currMB {
					// Next sibling's mt absorbs container's mb - remove container's mb
					// This prevents the mb from being transferred to last child
					currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
					curr.MarginBottom = nil
					if t.tracer != nil && t.tracer.IsEnabled() {
						t.tracer.TraceMarginCollapse("container-sibling-absorb-mb", curr.TraceID(),
							currBeforeMT, currBeforeMB, curr.MarginTop, curr.MarginBottom,
							container.ContainerKind.String())
					}
				}
			}
			// Also check if the last child's mb should be absorbed by next sibling's mt
			// This implements CSS "through-the-container" collapsing
			if len(curr.Children) > 0 && next.MarginTop != nil {
				lastChild := curr.Children[len(curr.Children)-1]
				if lastChild.MarginBottom != nil {
					lastMB := *lastChild.MarginBottom
					nextMT := *next.MarginTop
					if nextMT >= lastMB {
						// Next sibling's mt absorbs last child's mb
						beforeMT, beforeMB := lastChild.MarginTop, lastChild.MarginBottom
						lastChild.MarginBottom = nil
						if t.tracer != nil && t.tracer.IsEnabled() {
							t.tracer.TraceMarginCollapse("through-container-absorb-mb", lastChild.TraceID(),
								beforeMT, beforeMB, lastChild.MarginTop, lastChild.MarginBottom,
								container.ContainerKind.String())
						}
					}
				}
			}
			continue
		}
		if curr.MarginBottom != nil {
			if next.MarginTop != nil {
				// Both have margins: collapse with max()
				collapsed := collapseValues(*curr.MarginBottom, *next.MarginTop)
				next.MarginTop = ptrFloat64(collapsed)
			} else {
				// Only curr has margin-bottom: transfer to next.mt
				next.MarginTop = curr.MarginBottom
			}
			curr.MarginBottom = nil
			t.traceSibling(next, nextBeforeMT, nextBeforeMB, container)
		}
	}
}

// traceSibling logs sibling collapse operation if tracer is enabled.
func (t *ContentTree) traceSibling(next *ContentNode, beforeMT, beforeMB *float64, container *ContentNode) {
	if t.tracer != nil && t.tracer.IsEnabled() {
		t.tracer.TraceMarginCollapse("sibling", next.TraceID(),
			beforeMT, beforeMB, next.MarginTop, next.MarginBottom,
			container.ContainerKind.String())
	}
}

// applyCollapsedMargins updates content entries with collapsed margin values.
// This creates new style variants as needed (via deduplication in StyleRegistry).
//
// For each content node that has modified margins (compared to the original style),
// a new style is registered with the updated margin values. The content entry's
// Style field is updated to reference this new style.
//
// This also handles wrapper entries - when a virtual container's margins change,
// the corresponding wrapper entry's style is updated.
func (sb *StorylineBuilder) applyCollapsedMargins(tree *ContentTree) {
	if sb.styles == nil {
		return
	}

	tracer := sb.styles.Tracer()

	// First, update wrapper entries based on their virtual container's final margins
	for wrapperIndex, containerNode := range tree.WrapperMap {
		ref := &sb.contentEntries[wrapperIndex]
		if ref.Style == "" {
			continue
		}

		// Get original style's properties
		def, ok := sb.styles.Get(ref.Style)
		if !ok {
			continue
		}

		// Resolve inheritance to get full property set
		resolved := sb.styles.resolveInheritance(def)

		// Check if margins need to be modified
		originalMT := extractMarginPtr(resolved.Properties, SymMarginTop)
		originalMB := extractMarginPtr(resolved.Properties, SymMarginBottom)

		// Compare with collapsed values from the virtual container
		mtChanged := !marginsEqual(originalMT, containerNode.MarginTop)
		mbChanged := !marginsEqual(originalMB, containerNode.MarginBottom)

		if !mtChanged && !mbChanged {
			continue // No changes needed
		}

		// Make a copy of properties and apply collapsed margins
		props := make(map[KFXSymbol]any, len(resolved.Properties))
		for k, v := range resolved.Properties {
			props[k] = v
		}

		// Apply collapsed margins from the virtual container
		if containerNode.MarginTop == nil || *containerNode.MarginTop == 0 {
			delete(props, SymMarginTop)
		} else {
			props[SymMarginTop] = DimensionValue(*containerNode.MarginTop, SymUnitLh)
		}

		if containerNode.MarginBottom == nil || *containerNode.MarginBottom == 0 {
			delete(props, SymMarginBottom)
		} else {
			props[SymMarginBottom] = DimensionValue(*containerNode.MarginBottom, SymUnitLh)
		}

		// Register the new style, preserving the original style's usage type
		originalStyle := ref.Style
		originalUsage := sb.styles.GetUsage(originalStyle)
		newStyle := sb.styles.RegisterResolvedRaw(props, originalUsage, true)
		ref.Style = newStyle

		// Trace the style variant creation
		if tracer != nil && tracer.IsEnabled() {
			tracer.TraceStyleVariant(originalStyle, newStyle,
				fmt.Sprintf("wrapper[%d]", wrapperIndex),
				containerNode.MarginTop, containerNode.MarginBottom)
		}

		// Also update RawEntry if present (wrapper entries use RawEntry for serialization)
		if ref.RawEntry != nil {
			ref.RawEntry = ref.RawEntry.Set(SymStyle, SymbolByName(newStyle))
		}
	}

	// Then update regular content nodes
	for _, node := range tree.AllContentNodes() {
		// Get the ContentRef for this node
		ref := sb.getContentRefForNode(node)
		if ref == nil || ref.Style == "" {
			continue // No style to modify
		}

		// Get original style's properties
		def, ok := sb.styles.Get(ref.Style)
		if !ok {
			continue
		}

		// Resolve inheritance to get full property set
		resolved := sb.styles.resolveInheritance(def)

		// Check if margins need to be modified
		originalMT := extractMarginPtr(resolved.Properties, SymMarginTop)
		originalMB := extractMarginPtr(resolved.Properties, SymMarginBottom)

		// Compare with collapsed values
		mtChanged := !marginsEqual(originalMT, node.MarginTop)
		mbChanged := !marginsEqual(originalMB, node.MarginBottom)

		if !mtChanged && !mbChanged {
			continue // No changes needed
		}

		// Make a copy of properties and apply collapsed margins
		props := make(map[KFXSymbol]any, len(resolved.Properties))
		for k, v := range resolved.Properties {
			props[k] = v
		}

		// Apply collapsed margins
		if node.MarginTop == nil || *node.MarginTop == 0 {
			delete(props, SymMarginTop)
		} else {
			props[SymMarginTop] = DimensionValue(*node.MarginTop, SymUnitLh)
		}

		if node.MarginBottom == nil || *node.MarginBottom == 0 {
			delete(props, SymMarginBottom)
		} else {
			props[SymMarginBottom] = DimensionValue(*node.MarginBottom, SymUnitLh)
		}

		// Register the new style, preserving the original style's usage type
		originalStyle := ref.Style
		originalUsage := sb.styles.GetUsage(originalStyle)
		newStyle := sb.styles.RegisterResolvedRaw(props, originalUsage, true)

		// Trace the style variant creation
		if tracer != nil && tracer.IsEnabled() {
			tracer.TraceStyleVariant(originalStyle, newStyle, node.TraceID(),
				node.MarginTop, node.MarginBottom)
		}

		// Update content entry
		ref.Style = newStyle

		// Also update RawEntry if present (for mixed content entries)
		if ref.RawEntry != nil {
			ref.RawEntry = ref.RawEntry.Set(SymStyle, SymbolByName(newStyle))
		}
	}
}

// getContentRefForNode returns the ContentRef for a ContentNode.
// For direct entries (Index >= 0), returns from contentEntries.
// For child refs (Index < -1), decodes the composite index and returns from childRefs.
// Returns nil for virtual container nodes (Index == -1).
func (sb *StorylineBuilder) getContentRefForNode(node *ContentNode) *ContentRef {
	if node.Index == -1 {
		return nil // Virtual container node
	}
	if node.Index >= 0 {
		return &sb.contentEntries[node.Index]
	}
	// Negative composite index: -(parentIndex*1000 + childIndex + 2)
	// The +2 offset avoids collision with Index=-1 for virtual containers.
	composite := -node.Index - 2
	parentIndex := composite / 1000
	childIndex := composite % 1000
	if parentIndex < len(sb.contentEntries) {
		parent := &sb.contentEntries[parentIndex]
		if childIndex < len(parent.childRefs) {
			return &parent.childRefs[childIndex]
		}
	}
	return nil
}

// marginsEqual compares two margin pointers for equality.
// Two nil pointers are equal, and two non-nil pointers are equal if their values are equal.
func marginsEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// Use epsilon comparison for floating point
	const epsilon = 1e-9
	diff := *a - *b
	return diff >= -epsilon && diff <= epsilon
}
