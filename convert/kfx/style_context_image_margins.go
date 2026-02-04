package kfx

// Helpers for KP3-parity image margin tweaks.

// ensureFixedBlockImageMargins returns a resolved style name that is identical to baseStyle,
// but with margin-top/margin-bottom set to the given lh values.
//
// If styles is nil or the style can't be found, it returns baseStyle unchanged.
func ensureFixedBlockImageMargins(styles *StyleRegistry, baseStyle string, mtLh, mbLh float64) string {
	if styles == nil || baseStyle == "" {
		return baseStyle
	}
	def, ok := styles.Get(baseStyle)
	if !ok {
		return baseStyle
	}

	props := make(map[KFXSymbol]any, len(def.Properties)+2)
	for k, v := range def.Properties {
		props[k] = v
	}
	props[SymMarginTop] = DimensionValue(mtLh, SymUnitLh)
	props[SymMarginBottom] = DimensionValue(mbLh, SymUnitLh)

	// Preserve the original usage classification and mark this as a resolved variant.
	return styles.RegisterResolved(props, styles.GetUsage(baseStyle), true)
}
