package kfx

import (
	"strconv"
	"strings"
)

// ConvertFontWeight converts CSS font-weight values to KFX symbols.
// CSS: bold, bolder, lighter, normal, 100-900
// KFX: $361 (bold), $362 (semibold), $363 (light), $364 (medium), $350 (normal)
func ConvertFontWeight(css CSSValue) (KFXSymbol, bool) {
	if css.Keyword != "" {
		switch strings.ToLower(css.Keyword) {
		case "bold", "bolder":
			return SymBold, true // $361
		case "lighter":
			return SymLight, true // $363
		case "normal":
			return SymNormal, true // $350
		case "medium":
			return SymMedium, true // $364
		}
	}

	// Numeric font-weight (100-900)
	if css.Value > 0 {
		weight := int(css.Value)
		switch {
		case weight >= 700:
			return SymBold, true // $361
		case weight >= 600:
			return SymSemibold, true // $362
		case weight >= 500:
			return SymMedium, true // $364
		case weight <= 300:
			return SymLight, true // $363
		default:
			return SymNormal, true // $350
		}
	}

	return 0, false
}

// ConvertFontStyle converts CSS font-style values to KFX symbols.
// CSS: italic, oblique, normal
// KFX: $382 (italic), $350 (normal)
func ConvertFontStyle(css CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "italic", "oblique":
		return SymItalic, true // $382
	case "normal":
		return SymNormal, true // $350
	}
	return 0, false
}

// ConvertTextAlign converts CSS text-align values to KFX symbols.
// CSS: left, right, center, justify, start, end
// KFX: $680 (start), $681 (end), $320 (center), $321 (justify)
func ConvertTextAlign(css CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "left", "start":
		return SymStart, true // $680
	case "right", "end":
		return SymEnd, true // $681
	case "center":
		return SymCenter, true // $320
	case "justify":
		return SymJustify, true // $321
	}
	return 0, false
}

// TextDecorationResult holds the result of parsing text-decoration.
type TextDecorationResult struct {
	Underline     bool
	Strikethrough bool
	None          bool
}

// ConvertTextDecoration parses CSS text-decoration values.
// Returns which decorations are set.
// CSS: underline, line-through, none
// KFX: $23 (underline), $27 (strikethrough)
func ConvertTextDecoration(css CSSValue) TextDecorationResult {
	result := TextDecorationResult{}
	raw := strings.ToLower(css.Raw)

	if strings.Contains(raw, "underline") {
		result.Underline = true
	}
	if strings.Contains(raw, "line-through") {
		result.Strikethrough = true
	}
	if raw == "none" {
		result.None = true
	}

	return result
}

// ConvertVerticalAlign converts CSS vertical-align to KFX baseline_shift.
// CSS: super, sub, baseline, middle, top, bottom, or length
// KFX: baseline_shift with positive (super) or negative (sub) value
func ConvertVerticalAlign(css CSSValue) (StructValue, bool) {
	switch strings.ToLower(css.Keyword) {
	case "super":
		// Superscript: positive shift
		return DimensionValue(0.5, SymUnitEm), true
	case "sub":
		// Subscript: negative shift
		return DimensionValue(-0.3, SymUnitEm), true
	case "baseline":
		return DimensionValue(0, SymUnitEm), true
	}

	// Handle percentage or length values
	if css.IsNumeric() {
		dim, err := MakeDimensionValue(css)
		if err == nil {
			return dim, true
		}
	}

	return nil, false
}

// ConvertDisplay converts CSS display values.
// Returns the KFX render mode symbol, or handles visibility.
// CSS: block, inline, none
// KFX: $602 (block), visibility handling for none
func ConvertDisplay(css CSSValue) (symbol KFXSymbol, isVisible bool, ok bool) {
	switch strings.ToLower(css.Keyword) {
	case "block":
		return SymBlock, true, true // $602
	case "inline":
		return SymInline, true, true // $283
	case "none":
		// display:none means invisible
		return 0, false, true
	}
	return 0, true, false
}

// ConvertFloat converts CSS float values to KFX symbols.
// CSS: left, right, none
// KFX: $680 (start/left), $681 (end/right), $349 (none)
func ConvertFloat(css CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "left":
		return SymStart, true // $680
	case "right":
		return SymEnd, true // $681
	case "none":
		return SymNone, true // $349
	}
	return 0, false
}

// ConvertPageBreak converts CSS page-break-* values.
// CSS: always, avoid, auto
// KFX: $352 (always), $353 (avoid)
func ConvertPageBreak(css CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "always":
		return SymAlways, true // $352
	case "avoid":
		return SymAvoid, true // $353
	case "auto":
		return SymAuto, true // $383
	}
	return 0, false
}

// ParseColor parses a CSS color value to RGB integers.
// Supports: #RGB, #RRGGBB, rgb(r,g,b), color keywords
// Returns r, g, b values (0-255) and ok.
func ParseColor(css CSSValue) (r, g, b int, ok bool) {
	raw := strings.TrimSpace(css.Raw)

	// Handle hex colors
	if strings.HasPrefix(raw, "#") {
		hex := raw[1:]
		switch len(hex) {
		case 3:
			// #RGB -> #RRGGBB
			rVal, _ := strconv.ParseInt(string(hex[0])+string(hex[0]), 16, 64)
			gVal, _ := strconv.ParseInt(string(hex[1])+string(hex[1]), 16, 64)
			bVal, _ := strconv.ParseInt(string(hex[2])+string(hex[2]), 16, 64)
			return int(rVal), int(gVal), int(bVal), true
		case 6:
			// #RRGGBB
			rVal, _ := strconv.ParseInt(hex[0:2], 16, 64)
			gVal, _ := strconv.ParseInt(hex[2:4], 16, 64)
			bVal, _ := strconv.ParseInt(hex[4:6], 16, 64)
			return int(rVal), int(gVal), int(bVal), true
		}
	}

	// Handle rgb() function
	if strings.HasPrefix(strings.ToLower(raw), "rgb(") {
		inner := strings.TrimPrefix(strings.ToLower(raw), "rgb(")
		inner = strings.TrimSuffix(inner, ")")
		parts := strings.Split(inner, ",")
		if len(parts) == 3 {
			rVal, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			gVal, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			bVal, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
			return rVal, gVal, bVal, true
		}
	}

	// Handle common color keywords
	switch strings.ToLower(raw) {
	case "black":
		return 0, 0, 0, true
	case "white":
		return 255, 255, 255, true
	case "red":
		return 255, 0, 0, true
	case "green":
		return 0, 128, 0, true
	case "blue":
		return 0, 0, 255, true
	case "gray", "grey":
		return 128, 128, 128, true
	case "silver":
		return 192, 192, 192, true
	case "maroon":
		return 128, 0, 0, true
	case "navy":
		return 0, 0, 128, true
	case "teal":
		return 0, 128, 128, true
	case "olive":
		return 128, 128, 0, true
	case "purple":
		return 128, 0, 128, true
	case "fuchsia", "magenta":
		return 255, 0, 255, true
	case "aqua", "cyan":
		return 0, 255, 255, true
	case "lime":
		return 0, 255, 0, true
	case "yellow":
		return 255, 255, 0, true
	case "orange":
		return 255, 165, 0, true
	case "brown":
		return 165, 42, 42, true
	case "pink":
		return 255, 192, 203, true
	}

	return 0, 0, 0, false
}

// MakeColorValue creates a KFX color struct from RGB values.
// KFX color format uses fill_color with RGB components.
func MakeColorValue(r, g, b int) StructValue {
	// KFX uses 0.0-1.0 range for color components
	return NewStruct().
		SetFloat(SymValue, float64(r)/255.0).
		Set(85, float64(g)/255.0). // $85 = green component
		Set(86, float64(b)/255.0)  // $86 = blue component
}
