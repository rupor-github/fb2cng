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
// NOTE: fb2cng and KP3 both use em units for horizontal spacing (margin-left/right,
// padding-left/right) and text-indent, so these values scale with the viewer font
// size.
//
// See docs/kfxstructure.md §7.10.2 "KP3 unit conventions" for full documentation.

const (
	// SignificantFigures is the number of significant figures for Ion decimal values.
	// Used for vertical margins (margin-top, margin-bottom) in lh units.
	// KP3 uses 6 significant figures, not 6 decimal places.
	//
	// This is important for values >= 1:
	//   - 5/3 with 6 sig figs → 1.66667 (5 decimals)
	//   - 5/3 with 6 decimal places → 1.666667 (6 decimals) -- WRONG
	//   - 5/6 with 6 sig figs → 0.833333 (6 decimals) -- same because < 1
	//
	// Example: 5/3 → 1.66667 (6 significant figures, matching KP3 output)
	SignificantFigures = 6

	// LineHeightPrecision is the number of decimal places for line-height values.
	// KP3 uses 4-5 decimal places for line-height (e.g., 1.0101, 1.33249).
	// Using more precision than KP3 may cause subtle rendering differences.
	//
	// Example: 100/99 → 1.0101 (4 decimal places, matching KP3 output)
	LineHeightPrecision = 5

	// WidthPercentPrecision is the number of decimal places for width percentages.
	// KP3 uses 3 decimal places for image width percentages (e.g., 19.531, 74.219).
	// These values are derived from imageWidth / 512 * 100.
	//
	// Example: 100/512*100 → 19.531 (3 decimal places, matching KP3 output)
	WidthPercentPrecision = 3

	// DefaultLineHeightLh is the default line-height used for text styles
	// with default font-size (1rem). KP3 uses 1lh for these styles.
	DefaultLineHeightLh = 1.0

	// AdjustedLineHeightLh is the line-height used for text styles with
	// non-default font-size. KP3 uses 100/99 ≈ 1.0101lh for these styles.
	// This affects margin calculations: margins are divided by this factor
	// in addition to LineHeightRatio when converting from em to lh.
	AdjustedLineHeightLh = 100.0 / 99.0 // 1.01010101...

	// SectionTitleHeaderLineHeightLh is the KP3 line-height used for nested section
	// title header text (font-size: 120% -> 1.125rem after KP3 compression) when
	// layout-hints: [treat_as_title] is present.
	//
	// Observed in KP3 reference output for testdata/_Test.fb2.
	SectionTitleHeaderLineHeightLh = 0.982323

	// DefaultFontSizeEm is the default font-size in document_data ($16).
	// KP3 uses 1em as the base font size for the document.
	DefaultFontSizeEm = 1.0

	// DefaultLineHeightEm is the default line-height in document_data ($42).
	// KP3 uses 1.2em as the base line height, which equals LineHeightRatio.
	// This value appears in document_data and defines the relationship
	// between em and lh units throughout the document.
	DefaultLineHeightEm = 1.2

	// LineHeightRatio is the assumed line-height multiplier (1lh = 1.2em).
	// Used to convert em → lh for vertical spacing properties.
	// Example: 0.3em CSS → 0.25lh KFX (0.3 / 1.2 = 0.25)
	// KP3 uses 1.2em as the base line-height for vertical margin calculations.
	// This value must match DefaultLineHeightEm.
	LineHeightRatio = DefaultLineHeightEm

	// KP3BaseWidthEm is the base content width in em units used by KP3 for horizontal
	// calculations. KP3 assumes a viewport of 32em for horizontal-tb writing mode.
	// This value is defined as constant 'b' in com/amazon/yj/F/a/b.java.
	KP3BaseWidthEm = 32.0

	// KP3PixelsPerEm is the pixels-per-em ratio used by KP3 (standard web default).
	// This value is defined as constant 'i' in com/amazon/yj/F/a/b.java.
	KP3PixelsPerEm = 16.0

	// KP3ContentWidthPx is the standard content width in pixels used by KP3 for
	// calculating block image width percentages. This is KP3's "IDEAL" viewport:
	// KP3BaseWidthEm × KP3PixelsPerEm = 32em × 16px/em = 512px.
	//
	// All block images in text flow use this as their reference width, regardless
	// of the actual screen dimensions. This constant is defined as 'd' in
	// com/amazon/yj/F/a/b.java: public static final Double d = b * i; // 512.0
	//
	// See also com/amazon/yj/style/merger/e/a.java: public static final int m = 512;
	KP3ContentWidthPx = KP3BaseWidthEm * KP3PixelsPerEm // 512.0

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

	// ExToEmFactor is the conversion factor from CSS ex units to em units.
	// KP3 converts all ex values to em early in the normalization pipeline,
	// using the formula: em = ex * ExToEmFactor.
	//
	// This is defined in com/amazon/yj/F/a/b.java:24 as constant 'e = 0.44'
	// and used in com/amazon/yjhtmlmapper/i/c.java:8-15 for the ex→em mapping.
	// The normalization sweep in com/amazon/yjhtmlmapper/h/b.java:253-263
	// iterates over all CSS properties and replaces ex values with em equivalents.
	//
	// Examples:
	//   1ex → 0.44em
	//   2ex → 0.88em
	//   0.5ex → 0.22em
	ExToEmFactor = 0.44
)

// Unit Preference by Property
//
// | CSS Property    | Unit | Notes                              |
// |-----------------|------|------------------------------------|
// | font-size       | rem  | NOT %. Using % breaks text-align   |
// | margin-top      | lh   | Line-height units for vertical     |
// | margin-bottom   | lh   | Line-height units for vertical     |
// | margin-left     | em   | Font-relative, scales with viewer  |
// | margin-right    | em   | Font-relative, scales with viewer  |
// | padding-top     | lh   | Line-height units for vertical     |
// | padding-bottom  | lh   | Line-height units for vertical     |
// | padding-left    | em   | Font-relative, scales with viewer  |
// | padding-right   | em   | Font-relative, scales with viewer  |
// | text-indent     | em   | Font-relative, scales with viewer  |
// | line-height     | lh   | Line-height units                  |

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
// that should use em units for font-relative scaling.
func isHorizontalSpacingProperty(sym KFXSymbol) bool {
	switch sym {
	case SymMarginLeft, SymMarginRight, SymPaddingLeft, SymPaddingRight:
		return true
	}
	return false
}

// isMarginProperty returns true if the symbol is a margin property.
// KFX does not support negative margins, so these need special handling.
func isMarginProperty(sym KFXSymbol) bool {
	switch sym {
	case SymMarginTop, SymMarginBottom, SymMarginLeft, SymMarginRight:
		return true
	}
	return false
}

// RoundDecimals rounds a float64 to the specified number of decimal places.
// Use with precision constants: LineHeightPrecision, WidthPercentPrecision.
//
// Examples:
//
//	RoundDecimals(1.01010101, LineHeightPrecision) → 1.0101     (5 decimals for line-height)
//	RoundDecimals(19.53125, WidthPercentPrecision) → 19.531     (3 decimals for image widths)
func RoundDecimals(v float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(v*multiplier) / multiplier
}

// RoundSignificant rounds a float64 to the specified number of significant figures.
// KP3 uses 6 significant figures for margin values, not 6 decimal places.
//
// Examples:
//
//	RoundSignificant(1.666666667, SignificantFigures) → 1.66667  (6 sig figs = 5 decimals for values >= 1)
//	RoundSignificant(0.833333333, SignificantFigures) → 0.833333 (6 sig figs = 6 decimals for values < 1)
//	RoundSignificant(0.416666667, SignificantFigures) → 0.416667 (6 sig figs = 6 decimals for values < 1)
func RoundSignificant(v float64, sigFigs int) float64 {
	if v == 0 {
		return 0
	}
	// Calculate the magnitude (order of magnitude)
	magnitude := math.Floor(math.Log10(math.Abs(v)))
	// Calculate multiplier to shift decimal point
	multiplier := math.Pow(10, float64(sigFigs-1)-magnitude)
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
// The result is rounded to SignificantFigures.
func PercentToRem(percent float64) float64 {
	if percent > 100 {
		// Compress values above 100% towards 1rem
		return RoundSignificant(1+(percent-100)/FontSizeCompressionFactor, SignificantFigures)
	}
	// Direct conversion for values at or below 100%
	return RoundSignificant(percent/100, SignificantFigures)
}

// ImageWidthPercent calculates the width percentage for a block image.
// KP3 uses a fixed 512px content width (KP3ContentWidthPx) for all block image
// calculations, regardless of the actual screen dimensions.
//
// The formula is: widthPercent = imageWidthPx * 100 / KP3ContentWidthPx
//
// The result is clamped to [0, 100] and rounded to WidthPercentPrecision decimal places.
//
// Examples:
//
//	ImageWidthPercent(380) → 74.219%  (380 * 100 / 512, rounded to 3 decimals)
//	ImageWidthPercent(240) → 46.875%  (240 * 100 / 512)
//	ImageWidthPercent(512) → 100%     (clamped)
//	ImageWidthPercent(600) → 100%     (clamped)
func ImageWidthPercent(imageWidthPx int) float64 {
	percent := float64(imageWidthPx) / KP3ContentWidthPx * 100
	return RoundDecimals(min(max(percent, 0), 100), WidthPercentPrecision)
}

// PxToLh converts pixels to line-height units for vertical spacing.
// Uses the formula: lh = px / KP3PixelsPerEm / LineHeightRatio
//
// The conversion chain: px → em → lh
//   - px → em: px / KP3PixelsPerEm (16px = 1em)
//   - em → lh: em / LineHeightRatio (1em = 1.2lh, so lh = em / 1.2)
//
// Combined: lh = px / 16 / 1.2 = px / 19.2
//
// Examples:
//
//	PxToLh(19.2) → 1.0lh
//	PxToLh(9.6) → 0.5lh
//	PxToLh(-8) → -0.416667lh (negative values preserved)
func PxToLh(px float64) float64 {
	return RoundSignificant(px/KP3PixelsPerEm/LineHeightRatio, SignificantFigures)
}

// PtToLh converts points to line-height units for vertical spacing.
// Uses the formula: lh = pt * PtToPxRatio / KP3PixelsPerEm / LineHeightRatio
//
// The conversion chain: pt → px → em → lh
//   - pt → px: pt * (96/72) = pt * 1.333... (CSS standard: 72pt = 1in = 96px)
//   - px → em → lh: (see PxToLh)
//
// Combined: lh = pt * (4/3) / 16 / 1.2 = pt / 14.4
//
// Examples:
//
//	PtToLh(14.4) → 1.0lh
//	PtToLh(7.2) → 0.5lh
//	PtToLh(-8) → -0.555556lh (negative values preserved)
func PtToLh(pt float64) float64 {
	// CSS standard: 72pt = 96px, so 1pt = 96/72 = 4/3 px
	const PtToPxRatio = 96.0 / 72.0 // 1.333...
	return RoundSignificant(pt*PtToPxRatio/KP3PixelsPerEm/LineHeightRatio, SignificantFigures)
}

// PtToEm converts points to em units for horizontal spacing.
// Uses the formula: em = pt * PtToPxRatio / KP3PixelsPerEm
//
// The conversion chain: pt → px → em
//   - pt → px: pt * (96/72) = pt * 1.333... (CSS standard: 72pt = 1in = 96px)
//   - px → em: px / KP3PixelsPerEm (16px = 1em)
//
// Combined: em = pt * (4/3) / 16 = pt / 12
//
// Examples:
//
//	PtToEm(12) → 1.0em    (12pt = 16px = 1em)
//	PtToEm(24) → 2.0em
//	PtToEm(-8) → -0.666667em (negative values preserved)
func PtToEm(pt float64) float64 {
	// CSS standard: 72pt = 96px, so 1pt = 96/72 = 4/3 px
	const PtToPxRatio = 96.0 / 72.0 // 1.333...
	return RoundSignificant(pt*PtToPxRatio/KP3PixelsPerEm, SignificantFigures)
}
