package kfx

import (
	"fbc/css"
	"fmt"
	"strings"
)

// CSSValueToKFX converts a CSS value with units to KFX dimension representation.
// Returns the numeric value and the KFX unit symbol.
// Note: KP3 uses specific units for different properties:
//   - text-indent: % (percent)
//   - margins: lh (line-height units)
//   - font-size in inline styles: rem
//
// This function preserves CSS units as-is. Property-specific conversion
// happens in the style builder or during fragment generation.
func CSSValueToKFX(val css.Value) (value float64, unit KFXSymbol, err error) {
	switch val.Unit {
	case "em":
		return val.Value, SymUnitEm, nil // $308
	case "ex":
		// KP3 maps ex to em unit (com/amazon/yjhtmlmapper/i/d.java:17).
		// Normally ex values are already converted to em by normalizeCSSProperties(),
		// but this serves as a safety net for any ex values that bypass normalization.
		return val.Value, SymUnitEm, nil // $308 (em, not ex)
	case "%":
		return val.Value, SymUnitPercent, nil // $314
	case "px":
		return val.Value, SymUnitPx, nil // $319
	case "pt":
		return val.Value, SymUnitPt, nil // $318
	case "cm":
		return val.Value, SymUnitCm, nil // $315
	case "mm":
		return val.Value, SymUnitMm, nil // $316
	case "in":
		return val.Value, SymUnitIn, nil // $317
	case "rem":
		return val.Value, SymUnitRem, nil // $505
	case "lh":
		return val.Value, SymUnitLh, nil // $310
	case "":
		// Unitless - typically ratio for line-height, but also valid for zero values
		// The caller should handle property-specific unit selection
		return val.Value, SymUnitLh, nil // $310 (lh)
	default:
		return 0, 0, fmt.Errorf("unsupported unit: %s", val.Unit)
	}
}

// MakeDimensionValue creates a KFX dimension struct from CSS value.
func MakeDimensionValue(val css.Value) (StructValue, error) {
	value, unit, err := CSSValueToKFX(val)
	if err != nil {
		return nil, err
	}
	return DimensionValue(value, unit), nil
}

// MakeBorderRadiusValue converts a CSS border-radius value to a KFX value.
//
// KP3 reference: com/amazon/yjhtmlmapper/transformers/BorderRadiusTransformer.java
//
// CSS border-*-radius accepts two space-separated values for elliptical corners:
//
//	border-top-left-radius: 10px 20px  (horizontal-radius vertical-radius)
//
// KP3 splits the value by space, requires exactly 2 tokens for the two-value case:
//   - If both values are identical → single DimensionValue (measure)
//   - If they differ → Ion list of two DimensionValue items [horizontal, vertical]
//
// Single-value input falls through to the standard MakeDimensionValue path.
// Returns (value, true) on success, (nil, false) on failure.
func MakeBorderRadiusValue(cssVal css.Value, rawVal string, unit string) (any, bool) {
	// Determine the raw string to inspect for space-separated pairs.
	raw := cssVal.Raw
	if raw == "" {
		raw = rawVal
	}
	raw = strings.TrimSpace(raw)

	parts := strings.Fields(raw)

	switch len(parts) {
	case 0:
		// Empty value — nothing to emit.
		return nil, false

	case 1:
		// Single value — use standard dimension conversion.
		if dim, err := MakeDimensionValue(cssVal); err == nil {
			return dim, true
		}
		if rawVal != "" {
			if dim, err := MakeDimensionValue(parseStyleMapCSSValue(rawVal, unit)); err == nil {
				return dim, true
			}
		}
		return nil, false

	case 2:
		// Two-value elliptical radius — KP3 BorderRadiusTransformer.java:54-73.
		// Parse each token independently.
		first := parseStyleMapCSSValue(parts[0], unit)
		second := parseStyleMapCSSValue(parts[1], unit)

		dim1, err1 := MakeDimensionValue(first)
		dim2, err2 := MakeDimensionValue(second)
		if err1 != nil || err2 != nil {
			return nil, false
		}

		// KP3 compares the two tokens: if identical, emit single measure.
		// The comparison reconstructs: var4[0].equals(var4[1] + this.k.d())
		// We simplify: compare the parsed numeric value and unit.
		if first.Value == second.Value && first.Unit == second.Unit {
			return dim1, true
		}

		// Different radii — emit Ion list of two dimensions [horizontal, vertical].
		list := NewList()
		list.Add(dim1)
		list.Add(dim2)
		return ListValue(list), true

	default:
		// 3+ tokens — KP3 throws INVALID_PROPERTY_VALUE. Skip silently.
		return nil, false
	}
}
