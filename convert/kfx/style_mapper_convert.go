package kfx

import (
	"maps"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"fbc/css"
)

func convertStyleMapLineHeight(cssVal css.Value, rawVal string) (StructValue, bool) {
	if strings.EqualFold(cssVal.Keyword, "normal") || strings.EqualFold(rawVal, "normal") {
		cssVal = css.Value{Value: 1.2, Unit: "em"}
	}
	if !cssVal.IsNumeric() && cssVal.Unit == "" {
		return nil, false
	}
	if cssVal.Unit == "" || cssVal.Unit == "lh" {
		return DimensionValue(cssVal.Value, SymUnitLh), true
	}
	if cssVal.Unit == "em" {
		return DimensionValue(cssVal.Value/LineHeightRatio, SymUnitLh), true
	}
	value, unit, err := CSSValueToKFX(cssVal)
	if err != nil {
		return nil, false
	}
	return DimensionValue(value, unit), true
}

// convertStyleMapDimension converts a CSS dimension value to KFX representation,
// applying property-specific unit preferences (see kp3_units.go Unit Preference table).
// Vertical spacing (margin-top/bottom) converts em → lh; horizontal spacing
// (margin-left/right, padding-left/right) and text-indent preserve em units for
// font-relative scaling, matching KP3 and the Amazon backend.
func convertStyleMapDimension(sym KFXSymbol, cssVal css.Value) (StructValue, bool) {
	if cssVal.IsKeyword() {
		return nil, false
	}
	if !cssVal.IsNumeric() && cssVal.Unit == "" {
		return nil, false
	}
	if cssVal.Value == 0 {
		return nil, false
	}
	switch {
	case isVerticalSpacingProperty(sym):
		if cssVal.Unit == "em" {
			return DimensionValue(cssVal.Value/LineHeightRatio, SymUnitLh), true
		}
	case isHorizontalSpacingProperty(sym):
		if cssVal.Unit == "em" {
			return DimensionValue(cssVal.Value, SymUnitEm), true
		}
	}
	value, unit, err := CSSValueToKFX(cssVal)
	if err != nil {
		return nil, false
	}
	return DimensionValue(value, unit), true
}

func convertStyleMapProp(prop string, cssVal css.Value, rawVal string, unit string, valType string, sourceAttr string, log *zap.Logger) (map[KFXSymbol]any, bool) {
	out := make(map[KFXSymbol]any)

	// Skip yj.semantics.heading_level - it should only be in storyline entries, not styles.
	// KP3 reference output doesn't include heading_level as a style property.
	if prop == "yj.semantics.heading_level" {
		return nil, false
	}

	// Skip CSS shorthand properties - they are already expanded by the CSS converter.
	// Otherwise we'd end up with both "padding" ($51) AND "padding_top" ($52), etc.
	cssProp := strings.ReplaceAll(prop, "_", "-")
	if IsShorthandProperty(cssProp) {
		return nil, false
	}

	switch prop {
	case "text_alignment":
		if sym, ok := ConvertTextAlign(cssVal); ok {
			out[SymTextAlignment] = sym
		}
	case "list_style":
		if sym, ok := ConvertListStyle(firstNonEmpty(formatCSSValue(cssVal), rawVal)); ok {
			out[SymListStyle] = sym
		}
	case "writing_mode":
		if sym, ok := ConvertWritingMode(cssVal); ok {
			out[SymWritingMode] = sym
		}
	case "text_combine":
		if sym, ok := ConvertTextCombine(cssVal); ok {
			out[SymTextCombine] = sym
		}
	case "text_orientation":
		if sym, ok := ConvertTextOrientation(cssVal); ok {
			out[SymTextOrientation] = sym
		}
	case "text_emphasis_style":
		if sym, ok := ConvertTextEmphasisStyle(cssVal); ok {
			out[SymTextEmphasisStyle] = sym
		}
	case "text_emphasis_color":
		if r, g, b, ok := ParseColor(cssVal); ok {
			out[SymTextEmphasisColor] = MakeColorValue(r, g, b)
		}
	case "text_color":
		if r, g, b, ok := ParseColor(cssVal); ok {
			out[SymTextColor] = MakeColorValue(r, g, b)
		}
	case "text_background_color":
		if r, g, b, ok := ParseColor(cssVal); ok {
			out[SymTextBackgroundColor] = MakeColorValue(r, g, b)
		}
	case "text_background_opacity":
		if cssVal.IsNumeric() {
			out[SymTextBackgroundOpacity] = cssVal.Value
		}
	case "text_emphasis_position_horizontal":
		if horiz, _, ok := ConvertTextEmphasisPosition(cssVal); ok && horiz != 0 {
			out[SymTextEmphasisPositionHorizontal] = horiz
		}
	case "text_emphasis_position_vertical":
		if _, vert, ok := ConvertTextEmphasisPosition(cssVal); ok && vert != 0 {
			out[SymTextEmphasisPositionVertical] = vert
		}
	case "text_emphasis_position_horizontal,text_emphasis_position_vertical":
		if horiz, vert, ok := ConvertTextEmphasisPosition(cssVal); ok {
			if horiz != 0 {
				out[SymTextEmphasisPositionHorizontal] = horiz
			}
			if vert != 0 {
				out[SymTextEmphasisPositionVertical] = vert
			}
		}
	case "underline", "overline", "strikethrough":
		// text-decoration sub-properties: emit SymNone for "none" to explicitly
		// suppress default decoration (e.g. underlines on links), SymSolid otherwise.
		if rawVal == "none" || strings.EqualFold(cssVal.Keyword, "none") {
			out[symbolForDecoration(prop)] = SymbolValue(SymNone)
		} else {
			out[symbolForDecoration(prop)] = SymbolValue(SymSolid)
		}
	case "fill_color":
		if r, g, b, ok := ParseColor(cssVal); ok {
			out[symbolIDOr(prop)] = MakeColorValue(r, g, b)
		}
	case "background_repeat":
		if sym, ok := ConvertBackgroundRepeat(cssVal); ok {
			out[SymBackgroundRepeat] = sym
		}
	case "background_positionx", "background_positiony":
		if dim, ok := parseDimensionFromCSS(cssVal, rawVal, unit); ok {
			out[symbolIDOr(prop)] = dim
			counterpart := SymBackgroundPositionY
			if prop == "background_positiony" {
				counterpart = SymBackgroundPositionX
			}
			if _, exists := out[counterpart]; !exists {
				out[counterpart] = DimensionValue(0, SymUnitPercent)
			}
		}
	case "background_position":
		if x, y, ok := parseXYPair(cssVal, rawVal); ok {
			out[SymBackgroundPositionX] = x
			out[SymBackgroundPositionY] = y
		}
	case "background_sizex", "background_sizey":
		if dim, ok := parseDimensionFromCSS(cssVal, rawVal, unit); ok {
			out[symbolIDOr(prop)] = dim
		}
	case "background_size":
		if x, y, ok := parseBackgroundSize(cssVal, rawVal); ok {
			if x != nil {
				out[symbolIDOr("background_sizex")] = x
			}
			if y != nil {
				out[symbolIDOr("background_sizey")] = y
			}
		}
	case "box_align":
		if val := firstNonEmpty(cssVal.Keyword, cssVal.Raw, rawVal); val != "" {
			if sym, ok := symbolIDFromString(val); ok {
				out[SymBoxAlign] = sym
			}
		}
	case "float":
		if sym, ok := ConvertFloat(cssVal); ok {
			out[SymFloat] = sym
		}
	case "font_weight":
		if sym, ok := ConvertFontWeight(cssVal); ok {
			out[SymFontWeight] = sym
		}
	case "line_height":
		if dim, ok := convertStyleMapLineHeight(cssVal, rawVal); ok {
			out[SymLineHeight] = dim
		}
	case "table_border_collapse":
		if val := firstNonEmpty(cssVal.Keyword, cssVal.Raw, rawVal); val != "" {
			out[symbolIDOr(prop)] = strings.EqualFold(val, "true")
		}
	case "link_unvisited_style":
		if val := firstNonEmpty(cssVal.Raw, rawVal); val != "" {
			out[SymLinkUnvisitedStyle] = val
		}
	case "language":
		if val := firstNonEmpty(cssVal.Keyword, cssVal.Raw, rawVal); val != "" {
			out[SymLanguage] = val
		}
	case "border_radius_top_left", "border_radius_top_right", "border_radius_bottom_left", "border_radius_bottom_right", "border_radius":
		// KP3 reference: BorderRadiusTransformer.java
		// Handles single values and two-value elliptical radius pairs (horizontal vertical).
		if val, ok := MakeBorderRadiusValue(cssVal, rawVal, unit); ok {
			out[symbolIDOr(prop)] = val
		}
	case "yj.border_path", "yj.max_crop", "yj.user_margin":
		switch prop {
		case "yj.border_path":
			if val := firstNonEmpty(cssVal.Raw, rawVal); val != "" {
				if path, ok := parsePolygonPath(val); ok {
					out[symbolIDOr(prop)] = path
				}
			}
		case "yj.max_crop":
			if crop, ok := parseMaxCropPercentage(firstNonEmpty(cssVal.Raw, rawVal)); ok {
				out[symbolIDOr(prop)] = crop
			}
		case "yj.user_margin":
			if margins := parsePageBleed(firstNonEmpty(cssVal.Raw, rawVal)); len(margins) > 0 {
				maps.Copy(out, margins)
			}
		}
	case "margin_left", "margin_right", "margin_top", "margin_bottom":
		if strings.EqualFold(cssVal.Keyword, "auto") || strings.EqualFold(cssVal.Raw, "auto") {
			out[symbolIDOr(prop)] = SymAuto
			break
		}
		if cssVal.IsNumeric() || cssVal.Unit != "" {
			if dim, ok := convertStyleMapDimension(symbolIDOr(prop), cssVal); ok {
				out[symbolIDOr(prop)] = dim
			}
		}
	case "padding_left", "padding_right", "padding_top", "padding_bottom":
		if cssVal.IsNumeric() || cssVal.Unit != "" {
			if dim, ok := convertStyleMapDimension(symbolIDOr(prop), cssVal); ok {
				out[symbolIDOr(prop)] = dim
			}
		}
	case "keep_lines_together":
		if cssVal.Keyword != "" || cssVal.Raw != "" || cssVal.IsNumeric() {
			keep := make(map[KFXSymbol]any)
			if count, ok := parseIntValue(cssVal, rawVal); ok {
				switch strings.ToLower(sourceAttr) {
				case "orphans":
					// KP3 maps CSS orphans to the keep "first" field.
					keep[SymKeepFirst] = count
				case "widows":
					// KP3 maps CSS widows to the keep "last" field.
					keep[SymKeepLast] = count
				}
			}
			if len(keep) > 0 {
				out[SymKeepLinesTogether] = keep
			}
		}
	case "shadows", "text_shadows":
		val := firstNonEmpty(cssVal.Raw, rawVal)
		if shadows, ok := parseShadows(val, prop == "text_shadows", log); ok {
			out[symbolIDOr(prop)] = shadows
		}
	case "transform":
		if sourceAttr == "-webkit-transform" {
			return nil, false
		}
		if cssVal.Raw != "" {
			out[SymTransform] = cssVal.Raw
		}
	case "border_color", "border_color_top", "border_color_left", "border_color_bottom", "border_color_right", "stroke_color":
		if r, g, b, ok := ParseColor(cssVal); ok {
			out[symbolIDOr(prop)] = MakeColorValue(r, g, b)
		}
	default:
		if sym, ok := symbolIDFromString(prop); ok && sym != 0 {
			switch valType {
			case "measure":
				dim, err := MakeDimensionValue(cssVal)
				if err == nil {
					out[sym] = dim
				} else if rawVal != "" {
					if dim, err := MakeDimensionValue(parseStyleMapCSSValue(rawVal, unit)); err == nil {
						out[sym] = dim
					}
				}
			case "int":
				if unit == "color_unit" {
					if r, g, b, ok := ParseColor(cssVal); ok {
						out[sym] = MakeColorValue(r, g, b)
						break
					}
					if rawVal != "" {
						if r, g, b, ok := ParseColor(parseStyleMapCSSValue(rawVal, unit)); ok {
							out[sym] = MakeColorValue(r, g, b)
							break
						}
					}
				}
				if cssVal.IsNumeric() {
					out[sym] = int(cssVal.Value)
					break
				}
				if n, err := strconv.Atoi(rawVal); err == nil {
					out[sym] = n
				}
			case "bool":
				val := rawVal
				if val == "" {
					val = cssVal.Keyword
				}
				if val == "" && cssVal.Raw != "" {
					val = cssVal.Raw
				}
				if val != "" {
					out[sym] = strings.EqualFold(val, "true")
				}
			case "composite":
				if cssVal.Raw != "" {
					out[sym] = cssVal.Raw
				} else if rawVal != "" {
					out[sym] = rawVal
				} else if !isEmptyCSSValue(cssVal) {
					out[sym] = formatCSSValue(cssVal)
				}
			case "string,string":
				parts := splitCSV(rawVal)
				if len(parts) > 0 {
					out[sym] = parts[0]
				}
			default:
				val := firstNonEmpty(cssVal.Keyword, formatCSSValue(cssVal), rawVal)
				if val != "" {
					if vsym, ok := symbolIDFromString(strings.ReplaceAll(val, "-", "_")); ok {
						out[sym] = vsym
					} else {
						out[sym] = val
					}
				}
			}
		}
	}

	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

func parseDimensionFromCSS(cssVal css.Value, rawVal, unit string) (StructValue, bool) {
	if pct, ok := positionKeywordPercent(firstNonEmpty(cssVal.Keyword, rawVal)); ok {
		return DimensionValue(pct, SymUnitPercent), true
	}
	if dim, err := MakeDimensionValue(cssVal); err == nil {
		return dim, true
	}
	if rawVal != "" {
		if dim, err := MakeDimensionValue(parseStyleMapCSSValue(rawVal, unit)); err == nil {
			return dim, true
		}
	}
	return nil, false
}

func parseXYPair(cssVal css.Value, rawVal string) (StructValue, StructValue, bool) {
	val := firstNonEmpty(cssVal.Raw, rawVal, cssVal.Keyword, formatCSSValue(cssVal))
	parts := splitStyleTokens(val)
	switch len(parts) {
	case 0:
		return nil, nil, false
	case 1:
		token := parts[0]
		if pct, ok := positionKeywordPercent(token); ok {
			switch strings.ToLower(token) {
			case "left", "right", "center":
				return DimensionValue(pct, SymUnitPercent), DimensionValue(50, SymUnitPercent), true
			case "top", "bottom":
				return DimensionValue(50, SymUnitPercent), DimensionValue(pct, SymUnitPercent), true
			default:
				return nil, nil, false
			}
		}
		if dim, ok := parseDimensionToken(token); ok {
			return dim, DimensionValue(50, SymUnitPercent), true
		}
		return nil, nil, false
	}

	x, okx := parseDimensionToken(parts[0])
	y, oky := parseDimensionToken(parts[1])
	if !okx || !oky {
		return nil, nil, false
	}
	return x, y, true
}

func parseBackgroundSize(cssVal css.Value, rawVal string) (StructValue, StructValue, bool) {
	val := firstNonEmpty(cssVal.Raw, rawVal, cssVal.Keyword, formatCSSValue(cssVal))
	parts := splitStyleTokens(val)
	if len(parts) == 0 {
		return nil, nil, false
	}

	var x, y StructValue
	if len(parts) == 1 {
		if dim, ok := parseDimensionToken(parts[0]); ok {
			x = dim
		} else {
			return nil, nil, false
		}
	} else {
		firstDim, firstOK := parseDimensionToken(parts[0])
		secondDim, secondOK := parseDimensionToken(parts[1])
		switch {
		case firstOK && secondOK:
			x, y = firstDim, secondDim
		case strings.EqualFold(parts[0], "auto") && secondOK:
			y = secondDim
		case strings.EqualFold(parts[1], "auto") && firstOK:
			x = firstDim
		default:
			return nil, nil, false
		}
	}

	if x == nil && y == nil {
		return nil, nil, false
	}
	return x, y, true
}

func parseDimensionToken(token string) (StructValue, bool) {
	token = strings.TrimSpace(token)
	if token == "" || strings.EqualFold(token, "auto") {
		return nil, false
	}
	if pct, ok := positionKeywordPercent(token); ok {
		return DimensionValue(pct, SymUnitPercent), true
	}
	css := parseStyleMapCSSValue(token, "")
	if !css.IsNumeric() {
		return nil, false
	}
	if dim, err := MakeDimensionValue(css); err == nil {
		return dim, true
	}
	return nil, false
}

func parseMaxCropPercentage(val string) (StructValue, bool) {
	parts := splitStyleTokens(val)
	if len(parts) == 0 {
		return nil, false
	}

	var top, right, bottom, left float64
	switch len(parts) {
	case 1:
		v, ok := parsePercent(parts[0])
		if !ok {
			return nil, false
		}
		top, right, bottom, left = v, v, v, v
	case 2:
		v1, ok1 := parsePercent(parts[0])
		v2, ok2 := parsePercent(parts[1])
		if !ok1 || !ok2 {
			return nil, false
		}
		top, bottom = v1, v1
		right, left = v2, v2
	case 4:
		v1, ok1 := parsePercent(parts[0])
		v2, ok2 := parsePercent(parts[1])
		v3, ok3 := parsePercent(parts[2])
		v4, ok4 := parsePercent(parts[3])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, false
		}
		top, right, bottom, left = v1, v2, v3, v4
	default:
		return nil, false
	}

	if top < 0 || right < 0 || bottom < 0 || left < 0 {
		return nil, false
	}
	if top+bottom > 100 || left+right > 100 {
		return nil, false
	}

	return NewStruct().
		SetStruct(SymTop, DimensionValue(top, SymUnitPercent)).
		SetStruct(SymRight, DimensionValue(right, SymUnitPercent)).
		SetStruct(SymBottom, DimensionValue(bottom, SymUnitPercent)).
		SetStruct(SymLeft, DimensionValue(left, SymUnitPercent)), true
}

func parsePageBleed(val string) map[KFXSymbol]any {
	parts := splitStyleTokens(val)
	if len(parts) == 0 {
		return nil
	}

	result := make(map[KFXSymbol]any)
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "left", "all":
			result[symbolIDOr("yj.user_margin_left_percentage")] = DimensionValue(-100, SymUnitPercent)
		}
		switch strings.ToLower(part) {
		case "right", "all":
			result[symbolIDOr("yj.user_margin_right_percentage")] = DimensionValue(-100, SymUnitPercent)
		}
		switch strings.ToLower(part) {
		case "top", "all":
			result[symbolIDOr("yj.user_margin_top_percentage")] = DimensionValue(-100, SymUnitPercent)
		}
		switch strings.ToLower(part) {
		case "bottom", "all":
			result[symbolIDOr("yj.user_margin_bottom_percentage")] = DimensionValue(-100, SymUnitPercent)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// KVG path command constants for yj.border_path serialization.
// These match KP3's KVG path builder commands (com.amazon.B.d.c.a.d).
const (
	kvgMoveTo    = float64(0) // MOVE_TO: start a new sub-path at (x, y)
	kvgLineTo    = float64(1) // LINE_TO: draw line to (x, y)
	kvgClosePath = float64(4) // CLOSE_PATH: close the current sub-path
)

// parsePolygonPath parses a CSS polygon() function value into a KVG path
// represented as a flat Ion list of float64 values.
//
// KP3 reference: ShapeOutsideTransformer.java
//   - Only accepts polygon() function (rejects circle, ellipse, inset)
//   - Each coordinate pair must have exactly 2 values separated by whitespace
//   - Both X and Y must use percent (%) units
//   - Converts percent (0-100) to fractional (0.0-1.0) by dividing by 100
//   - Builds KVG path: first point → moveTo, subsequent → lineTo, then closePath
//
// The output is a flat ListValue of float64 values:
//
//	[command, x, y, command, x, y, ..., closePath]
//
// Example: polygon(0% 0%, 100% 0%, 100% 100%, 0% 100%)
// → [0, 0, 0, 1, 1, 0, 1, 1, 1, 1, 0, 1, 4]
func parsePolygonPath(val string) (ListValue, bool) {
	lower := strings.ToLower(strings.TrimSpace(val))

	// KP3: only accepts polygon() function.
	if !strings.HasPrefix(lower, "polygon") {
		return nil, false
	}

	// Extract content between parentheses.
	openIdx := strings.IndexByte(lower, '(')
	closeIdx := strings.IndexByte(lower, ')')
	if openIdx < 0 || closeIdx < 0 || openIdx+1 >= len(lower) || openIdx+1 >= closeIdx {
		return nil, false
	}
	inner := lower[openIdx+1 : closeIdx]

	// Split by comma to get coordinate pairs.
	pairs := strings.Split(inner, ",")
	if len(pairs) == 0 {
		return nil, false
	}

	path := NewList()
	first := true

	for _, pair := range pairs {
		tokens := strings.Fields(strings.TrimSpace(pair))
		// KP3: exactly 2 values per coordinate pair.
		if len(tokens) != 2 {
			return nil, false
		}

		// KP3: both X and Y must use percent units.
		x, okx := parsePolygonPercent(tokens[0])
		y, oky := parsePolygonPercent(tokens[1])
		if !okx || !oky {
			return nil, false
		}

		// KP3: first point is moveTo, subsequent are lineTo.
		if first {
			path.Add(kvgMoveTo)
			first = false
		} else {
			path.Add(kvgLineTo)
		}
		path.Add(x)
		path.Add(y)
	}

	// Must have at least one point.
	if first {
		return nil, false
	}

	// KP3: close the path after all points.
	path.Add(kvgClosePath)
	return path, true
}

// parsePolygonPercent parses a single polygon coordinate token.
// KP3 requires percent units and converts to fractional (0.0-1.0).
func parsePolygonPercent(token string) (float64, bool) {
	token = strings.TrimSpace(token)
	if !strings.HasSuffix(token, "%") {
		return 0, false
	}
	numStr := strings.TrimSuffix(token, "%")
	if numStr == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, false
	}
	return v / 100.0, true
}

func parseShadows(raw string, isText bool, log *zap.Logger) ([]StructValue, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}

	shadows := make([]StructValue, 0)
	for _, part := range splitShadowEntries(raw) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var dims []StructValue
		var color int64
		colorSet := false

		for token := range strings.FieldsSeq(part) {
			lower := strings.ToLower(token)
			if lower == "inset" || lower == "outset" {
				if log != nil {
					log.Debug("Shadow inset/outset ignored", zap.String("value", raw))
				}
				continue
			}
			if dim, ok := parseDimensionToken(token); ok {
				if v, _, ok := measureParts(dim); ok && v == 0 {
					dim = DimensionValue(0, SymUnitPx)
				}
				dims = append(dims, dim)
				continue
			}
			if r, g, b, ok := ParseColor(css.Value{Raw: token, Keyword: token}); ok {
				color = MakeColorValue(r, g, b)
				colorSet = true
				continue
			}
			if log != nil {
				log.Debug("Shadow token unparsed", zap.String("token", token))
			}
		}

		if len(dims) < 2 {
			if log != nil {
				log.Debug("Shadow missing required offsets", zap.String("value", part))
			}
			continue
		}

		shadow := NewStruct().
			SetStruct(symbolIDOr("horizontal_offset"), dims[0]).
			SetStruct(symbolIDOr("vertical_offset"), dims[1])
		if len(dims) >= 3 {
			shadow.SetStruct(symbolIDOr("blur"), dims[2])
		}
		if !isText {
			if len(dims) >= 4 {
				shadow.SetStruct(symbolIDOr("spread"), dims[3])
			} else {
				shadow.SetStruct(symbolIDOr("spread"), DimensionValue(0, SymUnitPx))
			}
		}
		if colorSet {
			shadow.Set(symbolIDOr("color"), color)
		}
		shadows = append(shadows, shadow)
	}

	if len(shadows) == 0 {
		return nil, false
	}
	return shadows, true
}

func splitShadowEntries(raw string) []string {
	parts := make([]string, 0)
	start := 0
	depth := 0
	for i, r := range raw {
		switch r {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(raw[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(raw) {
		parts = append(parts, strings.TrimSpace(raw[start:]))
	}
	return parts
}
