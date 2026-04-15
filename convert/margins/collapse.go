package margins

// This file implements CSS margin collapsing as a post-processing step.
//
// The collapser runs multiple phases for each container:
//   Phase 0: Strip margin-bottom for empty-line predecessors
//   Phase 1: Self-collapse for empty nodes
//   Phase 2: Adjacent sibling collapsing (pre-pass)
//   Phase 3: First child ↔ Parent margin-top collapsing
//   Phase 4: Recurse into children
//   Phase 5: Strip middle margin-bottom (stanza-like containers)
//   Phase 6: Last child ↔ Parent margin-bottom collapsing
//   Phase 7: Adjacent sibling collapsing (post-pass)
//
// The collapse algorithm:
//   - Both positive: use maximum
//   - Both negative: use minimum (most negative)
//   - Mixed signs: add them

// CollapseValues implements the CSS margin collapse algorithm.
//   - Both positive: use maximum
//   - Both negative: use minimum (most negative)
//   - Mixed signs: add them
func CollapseValues(a, b float64) float64 {
	if a >= 0 && b >= 0 {
		return max(a, b)
	} else if a <= 0 && b <= 0 {
		return min(a, b)
	}
	return a + b
}

// CollapseTree applies CSS margin collapsing on the given tree.
// This is called after all content is generated and margins are captured.
func CollapseTree(tree *ContentTree) {
	collapseNode(tree, tree.Root)
}

// collapseNode recursively processes a container node and its descendants.
func collapseNode(t *ContentTree, node *ContentNode) {
	// Root is effectively at the end-of-storyline context.
	collapseNodeWithContext(t, node, false, true)
}

// collapseNodeWithContext is the internal version that tracks whether this node
// is the last child of its parent and whether it is at the end of the storyline.
func collapseNodeWithContext(t *ContentTree, node *ContentNode, isLastChildOfParent bool, isAtStorylineEnd bool) {
	if len(node.Children) == 0 {
		return
	}

	// Step 0: Strip margin-bottom for nodes marked for stripping (due to empty-line following)
	stripMarkedMarginBottom(t, node)

	// Step 1: Empty node collapsing
	collapseEmptyNodes(t, node)

	// Step 2: Sibling collapsing - transfer mb from each child to next sibling's mt
	collapseSiblings(t, node)

	// Step 3: First-child collapsing - transfer this node's mt to first child
	collapseFirstChild(t, node)

	// Step 4: Recurse into children
	for i, child := range node.Children {
		childIsLast := i == len(node.Children)-1
		childAtEnd := isAtStorylineEnd && childIsLast
		collapseNodeWithContext(t, child, childIsLast, childAtEnd)
	}

	// Step 5: Strip middle margin-bottom (for containers like stanza)
	stripMiddleMarginBottom(t, node)

	// Step 6: Last-child collapsing - pull up last child's mb to this node
	collapseLastChildWithContext(t, node, isLastChildOfParent, isAtStorylineEnd)

	// Step 7: Sibling collapsing (post-pass)
	collapseSiblings(t, node)
}

// collapseEmptyNodes handles Phase 1: self-collapse for empty nodes.
func collapseEmptyNodes(t *ContentTree, container *ContentNode) {
	for _, child := range container.Children {
		if child.IsEmpty() {
			mt := MarginValue(child.MarginTop)
			mb := MarginValue(child.MarginBottom)
			if mt != 0 || mb != 0 {
				beforeMT, beforeMB := child.MarginTop, child.MarginBottom

				collapsed := CollapseValues(mt, mb)
				child.MarginTop = nil
				child.MarginBottom = PtrFloat64(collapsed)

				traceCollapse(t, "empty", child, beforeMT, beforeMB, container)
			}
		}
	}
}

// stripMarkedMarginBottom removes margin-bottom from nodes marked with StripMarginBottom,
// and applies EmptyLineMarginBottom where set.
func stripMarkedMarginBottom(t *ContentTree, container *ContentNode) {
	children := container.Children
	for i, child := range children {
		// Handle EmptyLineMarginTop: when empty-line is followed by text.
		if child.EmptyLineMarginTop != nil {
			emptyLineMT := *child.EmptyLineMarginTop
			existingMT := 0.0
			if child.MarginTop != nil {
				existingMT = *child.MarginTop
			}
			if emptyLineMT > existingMT {
				beforeMT, beforeMB := child.MarginTop, child.MarginBottom
				child.MarginTop = PtrFloat64(emptyLineMT)
				traceCollapse(t, "emptyline-mt", child, beforeMT, beforeMB, container)
			}
		}

		// Handle EmptyLineMarginBottom: when empty-line is followed by image.
		if child.EmptyLineMarginBottom != nil {
			beforeMT, beforeMB := child.MarginTop, child.MarginBottom
			child.MarginBottom = child.EmptyLineMarginBottom
			traceCollapse(t, "emptyline-mb-before-image", child, beforeMT, beforeMB, container)
			continue
		}

		// Float images have fixed margins that don't participate in collapsing.
		if child.StripMarginBottom && child.MarginBottom != nil && !child.IsFloatImage {
			strippedMB := *child.MarginBottom

			beforeMT, beforeMB := child.MarginTop, child.MarginBottom
			child.MarginBottom = nil
			traceCollapse(t, "strip-emptyline-prev", child, beforeMT, beforeMB, container)

			// Transfer stripped margin to next sibling if it's larger than next's margin-top.
			if i+1 < len(children) {
				next := children[i+1]
				nextMT := 0.0
				if next.MarginTop != nil {
					nextMT = *next.MarginTop
				}
				if strippedMB > nextMT {
					beforeMT, beforeMB := next.MarginTop, next.MarginBottom
					next.MarginTop = PtrFloat64(strippedMB)
					traceCollapse(t, "transfer-stripped-mb", next, beforeMT, beforeMB, container)
				}
			}
		}
	}
}

// collapseFirstChild handles Phase 2: first child ↔ container margin-top.
func collapseFirstChild(t *ContentTree, container *ContentNode) {
	if len(container.Children) == 0 {
		return
	}
	if container.ContainerFlags.PreventsCollapseTop() {
		return
	}

	first := container.Children[0]
	beforeMT, beforeMB := first.MarginTop, first.MarginBottom

	if container.Index == -1 {
		if container.ContainerFlags.IsTitleBlockMode() {
			first.MarginTop = nil
			traceCollapse(t, "first-child", first, beforeMT, beforeMB, container)
			return
		}
		if container.HasWrapper {
			if first.MarginTop != nil {
				if container.MarginTop != nil {
					collapsed := CollapseValues(*container.MarginTop, *first.MarginTop)
					container.MarginTop = PtrFloat64(collapsed)
				} else {
					container.MarginTop = first.MarginTop
				}
				first.MarginTop = nil
				traceCollapse(t, "first-child", first, beforeMT, beforeMB, container)
			}
			return
		}
		// Purely virtual container: transfer mt to first child.
		if container.MarginTop != nil {
			if first.MarginTop != nil {
				collapsed := CollapseValues(*container.MarginTop, *first.MarginTop)
				first.MarginTop = PtrFloat64(collapsed)
			} else {
				first.MarginTop = container.MarginTop
			}
			container.MarginTop = nil
			traceCollapse(t, "first-child", first, beforeMT, beforeMB, container)
		}
		return
	}

	// Title-block mode: first child LOSES margin-top
	if container.ContainerFlags.IsTitleBlockMode() {
		first.MarginTop = nil
		traceCollapse(t, "first-child", first, beforeMT, beforeMB, container)
		return
	}

	// Standard mode: collapse first child's margin-top with container's margin-top
	if container.MarginTop != nil && first.MarginTop != nil {
		collapsed := CollapseValues(*container.MarginTop, *first.MarginTop)
		container.MarginTop = PtrFloat64(collapsed)
		first.MarginTop = nil
		traceCollapse(t, "first-child", first, beforeMT, beforeMB, container)
	} else if first.MarginTop != nil {
		container.MarginTop = first.MarginTop
		first.MarginTop = nil
		traceCollapse(t, "first-child", first, beforeMT, beforeMB, container)
	}
}

// collapseLastChildWithContext handles Phase 3: last child ↔ container margin-bottom.
func collapseLastChildWithContext(t *ContentTree, container *ContentNode, isLastChildOfParent bool, isAtStorylineEnd bool) {
	if len(container.Children) == 0 {
		return
	}
	if container.ContainerFlags.PreventsCollapseBottom() {
		return
	}

	last := container.Children[len(container.Children)-1]
	beforeMT, beforeMB := last.MarginTop, last.MarginBottom

	// Title-block mode: container KEEPS its margin-bottom for sibling collapsing
	if container.ContainerFlags.IsTitleBlockMode() {
		return
	}

	// Force-transfer-to-last-child mode
	if container.ContainerFlags.ForceTransfersMBToLastChild() {
		if container.MarginBottom != nil {
			if last.MarginBottom != nil {
				collapsed := CollapseValues(*container.MarginBottom, *last.MarginBottom)
				last.MarginBottom = PtrFloat64(collapsed)
			} else {
				last.MarginBottom = container.MarginBottom
			}
			container.MarginBottom = nil
			traceCollapse(t, "last-child", last, beforeMT, beforeMB, container)
		}
		return
	}

	// Transfer-to-last-child mode (only when container has siblings after it)
	if container.ContainerFlags.TransfersMBToLastChild() && !isLastChildOfParent {
		if container.MarginBottom != nil {
			if last.MarginBottom != nil {
				collapsed := CollapseValues(*container.MarginBottom, *last.MarginBottom)
				last.MarginBottom = PtrFloat64(collapsed)
			} else {
				last.MarginBottom = container.MarginBottom
			}
			container.MarginBottom = nil
			traceCollapse(t, "last-child", last, beforeMT, beforeMB, container)
		}
		return
	}

	// Root never renders — don't transfer child mb there.
	if container.ContainerKind == ContainerRoot {
		return
	}

	// Purely virtual container at storyline end: transfer mb DOWN to last child.
	if container.Index == -1 && !container.HasWrapper && isAtStorylineEnd {
		if container.MarginBottom != nil {
			if last.MarginBottom != nil {
				collapsed := CollapseValues(*container.MarginBottom, *last.MarginBottom)
				last.MarginBottom = PtrFloat64(collapsed)
			} else {
				last.MarginBottom = container.MarginBottom
			}
			container.MarginBottom = nil
			traceCollapse(t, "last-child", last, beforeMT, beforeMB, container)
		}
		return
	}

	// Float images act as a barrier.
	if last.IsFloatImage {
		return
	}

	if container.MarginBottom != nil && last.MarginBottom != nil {
		collapsed := CollapseValues(*container.MarginBottom, *last.MarginBottom)
		container.MarginBottom = PtrFloat64(collapsed)
		last.MarginBottom = nil
		traceCollapse(t, "last-child", last, beforeMT, beforeMB, container)
	} else if last.MarginBottom != nil {
		container.MarginBottom = last.MarginBottom
		last.MarginBottom = nil
		traceCollapse(t, "last-child", last, beforeMT, beforeMB, container)
	}
}

// stripMiddleMarginBottom removes margin-bottom from all non-last children.
func stripMiddleMarginBottom(t *ContentTree, container *ContentNode) {
	if !container.ContainerFlags.StripsMiddleMarginBottom() {
		return
	}

	children := container.Children
	if len(children) <= 1 {
		return
	}

	for i := 0; i < len(children)-1; i++ {
		child := children[i]
		if child.MarginBottom != nil {
			beforeMT, beforeMB := child.MarginTop, child.MarginBottom
			child.MarginBottom = nil
			traceCollapse(t, "strip-middle-mb", child, beforeMT, beforeMB, container)
		}
	}
}

// collapseSiblings handles Phase 4: adjacent sibling margin collapsing.
func collapseSiblings(t *ContentTree, container *ContentNode) {
	children := container.Children

	for i := 0; i < len(children)-1; i++ {
		curr := children[i]
		next := children[i+1]
		nextBeforeMT, nextBeforeMB := next.MarginTop, next.MarginBottom

		if !curr.IsContainer() {
			// Float images act as barriers.
			if curr.IsFloatImage || next.IsFloatImage {
				continue
			}

			// break-after-avoid + next is container: transfer mb to container
			if curr.HasBreakAfterAvoid && curr.MarginBottom != nil && next.IsContainer() {
				currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
				if next.MarginTop != nil {
					collapsed := CollapseValues(*curr.MarginBottom, *next.MarginTop)
					next.MarginTop = PtrFloat64(collapsed)
				} else {
					next.MarginTop = curr.MarginBottom
				}
				curr.MarginBottom = nil
				traceCollapse(t, "break-after-avoid-to-container", curr, currBeforeMT, currBeforeMB, container)
				traceCollapse(t, "sibling", next, nextBeforeMT, nextBeforeMB, container)
				continue
			}

			// CSS sibling margin collapsing between content nodes
			if curr.MarginBottom != nil && next.MarginTop != nil {
				currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
				collapsed := CollapseValues(*curr.MarginBottom, *next.MarginTop)
				next.MarginTop = PtrFloat64(collapsed)
				curr.MarginBottom = nil
				traceCollapse(t, "sibling-collapse", curr, currBeforeMT, currBeforeMB, container)
				traceCollapse(t, "sibling", next, nextBeforeMT, nextBeforeMB, container)
			}

			// CSS sibling collapsing between content node and following container
			if curr.MarginBottom != nil && next.IsContainer() {
				currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
				if next.MarginTop != nil {
					collapsed := CollapseValues(*curr.MarginBottom, *next.MarginTop)
					next.MarginTop = PtrFloat64(collapsed)
				} else {
					next.MarginTop = curr.MarginBottom
				}
				curr.MarginBottom = nil
				traceCollapse(t, "content-to-container-sibling", curr, currBeforeMT, currBeforeMB, container)
				traceCollapse(t, "sibling", next, nextBeforeMT, nextBeforeMB, container)
			}
			continue
		}

		// Virtual container (wrapper) nodes.
		if curr.ContainerFlags.IsTitleBlockMode() {
			var target *ContentNode
			switch {
			case !next.IsContainer():
				target = next
			case next.MarginTop != nil:
				target = next
			default:
				target = next
				for target != nil && target.IsContainer() {
					if len(target.Children) == 0 {
						target = nil
						break
					}
					target = target.Children[0]
				}
			}
			if curr.MarginBottom != nil && target != nil && target.MarginTop != nil {
				collapsed := CollapseValues(*curr.MarginBottom, *target.MarginTop)
				target.MarginTop = PtrFloat64(collapsed)
				curr.MarginBottom = nil
				traceCollapse(t, "sibling", next, nextBeforeMT, nextBeforeMB, container)
			}
			continue
		}

		// Containers with FlagTransferMBToLastChild
		if curr.ContainerFlags.TransfersMBToLastChild() {
			if curr.MarginBottom != nil && next.MarginTop != nil {
				currMB := *curr.MarginBottom
				nextMT := *next.MarginTop
				if nextMT >= currMB {
					currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
					curr.MarginBottom = nil
					traceCollapse(t, "container-sibling-absorb-mb", curr, currBeforeMT, currBeforeMB, container)
				}
			}
			if len(curr.Children) > 0 && next.MarginTop != nil {
				lastChild := curr.Children[len(curr.Children)-1]
				if lastChild.MarginBottom != nil {
					lastMB := *lastChild.MarginBottom
					nextMT := *next.MarginTop
					if nextMT >= lastMB {
						beforeMT, beforeMB := lastChild.MarginTop, lastChild.MarginBottom
						lastChild.MarginBottom = nil
						traceCollapse(t, "through-container-absorb-mb", lastChild, beforeMT, beforeMB, container)
					}
				}
			}
			continue
		}

		// Standard container: transfer mb to next sibling's mt
		if curr.MarginBottom != nil {
			if next.MarginTop != nil {
				collapsed := CollapseValues(*curr.MarginBottom, *next.MarginTop)
				next.MarginTop = PtrFloat64(collapsed)
			} else {
				next.MarginTop = curr.MarginBottom
			}
			curr.MarginBottom = nil
			traceCollapse(t, "sibling", next, nextBeforeMT, nextBeforeMB, container)
		}
	}
}

// traceCollapse logs a collapse operation if the tracer is enabled.
func traceCollapse(t *ContentTree, collapseType string, node *ContentNode, beforeMT, beforeMB *float64, container *ContentNode) {
	if t.Tracer != nil && t.Tracer.IsEnabled() {
		t.Tracer.TraceMarginCollapse(collapseType, node.TraceID(),
			beforeMT, beforeMB, node.MarginTop, node.MarginBottom,
			container.ContainerKind.String())
	}
}
