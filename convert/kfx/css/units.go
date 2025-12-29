package css

import (
	"fmt"

	"fbc/convert/kfx"
)

// CSSValueToKFX converts a CSS value with units to KFX dimension representation.
// Returns the numeric value and the KFX unit symbol.
func CSSValueToKFX(css CSSValue) (value float64, unit int, err error) {
	switch css.Unit {
	case "em":
		return css.Value, kfx.SymUnitEm, nil // $308
	case "ex":
		return css.Value, kfx.SymUnitEx, nil // $309
	case "%":
		// CSS percent to KFX ratio (divide by 100)
		return css.Value / 100, kfx.SymUnitRatio, nil // $310
	case "px":
		return css.Value, kfx.SymUnitPx, nil // $319
	case "pt":
		return css.Value, kfx.SymUnitPt, nil // $318
	case "cm":
		return css.Value, kfx.SymUnitCm, nil // $315
	case "mm":
		return css.Value, kfx.SymUnitMm, nil // $316
	case "in":
		return css.Value, kfx.SymUnitIn, nil // $317
	case "":
		// Unitless - typically ratio for line-height, or just a number
		return css.Value, kfx.SymUnitRatio, nil // $310
	default:
		return 0, 0, fmt.Errorf("unsupported unit: %s", css.Unit)
	}
}

// MakeDimensionValue creates a KFX dimension struct from CSS value.
func MakeDimensionValue(css CSSValue) (kfx.StructValue, error) {
	value, unit, err := CSSValueToKFX(css)
	if err != nil {
		return nil, err
	}
	return kfx.DimensionValue(value, unit), nil
}

// UnitSymbolName returns the name of a unit symbol for debugging.
func UnitSymbolName(unit int) string {
	switch unit {
	case kfx.SymUnitEm:
		return "em"
	case kfx.SymUnitEx:
		return "ex"
	case kfx.SymUnitRatio:
		return "ratio"
	case kfx.SymUnitPercent:
		return "percent"
	case kfx.SymUnitPx:
		return "px"
	case kfx.SymUnitPt:
		return "pt"
	case kfx.SymUnitCm:
		return "cm"
	case kfx.SymUnitMm:
		return "mm"
	case kfx.SymUnitIn:
		return "in"
	default:
		return fmt.Sprintf("$%d", unit)
	}
}
