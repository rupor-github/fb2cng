package kfx

import "math"

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
	// KPVDecimalPrecision is the maximum number of decimal places for Ion decimal values.
	// Amazon's KFX processing code uses setScale(3, RoundingMode.HALF_UP) for dimension
	// calculations (found in com/amazon/yj/F/a/b.java and other style processing classes).
	// Values with more than 3 decimal places may cause rendering issues in KP3.
	// From observation, KPV seems to handle up to 6 decimal places reliably.
	//
	// Example: 63.33333333333333 → 63.333 (3 decimal places)
	KPVDecimalPrecision = 3
	// KPVDefaultLineHeightLh is the default line-height used for text styles.
	// KP3 uses 1lh for the majority of base text styles.
	// Some specific styles (like inline images with baseline-style) may use 1.0101lh.
	KPVDefaultLineHeightLh = 1.0

	// KPVLineHeightRatio is the assumed line-height multiplier (1lh = 1.2em).
	// Used to convert em → lh for vertical spacing properties.
	// Example: 0.3em CSS → 0.25lh KFX (0.3 / 1.2 = 0.25)
	// KP3 uses 1.2em as the base line-height for vertical margin calculations.
	KPVLineHeightRatio = 1.2

	// KPVEmToPercentHorizontal is the em-to-percent ratio for horizontal spacing.
	// Used for margin-left, margin-right, padding-left, padding-right.
	// KP3 uses a base width of 32em, so 1em = 100/32 = 3.125%
	// Example: 1em CSS → 3.125% KFX, 2em CSS → 6.25% KFX
	KPVEmToPercentHorizontal = 3.125

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

// RoundKPVDecimal rounds a float64 to KPVDecimalPrecision decimal places.
// Amazon's KFX code uses 3 decimal places (setScale(3, RoundingMode.HALF_UP))
// for dimension values. Using more precision can cause rendering failures in KP3.
//
// Examples:
//
//	RoundKPVDecimal(63.33333333333333) → 63.333
//	RoundKPVDecimal(0.25) → 0.25 (unchanged, already within precision)
//	RoundKPVDecimal(84.765625) → 84.766
func RoundKPVDecimal(v float64) float64 {
	multiplier := math.Pow(10, KPVDecimalPrecision)
	return math.Round(v*multiplier) / multiplier
}
