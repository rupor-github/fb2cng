package kfx

// KPV Unit Conversion Constants
//
// Kindle Previewer (KPV) uses specific unit types for different CSS properties.
// Using incorrect units can cause rendering issues (e.g., text-align: center
// not working with percentage font-sizes).
//
// These constants define the conversion ratios used when transforming CSS units
// to KPV-preferred units. The values are derived from reference KFX file analysis.
//
// See docs/kfxstructure.md §7.10.2 "KPV unit conventions" for full documentation.

const (
	// KPVLineHeightRatio is the assumed line-height multiplier (1lh = 1.2em).
	// Used to convert em → lh for vertical spacing properties.
	// Example: 0.3em CSS → 0.25lh KFX (0.3 / 1.2 = 0.25)
	KPVLineHeightRatio = 1.2

	// KPVEmToPercentHorizontal is the em-to-percent ratio for horizontal spacing.
	// Used for margin-left, margin-right, padding-left, padding-right.
	// Example: 1em CSS → 6.25% KFX
	KPVEmToPercentHorizontal = 6.25

	// KPVEmToPercentTextIndent is the em-to-percent ratio for text-indent.
	// Text indent uses a different ratio than horizontal margins.
	// Example: 1em CSS → 3.125% KFX
	KPVEmToPercentTextIndent = 3.125

	// KPVPercentToRem is the divisor for converting percentage to rem (for font-size).
	// Example: 140% CSS → 1.4rem KFX (140 / 100 = 1.4)
	KPVPercentToRem = 100.0
)

// KPV Unit Preference by Property
//
// | CSS Property    | KPV Unit | Notes                              |
// |-----------------|----------|------------------------------------|
// | font-size       | rem      | NOT %. Using % breaks text-align   |
// | margin-top      | lh       | Line-height units for vertical     |
// | margin-bottom   | lh       | Line-height units for vertical     |
// | margin-left     | %        | Percentage for horizontal          |
// | margin-right    | %        | Percentage for horizontal          |
// | padding-top     | lh       | Line-height units for vertical     |
// | padding-bottom  | lh       | Line-height units for vertical     |
// | padding-left    | %        | Percentage for horizontal          |
// | padding-right   | %        | Percentage for horizontal          |
// | text-indent     | %        | Percentage                         |
// | line-height     | lh       | Line-height units                  |

// isVerticalSpacingProperty returns true if the symbol is a vertical spacing property
// that should use lh units in KPV.
func isVerticalSpacingProperty(sym KFXSymbol) bool {
	switch sym {
	case SymMarginTop, SymMarginBottom, SymPaddingTop, SymPaddingBottom:
		return true
	}
	return false
}

// isHorizontalSpacingProperty returns true if the symbol is a horizontal spacing property
// that should use % units in KPV.
func isHorizontalSpacingProperty(sym KFXSymbol) bool {
	switch sym {
	case SymMarginLeft, SymMarginRight, SymPaddingLeft, SymPaddingRight:
		return true
	}
	return false
}
