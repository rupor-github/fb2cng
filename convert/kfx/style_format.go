package kfx

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/amazon-ion/ion-go/ion"
)

// FormatStylePropsAsCSS formats KFX style properties as a CSS-like one-line string.
// Input can be StructValue, map[KFXSymbol]any, or map[string]any.
// Returns empty string if input is nil or has no properties.
func FormatStylePropsAsCSS(v any) string {
	props := normalizeToNameMap(v)
	if len(props) == 0 {
		return ""
	}

	cssProps := extractCSSPropsForDisplay(props)
	if len(cssProps) == 0 {
		return ""
	}

	parts := make([]string, 0, len(cssProps))
	for _, p := range cssProps {
		parts = append(parts, p.name+": "+p.value)
	}
	return strings.Join(parts, "; ")
}

// FormatStylePropsAsCSSMultiLine formats KFX style properties as multi-line CSS block.
// Returns a formatted string like:
//
//	CSS:
//	  .styleName {
//	    inherits: .parentStyle
//	    property: value;
//	  }
func FormatStylePropsAsCSSMultiLine(v any) string {
	props := normalizeToNameMap(v)
	if len(props) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("  CSS:\n")

	// Extract style name for selector
	styleName := extractStringValueFromMap(props, "style_name")
	if styleName != "" {
		fmt.Fprintf(&sb, "    .%s {\n", styleName)
	} else {
		sb.WriteString("    {\n")
	}

	// Show parent style inheritance if present
	if parentStyle := extractStringValueFromMap(props, "parent_style"); parentStyle != "" {
		fmt.Fprintf(&sb, "      inherits: .%s\n", parentStyle)
	}

	// Extract and format CSS properties
	cssProps := extractCSSPropsForDisplay(props)
	for _, prop := range cssProps {
		fmt.Fprintf(&sb, "      %s: %s;\n", prop.name, prop.value)
	}

	sb.WriteString("    }\n")

	return sb.String()
}

// extractStringValueFromMap extracts a string from various value types in a map.
func extractStringValueFromMap(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	switch n := v.(type) {
	case string:
		return n
	case SymbolByNameValue:
		return string(n)
	case ReadSymbolValue:
		return string(n)
	default:
		return ""
	}
}

// FormatDimensionAsCSS formats a KFX dimension value {$307: value, $306: unit} as CSS.
// Returns the formatted string like "3.125%" or "1lh".
func FormatDimensionAsCSS(v any) string {
	var value, unit any

	switch m := v.(type) {
	case StructValue:
		value = m[SymValue] // $307
		unit = m[SymUnit]   // $306
	case map[KFXSymbol]any:
		value = m[SymValue]
		unit = m[SymUnit]
	case map[string]any:
		// Handle both "value"/"unit" and "$307"/"$306" keys
		if v, ok := m["value"]; ok {
			value = v
		} else if v, ok := m["$307"]; ok {
			value = v
		}
		if u, ok := m["unit"]; ok {
			unit = u
		} else if u, ok := m["$306"]; ok {
			unit = u
		}
	default:
		return formatCSSGenericValue(v)
	}

	if value == nil {
		return formatCSSGenericValue(v)
	}

	valStr := formatCSSNumericValue(value)
	unitStr := formatCSSUnitValue(unit)

	return valStr + unitStr
}

// IsDimensionValue checks if a value is a KFX dimension (has value and unit fields).
func IsDimensionValue(v any) bool {
	switch m := v.(type) {
	case StructValue:
		_, hasValue := m[SymValue]
		_, hasUnit := m[SymUnit]
		return hasValue && hasUnit
	case map[KFXSymbol]any:
		_, hasValue := m[SymValue]
		_, hasUnit := m[SymUnit]
		return hasValue && hasUnit
	case map[string]any:
		_, hasValue := m["value"]
		_, hasValue2 := m["$307"]
		_, hasUnit := m["unit"]
		_, hasUnit2 := m["$306"]
		return (hasValue || hasValue2) && (hasUnit || hasUnit2)
	default:
		return false
	}
}

// -------- Internal helpers --------

// cssPropDisplay represents a CSS property for display purposes.
type cssPropDisplay struct {
	name  string
	value string
}

// NormalizeStyleMap converts style value to map[string]any with KFX property names as keys.
// This is useful for accessing style properties by name regardless of the input format.
// Input can be StructValue, map[KFXSymbol]any, or map[string]any.
func NormalizeStyleMap(v any) map[string]any {
	return normalizeToNameMap(v)
}

// normalizeToNameMap converts style value to map[string]any with KFX property names as keys.
func normalizeToNameMap(v any) map[string]any {
	switch m := v.(type) {
	case StructValue:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[k.Name()] = val
		}
		return result
	case map[KFXSymbol]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			result[k.Name()] = val
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(m))
		for k, val := range m {
			// Convert "$NNN" keys to property names
			if strings.HasPrefix(k, "$") {
				if id, err := strconv.Atoi(k[1:]); err == nil {
					result[KFXSymbol(id).Name()] = val
					continue
				}
			}
			result[k] = val
		}
		return result
	default:
		return nil
	}
}

// cssDisplayNameOverrides maps KFX property names to CSS names.
var cssDisplayNameOverrides = map[string]string{
	"text_alignment": "text-align",
	"letterspacing":  "letter-spacing",
	"text_color":     "color",
	"fill_color":     "background-color",
	"fill_opacity":   "opacity",
	"border_weight":  "border-width",
	"baseline_shift": "vertical-align",
	"first":          "orphans",
	"last":           "widows",
}

// skipDisplayProperties are KFX properties that shouldn't be output as CSS.
var skipDisplayProperties = map[string]bool{
	"style_name":   true,
	"parent_style": true,
}

// colorDisplayProperties are KFX properties that represent color values.
var colorDisplayProperties = map[string]bool{
	"text_color":            true,
	"text_background_color": true,
	"underline_color":       true,
	"strikethrough_color":   true,
	"fill_color":            true,
	"stroke_color":          true,
	"border_color":          true,
	"overline_color":        true,
	"text_emphasis_color":   true,
}

// kfxNameToCSSDisplay converts a KFX property name to CSS property name.
func kfxNameToCSSDisplay(name string) string {
	if override, ok := cssDisplayNameOverrides[name]; ok {
		return override
	}
	return strings.ReplaceAll(name, "_", "-")
}

// extractCSSPropsForDisplay extracts CSS-like properties from a normalized KFX style map.
func extractCSSPropsForDisplay(m map[string]any) []cssPropDisplay {
	var props []cssPropDisplay

	// Get sorted keys for deterministic output
	keys := make([]string, 0, len(m))
	for k := range m {
		if !skipDisplayProperties[k] {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	for _, kfxName := range keys {
		v := m[kfxName]
		cssName := kfxNameToCSSDisplay(kfxName)

		var cssValue string
		if colorDisplayProperties[kfxName] {
			cssValue = formatCSSColorValue(v)
		} else if IsDimensionValue(v) {
			cssValue = FormatDimensionAsCSS(v)
		} else {
			cssValue = formatCSSSymbolValue(v)
		}

		props = append(props, cssPropDisplay{cssName, cssValue})
	}

	return props
}

// formatCSSNumericValue formats a numeric value for CSS output.
func formatCSSNumericValue(v any) string {
	switch vv := v.(type) {
	case float64:
		return formatCSSFloat(vv)
	case int:
		return fmt.Sprintf("%d", vv)
	case int64:
		return fmt.Sprintf("%d", vv)
	case int32:
		return fmt.Sprintf("%d", vv)
	case string:
		// Handle Ion decimal notation like "2.08333d-1"
		return formatIonDecimalStr(vv)
	case *ion.Decimal:
		return formatIonDecimalStr(vv.String())
	default:
		return fmt.Sprintf("%v", vv)
	}
}

// formatCSSFloat formats a float for CSS output (no scientific notation).
func formatCSSFloat(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%d", int(f))
	}
	// Use %f and trim trailing zeros
	s := fmt.Sprintf("%.6f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// formatIonDecimalStr converts Ion decimal notation (e.g., "2.08333d-1") to CSS number.
func formatIonDecimalStr(s string) string {
	// Check for Ion decimal notation with 'd' or 'D' exponent
	if idx := strings.IndexAny(s, "dD"); idx >= 0 {
		// Replace 'd' with 'e' for Go's float parser
		normalized := strings.Replace(s, "d", "e", 1)
		normalized = strings.Replace(normalized, "D", "e", 1)
		if f, err := strconv.ParseFloat(normalized, 64); err == nil {
			return formatCSSFloat(f)
		}
	}
	// Try parsing as regular float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return formatCSSFloat(f)
	}
	return s
}

// formatCSSUnitValue formats a unit value for CSS output.
func formatCSSUnitValue(u any) string {
	switch unit := u.(type) {
	case SymbolValue:
		return kfxUnitSymbolToCSS(KFXSymbol(unit))
	case KFXSymbol:
		return kfxUnitSymbolToCSS(unit)
	case ReadSymbolValue:
		return kfxUnitNameToCSS(string(unit))
	case string:
		return kfxUnitNameToCSS(unit)
	default:
		return fmt.Sprintf("%v", u)
	}
}

// cssUnitOverrides maps KFX unit symbols to CSS unit strings.
var cssUnitOverrides = map[string]string{
	"percent": "%",
	"lh":      "lh",
	"em":      "em",
	"ex":      "ex",
	"rem":     "rem",
	"px":      "px",
	"pt":      "pt",
	"cm":      "cm",
	"mm":      "mm",
	"in":      "in",
	"ratio":   "", // unitless
}

// kfxUnitSymbolToCSS converts a KFX unit symbol to CSS unit string.
func kfxUnitSymbolToCSS(sym KFXSymbol) string {
	name := sym.Name()
	if override, ok := cssUnitOverrides[name]; ok {
		return override
	}
	return name
}

// kfxUnitNameToCSS converts a unit name (like "em", "ratio", "$308") to CSS unit.
func kfxUnitNameToCSS(u string) string {
	if override, ok := cssUnitOverrides[u]; ok {
		return override
	}
	// Handle "$308" style string representation
	if len(u) > 1 && u[0] == '$' {
		if id, err := strconv.Atoi(u[1:]); err == nil {
			return kfxUnitSymbolToCSS(KFXSymbol(id))
		}
	}
	return u
}

// formatCSSSymbolValue formats a KFX symbol value to CSS keyword.
func formatCSSSymbolValue(v any) string {
	switch s := v.(type) {
	case SymbolValue:
		return kfxValueSymbolToCSS(KFXSymbol(s))
	case KFXSymbol:
		return kfxValueSymbolToCSS(s)
	case ReadSymbolValue:
		return kfxValueNameToCSS(string(s))
	case string:
		// Handle "$320" style string representation
		if len(s) > 1 && s[0] == '$' {
			if id, err := strconv.Atoi(s[1:]); err == nil {
				return kfxValueSymbolToCSS(KFXSymbol(id))
			}
		}
		return s
	case []any:
		return formatCSSListValue(s)
	case ListValue:
		return formatCSSListValue([]any(s))
	case StructValue:
		return formatCSSMapValue(s)
	case map[KFXSymbol]any:
		return formatCSSMapValue(s)
	case map[string]any:
		return formatCSSMapStringValue(s)
	case bool:
		return fmt.Sprintf("%v", s)
	case int, int64, int32:
		return fmt.Sprintf("%d", s)
	case float64:
		return formatCSSFloat(s)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// cssValueSymbolOverrides maps KFX symbol names to CSS values.
var cssValueSymbolOverrides = map[string]string{
	"semibold": "600",
	"light":    "300",
	"medium":   "500",
}

// kfxValueSymbolToCSS converts a KFX value symbol to CSS keyword.
func kfxValueSymbolToCSS(sym KFXSymbol) string {
	name := sym.Name()
	if override, ok := cssValueSymbolOverrides[name]; ok {
		return override
	}
	return name
}

// kfxValueNameToCSS converts a value name to CSS.
func kfxValueNameToCSS(s string) string {
	if override, ok := cssValueSymbolOverrides[s]; ok {
		return override
	}
	// Handle "$NNN" style string representation
	if len(s) > 1 && s[0] == '$' {
		if id, err := strconv.Atoi(s[1:]); err == nil {
			return kfxValueSymbolToCSS(KFXSymbol(id))
		}
	}
	return s
}

// formatCSSListValue formats a list of values for CSS output.
func formatCSSListValue(items []any) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = formatCSSSymbolValue(item)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// formatCSSMapValue formats a nested KFX map for CSS output.
func formatCSSMapValue(m map[KFXSymbol]any) string {
	if len(m) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(m))
	keys := make([]KFXSymbol, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		val := m[k]
		keyName := k.Name()
		var valStr string
		if colorDisplayProperties[keyName] {
			valStr = formatCSSColorValue(val)
		} else if IsDimensionValue(val) {
			valStr = FormatDimensionAsCSS(val)
		} else {
			valStr = formatCSSSymbolValue(val)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", keyName, valStr))
	}
	return "{" + strings.Join(parts, "; ") + "}"
}

// formatCSSMapStringValue formats a nested map[string]any for CSS output.
func formatCSSMapStringValue(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		val := m[k]
		keyName := k
		// Convert "$NNN" to symbol name
		if strings.HasPrefix(k, "$") {
			if id, err := strconv.Atoi(k[1:]); err == nil {
				keyName = KFXSymbol(id).Name()
			}
		}
		var valStr string
		if colorDisplayProperties[keyName] {
			valStr = formatCSSColorValue(val)
		} else if IsDimensionValue(val) {
			valStr = FormatDimensionAsCSS(val)
		} else {
			valStr = formatCSSSymbolValue(val)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", keyName, valStr))
	}
	return "{" + strings.Join(parts, "; ") + "}"
}

// formatCSSColorValue formats an ARGB color value as hex string.
func formatCSSColorValue(v any) string {
	var colorInt uint32
	switch c := v.(type) {
	case int:
		colorInt = uint32(c)
	case int32:
		colorInt = uint32(c)
	case int64:
		colorInt = uint32(c)
	case uint32:
		colorInt = c
	case uint64:
		colorInt = uint32(c)
	case float64:
		colorInt = uint32(c)
	default:
		// Not a numeric color, fall back to default formatting
		return formatCSSSymbolValue(v)
	}

	// Format as #AARRGGBB hex string
	return fmt.Sprintf("#%08X", colorInt)
}

// formatCSSGenericValue formats a generic value for CSS output.
func formatCSSGenericValue(v any) string {
	switch val := v.(type) {
	case nil:
		return "none"
	case bool:
		return fmt.Sprintf("%v", val)
	case int, int64, int32:
		return fmt.Sprintf("%d", val)
	case float64:
		return formatCSSFloat(val)
	case string:
		return fmt.Sprintf("%q", val)
	case SymbolValue:
		return kfxValueSymbolToCSS(KFXSymbol(val))
	case SymbolByNameValue:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// -------- Exported helpers for kfxdump and other tools --------

// FormatCSSFloat formats a float for CSS output (no scientific notation).
func FormatCSSFloat(f float64) string {
	return formatCSSFloat(f)
}

// FormatIonDecimal converts Ion decimal notation (e.g., "2.08333d-1") to CSS number.
func FormatIonDecimal(s string) string {
	return formatIonDecimalStr(s)
}

// FormatCSSSymbolValue formats a KFX symbol value to CSS keyword.
// Handles SymbolValue, KFXSymbol, ReadSymbolValue, string, lists, and maps.
func FormatCSSSymbolValue(v any) string {
	return formatCSSSymbolValue(v)
}

// FormatCSSColorValue formats an ARGB color value as hex string (#AARRGGBB).
func FormatCSSColorValue(v any) string {
	return formatCSSColorValue(v)
}

// FormatCSSListValue formats a list of values for CSS output.
func FormatCSSListValue(items []any) string {
	return formatCSSListValue(items)
}

// FormatCSSMapValue formats a nested KFX map (map[KFXSymbol]any) for CSS output.
func FormatCSSMapValue(m map[KFXSymbol]any) string {
	return formatCSSMapValue(m)
}

// FormatCSSMapStringValue formats a nested map[string]any for CSS output.
func FormatCSSMapStringValue(m map[string]any) string {
	return formatCSSMapStringValue(m)
}

// KFXSymbolToCSS converts a KFX symbol to CSS keyword, applying value overrides.
func KFXSymbolToCSS(sym KFXSymbol) string {
	return kfxValueSymbolToCSS(sym)
}

// KFXUnitToCSS converts a unit name (like "em", "ratio", "$308") to CSS unit.
func KFXUnitToCSS(u string) string {
	return kfxUnitNameToCSS(u)
}

// IsColorProperty returns true if the property name represents a color value.
func IsColorProperty(propName string) bool {
	return colorDisplayProperties[propName]
}
