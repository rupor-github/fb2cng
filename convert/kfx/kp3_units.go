package kfx

import "math"

// KP3 Unit Conversion Constants
//
// Kindle Previewer 3 (KP3) uses specific unit types for different CSS properties.
// Using incorrect units can cause rendering issues (e.g., text-align: center
// not working with percentage font-sizes).
//
// These constants define the conversion ratios used when transforming CSS units
// to KP3-preferred units. The values are derived from reference KFX file analysis.
//
// See docs/kfxstructure.md §7.10.2 "KP3 unit conventions" for full documentation.

const (
	// DecimalPrecision is the maximum number of decimal places for Ion decimal values.
	// Amazon's KFX processing code uses setScale(3, RoundingMode.HALF_UP) for dimension
	// calculations (found in com/amazon/yj/F/a/b.java and other style processing classes).
	// Values with more than 3 decimal places may cause rendering issues in KP3.
	// From observation, KP3 seems to handle up to 6 decimal places reliably.
	//
	// Example: 63.33333333333333 → 63.333 (3 decimal places)
	DecimalPrecision = 3

	// DefaultLineHeightLh is the default line-height used for text styles.
	// KP3 uses 1lh for the majority of base text styles.
	// Some specific styles (like inline images with baseline-style) may use 1.0101lh.
	DefaultLineHeightLh = 1.0

	// LineHeightRatio is the assumed line-height multiplier (1lh = 1.2em).
	// Used to convert em → lh for vertical spacing properties.
	// Example: 0.3em CSS → 0.25lh KFX (0.3 / 1.2 = 0.25)
	// KP3 uses 1.2em as the base line-height for vertical margin calculations.
	LineHeightRatio = 1.2

	// EmToPercentHorizontal is the em-to-percent ratio for horizontal spacing.
	// Used for margin-left, margin-right, padding-left, padding-right.
	// KP3 uses a base width of 32em, so 1em = 100/32 = 3.125%
	// Example: 1em CSS → 3.125% KFX, 2em CSS → 6.25% KFX
	EmToPercentHorizontal = 3.125

	// EmToPercentTextIndent is the em-to-percent ratio for text-indent.
	// Text indent uses a different ratio than horizontal margins.
	// Example: 1em CSS → 3.125% KFX
	EmToPercentTextIndent = 3.125

	// FontSizeCompressionFactor is the divisor for KP3's font-size percentage compression.
	// KP3 compresses percentage font-sizes using the formula:
	//   rem = 1 + (percent - 100) / FontSizeCompressionFactor
	//
	// This compresses the range of font-sizes towards 1rem:
	//   140% → 1.25rem (not 1.4rem)
	//   120% → 1.125rem (not 1.2rem)
	//   100% → 1rem
	//   80%  → 0.875rem (not 0.8rem)
	//   70%  → 0.8125rem (not 0.7rem)
	//
	// The formula can also be written as: rem = 1 + (percent - 100) / 100 * 0.625
	// where 0.625 = 100 / FontSizeCompressionFactor
	FontSizeCompressionFactor = 160.0
)

// KP3 Unit Preference by Property
//
// | CSS Property    | KP3 Unit | Notes                              |
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
// that should use lh units in KP3.
func isVerticalSpacingProperty(sym KFXSymbol) bool {
	switch sym {
	case SymMarginTop, SymMarginBottom, SymPaddingTop, SymPaddingBottom:
		return true
	}
	return false
}

// isHorizontalSpacingProperty returns true if the symbol is a horizontal spacing property
// that should use % units in KP3.
func isHorizontalSpacingProperty(sym KFXSymbol) bool {
	switch sym {
	case SymMarginLeft, SymMarginRight, SymPaddingLeft, SymPaddingRight:
		return true
	}
	return false
}

// RoundDecimal rounds a float64 to DecimalPrecision decimal places.
// Amazon's KFX code uses 3 decimal places (setScale(3, RoundingMode.HALF_UP))
// for dimension values. Using more precision can cause rendering failures in KP3.
//
// Examples:
//
//	RoundDecimal(63.33333333333333) → 63.333
//	RoundDecimal(0.25) → 0.25 (unchanged, already within precision)
//	RoundDecimal(84.765625) → 84.766
func RoundDecimal(v float64) float64 {
	multiplier := math.Pow(10, DecimalPrecision)
	return math.Round(v*multiplier) / multiplier
}

// PercentToRem converts a CSS percentage font-size to KP3 rem.
//
// KP3 applies compression only to font-sizes ABOVE 100%:
//   - Values > 100%: rem = 1 + (percent - 100) / FontSizeCompressionFactor
//   - Values <= 100%: rem = percent / 100 (direct conversion)
//
// This compresses large font-sizes towards 1rem while preserving small ones:
//   - 140% → 1.25rem (compressed from 1.4)
//   - 120% → 1.125rem (compressed from 1.2)
//   - 100% → 1rem
//   - 80%  → 0.8rem (direct, not compressed)
//   - 70%  → 0.7rem (direct, not compressed)
//
// The result is rounded to DecimalPrecision decimal places.
func PercentToRem(percent float64) float64 {
	if percent > 100 {
		// Compress values above 100% towards 1rem
		return RoundDecimal(1 + (percent-100)/FontSizeCompressionFactor)
	}
	// Direct conversion for values at or below 100%
	return RoundDecimal(percent / 100)
}
