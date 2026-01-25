package kfx

import (
	"maps"
	"strings"
)

// PushBlock enters a block container scope with margin distribution based on item count.
//
// Unlike Push() which only passes CSS-inherited properties, PushBlock() also passes
// margin-left and margin-right so children are properly indented within the container.
// This matches KP3 behavior where container margins apply to each child element directly
// rather than being applied to a wrapper block.
//
// For margin-left and margin-right, values are accumulated (cumulative merge) matching
// KP3's YJCumulativeRuleMerger behavior. This means nested blocks like poem > stanza > verse
// will have their margin-left values summed (e.g., poem's 3em + verse's 2em = 5em).
// The contributor tracking (marginOrigins) records which style names contributed to each
// margin, enabling YJCumulativeInSameContainerRuleMerger to avoid double-counting when
// the same style is applied again during resolveProperties.
//
// itemCount: total number of items that will be processed in this container.
// Use Advance() after processing each item to move to the next position.
// Use itemCount=1 for single-element containers where no margin distribution is needed.
//
// titleBlockMargins: optional flag to use title-block margin style. When true, spacing
// between elements is via margin-top on following elements (first element loses margin-top).
// When false (default), elements keep their margin-bottom for inter-element spacing.
// Use true for poems/stanzas where verses use margin-top spacing.
//
// The container's vertical margins are pushed onto the stack and will be
// distributed to first/last children when Resolve() is called.
func (sc StyleContext) PushBlock(tag, classes string, itemCount int, titleBlockMargins ...bool) StyleContext {
	// Ensure itemCount is at least 1 for single-element containers
	if itemCount < 1 {
		itemCount = 1
	}

	// Copy existing inherited properties
	newInherited := make(map[KFXSymbol]any, len(sc.inherited))
	maps.Copy(newInherited, sc.inherited)

	// Copy existing margin origins (deep copy to preserve contributor sets)
	newMarginOrigins := make(map[KFXSymbol]*marginOrigin, len(sc.marginOrigins))
	for sym, origin := range sc.marginOrigins {
		newContributors := make(map[string]bool, len(origin.contributors))
		for k, v := range origin.contributors {
			newContributors[k] = v
		}
		newMarginOrigins[sym] = &marginOrigin{
			value:        origin.value,
			contributors: newContributors,
		}
	}

	// Collect container's resolved properties for margin extraction
	var containerProps map[KFXSymbol]any

	// mergeBlockProperty applies block-inherited properties with cumulative merge for margins.
	// This matches KP3's behavior where nested block margins accumulate.
	// Also tracks which style names contributed to the margin for later same-container detection.
	mergeBlockProperty := func(sym KFXSymbol, val any, styleName string) {
		if !isBlockInheritedProperty(sym) {
			return
		}
		// For margin-left/right, use cumulative merge (add values) and track contributors
		// This matches KP3's YJCumulativeRuleMerger for these properties
		if sym == SymMarginLeft || sym == SymMarginRight {
			origin := newMarginOrigins[sym]
			if origin == nil {
				// First contributor for this margin
				origin = &marginOrigin{
					value:        val,
					contributors: map[string]bool{styleName: true},
				}
				newMarginOrigins[sym] = origin
				newInherited[sym] = val
				return
			}

			// Check if this style already contributed - if so, override (not accumulate)
			if origin.contributors[styleName] {
				// Same style - override value but don't accumulate
				origin.value = val
				newInherited[sym] = val
				return
			}

			// Different style - accumulate values
			if merged, ok := mergeCumulative(origin.value, val); ok {
				origin.value = merged
				origin.contributors[styleName] = true
				newInherited[sym] = merged
				return
			}
			// Fallback: can't merge, just override
			origin.value = val
			origin.contributors[styleName] = true
			newInherited[sym] = val
			return
		}
		// For other block-inherited properties, use override (normal assignment)
		newInherited[sym] = val
	}

	// Add block-inherited properties from tag defaults
	if tag != "" && sc.registry != nil {
		if def, ok := sc.registry.Get(tag); ok {
			resolved := sc.registry.resolveInheritance(def)
			containerProps = resolved.Properties
			for sym, val := range resolved.Properties {
				mergeBlockProperty(sym, val, tag)
			}
		}
	}

	// Parse and add block-inherited properties from each class
	var classList []string
	if classes != "" {
		classList = strings.Fields(classes)
		for _, class := range classList {
			if sc.registry != nil {
				sc.registry.EnsureBaseStyle(class)
				if def, ok := sc.registry.Get(class); ok {
					resolved := sc.registry.resolveInheritance(def)
					// Class properties override tag properties
					if containerProps == nil {
						containerProps = make(map[KFXSymbol]any)
					}
					for sym, val := range resolved.Properties {
						containerProps[sym] = val
						mergeBlockProperty(sym, val, class)
					}
				}
			}
		}
	}

	// Append to scope chain
	newScopes := append(sc.scopes, StyleScope{Tag: tag, Classes: classList})

	// Build the new context
	newCtx := StyleContext{
		registry:      sc.registry,
		inherited:     newInherited,
		marginOrigins: newMarginOrigins,
		scopes:        newScopes,
		emptyLine:     sc.emptyLine, // Preserve empty-line tracking
	}

	// Push container frame onto stack
	mt := extractMarginValue(containerProps, SymMarginTop)
	mb := extractMarginValue(containerProps, SymMarginBottom)

	// Determine if this container is the last item in its parent.
	// This affects whether the container's margin-bottom should be applied to its last child.
	isLastInParent := false
	if len(sc.containerStack) > 0 {
		parentFrame := sc.containerStack[len(sc.containerStack)-1]
		isLastInParent = parentFrame.currentItem == parentFrame.itemCount-1
	} else {
		// Top-level container (no parent container) - treat as last
		isLastInParent = true
	}

	// Check if title-block margin style was requested
	useTitleBlockMargins := len(titleBlockMargins) > 0 && titleBlockMargins[0]

	// Copy parent's container stack and push new frame
	newStack := make([]containerFrame, len(sc.containerStack), len(sc.containerStack)+1)
	copy(newStack, sc.containerStack)
	newStack = append(newStack, containerFrame{
		marginTop:         mt,
		marginBottom:      mb,
		itemCount:         itemCount,
		currentItem:       0,
		isLastInParent:    isLastInParent,
		titleBlockMargins: useTitleBlockMargins,
	})
	newCtx.containerStack = newStack

	// Trace container entry if tracer is enabled
	if sc.registry != nil && sc.registry.Tracer().IsEnabled() {
		sc.registry.Tracer().TraceContainerEnter(tag, classes, itemCount, mt, mb, isLastInParent, useTitleBlockMargins, newCtx.ScopePath(), newCtx.ContainerPath())
	}

	return newCtx
}
