package kfx

import (
	"fmt"
	"maps"
	"reflect"
	"strings"

	"go.uber.org/zap"
)

// Converter converts CSS rules to KFX style definitions.
type Converter struct {
	log    *zap.Logger
	tracer *StyleTracer
}

// NewConverter creates a new CSS-to-KFX converter.
func NewConverter(log *zap.Logger) *Converter {
	if log == nil {
		log = zap.NewNop()
	}
	return &Converter{log: log.Named("css-converter")}
}

func (c *Converter) SetTracer(t *StyleTracer) {
	c.tracer = t
}

// ConversionResult holds the result of converting a CSS rule to KFX.
type ConversionResult struct {
	Style    StyleDef
	Warnings []string
}

// zeroValueProps lists CSS properties that should be dropped when their value is zero.
// Based on Amazon's YJHtmlMapper (com.amazon.yjhtmlmapper.g.a) which filters these
// properties with zero values as they are effectively no-ops.
var zeroValueProps = map[string]bool{
	"font-size":      true,
	"padding-right":  true,
	"padding-left":   true,
	"padding-top":    true,
	"padding-bottom": true,
	"margin-right":   true,
	"margin-left":    true,
	"margin-top":     true,
	"margin-bottom":  true,
}

// textDecorationControlTags lists HTML elements for which text-decoration: none
// has semantic meaning and must be preserved. For all other elements, KP3 strips
// text-decoration: none as a no-op (com/amazon/yjhtmlmapper/h/b.java:373-395).
//
// For reflowable books (which is what fb2cng produces), <a> is always in this set.
// In KP3, the <a> exemption depends on a fixed-layout flag: in fixed-layout books
// <a> is treated as a normal element (text-decoration: none is stripped), but in
// reflowable books <a> is exempted (text-decoration: none is preserved to allow
// removing the default hyperlink underline).
var textDecorationControlTags = map[string]bool{
	"u":      true,
	"a":      true,
	"ins":    true,
	"del":    true,
	"s":      true,
	"strike": true,
	"br":     true,
}

func normalizeCSSProperties(props map[string]CSSValue, element string, tracer *StyleTracer, context string) map[string]CSSValue {
	if len(props) == 0 {
		return props
	}

	normalized := make(map[string]CSSValue, len(props))
	changed := false

	for name, val := range props {
		if shouldDropZeroValue(name, val) || isEmptyCSSValue(val) {
			changed = true
			continue
		}
		// KP3 converts ex units to em early in the normalization pipeline
		// (com/amazon/yjhtmlmapper/h/b.java:253-263). The conversion uses
		// a 0.44 factor defined in com/amazon/yj/F/a/b.java:24.
		if val.Unit == "ex" {
			val.Value *= ExToEmFactor
			val.Unit = "em"
			if val.Raw != "" {
				val.Raw = fmt.Sprintf("%g%s", val.Value, val.Unit)
			}
			changed = true
		}
		// KP3 removes text-decoration: none for elements that are NOT in the
		// decoration-control set (com/amazon/yjhtmlmapper/h/b.java:373-395).
		// For elements like <u>, <a>, <ins>, <del>, <s>, <strike>, <br>,
		// text-decoration: none has semantic meaning (e.g., removing the inherent
		// underline from <u>) so it is preserved. For all other elements it's a
		// no-op and is stripped.
		// When element is empty (class-only selector), we conservatively keep it
		// since we can't determine which element the class applies to.
		if name == "text-decoration" && strings.EqualFold(strings.TrimSpace(val.Keyword), "none") {
			if element != "" && !textDecorationControlTags[strings.ToLower(element)] {
				changed = true
				continue
			}
		}
		if name == "text-decoration" && val.Keyword == "" && strings.EqualFold(strings.TrimSpace(val.Raw), "none") {
			if element != "" && !textDecorationControlTags[strings.ToLower(element)] {
				changed = true
				continue
			}
		}
		normalized[name] = val
	}

	if tracer != nil && tracer.IsEnabled() && changed && context != "" {
		tracer.TraceNormalize(context, cssValuesToStrings(props), cssValuesToStrings(normalized))
	}

	return normalized
}

func shouldDropZeroValue(name string, val CSSValue) bool {
	if !zeroValueProps[name] {
		return false
	}
	if !val.IsNumeric() {
		return false
	}
	if val.Value == 0 && val.Keyword == "" {
		return true
	}
	if val.Raw != "" {
		raw := strings.TrimSpace(val.Raw)
		if raw == "0" || raw == "0px" || raw == "0%" || raw == "0em" || raw == "0rem" {
			return true
		}
	}
	return false
}

func isEmptyCSSValue(val CSSValue) bool {
	return val.Raw == "" && val.Keyword == "" && val.Value == 0 && val.Unit == ""
}

func cssValuesToStrings(src map[string]CSSValue) map[string]string {
	out := make(map[string]string, len(src))
	for k, v := range src {
		if s := formatCSSValue(v); s != "" {
			out[k] = s
		}
	}
	return out
}

func formatCSSValue(val CSSValue) string {
	switch {
	case val.Raw != "":
		return val.Raw
	case val.Keyword != "":
		return val.Keyword
	case val.Value != 0 || val.Unit != "":
		return fmt.Sprintf("%g%s", val.Value, val.Unit)
	default:
		return ""
	}
}

// ConvertRule converts a single CSS rule to a KFX StyleDef.
func (c *Converter) ConvertRule(rule CSSRule) ConversionResult {
	result := ConversionResult{
		Style: StyleDef{
			Name:       rule.Selector.StyleName(),
			Properties: make(map[KFXSymbol]any),
		},
		Warnings: make([]string, 0),
	}

	props := normalizeCSSProperties(rule.Properties, rule.Selector.Element, c.tracer, rule.Selector.Raw)

	for propName, propValue := range props {
		before := snapshotProps(result.Style.Properties)
		c.convertProperty(propName, propValue, result.Style.Properties, &result.Warnings)
		if c.tracer != nil && c.tracer.IsEnabled() {
			emitted := diffProps(before, result.Style.Properties)
			if len(emitted) > 0 {
				c.tracer.TraceMap(propName, "", emitted)
			}
		}
	}

	if rule.Selector.Ancestor != nil && result.Style.Parent == "" {
		descendantName := rule.Selector.descendantBaseName()
		if descendantName != "" && descendantName != result.Style.Name {
			result.Style.Parent = descendantName
		}
	}

	return result
}

// convertProperty converts a single CSS property to KFX properties.
func (c *Converter) convertProperty(name string, value CSSValue, props map[KFXSymbol]any, warnings *[]string) {
	// Handle shorthand properties first
	if IsShorthandProperty(name) {
		c.expandShorthand(name, value, props, warnings)
		return
	}

	// Handle special properties
	if IsSpecialProperty(name) {
		c.convertSpecialProperty(name, value, props, warnings)
		return
	}

	// Look up the KFX symbol
	kfxSym := KFXPropertySymbol(name)
	if kfxSym == SymbolUnknown {
		// Unknown property - log at debug level
		c.log.Debug("Unknown CSS property", zap.String("property", name))
		return
	}

	// Convert based on property type
	switch name {
	case "font-weight":
		if sym, ok := ConvertFontWeight(value); ok {
			c.mergeProp(props, SymFontWeight, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "font-style":
		if sym, ok := ConvertFontStyle(value); ok {
			c.mergeProp(props, SymFontStyle, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "text-align":
		if sym, ok := ConvertTextAlign(value); ok {
			c.mergeProp(props, SymTextAlignment, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "hyphens", "-webkit-hyphens":
		if sym, ok := ConvertHyphens(value); ok {
			c.mergeProp(props, SymHyphens, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "writing-mode", "-webkit-writing-mode":
		if sym, ok := ConvertWritingMode(value); ok {
			c.mergeProp(props, SymWritingMode, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "text-orientation":
		if sym, ok := ConvertTextOrientation(value); ok {
			c.mergeProp(props, SymTextOrientation, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "text-combine-upright", "text-combine":
		if sym, ok := ConvertTextCombine(value); ok {
			c.mergeProp(props, SymTextCombine, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "text-emphasis-style", "-webkit-text-emphasis-style":
		if sym, ok := ConvertTextEmphasisStyle(value); ok {
			c.mergeProp(props, SymTextEmphasisStyle, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "text-emphasis-color", "-webkit-text-emphasis-color":
		if r, g, b, ok := ParseColor(value); ok {
			c.mergeProp(props, SymTextEmphasisColor, MakeColorValue(r, g, b))
		} else {
			*warnings = append(*warnings, "unable to parse text-emphasis-color: "+value.Raw)
		}

	case "float":
		if sym, ok := ConvertFloat(value); ok {
			c.mergeProp(props, SymFloat, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "clear":
		if sym, ok := ConvertClear(value); ok {
			c.mergeProp(props, SymFloatClear, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "yj-break-before", "yj-break-after":
		// KFX-specific break properties from stylemap
		if sym, ok := convertYjBreak(value); ok {
			c.mergeProp(props, kfxSym, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "underline", "overline", "strikethrough":
		// Text decoration boolean properties from stylemap.
		// These come from stylemap entries that set them directly.
		// The value is typically "true" or the property name itself.
		if !strings.EqualFold(value.Keyword, "none") && !strings.EqualFold(value.Raw, "none") {
			c.mergeProp(props, kfxSym, true)
		}

	case "baseline-style":
		// Baseline style from vertical-align mapping in stylemap.
		// Values: center, top, bottom
		if sym, ok := ConvertBaselineStyle(value); ok {
			c.mergeProp(props, kfxSym, sym)
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "table-border-collapse":
		// Table border collapse: true/false
		val := strings.ToLower(firstNonEmpty(value.Keyword, value.Raw))
		c.mergeProp(props, kfxSym, strings.EqualFold(val, "true") || strings.EqualFold(val, "collapse"))

	case "border-spacing-vertical", "border-spacing-horizontal":
		// Table border spacing dimensions
		c.setDimensionProperty(kfxSym, value, props, warnings)

	case "color":
		if r, g, b, ok := ParseColor(value); ok {
			c.mergeProp(props, SymTextColor, MakeColorValue(r, g, b))
		} else {
			*warnings = append(*warnings, "unable to parse color: "+value.Raw)
		}

	case "background-color":
		if r, g, b, ok := ParseColor(value); ok {
			c.mergeProp(props, SymFillColor, MakeColorValue(r, g, b))
		} else {
			*warnings = append(*warnings, "unable to parse background-color: "+value.Raw)
		}

	case "border-color":
		if r, g, b, ok := ParseColor(value); ok {
			c.mergeProp(props, SymBorderColor, MakeColorValue(r, g, b))
		} else {
			*warnings = append(*warnings, "unable to parse border-color: "+value.Raw)
		}

	case "font-family":
		// Transform CSS font-family to KFX format.
		// KP3 keeps generic families (e.g. monospace) unprefixed.
		kfxFamily := ToKFXFontFamilyFromCSS(value.Raw)
		if kfxFamily != "" {
			c.mergeProp(props, SymFontFamily, kfxFamily)
		}

	case "font-size":
		// Handle keyword values first
		// Amazon converts: smaller -> 0.8333em (5/6), larger -> 1.2em
		switch strings.ToLower(value.Keyword) {
		case "smaller":
			// Amazon uses 0.8333333333333334em (5/6)
			c.mergeProp(props, kfxSym, DimensionValue(0.8333333333333334, SymUnitEm))
			return
		case "larger":
			// Amazon uses 1.2em for larger
			c.mergeProp(props, kfxSym, DimensionValue(1.2, SymUnitEm))
			return
		}
		// KP3 compresses percentage font-sizes towards 1rem using the formula:
		// rem = 1 + (percent - 100) / 160
		// This is important for title rendering - percent units cause alignment issues
		if value.IsNumeric() {
			switch value.Unit {
			case "%":
				// Convert percentage to rem with KP3's compression formula
				// 140% -> 1.25rem, 120% -> 1.125rem, 100% -> 1rem
				remValue := PercentToRem(value.Value)
				c.mergeProp(props, kfxSym, DimensionValue(remValue, SymUnitRem))
			case "em":
				// Keep em units for font-size to enable relative merging.
				// When nested inline styles are merged (e.g., sup + link-footnote),
				// the stylelist rule font_size,true,*,em triggers YJRelativeRuleMerger
				// which multiplies: sup (0.75rem) * link-footnote (0.8em) = 0.6rem
				// The em→rem conversion happens at output time in BuildFragments.
				//
				// NOTE: This differs from KP3, which creates overlapping style events
				// and stores 0.75rem in both (ignoring the 0.8em). Our CSS-correct
				// multiplication produces the same visual result but with proper cascade.
				c.mergeProp(props, kfxSym, DimensionValue(value.Value, SymUnitEm))
			default:
				dim, err := MakeDimensionValue(value)
				if err != nil {
					*warnings = append(*warnings, "unable to convert "+name+": "+err.Error())
					return
				}
				c.mergeProp(props, kfxSym, dim)
			}
		}

	case "text-indent":
		// KP3 uses % for text-indent. Convert em → % using EmToPercentTextIndent ratio.
		// Note: text-indent: 0 is meaningful in KFX - it explicitly sets no indentation,
		// which overrides any inherited text-indent. KP3 uses "text-indent: 0%" explicitly.
		if value.IsNumeric() {
			if value.Unit == "" || value.Unit == "%" {
				c.mergeProp(props, kfxSym, DimensionValue(value.Value, SymUnitPercent))
				return
			}
			if value.Unit == "em" {
				c.mergeProp(props, kfxSym, DimensionValue(value.Value*EmToPercentTextIndent, SymUnitPercent))
				return
			}
			dim, err := MakeDimensionValue(value)
			if err != nil {
				*warnings = append(*warnings, "unable to convert "+name+": "+err.Error())
				return
			}
			c.mergeProp(props, kfxSym, dim)
		}

	case "line-height":
		// KP3 uses lh units for line-height. Convert em → lh using LineHeightRatio.
		if value.IsNumeric() {
			if value.Unit == "" || value.Unit == "lh" {
				// Unitless or already lh - use lh unit
				c.mergeProp(props, kfxSym, DimensionValue(value.Value, SymUnitLh))
				return
			}
			if value.Unit == "em" {
				// Convert em to lh
				c.mergeProp(props, kfxSym, DimensionValue(value.Value/LineHeightRatio, SymUnitLh))
				return
			}
			dim, err := MakeDimensionValue(value)
			if err != nil {
				*warnings = append(*warnings, "unable to convert "+name+": "+err.Error())
				return
			}
			c.mergeProp(props, kfxSym, dim)
		}

	case "margin-top", "margin-bottom", "margin-left", "margin-right",
		"padding-top", "padding-bottom", "padding-left", "padding-right":
		// Route through setDimensionProperty for proper KP3 unit conversion
		c.setDimensionProperty(kfxSym, value, props, warnings)

	default:
		// Dimension properties (font-size, margins, line-height, etc.)
		if value.IsNumeric() || value.Unit != "" || (value.Value != 0) {
			dim, err := MakeDimensionValue(value)
			if err != nil {
				*warnings = append(*warnings, "unable to convert "+name+": "+err.Error())
				return
			}
			c.mergeProp(props, kfxSym, dim)
		} else if value.IsKeyword() {
			// Some dimension properties accept keywords like "auto"
			switch strings.ToLower(value.Keyword) {
			case "auto":
				c.mergeProp(props, kfxSym, SymAuto)
			case "inherit":
				// Skip inherit - KFX handles inheritance differently
			default:
				c.log.Debug("Unhandled keyword value",
					zap.String("property", name),
					zap.String("value", value.Keyword))
			}
		}
	}
}

func (c *Converter) mergeProp(props map[KFXSymbol]any, sym KFXSymbol, val any) {
	mergePropertyWithRules(props, sym, val, mergeContextInline, c.tracer)
}

func (c *Converter) logUnsupportedValue(property string, value CSSValue) {
	if v := formatCSSValue(value); v != "" {
		c.log.Debug("Unsupported CSS value ignored",
			zap.String("property", property),
			zap.String("value", v))
	}
}

func snapshotProps(src map[KFXSymbol]any) map[KFXSymbol]any {
	out := make(map[KFXSymbol]any, len(src))
	maps.Copy(out, src)
	return out
}

func diffProps(before, after map[KFXSymbol]any) map[KFXSymbol]any {
	out := make(map[KFXSymbol]any)
	for k, v := range after {
		if ov, ok := before[k]; !ok || !reflect.DeepEqual(ov, v) {
			out[k] = v
		}
	}
	return out
}

// expandShorthand expands CSS shorthand properties into individual properties.
func (c *Converter) expandShorthand(name string, value CSSValue, props map[KFXSymbol]any, warnings *[]string) {
	switch name {
	case "margin":
		c.expandBoxShorthand(value, props, warnings,
			SymMarginTop, SymMarginRight, SymMarginBottom, SymMarginLeft)

	case "padding":
		c.expandBoxShorthand(value, props, warnings,
			SymPaddingTop, SymPaddingRight, SymPaddingBottom, SymPaddingLeft)

	case "border":
		c.expandBorderShorthand(value, props, warnings)

	case "background":
		c.expandBackgroundShorthand(value, props, warnings)
	}
}

// expandBoxShorthand expands a CSS box model shorthand (margin, padding) to individual properties.
// CSS shorthand formats:
//   - 1 value: all sides
//   - 2 values: top/bottom, left/right
//   - 3 values: top, left/right, bottom
//   - 4 values: top, right, bottom, left
func (c *Converter) expandBoxShorthand(value CSSValue, props map[KFXSymbol]any, warnings *[]string,
	symTop, symRight, symBottom, symLeft KFXSymbol,
) {
	raw := strings.TrimSpace(value.Raw)
	parts := strings.Fields(raw)

	if len(parts) == 0 {
		return
	}

	// Parse each part as a CSS value
	parsedValues := make([]CSSValue, len(parts))
	for i, part := range parts {
		parsedValues[i] = c.parseShorthandValue(part)
	}

	// Apply values based on count
	var top, right, bottom, left CSSValue
	switch len(parsedValues) {
	case 1:
		top, right, bottom, left = parsedValues[0], parsedValues[0], parsedValues[0], parsedValues[0]
	case 2:
		top, bottom = parsedValues[0], parsedValues[0]
		right, left = parsedValues[1], parsedValues[1]
	case 3:
		top = parsedValues[0]
		right, left = parsedValues[1], parsedValues[1]
		bottom = parsedValues[2]
	case 4:
		top, right, bottom, left = parsedValues[0], parsedValues[1], parsedValues[2], parsedValues[3]
	default:
		*warnings = append(*warnings, "invalid shorthand value: "+raw)
		return
	}

	// Convert and set each value
	c.setDimensionProperty(symTop, top, props, warnings)
	c.setDimensionProperty(symRight, right, props, warnings)
	c.setDimensionProperty(symBottom, bottom, props, warnings)
	c.setDimensionProperty(symLeft, left, props, warnings)
}

// expandBorderShorthand expands CSS border shorthand to individual properties.
// CSS border format: [width] [style] [color]
// Example: "1px solid black" -> border-width: 1px, border-style: solid, border-color: black
func (c *Converter) expandBorderShorthand(value CSSValue, props map[KFXSymbol]any, _ *[]string) {
	raw := strings.TrimSpace(value.Raw)
	for part := range strings.FieldsSeq(raw) {
		part = strings.ToLower(part)

		// Check for border style keywords
		switch part {
		case "solid", "dashed", "dotted", "double", "groove", "ridge", "inset", "outset", "none", "hidden":
			if sym, ok := ConvertBorderStyle(part); ok {
				c.mergeProp(props, SymBorderStyle, sym)
			}
			continue
		}

		// Check for color (named color or hex)
		if r, g, b, ok := ParseColor(CSSValue{Raw: part, Keyword: part}); ok {
			c.mergeProp(props, SymBorderColor, MakeColorValue(r, g, b))
			continue
		}

		// Try to parse as dimension (border width)
		parsed := c.parseShorthandValue(part)
		if parsed.Value != 0 || parsed.Unit != "" {
			dim, err := MakeDimensionValue(parsed)
			if err == nil {
				c.mergeProp(props, SymBorderWeight, dim)
			}
		}
	}
}

// expandBackgroundShorthand expands CSS background shorthand.
// For KFX, we only extract the background-color component.
// CSS background shorthand can contain: color, image, position, size, repeat, attachment, origin, clip
// We only care about color values (hex, rgb, rgba, named colors).
func (c *Converter) expandBackgroundShorthand(value CSSValue, props map[KFXSymbol]any, _ *[]string) {
	raw := strings.TrimSpace(value.Raw)
	if raw == "" || raw == "none" || raw == "transparent" {
		return
	}

	// Split into parts and look for a color value
	for part := range strings.FieldsSeq(raw) {
		// Check if this part is a color
		if r, g, b, ok := ParseColor(CSSValue{Raw: part, Keyword: part}); ok {
			// Convert to KFX color format (ARGB)
			color := int64(0xFF<<24 | r<<16 | g<<8 | b)
			c.mergeProp(props, SymFillColor, color)
			return
		}
	}
}

// parseShorthandValue parses a single value from a shorthand property.
func (c *Converter) parseShorthandValue(s string) CSSValue {
	s = strings.TrimSpace(s)
	val := CSSValue{Raw: s}

	// Check for keywords
	if s == "auto" || s == "inherit" || s == "initial" {
		val.Keyword = s
		return val
	}

	// Try to parse as dimension
	if len(s) > 0 {
		numEnd := 0
		for i, ch := range s {
			if (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == '+' {
				numEnd = i + 1
			} else {
				break
			}
		}

		if numEnd > 0 {
			val.Value, _ = parseNumber(s[:numEnd])
			val.Unit = strings.ToLower(s[numEnd:])

			// Handle percentage
			if val.Unit == "%" {
				val.Unit = "%"
			}
		}
	}

	return val
}

// setDimensionProperty sets a dimension property from a CSS value.
// KP3 uses specific units for different properties (see kpv_units.go):
//   - Vertical spacing (margin-top/bottom, padding-top/bottom): lh
//   - Horizontal spacing (margin-left/right, padding-left/right): %
//   - font-size: rem
//   - text-indent: %
//   - line-height: lh
//
// Note: KFX does not support negative margins. Negative margin values are
// silently dropped with a warning logged.
func (c *Converter) setDimensionProperty(sym KFXSymbol, value CSSValue, props map[KFXSymbol]any, warnings *[]string) {
	// Handle keywords
	if value.IsKeyword() {
		switch strings.ToLower(value.Keyword) {
		case "auto":
			c.mergeProp(props, sym, SymAuto)
		case "0", "inherit", "initial":
			// Skip or use default
		}
		return
	}

	// KFX does not support negative margins - skip with warning
	if isMarginProperty(sym) && value.Value < 0 {
		*warnings = append(*warnings, fmt.Sprintf("negative margin not supported in KFX, ignoring %s: %v%s", sym, value.Value, value.Unit))
		return
	}

	// For zero values, we still need to emit them to override any defaults
	// (e.g., User-Agent stylesheet sets margin-top: 1em for <p>, and CSS may override with 0).
	// KP3 uses the appropriate unit for each property type.
	if value.Value == 0 {
		switch {
		case isVerticalSpacingProperty(sym):
			c.mergeProp(props, sym, DimensionValue(0, SymUnitLh))
		case isHorizontalSpacingProperty(sym):
			c.mergeProp(props, sym, DimensionValue(0, SymUnitPercent))
		default:
			c.mergeProp(props, sym, DimensionValue(0, SymUnitEm))
		}
		return
	}

	// Convert em to KP3-preferred units based on property type
	convertedValue := value.Value
	var convertedUnit KFXSymbol

	switch {
	case isVerticalSpacingProperty(sym):
		// Vertical spacing: convert to lh units
		switch value.Unit {
		case "em":
			convertedValue = value.Value / LineHeightRatio
			convertedUnit = SymUnitLh
		case "px":
			convertedValue = PxToLh(value.Value)
			convertedUnit = SymUnitLh
		case "pt":
			convertedValue = PtToLh(value.Value)
			convertedUnit = SymUnitLh
		default:
			var err error
			_, convertedUnit, err = CSSValueToKFX(value)
			if err != nil {
				*warnings = append(*warnings, "unable to convert vertical spacing: "+err.Error())
				return
			}
		}

	case isHorizontalSpacingProperty(sym):
		// Horizontal spacing: convert to % units
		switch value.Unit {
		case "em":
			convertedValue = value.Value * EmToPercentHorizontal
			convertedUnit = SymUnitPercent
		case "px":
			convertedValue = PxToPercent(value.Value)
			convertedUnit = SymUnitPercent
		case "pt":
			convertedValue = PtToPercent(value.Value)
			convertedUnit = SymUnitPercent
		default:
			var err error
			_, convertedUnit, err = CSSValueToKFX(value)
			if err != nil {
				*warnings = append(*warnings, "unable to convert horizontal spacing: "+err.Error())
				return
			}
		}

	case sym == SymFontSize:
		// Font-size: % -> rem with compression, em -> rem
		switch value.Unit {
		case "%":
			// Use KP3's compression formula: rem = 1 + (percent - 100) / 160
			convertedValue = PercentToRem(value.Value)
			convertedUnit = SymUnitRem
		case "em":
			convertedUnit = SymUnitRem
		default:
			var err error
			_, convertedUnit, err = CSSValueToKFX(value)
			if err != nil {
				*warnings = append(*warnings, "unable to convert font-size: "+err.Error())
				return
			}
		}

	default:
		// Default: preserve CSS units
		var err error
		_, convertedUnit, err = CSSValueToKFX(value)
		if err != nil {
			*warnings = append(*warnings, "unable to convert dimension: "+err.Error())
			return
		}
	}

	c.mergeProp(props, sym, DimensionValue(convertedValue, convertedUnit))
}

func normalizeBreakValue(val CSSValue) CSSValue {
	switch strings.ToLower(val.Keyword) {
	case "page":
		val.Keyword = "always"
	case "avoid-page":
		val.Keyword = "avoid"
	}
	return val
}

func (c *Converter) applyBreakBefore(value CSSValue, props map[KFXSymbol]any) {
	value = normalizeBreakValue(value)
	if sym, ok := ConvertPageBreak(value); ok && sym == SymAvoid {
		c.mergeProp(props, SymKeepFirst, sym)
	}
}

func (c *Converter) applyBreakAfter(value CSSValue, props map[KFXSymbol]any) {
	value = normalizeBreakValue(value)
	if sym, ok := ConvertPageBreak(value); ok && sym == SymAvoid {
		c.mergeProp(props, SymKeepLast, sym)
	}
}

func (c *Converter) applyBreakInside(value CSSValue, props map[KFXSymbol]any) {
	value = normalizeBreakValue(value)
	if sym, ok := ConvertPageBreak(value); ok && sym == SymAvoid {
		c.mergeProp(props, SymBreakInside, SymbolValue(SymAvoid))
	}
}

// convertSpecialProperty handles properties that need custom conversion logic.
// NOTE: CSS "display" is intentionally NOT converted to KFX "render" style property.
// Amazon's code (com/amazon/yjhtmlmapper/e/j.java:172) removes display from CSS styles.
// The display property is only used internally for element classification (block vs inline).
// KFX "render" property is only set on content entries (like inline images), not from CSS.
func (c *Converter) convertSpecialProperty(name string, value CSSValue, props map[KFXSymbol]any, _ *[]string) {
	switch name {
	case "text-decoration":
		dec := ConvertTextDecoration(value)
		if dec.Underline {
			c.mergeProp(props, SymUnderline, SymbolValue(SymSolid))
		}
		if dec.Strikethrough {
			c.mergeProp(props, SymStrikethrough, SymbolValue(SymSolid))
		}
	case "vertical-align":
		if vaResult, ok := ConvertVerticalAlign(value); ok {
			if vaResult.UseBaselineStyle {
				c.mergeProp(props, SymBaselineStyle, vaResult.BaselineStyle)
			} else if vaResult.UseBaselineShift {
				c.mergeProp(props, SymBaselineShift, vaResult.BaselineShift)
			}
		} else {
			c.logUnsupportedValue(name, value)
		}
	case "page-break-before":
		// In KFX, page-break-before: always is handled by section boundaries, not styles.
		// Only convert "avoid" to yj-break-before: avoid
		c.applyBreakBefore(value, props)

	case "page-break-after":
		// Only convert "avoid" - "always" is handled by section boundaries
		c.applyBreakAfter(value, props)

	case "page-break-inside":
		c.applyBreakInside(value, props)

	case "break-before":
		c.applyBreakBefore(value, props)

	case "break-after":
		c.applyBreakAfter(value, props)

	case "break-inside":
		c.applyBreakInside(value, props)

	case "text-emphasis-position", "-webkit-text-emphasis-position":
		if horiz, vert, ok := ConvertTextEmphasisPosition(value); ok {
			if horiz != 0 {
				c.mergeProp(props, SymTextEmphasisPositionHorizontal, horiz)
			}
			if vert != 0 {
				c.mergeProp(props, SymTextEmphasisPositionVertical, vert)
			}
		} else {
			c.logUnsupportedValue(name, value)
		}

	case "white-space":
		// Amazon converts white-space to a line-wrap boolean:
		// - nowrap -> white_space: nowrap
		// - other values (normal, pre, pre-wrap, pre-line) -> no KFX output (default wrapping)
		// Whitespace preservation (for pre) is handled at content generation level.
		if strings.ToLower(value.Keyword) == "nowrap" || strings.ToLower(value.Raw) == "nowrap" {
			c.mergeProp(props, SymWhiteSpace, SymbolValue(SymNowrap))
		}
	}
}

// ConvertStylesheet converts an entire CSS stylesheet to KFX style definitions.
// This includes special handling for drop caps: it detects .has-dropcap .dropcap
// patterns and extracts font-size to calculate dropcap-lines for the parent style.
func (c *Converter) ConvertStylesheet(sheet *Stylesheet) ([]StyleDef, []string) {
	styles := make([]StyleDef, 0, len(sheet.Rules))
	allWarnings := make([]string, 0)

	// Track seen style names to merge properties for same selector
	styleMap := make(map[string]*StyleDef)
	var styleOrder []string

	// First pass: detect drop cap patterns and extract font-size
	dropcapInfo := c.detectDropcapPatterns(sheet)

	for _, rule := range sheet.Rules {
		result := c.ConvertRule(rule)
		allWarnings = append(allWarnings, result.Warnings...)

		// Skip empty styles
		if len(result.Style.Properties) == 0 {
			continue
		}

		styleName := result.Style.Name

		// Apply drop cap properties if this is a has-dropcap style
		if info, ok := dropcapInfo[styleName]; ok {
			result.Style.Properties[SymDropcapChars] = info.chars
			result.Style.Properties[SymDropcapLines] = info.lines
		}

		c.tracer.TraceCSSConvert(rule.Selector.Raw, result.Style.Properties)

		if existing, ok := styleMap[styleName]; ok {
			mergeAllWithRules(existing.Properties, result.Style.Properties, mergeContextInline, c.tracer)
		} else {
			// New style
			styleCopy := result.Style
			styleMap[styleName] = &styleCopy
			styleOrder = append(styleOrder, styleName)
		}
	}

	// Build result in order
	for _, name := range styleOrder {
		styles = append(styles, *styleMap[name])
	}

	// Add stylesheet warnings
	allWarnings = append(allWarnings, sheet.Warnings...)

	return styles, allWarnings
}

// dropcapConfig holds drop cap configuration extracted from CSS.
type dropcapConfig struct {
	chars int // Number of characters (usually 1)
	lines int // Number of lines to span (derived from font-size)
}

// detectDropcapPatterns scans the stylesheet for drop cap patterns.
// It looks for selectors matching *.has-dropcap .dropcap (or similar)
// and extracts font-size to calculate dropcap-lines.
// Returns a map from parent style name (e.g., "has-dropcap") to dropcap config.
func (c *Converter) detectDropcapPatterns(sheet *Stylesheet) map[string]dropcapConfig {
	result := make(map[string]dropcapConfig)

	for _, rule := range sheet.Rules {
		// Look for descendant selectors where descendant is "dropcap"
		if rule.Selector.Ancestor == nil {
			continue
		}

		descendantName := rule.Selector.descendantBaseName()
		if descendantName != "dropcap" {
			continue
		}

		// Get the parent style name
		parentName := rule.Selector.Ancestor.StyleName()

		// Extract font-size to calculate lines
		fontSize, hasFontSize := rule.GetProperty("font-size")
		if !hasFontSize {
			// Default to 3 lines if no font-size specified
			result[parentName] = dropcapConfig{chars: 1, lines: 3}
			continue
		}

		// Calculate lines from font-size
		// Typical drop cap: font-size: 3.2em means ~3 lines
		lines := 3 // default
		if fontSize.Value > 0 {
			// Round to nearest integer
			lines = max(2, min(10, int(fontSize.Value+0.5)))
		}

		result[parentName] = dropcapConfig{chars: 1, lines: lines}

		c.log.Debug("Detected drop cap pattern",
			zap.String("parent", parentName),
			zap.Float64("font-size", fontSize.Value),
			zap.String("unit", fontSize.Unit),
			zap.Int("lines", lines))
	}

	return result
}
