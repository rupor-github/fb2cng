package kfx

import "maps"

// EnsureBaseStyle ensures a style exists, creating a minimal one if needed.
// Unlike EnsureStyle it does NOT mark the style as used for output.
// This is used for resolving style combinations into KP3-like "s.." styles.
func (sr *StyleRegistry) EnsureBaseStyle(name string) {
	if _, exists := sr.styles[name]; exists {
		return
	}

	// Try to infer parent style from naming conventions
	parent := sr.inferParentStyle(name)

	// Log auto-creation of unknown styles (not defined in CSS)
	sr.tracer.TraceAutoCreate(name, parent)

	// These auto-created styles may be created after sr.PostProcessForKFX() has
	// already run (e.g. from runtime EnsureBaseStyle calls while building content).
	// Apply the same KFX enhancements here so late-created styles match KP3 output.
	def := NewStyle(name).
		Inherit(parent).
		Build()
	def = sr.applyKFXEnhancements(name, def)
	sr.Register(def)
}

// ResolveStyle ensures a style exists, resolves it to a generated name, and tracks usage type.
// Returns the generated style name (like "s1J") that should be used in KFX output.
//
// The usage parameter (styleUsageText, styleUsageImage, styleUsageWrapper) affects how the
// style is post-processed in BuildFragments (e.g., text styles get line-height adjustments).
//
// Note: This does NOT mark the style as "used" for output filtering. Call RecomputeUsedStyles
// before BuildFragments to determine which styles are actually referenced in the final content.
func (sr *StyleRegistry) ResolveStyle(name string, usage styleUsage) string {
	if name == "" {
		return ""
	}
	sr.EnsureBaseStyle(name)

	// Resolve inheritance to get final properties
	def := sr.styles[name]
	resolved := sr.resolveInheritance(def)

	// Filter out height: auto - KP3 never outputs this in styles (it's the implied default).
	// The value may be stored as either KFXSymbol or SymbolValue depending on source.
	props := resolved.Properties
	if h, ok := props[SymHeight]; ok {
		isAuto := false
		switch v := h.(type) {
		case SymbolValue:
			isAuto = KFXSymbol(v) == SymAuto
		case KFXSymbol:
			isAuto = v == SymAuto
		}
		if isAuto {
			// Make a copy to avoid modifying the original
			props = make(map[KFXSymbol]any, len(resolved.Properties))
			maps.Copy(props, resolved.Properties)
			delete(props, SymHeight)
		}
	}

	// Check if we already have a generated name for this property set
	sig := styleSignature(props)
	if genName, ok := sr.resolved[sig]; ok {
		// Don't add styleUsageText to inline-only styles - they should inherit
		// line-height from parent, not get the default line-height added.
		if usage == styleUsageText && sr.hasInlineUsage(genName) {
			// Just mark as used, don't change usage type
		} else {
			sr.usage[genName] = sr.usage[genName] | usage
		}
		return genName
	}

	// Generate a new name and register the resolved style
	genName := sr.nextResolvedStyleName()
	sr.resolved[sig] = genName
	sr.usage[genName] = usage
	sr.Register(StyleDef{Name: genName, Properties: props})
	return genName
}

func (sr *StyleRegistry) hasTextUsage(name string) bool {
	return sr.usage[name]&styleUsageText != 0
}

func (sr *StyleRegistry) hasInlineUsage(name string) bool {
	return sr.usage[name]&styleUsageInline != 0
}

func (sr *StyleRegistry) hasImageUsage(name string) bool {
	return sr.usage[name]&styleUsageImage != 0
}

// GetUsage returns the usage flags for a style. Used when creating style variants
// that should inherit the original style's usage type.
func (sr *StyleRegistry) GetUsage(name string) styleUsage {
	return sr.usage[name]
}

func (sr *StyleRegistry) nextResolvedStyleName() string {
	sr.resolvedCounter++
	return "s" + toBase36(sr.resolvedCounter)
}

// ResolveCoverImageStyle creates a minimal style for cover images in container-type sections.
// Unlike block images, this doesn't include width constraints since the page template
// already defines the container dimensions. Reference KFX cover image style has:
//   - font_size: 1rem (in unit $505)
//   - line_height: 1.0101lh
func (sr *StyleRegistry) ResolveCoverImageStyle() string {
	// Build minimal properties matching KP3 cover image style exactly
	props := map[KFXSymbol]any{
		SymFontSize:   DimensionValue(1, SymUnitRem),                  // font-size: 1rem
		SymLineHeight: DimensionValue(DefaultLineHeightLh, SymUnitLh), // line-height: 1.0101lh
	}

	sig := styleSignature(props)
	if name, ok := sr.resolved[sig]; ok {
		sr.used[name] = true
		return name
	}

	name := sr.nextResolvedStyleName()
	sr.resolved[sig] = name
	sr.used[name] = true
	sr.Register(StyleDef{Name: name, Properties: props})
	return name
}

// resolveInheritance flattens a style by merging all parent properties.
// Child properties override parent properties. Handles inheritance chains.
// Always returns a new StyleDef with a copied Properties map to prevent
// mutations from affecting the original styles in the registry.
func (sr *StyleRegistry) resolveInheritance(def StyleDef) StyleDef {
	if def.Parent == "" {
		// Even with no inheritance, we must return a copy of Properties
		// to prevent callers from mutating the registry's stored style.
		copied := make(map[KFXSymbol]any, len(def.Properties))
		maps.Copy(copied, def.Properties)
		return StyleDef{
			Name:       def.Name,
			Parent:     def.Parent,
			Properties: copied,
		}
	}

	// Build inheritance chain (child -> parent -> grandparent -> ...)
	chain := []StyleDef{def}
	visited := map[string]bool{def.Name: true}

	current := def
	for current.Parent != "" {
		if visited[current.Parent] {
			// Circular inheritance detected - break the chain
			break
		}
		parent, exists := sr.styles[current.Parent]
		if !exists {
			// Parent not found - stop here
			break
		}
		visited[current.Parent] = true
		chain = append(chain, parent)
		current = parent
	}

	// Merge properties from root ancestor to child (child overrides parent)
	merged := make(map[KFXSymbol]any)
	for i := len(chain) - 1; i >= 0; i-- {
		maps.Copy(merged, chain[i].Properties)
	}

	sr.tracer.TraceInheritance(def.Name, def.Parent, merged)

	return StyleDef{
		Name:       def.Name,
		Parent:     "", // Flattened - no parent needed
		Properties: merged,
	}
}
