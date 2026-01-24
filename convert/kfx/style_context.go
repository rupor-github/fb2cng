package kfx

import (
	"maps"
	"strings"
)

// StyleScope represents a single level in the element hierarchy.
// It captures both the element tag and its classes at that level.
type StyleScope struct {
	Tag     string   // HTML element tag: "div", "p", "h1", "span", etc.
	Classes []string // CSS classes applied to this element
}

// marginOrigin tracks which styles contributed to an inherited margin value.
// This is used to implement YJCumulativeInSameContainerRuleMerger correctly:
// - Same style contributing twice → override (don't double-count)
// - Different styles contributing → accumulate
type marginOrigin struct {
	value        any             // The margin value
	contributors map[string]bool // Style names that contributed to this margin
}

// emptyLineState holds mutable state for empty line margin handling.
// This is shared across all container frames to allow margin propagation
// across container boundaries (e.g., empty line before a poem affects
// the poem's first verse).
type emptyLineState struct {
	// pendingMargin is the margin from the last empty line that should
	// be applied to the next content element's margin-top.
	pendingMargin float64

	// keepMarginBottom, when true, signals that subsequent elements should
	// keep their margin-bottom even if position filtering would normally
	// remove it (for middle elements). Once set by an empty line, this
	// stays true until container processing completes.
	keepMarginBottom bool
}

// containerFrame holds margin info for a single container level in the stack.
// When entering a container (poem, stanza, cite, etc.), a frame is pushed.
// When processing completes, the frame is effectively "popped" via Go's value semantics.
type containerFrame struct {
	marginTop         float64 // Container's margin-top (for first child)
	marginBottom      float64 // Container's margin-bottom (for last child)
	itemCount         int     // Total items in this container
	currentItem       int     // Current item index (0-based)
	isLastInParent    bool    // Whether this container is the last item in its parent
	titleBlockMargins bool    // If true, use title-block margin style (spacing via margin-top)
}

// StyleContext accumulates inherited CSS properties as we descend the element hierarchy.
// This mimics how browsers propagate inherited properties from parent to child.
//
// In CSS, some properties (font-*, color, text-align, line-height, etc.) automatically
// inherit from parent to child elements. Other properties (margin, padding, border, etc.)
// do NOT inherit - they apply only to the element where they're defined.
//
// When resolving a style for an element:
// 1. Inherited properties come from the accumulated context (ancestors)
// 2. Non-inherited properties come only from the element's own tag/classes
//
// StyleContext also supports position tracking for block contexts. When processing
// items within a block (annotation, epigraph, etc.), the containerStack tracks position
// and applies KP3-compatible position filtering (margin collapsing simulation).
//
// Empty-line handling: Instead of creating content entries for empty-lines, we store
// their margin in emptyLineState.pendingMargin. The next element's margin-top is
// set to the empty-line's margin, and its margin-bottom is preserved (not removed
// by position filtering), matching KP3 behavior. The emptyLineState is shared across
// all context copies to allow propagation across container boundaries.
//
// Container identity tracking: For margin-left/right, we track which style names
// contributed to the inherited value. This implements YJCumulativeInSameContainerRuleMerger:
// when resolving an element's style, if the same style name already contributed to the
// inherited margin, we don't accumulate again (override instead). Different style names
// DO accumulate (e.g., poem margin + verse margin).
type StyleContext struct {
	// registry is the style registry used for style lookups and registration.
	registry *StyleRegistry

	// Inherited properties accumulated from ancestors.
	// Only CSS-inherited properties are stored here.
	inherited map[KFXSymbol]any

	// marginOrigins tracks which styles contributed to inherited margin-left/right values.
	// Key is SymMarginLeft or SymMarginRight, value is marginOrigin with contributor tracking.
	// This enables YJCumulativeInSameContainerRuleMerger to correctly decide whether to
	// accumulate (different contributors) or override (same contributor).
	marginOrigins map[KFXSymbol]*marginOrigin

	// Full scope chain from root to current level (for debugging/future use)
	scopes []StyleScope

	// containerStack is a stack of container frames for nested margin handling.
	// When entering a container (poem, stanza, cite, etc.), push a frame.
	// When exiting, it's effectively popped via Go's value semantics.
	// Top of stack is used for vertical margin distribution.
	containerStack []containerFrame

	// emptyLine holds shared state for empty line margin handling.
	// This is a pointer to allow mutation across value copies of StyleContext.
	// All contexts within a processing scope share the same emptyLineState.
	emptyLine *emptyLineState
}

// NewStyleContext creates an empty context (root level) with the given style registry.
// Empty-line tracking is enabled by default for all contexts.
func NewStyleContext(registry *StyleRegistry) StyleContext {
	return StyleContext{
		registry:      registry,
		inherited:     make(map[KFXSymbol]any),
		marginOrigins: make(map[KFXSymbol]*marginOrigin),
		scopes:        nil,
		emptyLine:     &emptyLineState{}, // Shared empty line state
	}
}

// ScopePath returns a CSS-like path string showing the element hierarchy.
// Example: "div.poem > div.stanza" or "body > p.verse"
func (sc StyleContext) ScopePath() string {
	if len(sc.scopes) == 0 {
		return "(root)"
	}
	parts := make([]string, 0, len(sc.scopes))
	for _, scope := range sc.scopes {
		part := scope.Tag
		if part == "" {
			part = "*"
		}
		for _, class := range scope.Classes {
			part += "." + class
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " > ")
}

// ContainerPath returns a summary of the container stack with positions.
// Example: "poem[2/3] > stanza[5/14]" showing current item / total items.
func (sc StyleContext) ContainerPath() string {
	if len(sc.containerStack) == 0 {
		return "(no containers)"
	}
	parts := make([]string, 0, len(sc.containerStack))
	for i, frame := range sc.containerStack {
		// Try to get a name from the corresponding scope
		name := "container"
		if i < len(sc.scopes) {
			scope := sc.scopes[i]
			if len(scope.Classes) > 0 {
				name = scope.Classes[len(scope.Classes)-1] // Use last class as name
			} else if scope.Tag != "" {
				name = scope.Tag
			}
		}
		parts = append(parts, formatContainerFrame(name, frame))
	}
	return strings.Join(parts, " > ")
}

// formatContainerFrame formats a single container frame for display.
func formatContainerFrame(name string, frame containerFrame) string {
	pos := frame.currentItem + 1 // 1-based for display
	total := frame.itemCount
	result := name + "[" + itoa(pos) + "/" + itoa(total) + "]"
	if frame.titleBlockMargins {
		result += "*" // Mark title-block mode
	}
	return result
}

// itoa is a simple int-to-string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// Push enters a new element scope and returns a new context with that element's
// inherited properties added. Non-inherited properties are ignored for inheritance.
//
// tag: HTML element type ("div", "p", "h1", etc.)
// classes: space-separated CSS classes ("section poem" or "" for none)
func (sc StyleContext) Push(tag, classes string) StyleContext {
	// Copy existing inherited properties
	newInherited := make(map[KFXSymbol]any, len(sc.inherited))
	maps.Copy(newInherited, sc.inherited)

	// Add inherited properties from tag defaults
	if tag != "" && sc.registry != nil {
		if def, ok := sc.registry.Get(tag); ok {
			resolved := sc.registry.resolveInheritance(def)
			for sym, val := range resolved.Properties {
				if isInheritedProperty(sym) {
					newInherited[sym] = val
				}
			}
		}
	}

	// Parse and add inherited properties from each class
	var classList []string
	if classes != "" {
		classList = strings.Fields(classes)
		for _, class := range classList {
			if sc.registry != nil {
				sc.registry.EnsureBaseStyle(class)
				if def, ok := sc.registry.Get(class); ok {
					resolved := sc.registry.resolveInheritance(def)
					for sym, val := range resolved.Properties {
						if isInheritedProperty(sym) {
							newInherited[sym] = val
						}
					}
				}
			}
		}
	}

	// Append to scope chain
	newScopes := append(sc.scopes, StyleScope{Tag: tag, Classes: classList})

	return StyleContext{
		registry:  sc.registry,
		inherited: newInherited,
		scopes:    newScopes,
		emptyLine: sc.emptyLine, // Preserve empty-line tracking
	}
}

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

// Advance moves to the next item in the current container.
// Call this after processing each item in a container.
// Returns a new StyleContext with updated position in the top container frame.
//
// If there's no container stack, this is a no-op.
func (sc StyleContext) Advance() StyleContext {
	if len(sc.containerStack) == 0 {
		return sc // No container stack - nothing to advance
	}

	// Copy stack and increment current item in top frame
	newStack := make([]containerFrame, len(sc.containerStack))
	copy(newStack, sc.containerStack)
	newStack[len(newStack)-1].currentItem++

	return StyleContext{
		registry:       sc.registry,
		inherited:      sc.inherited,
		marginOrigins:  sc.marginOrigins,
		scopes:         sc.scopes,
		containerStack: newStack,
		emptyLine:      sc.emptyLine,
	}
}

// HasPosition returns true if this context has position information set.
// This is true when a container stack with tracked positions is present.
func (sc StyleContext) HasPosition() bool {
	return len(sc.containerStack) > 0
}

// AddEmptyLineMargin adds margin from an empty-line element to the pending margin.
// This margin will be set as the next element's margin-top when Resolve() is called.
// The margin value should be in line-height units (lh).
//
// Additionally, this sets keepMarginBottom so the next element preserves its
// margin-bottom even if position filtering would normally remove it.
//
// This method modifies shared state, so all copies of this context within
// the same processing scope will see the changes.
func (sc StyleContext) AddEmptyLineMargin(margin float64) {
	// emptyLine is always initialized by NewStyleContext() and preserved across copies
	if sc.emptyLine == nil {
		return // Safety check - should never happen
	}
	// Replace (not accumulate) - empty-line margin overwrites the following
	// element's margin-top, matching KP3 reference behavior.
	sc.emptyLine.pendingMargin = margin
	sc.emptyLine.keepMarginBottom = true
}

// ConsumePendingMargin returns and clears any accumulated empty-line margin.
// Returns 0 if no margin is pending.
func (sc StyleContext) ConsumePendingMargin() float64 {
	if sc.emptyLine == nil {
		return 0 // Safety check - should never happen
	}
	margin := sc.emptyLine.pendingMargin
	sc.emptyLine.pendingMargin = 0
	return margin
}

// ConsumeKeepMarginBottom returns the keepMarginBottom flag without clearing it.
// Returns true if the current element should keep its margin-bottom.
//
// Once an empty-line is encountered, ALL subsequent elements in the block
// keep their margin-bottom (not just the immediate next element). The flag
// stays true until the block processing completes.
func (sc StyleContext) ConsumeKeepMarginBottom() bool {
	if sc.emptyLine == nil {
		return false // Safety check - should never happen
	}
	// Don't clear the flag - once set by empty-line, all subsequent
	// elements in the block should keep their margin-bottom
	return sc.emptyLine.keepMarginBottom
}

// HasPendingMargin returns true if there's accumulated empty-line margin to apply.
func (sc StyleContext) HasPendingMargin() bool {
	if sc.emptyLine == nil {
		return false // Safety check - should never happen
	}
	return sc.emptyLine.pendingMargin > 0
}

// GetEmptyLineMargin extracts the margin-top value from the emptyline style.
// This returns only the emptyline's own margin, without any inherited context.
// Returns the margin value in lh units, or 0 if not found.
func (sc StyleContext) GetEmptyLineMargin(styleSpec string, registry *StyleRegistry) float64 {
	// Look up the emptyline style directly without context inheritance
	if def, ok := registry.Get(styleSpec); ok {
		resolved := registry.resolveInheritance(def)
		if mt, ok := resolved.Properties[SymMarginTop]; ok {
			if val, unit, ok := measureParts(mt); ok && unit == SymUnitLh {
				return val
			}
		}
	}
	return 0
}

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

// ResolveImage creates the final style for an image element within this context.
// Unlike Resolve(), this does NOT inherit from kfx-unknown (images don't need line-height).
// Position-based margin filtering is applied from the container stack.
//
// classes: space-separated CSS classes (e.g., "image-vignette")
// Returns the registered style name.
func (sc StyleContext) ResolveImage(classes string) string {
	merged := make(map[KFXSymbol]any)

	if sc.registry == nil {
		return ""
	}

	// Resolve classes directly - no kfx-unknown base, no tag defaults
	// Images only get properties from their specific classes
	for class := range strings.FieldsSeq(classes) {
		if def, ok := sc.registry.Get(class); ok {
			resolved := sc.registry.resolveInheritance(def)
			for k, v := range resolved.Properties {
				// Skip text-specific properties that don't apply to images
				switch k {
				case SymLineHeight, SymTextIndent, SymTextAlignment:
					continue
				}
				merged[k] = v
			}
		}
	}

	// Apply vertical margin distribution from container stack
	sc.applyContainerMargins(merged)

	// Use RegisterResolvedRaw to avoid adding kfx-unknown base (no line-height for images)
	// Standard filtering (height: auto, table props) is still applied
	return sc.registry.RegisterResolvedRaw(merged)
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
