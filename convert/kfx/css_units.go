package kfx

import (
	"fmt"
)

// CSSValueToKFX converts a CSS value with units to KFX dimension representation.
// Returns the numeric value and the KFX unit symbol.
func CSSValueToKFX(css CSSValue) (value float64, unit KFXSymbol, err error) {
	switch css.Unit {
	case "em":
		return css.Value, SymUnitEm, nil // $308
	case "ex":
		return css.Value, SymUnitEx, nil // $309
	case "%":
		// CSS percent to KFX ratio (divide by 100)
		return css.Value / 100, SymUnitRatio, nil // $310
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
	case "":
		// Unitless - typically ratio for line-height, or just a number
		return css.Value, SymUnitRatio, nil // $310
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
