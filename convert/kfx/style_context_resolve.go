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

					// 1) Standard descendant selectors: ancestor--class or ancestor--tag
					styleName := anc + "--" + desc
					if def, ok := sc.registry.Get(styleName); ok {
						resolved := sc.registry.resolveInheritance(def)
						sc.registry.mergePropertiesWithContext(merged, resolved.Properties, mergeContextInline)
					}

					// 2) Element-qualified class selectors in descendant position.
					// CSS like ".section-title h2.section-title-header" should not apply to
					// other tags that share the class (e.g., h3.section-title-header).
					if tag != "" && desc != tag {
						qualified := anc + "--" + tag + "." + desc
						if def, ok := sc.registry.Get(qualified); ok {
							resolved := sc.registry.resolveInheritance(def)
							sc.registry.mergePropertiesWithContext(merged, resolved.Properties, mergeContextInline)
						}
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
// Empty-line handling:
// - Element after empty-line gets margin-top from empty-line (replaces own margin-top)
// - Element after empty-line keeps its margin-bottom (even if middle element)
//
// Note: Container margin distribution and title-block filtering are now handled by
// post-processing in CollapseMargins() for centralized margin logic. This method
// only handles CSS property resolution and empty-line margin injection.
//
// tag: HTML element type ("p", "h1", "span", etc.)
// classes: space-separated CSS classes (or "" for none)
// Returns the registered style name.
func (sc StyleContext) Resolve(tag, classes string) string {
	merged := sc.resolveProperties(tag, classes)

	// NOTE: We do NOT apply the pending empty-line margin here.
	// Empty-line margins are stored separately in ContentRef.EmptyLineMarginTop
	// and applied during post-processing (applyEmptyLineMargins) to avoid
	// font-size scaling that would occur if the margin was baked into the style.
	//
	// The pending margin is consumed (cleared) by the caller after Resolve(),
	// which then stores it in ContentRef.EmptyLineMarginTop for post-processing.

	return sc.registry.RegisterResolved(merged, styleUsageText, true)
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

	return sc.registry.RegisterResolved(merged, styleUsageText, false)
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
		if v, unit, ok := measureParts(val); ok {
			switch unit {
			case SymUnitLh:
				return v
			case SymUnitEm:
				// ExtractContainerMargins() expects lh units.
				// CSS converter already converts em -> lh by dividing by LineHeightRatio.
				return RoundSignificant(v/LineHeightRatio, SignificantFigures)
			}
		}
	}
	return 0
}

// isInlineOnlyProperty returns true for properties that are appropriate for inline
// style events. Block-level properties (margins, text-align, text-indent) should NOT
// appear in inline styles - they're inherited from the parent block element.
//
// KP3 style events contain only delta properties (what differs from parent), and
// these deltas never include block-level properties like margins or text alignment.
func isInlineOnlyProperty(sym KFXSymbol) bool {
	switch sym {
	// Block-level properties - NOT for inline styles
	case SymMarginLeft, SymMarginRight, SymMarginTop, SymMarginBottom:
		return false
	case SymTextAlignment, SymTextIndent:
		return false
	case SymPaddingLeft, SymPaddingRight, SymPaddingTop, SymPaddingBottom:
		return false
	// All other properties are OK for inline styles
	default:
		return true
	}
}

// isDropcapGlyphDeltaProperty returns true for properties that are appropriate for
// the first-glyph style_event in a dropcap paragraph.
//
// KP3 emits a style_event for the first glyph, but it only contains lightweight
// text styling (e.g., font-weight: bold). Dropcap geometry is driven by paragraph
// dropcap-* properties and should not be repeated inline.
func isDropcapGlyphDeltaProperty(sym KFXSymbol) bool {
	// Explicitly exclude dropcap geometry / layout properties.
	switch sym {
	case SymFontSize, SymLineHeight:
		return false
	case SymFloat, SymFloatClear:
		return false
	case SymPaddingLeft, SymPaddingRight, SymPaddingTop, SymPaddingBottom:
		return false
	default:
		return isInlineOnlyProperty(sym)
	}
}

// ResolveDropcapGlyphDelta creates a delta-only style for the first glyph in a
// dropcap paragraph.
//
// classes should typically be "has-dropcap--dropcap".
func (sc StyleContext) ResolveDropcapGlyphDelta(classes string) string {
	if sc.registry == nil {
		return ""
	}

	inlineProps := sc.resolveProperties("", classes)

	deltaProps := make(map[KFXSymbol]any)
	for sym, val := range inlineProps {
		if !isDropcapGlyphDeltaProperty(sym) {
			continue
		}
		if parentVal, hasParent := sc.inherited[sym]; hasParent {
			if propsEqual(val, parentVal) {
				continue
			}
		}
		deltaProps[sym] = val
	}

	if len(deltaProps) == 0 {
		return ""
	}

	// Register as delta style with inline usage; don't mark used here.
	return sc.registry.RegisterResolved(deltaProps, styleUsageInline, false)
}

// ResolveInlineDelta creates a delta-only style for inline style events.
// This matches KP3 behavior where style events contain only properties that differ
// from the parent element's style (the block/container style).
//
// The method:
// 1. Resolves the full inline style properties
// 2. Compares against the parent's resolved properties (stored in sc.inherited)
// 3. Filters out block-level properties (margins, text-align, text-indent)
// 4. Only includes line-height when font-size changes (for sub/sup)
//
// The style is NOT marked as used immediately - usage will be marked later
// after style event segmentation that may deduplicate some events.
//
// classes: space-separated CSS classes for the inline element (e.g., "strong", "sub link-footnote")
// Returns the registered style name for the delta-only style, or empty string if no delta.
func (sc StyleContext) ResolveInlineDelta(classes string) string {
	if sc.registry == nil {
		return ""
	}

	// Resolve the full inline style properties using the existing cascade logic.
	// This applies descendant selectors, inheritance, etc.
	inlineProps := sc.resolveProperties("", classes)

	// Build delta properties - only include properties that:
	// 1. Are appropriate for inline styles (not block-level)
	// 2. Differ from the parent's inherited properties
	deltaProps := make(map[KFXSymbol]any)

	// Track if font-size changed - we only include line-height when font-size differs
	fontSizeChanged := false
	if inlineFontSize, hasInline := inlineProps[SymFontSize]; hasInline {
		if parentFontSize, hasParent := sc.inherited[SymFontSize]; hasParent {
			fontSizeChanged = !propsEqual(inlineFontSize, parentFontSize)
		} else {
			fontSizeChanged = true // No parent font-size means it changed
		}
	}

	// KP3 special case: when a link-footnote is inside a superscript context (e.g., <sup><a>),
	// the font-size from link-footnote CSS (0.8em) should NOT compound with the parent's
	// superscript font-size (0.75rem).
	//
	// KP3 behavior differs based on context:
	// 1. In normal paragraphs: link inherits sup's font-size (0.75rem)
	// 2. In headings with explicit <sup><a>: link uses heading_font × 0.7
	// 3. In headings with direct <a> (link-footnote): link uses heading_font × 0.9 (same as sup)
	//
	// Detection: classes are like "sup link-footnote" where "sup" comes before "link-footnote".
	// We resolve the context classes (before link-footnote) to get their font-size/line-height.
	var contextFontSize, contextLineHeight any
	if fontSizeChanged && strings.Contains(classes, "link-footnote") {
		// Extract classes before "link-footnote" to check ancestor context
		parts := strings.Fields(classes)
		var contextClasses []string
		for _, p := range parts {
			if p == "link-footnote" {
				break
			}
			contextClasses = append(contextClasses, p)
		}

		// Check if we're in a heading context
		var headingFontSize any
		for _, scope := range sc.scopes {
			if isHeadingTag(scope.Tag) {
				// Get heading's font-size from registry
				if headingDef, ok := sc.registry.Get(scope.Tag); ok {
					resolved := sc.registry.resolveInheritance(headingDef)
					if fs, ok := resolved.Properties[SymFontSize]; ok {
						headingFontSize = fs
					}
				}
				break // Use innermost heading
			}
		}

		// Check if ancestor context (classes before link-footnote) has superscript baseline
		hasSupContext := false
		if len(contextClasses) > 0 {
			// Get the tag from our scope chain - this is important for heading contexts
			// where h1--sup has different properties than regular sup
			var tag string
			if len(sc.scopes) > 0 {
				tag = sc.scopes[len(sc.scopes)-1].Tag
			}
			contextProps := sc.resolveProperties(tag, strings.Join(contextClasses, " "))
			if baseline, ok := contextProps[SymBaselineStyle]; ok {
				hasSupContext = symbolToInt(baseline) == int(SymSuperscript)
			}
		}

		if headingFontSize != nil {
			// In heading context - calculate font-size based on heading
			if headingVal, headingUnit, ok := measureParts(headingFontSize); ok && headingUnit == SymUnitRem {
				if hasSupContext {
					// <sup><a> in heading: link uses heading × 0.7
					contextFontSize = DimensionValue(headingVal*0.7, SymUnitRem)
				} else {
					// Direct <a> (link-footnote) in heading: link uses heading × 0.9 (same as sup)
					// This matches KP3 behavior where link-footnote in heading gets sup factor
					contextFontSize = DimensionValue(headingVal*0.9, SymUnitRem)
				}
			}
		} else if hasSupContext {
			// Not in heading context but has sup - use sup's font-size (inherited from parent)
			var tag string
			if len(sc.scopes) > 0 {
				tag = sc.scopes[len(sc.scopes)-1].Tag
			}
			contextProps := sc.resolveProperties(tag, strings.Join(contextClasses, " "))
			if fs, ok := contextProps[SymFontSize]; ok {
				contextFontSize = fs
			} else if pfs, ok := sc.inherited[SymFontSize]; ok {
				contextFontSize = pfs
			}

			// Line-height from context
			if lh, ok := contextProps[SymLineHeight]; ok {
				contextLineHeight = lh
			} else if plh, ok := sc.inherited[SymLineHeight]; ok {
				contextLineHeight = plh
			}
		}
	}

	for sym, val := range inlineProps {
		// Skip block-level properties
		if !isInlineOnlyProperty(sym) {
			continue
		}

		// For link-footnote in superscript context, use context's font-size/line-height
		// instead of the compounded values from link-footnote CSS
		if contextFontSize != nil && sym == SymFontSize {
			val = contextFontSize
		}
		if contextLineHeight != nil && sym == SymLineHeight {
			val = contextLineHeight
		}

		// Special handling for line-height: only include if font-size changed.
		// For normal bold/italic, line-height should be inherited from parent.
		// For sub/sup with different font-size, line-height needs to be set
		// to maintain proper vertical rhythm (KP3 uses 1.33249lh for superscript).
		//
		// Also skip line-height if the parent doesn't have it - the parent's
		// line-height will be added during BuildFragments (via ensureDefaultLineHeight
		// or adjustLineHeightForFontSize), so including it in the delta would be
		// redundant or incorrect.
		if sym == SymLineHeight {
			if !fontSizeChanged {
				continue
			}
			if _, hasParent := sc.inherited[sym]; !hasParent {
				continue
			}
		}

		// Check if this property differs from parent's inherited value
		if parentVal, hasParent := sc.inherited[sym]; hasParent {
			if propsEqual(val, parentVal) {
				continue // Same as parent, don't include in delta
			}
		}

		// Property differs from parent or parent doesn't have it - include in delta
		deltaProps[sym] = val
	}

	// KP3 behavior: Calculate line-height to maintain constant absolute line spacing.
	// When an inline element has a different font-size than its parent, KP3 calculates:
	//   line-height = parent_lh × (parent_font / inline_font)
	// This keeps the absolute line spacing the same as the parent block.
	//
	// Example: sup in h1 (1.25rem heading, 1.125rem sup)
	//   line-height = 1.0101lh × (1.25 / 1.125) = 1.12205lh
	//
	// Only apply this when:
	// 1. Font-size is in the delta (different from parent)
	// 2. Parent has both font-size and line-height
	if _, hasFontSize := deltaProps[SymFontSize]; hasFontSize {
		parentFontSize, hasParentFont := sc.inherited[SymFontSize]
		parentLineHeight, hasParentLh := sc.inherited[SymLineHeight]
		if hasParentFont && hasParentLh {
			parentFontVal, parentFontUnit, parentFontOk := measureParts(parentFontSize)
			inlineFontVal, inlineFontUnit, inlineFontOk := measureParts(deltaProps[SymFontSize])
			parentLhVal, parentLhUnit, parentLhOk := measureParts(parentLineHeight)

			// Only calculate if we have valid dimensions and compatible units
			if parentFontOk && inlineFontOk && parentLhOk &&
				parentFontUnit == SymUnitRem && inlineFontUnit == SymUnitRem &&
				parentLhUnit == SymUnitLh && inlineFontVal > 0 {

				// Calculate ratio-based line-height to maintain absolute spacing
				ratio := parentFontVal / inlineFontVal
				adjustedLh := parentLhVal * ratio

				// Round to LineHeightPrecision decimal places (same as adjustLineHeightForFontSize)
				adjustedLh = RoundDecimals(adjustedLh, LineHeightPrecision)

				deltaProps[SymLineHeight] = DimensionValue(adjustedLh, SymUnitLh)
			}
		}
	}

	// If no delta properties, return empty string - no style event needed.
	// This matches KP3 behavior where style events are only created for actual
	// styling differences, not for inherited-only styles.
	if len(deltaProps) == 0 {
		return ""
	}

	// Register as delta style with inline usage to prevent automatic line-height addition.
	// Don't mark as used - that happens later after style event deduplication.
	return sc.registry.RegisterResolved(deltaProps, styleUsageInline, false)
}

// propsEqual compares two property values for equality.
// Handles dimension values (with units) and symbol values.
// Note: KFXSymbol and SymbolValue are different Go types but represent the same
// underlying KFX symbol values, so we need to handle cross-type comparisons.
func propsEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Try dimension comparison first
	aVal, aUnit, aOk := measureParts(a)
	bVal, bUnit, bOk := measureParts(b)
	if aOk && bOk {
		// Both are dimensions - compare value and unit
		return aVal == bVal && aUnit == bUnit
	}

	// Try symbol comparison - handle both KFXSymbol and SymbolValue types.
	// These are different Go types but represent the same KFX symbols.
	aSymVal := symbolToInt(a)
	bSymVal := symbolToInt(b)
	if aSymVal >= 0 && bSymVal >= 0 {
		return aSymVal == bSymVal
	}

	// Fall back to reflect.DeepEqual for other types
	return a == b
}

// symbolToInt extracts the integer symbol value from KFXSymbol or SymbolValue.
// Returns -1 if the value is not a symbol type.
func symbolToInt(v any) int {
	switch sv := v.(type) {
	case KFXSymbol:
		return int(sv)
	case SymbolValue:
		return int(sv)
	default:
		return -1
	}
}

// isHeadingTag returns true if the tag is an HTML heading element (h1-h6).
func isHeadingTag(tag string) bool {
	switch tag {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	}
	return false
}
