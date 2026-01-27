package kfx

// AddEmptyLineMargin adds margin from an empty-line element to the pending margin.
// This margin will be set as the next element's margin-top when Resolve() is called.
// The margin value should be in line-height units (lh).
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

// HasPendingMargin returns true if there's accumulated empty-line margin to apply.
func (sc StyleContext) HasPendingMargin() bool {
	if sc.emptyLine == nil {
		return false // Safety check - should never happen
	}
	return sc.emptyLine.pendingMargin > 0
}

// emptyLineMarginScale is the scaling factor KP3 applies to empty-line margin-top.
// KP3 uses only margin-top from the emptyline style and scales it by 0.5.
//
// Empirically verified:
//   - CSS `.emptyline { margin: 0.5em }` → KP3 outputs margin-top: 0.25lh
//   - CSS `.emptyline { margin: 1em }`   → KP3 outputs margin-top: 0.5lh
//
// The formula is: emptyline_margin_lh = CSS_margin_top_em / 2
// Since our CSS converter already converts em → lh (via / LineHeightRatio),
// we need to apply: margin_lh * (LineHeightRatio / 2) = margin_lh * 0.6
const emptyLineMarginScale = LineHeightRatio / 2.0 // 0.6

// emptyLineFallbackMarginLh is the fallback margin when the emptyline style
// exists but has no valid margin-top value. This corresponds to the default
// CSS `.emptyline { margin: 1em }` → 0.5lh after scaling.
const emptyLineFallbackMarginLh = 0.5

// GetEmptyLineMargin returns the margin value for empty-line elements in lh units.
//
// KP3 uses only the margin-top value from the emptyline CSS style (ignoring margin-bottom)
// and applies a scaling factor of 0.5 to the em value. Since our CSS converter already
// converts em → lh (dividing by LineHeightRatio=1.2), we compensate by multiplying
// by LineHeightRatio/2 = 0.6 to get the correct final value.
//
// Unit handling:
//   - lh: multiply by emptyLineMarginScale (0.6) to match KP3's em/2 behavior
//   - em: divide by 2 directly (rare case - CSS usually converts em → lh)
//   - other units: attempt conversion to lh, then scale
//
// Examples with default.css `.emptyline { margin: 1em }`:
//   - CSS 1em → converted to 0.833lh → scaled by 0.6 → 0.5lh (matches KP3)
//   - CSS 0.5em → converted to 0.417lh → scaled by 0.6 → 0.25lh (matches KP3)
func (sc StyleContext) GetEmptyLineMargin(styleSpec string, registry *StyleRegistry) float64 {
	def, ok := registry.Get(styleSpec)
	if !ok {
		return 0
	}

	// Resolve inheritance to get full properties (margin may be inherited)
	resolved := registry.resolveInheritance(def)

	// Extract margin-top value - KP3 only uses margin-top, ignoring margin-bottom
	marginTop, ok := resolved.Properties[SymMarginTop]
	if !ok {
		// Style exists but has no margin-top - use fallback
		return emptyLineFallbackMarginLh
	}

	val, unit, ok := measureParts(marginTop)
	if !ok || val <= 0 {
		return emptyLineFallbackMarginLh
	}

	// Apply unit-specific conversion to get final lh value
	switch unit {
	case SymUnitLh:
		// Already in lh units (from CSS em → lh conversion)
		// Apply scale factor to compensate: we want em/2, but have em/1.2
		// So multiply by 1.2/2 = 0.6
		return RoundSignificant(val*emptyLineMarginScale, SignificantFigures)

	case SymUnitEm:
		// Raw em value (rare - CSS converter usually converts to lh)
		// KP3 formula: em / 2
		return RoundSignificant(val/2.0, SignificantFigures)

	case SymUnitPercent:
		// Percentage - convert to lh assuming 100% = 1lh, then scale
		return RoundSignificant((val/100.0)*emptyLineMarginScale, SignificantFigures)

	case SymUnitPx:
		// Pixels - convert to em first (assuming 16px = 1em), then to lh
		emVal := val / KP3PixelsPerEm
		return RoundSignificant(emVal/2.0, SignificantFigures)

	case SymUnitPt:
		// Points - convert to em (assuming 12pt = 1em), then to lh
		emVal := val / 12.0
		return RoundSignificant(emVal/2.0, SignificantFigures)

	case SymUnitRem:
		// Rem - treat as em equivalent for vertical spacing, then apply scale
		return RoundSignificant(val/2.0, SignificantFigures)

	default:
		// Unknown unit - use fallback
		return emptyLineFallbackMarginLh
	}
}
