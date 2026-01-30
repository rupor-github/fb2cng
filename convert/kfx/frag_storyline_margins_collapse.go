package kfx

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
	// Root is effectively at the end-of-storyline context.
	t.collapseNodeWithContext(node, false, true)
}

// collapseNodeWithContext is the internal version that tracks whether this node
// is the last child of its parent and whether it is at the end of the storyline.
//
// isAtStorylineEnd is true when this node has no following siblings at any ancestor
// level (i.e., it's the final subtree in the storyline). This is needed to model
// KP3 behavior where margins from non-rendered virtual containers (like .section)
// materialize on the last rendered element only at storyline end.
func (t *ContentTree) collapseNodeWithContext(node *ContentNode, isLastChildOfParent bool, isAtStorylineEnd bool) {
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
		childIsLast := i == len(node.Children)-1
		childAtEnd := isAtStorylineEnd && childIsLast
		t.collapseNodeWithContext(child, childIsLast, childAtEnd)
	}

	// Step 5: Strip middle margin-bottom (for containers like stanza)
	// This must happen AFTER recursion so nested containers have their mb accumulated
	t.stripMiddleMarginBottom(node)

	// Step 6: Last-child collapsing - pull up last child's mb to this node
	// This must happen AFTER recursion so children's mb values are final
	t.collapseLastChildWithContext(node, isLastChildOfParent, isAtStorylineEnd)

	// Step 7: Sibling collapsing (post-pass)
	// Some margins (notably container mb) are only known after child processing and
	// last-child collapsing. Run sibling collapsing again to propagate those margins
	// to following siblings (KP3 behavior).
	t.collapseSiblings(node)
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

	// For container nodes that correspond to a wrapper entry (content_list), collapse
	// the first child's mt INTO the container (and remove it from the child).
	// This matches KP3 where wrapper mt stays on the wrapper, and first-child mt is removed.
	//
	// For purely virtual containers (no wrapper entry), we instead transfer the container
	// mt down to the first child, since there is nothing that can render the container mt.
	if container.Index == -1 {
		if container.ContainerFlags.IsTitleBlockMode() {
			// Title-block mode: container keeps its mt (on wrapper). Child loses mt.
			first.MarginTop = nil
			t.traceFirstChild(first, beforeMT, beforeMB, container)
			return
		}
		if container.HasWrapper {
			// Wrapper-backed container: collapse first child's mt into container.
			if first.MarginTop != nil {
				if container.MarginTop != nil {
					collapsed := collapseValues(*container.MarginTop, *first.MarginTop)
					container.MarginTop = ptrFloat64(collapsed)
				} else {
					container.MarginTop = first.MarginTop
				}
				first.MarginTop = nil
				t.traceFirstChild(first, beforeMT, beforeMB, container)
			}
			return
		}
		// Purely virtual container: transfer mt to first child.
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
func (t *ContentTree) collapseLastChildWithContext(container *ContentNode, isLastChildOfParent bool, isAtStorylineEnd bool) {
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
		return
	}

	// Special case: purely virtual containers (Index == -1) without a wrapper entry cannot
	// render their own margin-bottom. At storyline end, KP3 applies such container mb to the
	// last rendered child, and also prevents the last child's own mb from being pulled up into
	// a non-rendered container (which would lose trailing spacing).
	if container.Index == -1 && !container.HasWrapper && isAtStorylineEnd {
		if container.MarginBottom != nil {
			if last.MarginBottom != nil {
				collapsed := collapseValues(*container.MarginBottom, *last.MarginBottom)
				last.MarginBottom = ptrFloat64(collapsed)
			} else {
				last.MarginBottom = container.MarginBottom
			}
			container.MarginBottom = nil
			t.traceLastChild(last, beforeMT, beforeMB, container)
		}
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
			// Float images have fixed 2.6lh margins and act as barriers.
			// They do not participate in sibling collapsing in either direction.
			if curr.IsFloatImage || next.IsFloatImage {
				continue
			}

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

			// CSS sibling margin collapsing between content nodes:
			// When curr has mb and next has mt, collapse them (max/min/sum) and put
			// the result on next.mt. The current element's mb is removed.
			//
			if curr.MarginBottom != nil && next.MarginTop != nil {
				currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
				collapsed := collapseValues(*curr.MarginBottom, *next.MarginTop)
				next.MarginTop = ptrFloat64(collapsed)
				curr.MarginBottom = nil
				if t.tracer != nil && t.tracer.IsEnabled() {
					t.tracer.TraceMarginCollapse("sibling-collapse", curr.TraceID(),
						currBeforeMT, currBeforeMB, curr.MarginTop, curr.MarginBottom,
						container.ContainerKind.String())
				}
				// Trace against the immediate next sibling for context.
				t.traceSibling(next, nextBeforeMT, nextBeforeMB, container)
			}

			// CSS sibling margin collapsing between content node and following container:
			// Collapse curr.mb with container.mt and put the result on the container.
			// The container's first-child collapsing will then propagate it down.
			if curr.MarginBottom != nil && next.IsContainer() {
				currBeforeMT, currBeforeMB := curr.MarginTop, curr.MarginBottom
				if next.MarginTop != nil {
					collapsed := collapseValues(*curr.MarginBottom, *next.MarginTop)
					next.MarginTop = ptrFloat64(collapsed)
				} else {
					next.MarginTop = curr.MarginBottom
				}
				curr.MarginBottom = nil
				if t.tracer != nil && t.tracer.IsEnabled() {
					t.tracer.TraceMarginCollapse("content-to-container-sibling", curr.TraceID(),
						currBeforeMT, currBeforeMB, curr.MarginTop, curr.MarginBottom,
						container.ContainerKind.String())
				}
				t.traceSibling(next, nextBeforeMT, nextBeforeMB, container)
			}
			// Content nodes keep their margins otherwise
			continue
		}

		// Virtual container (wrapper) nodes: handle margin collapsing with next sibling.
		// Containers don't render directly; their margins go to content.
		//
		// Title-block mode is special:
		// - If the next sibling already has a margin-top, KP3 effectively collapses by max()
		//   and the container's mb disappears into the next sibling's mt.
		// - If the next sibling has no mt, KP3 keeps the container's mb on the container
		//   (it does NOT manufacture a mt on the next sibling).
		if curr.ContainerFlags.IsTitleBlockMode() {
			// Title-block wrappers are special:
			// - If the next sibling already has a margin-top, KP3 collapses by max()
			//   and the title-block's mb disappears into that mt.
			// - If the next sibling has no mt, KP3 keeps the title-block's mb on the wrapper
			//   (it does NOT manufacture a mt on the next sibling).
			//
			// Important detail: this collapsing happens BEFORE first-child collapsing.
			// If the next sibling is a container, its mt (if any) will be transferred down
			// to the first rendered child later. So we must prefer collapsing into the
			// container's mt when present, rather than descending to a child that may not
			// have mt yet.
			var target *ContentNode
			switch {
			case !next.IsContainer():
				// Next is a leaf content node.
				target = next
			case next.MarginTop != nil:
				// Next is a container that already has mt. Collapse into that mt and let
				// first-child collapsing propagate it down.
				target = next
			default:
				// Next is a container without mt; fall back to the first rendered descendant.
				target = next
				for target != nil && target.IsContainer() {
					if len(target.Children) == 0 {
						target = nil
						break
					}
					target = target.Children[0]
				}
			}
			// Only collapse if the chosen target already has mt.
			if curr.MarginBottom != nil && target != nil && target.MarginTop != nil {
				collapsed := collapseValues(*curr.MarginBottom, *target.MarginTop)
				target.MarginTop = ptrFloat64(collapsed)
				curr.MarginBottom = nil
				// Trace against the immediate next sibling for context.
				t.traceSibling(next, nextBeforeMT, nextBeforeMB, container)
			}
			continue
		}
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
