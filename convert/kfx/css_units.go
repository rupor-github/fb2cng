package kfx

import (
	"fmt"
)

// CSSValueToKFX converts a CSS value with units to KFX dimension representation.
// Returns the numeric value and the KFX unit symbol.
// Note: KPV uses specific units for different properties:
//   - text-indent: % (percent)
//   - margins: lh (line-height units)
//   - font-size in inline styles: rem
//
// This function preserves CSS units as-is. Property-specific conversion
// happens in the style builder or during fragment generation.
func CSSValueToKFX(css CSSValue) (value float64, unit KFXSymbol, err error) {
	switch css.Unit {
	case "em":
		return css.Value, SymUnitEm, nil // $308
	case "ex":
		return css.Value, SymUnitEx, nil // $309
	case "%":
		return css.Value, SymUnitPercent, nil // $314
	case "px":
		return css.Value, SymUnitPx, nil // $319
	case "pt":
		return css.Value, SymUnitPt, nil // $318
	case "cm":
		return css.Value, SymUnitCm, nil // $315
	case "mm":
		return css.Value, SymUnitMm, nil // $316
	case "in":
		return css.Value, SymUnitIn, nil // $317
	case "rem":
		return css.Value, SymUnitRem, nil // $505
	case "lh":
		return css.Value, SymUnitLh, nil // $310
	case "":
		// Unitless - typically ratio for line-height
		return css.Value, SymUnitLh, nil // $310 (lh)
	default:
		return 0, 0, fmt.Errorf("unsupported unit: %s", css.Unit)
	}
}

// MakeDimensionValue creates a KFX dimension struct from CSS value.
func MakeDimensionValue(css CSSValue) (StructValue, error) {
	value, unit, err := CSSValueToKFX(css)
	if err != nil {
		return nil, err
	}
	return DimensionValue(value, unit), nil
}
