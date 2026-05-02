package kfx

import (
	"maps"
	"strings"
)

// PushBlock enters a block container scope with margin-left/right accumulation.
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
// Vertical margins (margin-top/bottom) are handled by post-processing in CollapseMargins(),
// not by this method. Use ExtractContainerMargins() to get container margins for
// SetContainerMargins() calls.
func (sc StyleContext) PushBlock(tag, classes string) StyleContext {
	// Copy existing inherited properties
	newInherited := make(map[KFXSymbol]any, len(sc.inherited))
	maps.Copy(newInherited, sc.inherited)

	// Track accumulated font-size (same logic as Push)
	newAccumEm := sc.fontSizeAccumEm

	// Copy existing margin origins (deep copy to preserve contributor sets)
	newMarginOrigins := copyMarginOrigins(sc.marginOrigins)

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
			for sym, val := range resolved.Properties {
				mergeBlockProperty(sym, val, tag)
			}
			// Update accumulated font-size from tag
			if fs, ok := resolved.Properties[SymFontSize]; ok {
				newAccumEm = sc.fontSizeAccumEm * FontSizeMultiplier(fs)
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
					for sym, val := range resolved.Properties {
						mergeBlockProperty(sym, val, class)
					}
					// Update accumulated font-size from class
					if fs, ok := resolved.Properties[SymFontSize]; ok {
						mult := FontSizeMultiplier(fs)
						if _, unit, ok := measureParts(fs); ok && unit == SymUnitEm {
							newAccumEm *= mult
						} else {
							newAccumEm = sc.fontSizeAccumEm * mult
						}
					}
				}
			}
		}
	}

	// Append to scope chain
	newScopes := append(sc.scopes, StyleScope{Tag: tag, Classes: classList})

	// Build the new context
	return StyleContext{
		registry:        sc.registry,
		inherited:       newInherited,
		marginOrigins:   newMarginOrigins,
		fontSizeAccumEm: newAccumEm,
		scopes:          newScopes,
		emptyLine:       sc.emptyLine, // Preserve empty-line tracking
	}
}

// WithoutRootHorizontalMargins returns a copy of this context with horizontal
// margins contributed by synthetic html/body root scopes removed.
//
// This keeps root inherited properties and root descendant-selector scopes intact,
// but prevents page/root content insets from being baked into generated/nested
// structures where KP3 does not propagate them (for example title paragraphs that
// are wrappers around inline/image-only title art).
func (sc StyleContext) WithoutRootHorizontalMargins() StyleContext {
	out := StyleContext{
		registry:        sc.registry,
		inherited:       make(map[KFXSymbol]any, len(sc.inherited)),
		marginOrigins:   copyMarginOrigins(sc.marginOrigins),
		fontSizeAccumEm: sc.fontSizeAccumEm,
		scopes:          append([]StyleScope(nil), sc.scopes...),
		emptyLine:       sc.emptyLine,
	}
	maps.Copy(out.inherited, sc.inherited)

	for _, sym := range []KFXSymbol{SymMarginLeft, SymMarginRight} {
		origin := out.marginOrigins[sym]
		if origin == nil || (!origin.contributors["html"] && !origin.contributors["body"]) {
			continue
		}

		rootOnly := true
		for contributor := range origin.contributors {
			if contributor != "html" && contributor != "body" {
				rootOnly = false
				break
			}
		}
		if rootOnly {
			delete(out.inherited, sym)
			delete(out.marginOrigins, sym)
			continue
		}

		rootValue, ok := sc.rootHorizontalMargin(sym)
		if !ok {
			continue
		}
		adjusted, ok := subtractCumulative(out.inherited[sym], rootValue)
		if !ok {
			continue
		}
		if isZeroMargin(adjusted) {
			delete(out.inherited, sym)
		} else {
			out.inherited[sym] = adjusted
		}
		origin.value = adjusted
		delete(origin.contributors, "html")
		delete(origin.contributors, "body")
	}

	return out
}

func (sc StyleContext) rootHorizontalMargin(sym KFXSymbol) (any, bool) {
	if sc.registry == nil {
		return nil, false
	}

	var (
		rootValue any
		hasRoot   bool
	)
	for _, styleName := range []string{"html", "body"} {
		def, ok := sc.registry.Get(styleName)
		if !ok {
			continue
		}
		resolved := sc.registry.resolveInheritance(def)
		val, ok := resolved.Properties[sym]
		if !ok {
			continue
		}
		if !hasRoot {
			rootValue = val
			hasRoot = true
			continue
		}
		if merged, ok := mergeCumulative(rootValue, val); ok {
			rootValue = merged
		} else {
			rootValue = val
		}
	}
	return rootValue, hasRoot
}

func subtractCumulative(existing, subtract any) (any, bool) {
	if adjusted, ok := mergeMeasure(existing, subtract, func(ev, sv float64) float64 { return ev - sv }); ok {
		return adjusted, true
	}

	ev, eok := numericFromAny(existing)
	sv, sok := numericFromAny(subtract)
	if eok && sok {
		return ev - sv, true
	}
	return nil, false
}

// ExtractContainerMargins resolves the CSS for a container and returns its vertical margins.
// This is used to pass container margins to StorylineBuilder.SetContainerMargins()
// for post-processing margin collapsing.
//
// tag: HTML element type for the container (e.g., "div", "blockquote")
// classes: space-separated CSS classes (e.g., "poem", "annotation")
// Returns (marginTop, marginBottom) in line-height units.
func (sc StyleContext) ExtractContainerMargins(tag, classes string) (mt, mb float64) {
	if sc.registry == nil {
		return 0, 0
	}

	// Ensure base styles exist for container classes.
	// Without this, classes that are only ever used for container margin extraction
	// (i.e., not applied to any content element) may never be registered, causing
	// container margins to silently resolve to 0.
	if classes != "" {
		for class := range strings.FieldsSeq(classes) {
			sc.registry.EnsureBaseStyle(class)
		}
	}

	// Collect container's resolved properties
	var containerProps map[KFXSymbol]any

	// Get properties from tag
	if tag != "" {
		if def, ok := sc.registry.Get(tag); ok {
			resolved := sc.registry.resolveInheritance(def)
			containerProps = resolved.Properties
		}
	}

	// Get properties from classes (override tag properties)
	if classes != "" {
		classList := strings.FieldsSeq(classes)
		for class := range classList {
			if def, ok := sc.registry.Get(class); ok {
				resolved := sc.registry.resolveInheritance(def)
				if containerProps == nil {
					containerProps = make(map[KFXSymbol]any)
				}
				maps.Copy(containerProps, resolved.Properties)
			}
		}
	}

	// Extract margin values
	mt = extractMarginValue(containerProps, SymMarginTop)
	mb = extractMarginValue(containerProps, SymMarginBottom)
	return mt, mb
}
