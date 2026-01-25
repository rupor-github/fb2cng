package kfx

import "strings"

// resolveProperties builds the merged property map for an element within this context.
// This performs CSS-style property merging with proper inheritance and descendant selectors,
// but does NOT register the style. Use this when you need to apply additional processing
// (like position filtering) before registration.
//
// Order of application (later overrides earlier):
// 1. Inherited properties accumulated through Push calls
// 2. Element tag defaults (all properties)
// 3. Element's classes (all properties, in order)
// 4. Descendant selectors matching any scope ancestor with current tag/classes
//
// For margin-left/right, this implements YJCumulativeInSameContainerRuleMerger:
//   - If a style already contributed to the inherited margin (tracked in marginOrigins),
//     the new value overrides (doesn't accumulate) - same container semantics
//   - If a style is new, it would accumulate, but since we're resolving the final element
//     style (not entering a new container), we use override semantics for element classes
func (sc StyleContext) resolveProperties(tag, classes string) map[KFXSymbol]any {
	merged := make(map[KFXSymbol]any)

	if sc.registry == nil {
		return merged
	}

	// 1. Start with inherited properties from context
	sc.registry.mergePropertiesWithContext(merged, sc.inherited, mergeContextInline)

	// 2. Apply element tag defaults (all properties)
	// For margin-left/right, we need special handling:
	// - If the element (e.g., "p") has margin-left: 0 explicitly, it would normally
	//   override the inherited container margin (e.g., from poem)
	// - But in KFX block context, we want to PRESERVE container margins
	// - So we filter out zero margins from tag defaults if we have non-zero inherited margins
	if tag != "" {
		if def, ok := sc.registry.Get(tag); ok {
			resolved := sc.registry.resolveInheritance(def)
			propsToMerge := sc.filterZeroMarginsIfInherited(resolved.Properties)
			sc.registry.mergePropertiesWithContext(merged, propsToMerge, mergeContextInline)
		}
	}

	// 3. Apply element's classes (all properties, in order)
	// Use mergeContextClassOverride so class margins override tag margins
	// (CSS specificity: class > element selector, uses override rule instead of override-maximum)
	//
	// For margin-left/right: implement YJCumulativeInSameContainerRuleMerger correctly:
	// - If this class already contributed to the inherited margin (via PushBlock), SKIP the margin
	//   to avoid double-counting (same container semantics)
	// - If this class is NEW (different container), ACCUMULATE with inherited margin
	//
	// IMPORTANT: For classes with descendant selectors (e.g., "sub" has "h1--sub" when inside h1),
	// we apply the descendant selector INSTEAD OF the base class. This prevents the base class's
	// font-size from overriding inherited values when the descendant selector intentionally omits it.
	// For example: h1--sub only has baseline-style (no font-size), so sub/sup in headings inherit
	// the heading's font-size rather than getting the base sub's font-size: 0.75rem.
	classList := strings.Fields(classes)

	// Build a map of class -> descendant selector name for classes that should have REPLACEMENT
	// descendant selector behavior. This only applies to styles marked with DescendantReplacement
	// in the registry (e.g., sub, sup, small) where the descendant selector should COMPLETELY
	// REPLACE the base class (not just override).
	//
	// CSS descendant selectors (like ".section-title h2.section-title-header") use standard
	// CSS cascade behavior: base class is applied first, then descendant selector properties
	// override. These are handled in step 4 below.
	//
	// The replacement behavior is needed for sub/sup/small in headings: h1--sub only has
	// baseline-style (no font-size), so sub/sup in headings inherit the heading's font-size
	// rather than getting the base sub's font-size: 0.75rem.
	classToDescendantSelector := make(map[string]string)

	if len(sc.scopes) > 0 && classes != "" {
		for _, class := range classList {
			// Only check for replacement descendants for styles marked with DescendantReplacement
			isReplacement := sc.registry.IsDescendantReplacement(class)
			if !isReplacement {
				continue
			}

			// Check if any scope ancestor has a descendant selector for this class
			for _, scope := range sc.scopes {
				ancestors := make([]string, 0, len(scope.Classes)+1)
				ancestors = append(ancestors, scope.Classes...)
				if scope.Tag != "" {
					ancestors = append(ancestors, scope.Tag)
				}

				for _, anc := range ancestors {
					styleName := anc + "--" + class
					if _, ok := sc.registry.Get(styleName); ok {
						// Found a descendant selector; use it instead of the base class
						classToDescendantSelector[class] = styleName
						break
					}
				}
				if _, found := classToDescendantSelector[class]; found {
					break // Stop at first match (innermost scope takes precedence)
				}
			}
		}
	}

	if classes != "" {
		for class := range strings.FieldsSeq(classes) {
			// If there's a replacement descendant selector for this class, use it instead of the base class
			styleToApply := class
			if descSelector, hasDesc := classToDescendantSelector[class]; hasDesc {
				styleToApply = descSelector
			}

			if def, ok := sc.registry.Get(styleToApply); ok {
				resolved := sc.registry.resolveInheritance(def)
				// Handle margins with container-aware logic
				propsToMerge := sc.handleContainerAwareMargins(merged, resolved.Properties, class)
				sc.registry.mergePropertiesWithContext(merged, propsToMerge, mergeContextClassOverride)
			}
		}
	}

	// 4. Apply descendant selectors matching any scope ancestor with current tag/classes.
	// This mirrors CSS descendant rules like ".section-title h2.section-title-header".
	// These are OVERRIDE selectors (CSS cascade) - they add to/override the base class properties.
	// This is different from replacement selectors (step 3) which completely replace the base class.
	// Also apply direct child selectors (parent>child) from innermost scope only.
	if len(sc.scopes) > 0 {
		descCandidates := make([]string, 0, len(classList)+1)
		if tag != "" {
			descCandidates = append(descCandidates, tag)
		}
		descCandidates = append(descCandidates, classList...)

		// First, check direct child selectors (parent>child) from innermost scope only.
		// This mirrors CSS child combinator like ".footnote > p".
		innerScope := sc.scopes[len(sc.scopes)-1]
		directParents := make([]string, 0, len(innerScope.Classes)+1)
		directParents = append(directParents, innerScope.Classes...)
		if innerScope.Tag != "" {
			directParents = append(directParents, innerScope.Tag)
		}

		for _, parent := range directParents {
			for _, desc := range descCandidates {
				styleName := parent + ">" + desc // Direct child selector naming convention
				if def, ok := sc.registry.Get(styleName); ok {
					resolved := sc.registry.resolveInheritance(def)
					sc.registry.mergePropertiesWithContext(merged, resolved.Properties, mergeContextInline)
				}
			}
		}

		// Then, check descendant selectors (ancestor--descendant) from all scopes.
		// Skip classes that already had replacement behavior applied (sub, sup)
		// to avoid double-applying h1--sub etc.
		for _, scope := range sc.scopes {
			ancestors := make([]string, 0, len(scope.Classes)+1)
			ancestors = append(ancestors, scope.Classes...)
			if scope.Tag != "" {
				ancestors = append(ancestors, scope.Tag)
			}

			for _, anc := range ancestors {
				for _, desc := range descCandidates {
					// Skip if this class already had replacement descendant applied
					if _, hadReplacement := classToDescendantSelector[desc]; hadReplacement {
						continue
					}

					styleName := anc + "--" + desc
					if def, ok := sc.registry.Get(styleName); ok {
						resolved := sc.registry.resolveInheritance(def)
						sc.registry.mergePropertiesWithContext(merged, resolved.Properties, mergeContextInline)
					}
				}
			}
		}
	}

	return merged
}

// handleContainerAwareMargins implements YJCumulativeInSameContainerRuleMerger logic.
// For margin-left/right:
// - If styleName already contributed to inherited margin → skip (same container)
// - If styleName is new → accumulate with current merged value (different container)
// Other properties are returned unchanged.
//
// This modifies the merged map directly for margins and returns the remaining props.
func (sc StyleContext) handleContainerAwareMargins(merged, props map[KFXSymbol]any, styleName string) map[KFXSymbol]any {
	// Check if we need to handle margins specially
	hasMargins := false
	for _, sym := range []KFXSymbol{SymMarginLeft, SymMarginRight} {
		if _, ok := props[sym]; ok {
			hasMargins = true
			break
		}
	}

	if !hasMargins {
		return props // No margins to handle, return original
	}

	// Get tracer for margin accumulation tracing
	var tracer *StyleTracer
	if sc.registry != nil {
		tracer = sc.registry.Tracer()
	}

	// Get scope path for tracing
	scopePath := sc.ScopePath()

	// Create a copy without margins (they'll be handled separately)
	nonMarginProps := make(map[KFXSymbol]any, len(props))
	for sym, val := range props {
		if sym == SymMarginLeft || sym == SymMarginRight {
			// Handle margin with container-aware logic
			origin := sc.marginOrigins[sym]
			marginName := "margin-left"
			if sym == SymMarginRight {
				marginName = "margin-right"
			}

			if origin != nil && origin.contributors[styleName] {
				// Same container: style already contributed, SKIP to avoid double-counting
				if tracer.IsEnabled() {
					tracer.TraceMarginAccumulate(marginName, styleName, "skip", origin.value, val, merged[sym], scopePath)
				}
				continue
			}

			// Different container (or no prior contributors): ACCUMULATE
			if existing, ok := merged[sym]; ok {
				if accumulated, ok := mergeCumulative(existing, val); ok {
					merged[sym] = accumulated
					if tracer.IsEnabled() {
						tracer.TraceMarginAccumulate(marginName, styleName, "accumulate", existing, val, accumulated, scopePath)
					}
				} else {
					// Fallback: override if can't accumulate
					merged[sym] = val
					if tracer.IsEnabled() {
						tracer.TraceMarginAccumulate(marginName, styleName, "override", existing, val, val, scopePath)
					}
				}
			} else {
				// No existing value, just set
				merged[sym] = val
				if tracer.IsEnabled() {
					tracer.TraceMarginAccumulate(marginName, styleName, "set", nil, val, val, scopePath)
				}
			}
			continue
		}
		nonMarginProps[sym] = val
	}

	return nonMarginProps
}

// filterZeroMarginsIfInherited returns a copy of props with margin-left/right removed
// if they are zero and we already have non-zero inherited margins from a block container.
// This prevents element tag defaults (like "p { margin-left: 0 }") from overriding
// container margins (like poem's margin-left: 9.375%).
func (sc StyleContext) filterZeroMarginsIfInherited(props map[KFXSymbol]any) map[KFXSymbol]any {
	needsFilter := false
	for _, sym := range []KFXSymbol{SymMarginLeft, SymMarginRight} {
		if val, hasMargin := props[sym]; hasMargin {
			// Check if this is a zero margin and we have non-zero inherited
			if isZeroMargin(val) && !isZeroMargin(sc.inherited[sym]) {
				needsFilter = true
				break
			}
		}
	}

	if !needsFilter {
		return props // No filtering needed, return original
	}

	// Create filtered copy
	filtered := make(map[KFXSymbol]any, len(props))
	for sym, val := range props {
		if sym == SymMarginLeft || sym == SymMarginRight {
			// Skip zero margins if we have non-zero inherited
			if isZeroMargin(val) && !isZeroMargin(sc.inherited[sym]) {
				continue
			}
		}
		filtered[sym] = val
	}
	return filtered
}

// isZeroMargin returns true if the margin value is zero or nil.
func isZeroMargin(val any) bool {
	if val == nil {
		return true
	}
	v, _, ok := measureParts(val)
	return ok && v == 0
}

// ResolveProperty resolves the CSS cascade for an element and returns a specific property value.
// This is useful when you need a single property without registering the full style.
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// prop: the KFX symbol for the property to extract
// Returns the property value, or nil if not found.
func (sc StyleContext) ResolveProperty(tag, classes string, prop KFXSymbol) any {
	merged := sc.resolveProperties(tag, classes)
	return merged[prop]
}

// Resolve creates the final style for an element within this context.
// Vertical margins are computed based on container position (from containerStack or
// legacy wrapperMargins/position mechanism).
//
// KP3 position-based margin handling:
// - First element (not last): ADD wrapper's margin-top, KEEP own margin-bottom
// - Last element (not first): REMOVE own margin-top, REPLACE own margin-bottom with wrapper's
// - Single element (first AND last): ADD wrapper's margin-top, REPLACE margin-bottom with wrapper's
// - Middle elements: REMOVE both margin-top and margin-bottom
//
// Empty-line handling:
// - Element after empty-line gets margin-top from empty-line (replaces own margin-top)
// - Element after empty-line keeps its margin-bottom (even if middle element)
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// Returns the registered style name.
func (sc StyleContext) Resolve(tag, classes string) string {
	merged := sc.resolveProperties(tag, classes)

	// Apply vertical margin distribution from container stack
	sc.applyContainerMargins(merged)

	// Consume any pending empty-line margin and set as margin-top.
	// This implements KP3 behavior where empty-lines don't create content entries
	// but instead set the following element's margin-top to the empty-line's margin.
	// The empty-line margin replaces (not adds to) the element's own margin-top.
	pendingMargin := sc.ConsumePendingMargin()
	if pendingMargin > 0 {
		merged[SymMarginTop] = DimensionValue(pendingMargin, SymUnitLh)
	}

	return sc.registry.RegisterResolved(merged)
}

// ResolveNoMark creates the final style for an element but does NOT mark it as used.
// This is used when styles are resolved during processing but usage will be marked later
// (e.g., after style event segmentation that may deduplicate some events).
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// Returns the registered style name.
func (sc StyleContext) ResolveNoMark(tag, classes string) string {
	merged := sc.resolveProperties(tag, classes)

	// Note: No container margin application for inline styles.
	// Inline styles are resolved without position context since they're
	// used for style events within paragraph text, not block elements.

	return sc.registry.RegisterResolvedNoMark(merged)
}

// applyContainerMargins applies vertical margin distribution from the container stack.
//
// Two margin styles are supported based on frame.titleBlockMargins:
//
// Standard style (titleBlockMargins=false): spacing via margin-bottom
//   - First child: gets container's margin-top, keeps own margin-bottom
//   - Middle children: keeps own margin-top and margin-bottom
//   - Last child: keeps own margin-top, gets container's margin-bottom
//   - Single child: gets container's margins (if provided), else keeps own
//
// Title-block style (titleBlockMargins=true): spacing via margin-top
//   - First child: gets container's margin-top (replacing own), loses margin-bottom
//   - Non-first/non-last children: keep margin-top (creates spacing), lose margin-bottom
//   - Last child: keeps margin-top, gets container's margin-bottom
//   - Single child: gets container's margin-top, gets container's margin-bottom
func (sc StyleContext) applyContainerMargins(merged map[KFXSymbol]any) {
	if len(sc.containerStack) == 0 {
		return
	}

	frame := sc.containerStack[len(sc.containerStack)-1]
	pos := PositionFromIndex(frame.currentItem, frame.itemCount)

	// Capture original margins for tracing
	var tracer *StyleTracer
	var originalMT, originalMB float64
	if sc.registry != nil {
		tracer = sc.registry.Tracer()
		if tracer.IsEnabled() {
			originalMT = extractMarginValue(merged, SymMarginTop)
			originalMB = extractMarginValue(merged, SymMarginBottom)
		}
	}

	// Determine if container's margin-bottom should be applied to the last child.
	// For top-level containers: always apply.
	// For nested containers: apply only if this container is NOT the last item in parent.
	applyContainerMarginBottom := len(sc.containerStack) == 1 || !frame.isLastInParent

	// Track which container margins were applied
	var appliedContainerMT, appliedContainerMB float64

	if frame.titleBlockMargins {
		// Title-block style: use unified filtering logic
		applyTitleBlockFiltering(merged, pos)

		// First element: apply container's margin-top (if any)
		if pos.IsFirst && frame.marginTop > 0 {
			merged[SymMarginTop] = DimensionValue(frame.marginTop, SymUnitLh)
			appliedContainerMT = frame.marginTop
		}

		// Last element gets container's margin-bottom if applicable
		if pos.IsLast && applyContainerMarginBottom && frame.marginBottom > 0 {
			merged[SymMarginBottom] = DimensionValue(frame.marginBottom, SymUnitLh)
			appliedContainerMB = frame.marginBottom
		}
	} else {
		// Standard style: spacing via margin-bottom on elements
		if pos.IsFirst {
			// First element: use container's margin-top
			if frame.marginTop > 0 {
				merged[SymMarginTop] = DimensionValue(frame.marginTop, SymUnitLh)
				appliedContainerMT = frame.marginTop
			}
		}
		// Non-first elements keep their margin-top
		// All elements keep their margin-bottom for inter-element spacing

		// Last element gets container's margin-bottom if applicable
		if pos.IsLast && applyContainerMarginBottom && frame.marginBottom > 0 {
			merged[SymMarginBottom] = DimensionValue(frame.marginBottom, SymUnitLh)
			appliedContainerMB = frame.marginBottom
		}
	}

	// Trace position-based margin resolution
	if tracer.IsEnabled() {
		appliedMT := extractMarginValue(merged, SymMarginTop)
		appliedMB := extractMarginValue(merged, SymMarginBottom)
		tracer.TracePositionResolve(
			pos.String(),
			[2]float64{originalMT, originalMB},
			[2]float64{appliedMT, appliedMB},
			[2]float64{appliedContainerMT, appliedContainerMB},
			sc.ScopePath(),
			sc.ContainerPath(),
		)
	}
}

// StyleSpec returns the raw style specification string for an element within this context.
// This includes the tag, ancestor scope classes, and element's own classes.
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// Returns a space-separated style spec like "p poem stanza verse"
func (sc StyleContext) StyleSpec(tag, classes string) string {
	var parts []string

	// Start with the tag if provided
	if tag != "" {
		parts = append(parts, tag)
	}

	// Add ancestor scope classes (these form the context chain)
	// e.g., for poem > stanza context, adds "poem stanza"
	for _, scope := range sc.scopes {
		parts = append(parts, scope.Classes...)
	}

	// Add element's own classes
	if classes != "" {
		parts = append(parts, strings.Fields(classes)...)
	}

	return strings.Join(parts, " ")
}

// Inline style resolution for style events:
//
// KP3 style events contain only "delta" properties - properties that differ from the
// parent element's style. The renderer inherits everything else from the parent.
//
// For inline styles like sub/sup inside headings, we use descendant selectors
// (e.g., h1--sub) that REPLACE the base class properties rather than merging with them.
// This is implemented in resolveInlineStyle() in frag_storyline_helpers.go.
//
// When a descendant selector exists for ALL parts of a style spec, those selectors
// are used exclusively. This ensures that h1--sub (with only baseline-style) doesn't
// inherit font-size: 0.75rem from the base sub style, allowing the heading's font-size
// to be inherited instead.

// isInheritedProperty returns true for CSS properties that inherit by default.
// Reference: https://developer.mozilla.org/en-US/docs/Web/CSS/inheritance
func isInheritedProperty(sym KFXSymbol) bool {
	switch sym {
	// Font properties
	case SymFontFamily, SymFontSize, SymFontWeight, SymFontStyle:
		return true
	// Text properties
	case SymTextAlignment, SymTextIndent, SymLineHeight:
		return true
	// Color
	case SymTextColor:
		return true
	// Spacing that inherits
	case SymLetterspacing:
		return true
	default:
		return false
	}
}

// isBlockInheritedProperty returns true for properties that should be inherited
// in block container contexts. Unlike standard CSS inheritance, this includes
// margin-left and margin-right so children are properly indented within containers.
// This matches KP3 behavior where container margins apply to each child element.
func isBlockInheritedProperty(sym KFXSymbol) bool {
	if isInheritedProperty(sym) {
		return true
	}
	switch sym {
	case SymMarginLeft, SymMarginRight:
		return true
	default:
		return false
	}
}

// extractMarginValue extracts a margin value in lh units from properties.
// Returns 0 if the property doesn't exist or isn't in lh units.
func extractMarginValue(props map[KFXSymbol]any, sym KFXSymbol) float64 {
	if val, ok := props[sym]; ok {
		if v, unit, ok := measureParts(val); ok && unit == SymUnitLh {
			return v
		}
	}
	return 0
}
