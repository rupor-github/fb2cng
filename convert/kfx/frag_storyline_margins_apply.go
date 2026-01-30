package kfx

import (
	"fmt"
)

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
		newStyle := sb.styles.RegisterResolved(props, originalUsage, true)
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
		newStyle := sb.styles.RegisterResolved(props, originalUsage, true)

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
