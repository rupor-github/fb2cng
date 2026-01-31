package kfx

// tableElementProperties are properties that KFX requires to be on the table element,
// NOT in the style. KP3 moves these from style to element during table processing.
// See com/amazon/adapter/common/l/a/c/e.java lines 16-18 in KP3 source.
var tableElementProperties = map[KFXSymbol]bool{
	SymTableBorderCollapse:     true,
	SymBorderSpacingHorizontal: true,
	SymBorderSpacingVertical:   true,
}

// TableElementProps holds properties that should be set on the table element,
// extracted from CSS before filtering them from the style.
type TableElementProps struct {
	BorderCollapse    bool
	BorderSpacingH    any // DimensionValue or nil
	BorderSpacingV    any // DimensionValue or nil
	HasBorderCollapse bool
}

// GetTableElementProps extracts table element properties from the resolved "table" style.
// These properties are moved from style to element per KP3 behavior.
// Returns extracted values with defaults for missing properties.
func (sr *StyleRegistry) GetTableElementProps() TableElementProps {
	result := TableElementProps{
		BorderCollapse:    true,                           // default: collapse
		BorderSpacingH:    DimensionValue(0.9, SymUnitPt), // default: 0.9pt (Amazon reference)
		BorderSpacingV:    DimensionValue(0.9, SymUnitPt), // default: 0.9pt
		HasBorderCollapse: false,
	}

	// Get the merged "table" style properties before filtering
	sr.EnsureBaseStyle("table")
	def, exists := sr.styles["table"]
	if !exists {
		return result
	}
	resolved := sr.resolveInheritance(def)

	// Extract border_spacing if present in CSS
	if v, ok := resolved.Properties[SymBorderSpacingHorizontal]; ok {
		result.BorderSpacingH = v
	}
	if v, ok := resolved.Properties[SymBorderSpacingVertical]; ok {
		result.BorderSpacingV = v
	}

	// Extract table_border_collapse if present
	// CSS border-collapse: collapse -> true, separate -> false
	if v, ok := resolved.Properties[SymTableBorderCollapse]; ok {
		result.HasBorderCollapse = true
		if bv, ok := v.(bool); ok {
			result.BorderCollapse = bv
		}
	}

	return result
}
