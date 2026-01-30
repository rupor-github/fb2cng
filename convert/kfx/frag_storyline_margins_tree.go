package kfx

import (
	"sort"
)

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
