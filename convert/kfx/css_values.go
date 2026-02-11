package kfx

import (
	"fbc/css"
	"strconv"
	"strings"
)

// ConvertFontWeight converts CSS font-weight values to KFX symbols.
// CSS: bold, bolder, lighter, normal, 100-900
// KFX: $361 (bold), $362 (semibold), $363 (light), $364 (medium), $350 (normal)
func ConvertFontWeight(css css.CSSValue) (KFXSymbol, bool) {
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
func ConvertFontStyle(css css.CSSValue) (KFXSymbol, bool) {
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
// KFX: $59 (left), $680 (start), $61 (right), $681 (end), $320 (center), $321 (justify)
func ConvertTextAlign(css css.CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "left":
		return SymLeft, true // $59
	case "start":
		return SymStart, true // $680
	case "right":
		return SymRight, true // $61
	case "end":
		return SymEnd, true // $681
	case "center":
		return SymCenter, true // $320
	case "justify":
		return SymJustify, true // $321
	}
	return 0, false
}

// ConvertWritingMode maps writing-mode values to KFX symbols.
// CSS: horizontal-tb, vertical-rl, vertical-lr
// KFX: $557 (horizontal_tb), $559 (vertical_rl), $558 (vertical_lr)
func ConvertWritingMode(css css.CSSValue) (KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	switch val {
	case "horizontal-tb", "horizontal_tb":
		return SymHorizontalTb, true
	case "vertical-rl", "vertical_rl":
		return SymVerticalRl, true
	case "vertical-lr", "vertical_lr":
		return SymVerticalLr, true
	}
	if sym, ok := symbolIDFromString(val); ok {
		return sym, true
	}
	return 0, false
}

// ConvertTextCombine maps text-combine-upright to KFX symbols.
func ConvertTextCombine(css css.CSSValue) (KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	if val == "" {
		return 0, false
	}
	if sym, ok := symbolIDFromString(val); ok {
		return sym, true
	}
	return 0, false
}

// ConvertTextOrientation converts text-orientation to KFX symbols.
func ConvertTextOrientation(css css.CSSValue) (KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	switch val {
	case "mixed":
		return SymAuto, true
	case "upright":
		return SymUpright, true
	case "sideways", "sideways-rl", "sideways-lr":
		return SymSideways, true
	}
	if sym, ok := symbolIDFromString(val); ok {
		return sym, true
	}
	return 0, false
}

// ConvertTextEmphasisStyle maps text-emphasis-style to KFX symbols.
func ConvertTextEmphasisStyle(css css.CSSValue) (KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	if val == "" {
		return 0, false
	}

	fill := "filled"
	shape := ""
	for token := range strings.FieldsSeq(val) {
		switch token {
		case "filled", "open":
			fill = token
		case "dot", "circle", "double-circle", "triangle", "sesame":
			shape = token
		}
	}

	if shape == "" {
		shape = val
	}

	switch fill {
	case "open":
		switch shape {
		case "dot":
			return SymOpenDot, true
		case "circle":
			return SymOpenCircle, true
		case "double-circle", "doublecircle":
			return SymOpenDoubleCircle, true
		case "triangle":
			return SymOpenTriangle, true
		case "sesame":
			return SymOpenSesame, true
		}
	default:
		switch shape {
		case "dot":
			return SymFilledDot, true
		case "circle":
			return SymFilledCircle, true
		case "double-circle", "doublecircle":
			return SymFilledDoubleCircle, true
		case "triangle":
			return SymFilledTriangle, true
		case "sesame":
			return SymFilledSesame, true
		}
	}

	if sym, ok := symbolIDFromString(val); ok {
		return sym, true
	}

	return 0, false
}

// ConvertTextEmphasisPosition splits position into horizontal/vertical symbols.
func ConvertTextEmphasisPosition(css css.CSSValue) (KFXSymbol, KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	if val == "" {
		return 0, 0, false
	}

	var horiz, vert KFXSymbol
	for _, part := range strings.FieldsFunc(val, func(r rune) bool { return r == ' ' || r == ',' }) {
		switch part {
		case "over", "top":
			vert = SymTop
		case "under", "bottom":
			vert = SymBottom
		case "left":
			horiz = SymLeft
		case "right":
			horiz = SymRight
		}
	}

	if horiz == 0 && vert == 0 {
		return 0, 0, false
	}
	return horiz, vert, true
}

// ConvertListStyle converts list-style-type values to KFX symbols.
func ConvertListStyle(value string) (KFXSymbol, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "disc":
		return SymListStyleDisc, true
	case "square":
		return SymListStyleSquare, true
	case "circle":
		return SymListStyleCircle, true
	case "none":
		return SymNone, true
	case "decimal", "numeric":
		return SymListStyleNumber, true
	}

	if id := SymbolID(strings.ToLower(value)); id != -1 {
		return id, true
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
func ConvertTextDecoration(css css.CSSValue) TextDecorationResult {
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

// VerticalAlignResult holds the result of converting vertical-align.
type VerticalAlignResult struct {
	UseBaselineStyle bool      // true if using $44 baseline_style
	BaselineStyle    KFXSymbol // $370 superscript or $371 subscript
	UseBaselineShift bool      // true if using $31 baseline_shift
	BaselineShift    StructValue
}

// ConvertVerticalAlign converts CSS vertical-align to KFX properties.
// CSS: super, sub -> KFX: baseline_style ($44)
// CSS: baseline, length/percent values -> KFX: baseline_shift ($31)
// KP3 uses baseline_style for super/sub which is more compatible.
func ConvertVerticalAlign(css css.CSSValue) (VerticalAlignResult, bool) {
	result := VerticalAlignResult{}

	switch strings.ToLower(css.Keyword) {
	case "super":
		// Use baseline_style: superscript (KP3 compatible)
		result.UseBaselineStyle = true
		result.BaselineStyle = SymSuperscript
		return result, true
	case "sub":
		// Use baseline_style: subscript (KP3 compatible)
		result.UseBaselineStyle = true
		result.BaselineStyle = SymSubscript
		return result, true
	case "baseline":
		result.UseBaselineShift = true
		result.BaselineShift = DimensionValue(0, SymUnitEm)
		return result, true
	}

	// Handle percentage or length values
	if css.IsNumeric() {
		dim, err := MakeDimensionValue(css)
		if err == nil {
			result.UseBaselineShift = true
			result.BaselineShift = dim
			return result, true
		}
	}

	return result, false
}

// ConvertBorderStyle converts CSS border-style values to KFX symbols.
// CSS: solid, dashed, dotted, double, groove, ridge, inset, outset, none, hidden
// KFX: $328 (solid), $323 (dashed), $324 (dotted), $349 (none)
func ConvertBorderStyle(style string) (KFXSymbol, bool) {
	switch strings.ToLower(style) {
	case "solid":
		return SymSolid, true // $328
	case "dashed":
		return SymDashed, true // $323
	case "dotted":
		return SymDotted, true // $324
	case "none", "hidden":
		return SymNone, true // $349
	case "double", "groove", "ridge", "inset", "outset":
		// Fall back to solid for unsupported styles
		return SymSolid, true
	}
	return 0, false
}

// ConvertFloat converts CSS float values to KFX symbols.
// CSS: left, right, none
// KFX: $59 (left), $61 (right), $349 (none)
func ConvertFloat(css css.CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "left":
		return SymLeft, true // $59
	case "right":
		return SymRight, true // $61
	case "none":
		return SymNone, true // $349
	}
	if sym, ok := symbolIDFromString(strings.ReplaceAll(strings.ToLower(css.Keyword), "-", "_")); ok {
		return sym, true
	}
	return 0, false
}

// ConvertClear converts CSS clear values to KFX symbols for yj.float_clear.
// CSS: left, right, both, none
// KFX: $59 (left), $61 (right), $421 (both), $349 (none)
func ConvertClear(css css.CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "left":
		return SymLeft, true
	case "right":
		return SymRight, true
	case "both":
		return SymBoth, true
	case "none":
		return SymNone, true
	}
	return 0, false
}

// ConvertHyphens converts CSS hyphens values to KFX symbols.
// CSS: none, auto, manual
// KFX: $349 (none), $383 (auto), $384 (manual)
// Note: KFX also defines "unknown" ($348) and "enabled" ($441) values,
// but these are not standard CSS values and are not mapped here.
func ConvertHyphens(css css.CSSValue) (KFXSymbol, bool) {
	switch strings.ToLower(css.Keyword) {
	case "none":
		return SymNone, true // $349
	case "auto":
		return SymAuto, true // $383
	case "manual":
		return SymManual, true // $384
	}
	return 0, false
}

// ConvertPageBreak converts CSS page-break-* values.
// CSS: always, avoid, auto
// KFX: $352 (always), $353 (avoid)
func ConvertPageBreak(css css.CSSValue) (KFXSymbol, bool) {
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

// convertYjBreak converts yj-break-before/yj-break-after values.
// Values come from stylemap: always, avoid, auto
// KFX: $352 (always), $353 (avoid), $383 (auto)
func convertYjBreak(css css.CSSValue) (KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	switch val {
	case "always":
		return SymAlways, true
	case "avoid":
		return SymAvoid, true
	case "auto":
		return SymAuto, true
	}
	return 0, false
}

// ConvertBaselineStyle converts baseline-style values from stylemap.
// Used by vertical-align mapping in stylemap for super/sub positioning.
// Values: center, top, bottom, superscript, subscript
// KFX: $320 (center), $58 (top), $60 (bottom), $370 (superscript), $371 (subscript)
func ConvertBaselineStyle(css css.CSSValue) (KFXSymbol, bool) {
	val := strings.ToLower(firstNonEmpty(css.Keyword, css.Raw))
	switch val {
	case "center":
		return SymCenter, true
	case "top":
		return SymTop, true
	case "bottom":
		return SymBottom, true
	case "superscript", "super":
		return SymSuperscript, true
	case "subscript", "sub":
		return SymSubscript, true
	}
	if sym, ok := symbolIDFromString(val); ok {
		return sym, true
	}
	return 0, false
}

// ParseColor parses a CSS color value to RGB integers.
// Supports: #RGB, #RRGGBB, rgb(r,g,b), color keywords
// Returns r, g, b values (0-255) and ok.
func ParseColor(css css.CSSValue) (r, g, b int, ok bool) {
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

	// Handle rgb()/rgba() functions (alpha ignored)
	lowerRaw := strings.ToLower(raw)
	if strings.HasPrefix(lowerRaw, "rgb(") || strings.HasPrefix(lowerRaw, "rgba(") {
		inner := strings.TrimPrefix(lowerRaw, "rgba(")
		if inner == lowerRaw {
			inner = strings.TrimPrefix(lowerRaw, "rgb(")
		}
		inner = strings.TrimSuffix(inner, ")")
		parts := strings.Split(inner, ",")
		if len(parts) >= 3 {
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

// MakeColorValue creates a KFX color value from RGB values.
// KFX color format uses a packed ARGB integer (alpha always 255).
func MakeColorValue(r, g, b int) int64 {
	return int64(0xFF)<<24 | int64(r)<<16 | int64(g)<<8 | int64(b)
}
