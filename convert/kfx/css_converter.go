package kfx

import (
	"maps"
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

// ConvertRule converts a single CSS rule to a KFX StyleDef.
func (c *Converter) ConvertRule(rule CSSRule) ConversionResult {
	result := ConversionResult{
		Style: StyleDef{
			Name:       rule.Selector.StyleName(),
			Properties: make(map[KFXSymbol]any),
		},
		Warnings: make([]string, 0),
	}

	// Process each CSS property
	for propName, propValue := range rule.Properties {
		c.convertProperty(propName, propValue, &result)
	}

	return result
}

// convertProperty converts a single CSS property to KFX properties.
func (c *Converter) convertProperty(name string, value CSSValue, result *ConversionResult) {
	// Handle shorthand properties first
	if IsShorthandProperty(name) {
		c.expandShorthand(name, value, result)
		return
	}

	// Handle special properties
	if IsSpecialProperty(name) {
		c.convertSpecialProperty(name, value, result)
		return
	}

	// Look up the KFX symbol
	kfxSym := KFXPropertySymbol(name)
	if kfxSym == SymbolUnknown {
		// Unknown property - log at debug level
		c.log.Debug("unknown CSS property", zap.String("property", name))
		return
	}

	// Convert based on property type
	switch name {
	case "font-weight":
		if sym, ok := ConvertFontWeight(value); ok {
			result.Style.Properties[SymFontWeight] = sym
		}

	case "font-style":
		if sym, ok := ConvertFontStyle(value); ok {
			result.Style.Properties[SymFontStyle] = sym
		}

	case "text-align":
		if sym, ok := ConvertTextAlign(value); ok {
			result.Style.Properties[SymTextAlignment] = sym
		}

	case "float":
		if sym, ok := ConvertFloat(value); ok {
			result.Style.Properties[SymFloat] = sym
		}

	case "color":
		if r, g, b, ok := ParseColor(value); ok {
			result.Style.Properties[SymTextColor] = MakeColorValue(r, g, b)
		} else {
			result.Warnings = append(result.Warnings, "unable to parse color: "+value.Raw)
		}

	case "font-family":
		// Font family is stored as string, actual font resolution is separate
		result.Style.Properties[SymFontFamily] = value.Raw

	case "font-size":
		// KPV converts percentage font-sizes to rem (140% -> 1.4rem)
		// This is important for title rendering - percent units cause alignment issues
		if value.IsNumeric() {
			if value.Unit == "%" {
				// Convert percentage to rem: 140% -> 1.4rem
				remValue := value.Value / KPVPercentToRem
				result.Style.Properties[kfxSym] = DimensionValue(remValue, SymUnitRem)
			} else {
				dim, err := MakeDimensionValue(value)
				if err != nil {
					result.Warnings = append(result.Warnings, "unable to convert "+name+": "+err.Error())
					return
				}
				result.Style.Properties[kfxSym] = dim
			}
		}

	case "text-indent":
		// KPV uses % for text-indent. Convert em → % using KPVEmToPercentTextIndent ratio.
		// Ignore zero values.
		if value.IsNumeric() && value.Value != 0 {
			if value.Unit == "" || value.Unit == "%" {
				result.Style.Properties[kfxSym] = DimensionValue(value.Value, SymUnitPercent)
				return
			}
			if value.Unit == "em" {
				result.Style.Properties[kfxSym] = DimensionValue(value.Value*KPVEmToPercentTextIndent, SymUnitPercent)
				return
			}
			dim, err := MakeDimensionValue(value)
			if err != nil {
				result.Warnings = append(result.Warnings, "unable to convert "+name+": "+err.Error())
				return
			}
			result.Style.Properties[kfxSym] = dim
		}

	case "line-height":
		// KPV uses lh units for line-height. Convert em → lh using KPVLineHeightRatio.
		if value.IsNumeric() {
			if value.Unit == "" || value.Unit == "lh" {
				// Unitless or already lh - use lh unit
				result.Style.Properties[kfxSym] = DimensionValue(value.Value, SymUnitLh)
				return
			}
			if value.Unit == "em" {
				// Convert em to lh
				result.Style.Properties[kfxSym] = DimensionValue(value.Value/KPVLineHeightRatio, SymUnitLh)
				return
			}
			dim, err := MakeDimensionValue(value)
			if err != nil {
				result.Warnings = append(result.Warnings, "unable to convert "+name+": "+err.Error())
				return
			}
			result.Style.Properties[kfxSym] = dim
		}

	case "margin-top", "margin-bottom", "margin-left", "margin-right",
		"padding-top", "padding-bottom", "padding-left", "padding-right":
		// Route through setDimensionProperty for proper KPV unit conversion
		c.setDimensionProperty(kfxSym, value, result)

	default:
		// Dimension properties (font-size, margins, line-height, etc.)
		if value.IsNumeric() || value.Unit != "" || (value.Value != 0) {
			dim, err := MakeDimensionValue(value)
			if err != nil {
				result.Warnings = append(result.Warnings, "unable to convert "+name+": "+err.Error())
				return
			}
			result.Style.Properties[kfxSym] = dim
		} else if value.IsKeyword() {
			// Some dimension properties accept keywords like "auto"
			switch strings.ToLower(value.Keyword) {
			case "auto":
				result.Style.Properties[kfxSym] = SymAuto
			case "inherit":
				// Skip inherit - KFX handles inheritance differently
			default:
				c.log.Debug("unhandled keyword value",
					zap.String("property", name),
					zap.String("value", value.Keyword))
			}
		}
	}
}

// expandShorthand expands CSS shorthand properties into individual properties.
func (c *Converter) expandShorthand(name string, value CSSValue, result *ConversionResult) {
	switch name {
	case "margin":
		c.expandBoxShorthand(value, result,
			SymMarginTop, SymMarginRight, SymMarginBottom, SymMarginLeft)

	case "padding":
		c.expandBoxShorthand(value, result,
			SymPaddingTop, SymPaddingRight, SymPaddingBottom, SymPaddingLeft)

	case "border":
		c.expandBorderShorthand(value, result)
	}
}

// expandBoxShorthand expands a CSS box model shorthand (margin, padding) to individual properties.
// CSS shorthand formats:
//   - 1 value: all sides
//   - 2 values: top/bottom, left/right
//   - 3 values: top, left/right, bottom
//   - 4 values: top, right, bottom, left
func (c *Converter) expandBoxShorthand(value CSSValue, result *ConversionResult,
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
		result.Warnings = append(result.Warnings, "invalid shorthand value: "+raw)
		return
	}

	// Convert and set each value
	c.setDimensionProperty(symTop, top, result)
	c.setDimensionProperty(symRight, right, result)
	c.setDimensionProperty(symBottom, bottom, result)
	c.setDimensionProperty(symLeft, left, result)
}

// expandBorderShorthand expands CSS border shorthand to individual properties.
// CSS border format: [width] [style] [color]
// Example: "1px solid black" -> border-width: 1px, border-style: solid, border-color: black
func (c *Converter) expandBorderShorthand(value CSSValue, result *ConversionResult) {
	raw := strings.TrimSpace(value.Raw)
	for part := range strings.FieldsSeq(raw) {
		part = strings.ToLower(part)

		// Check for border style keywords
		switch part {
		case "solid", "dashed", "dotted", "double", "groove", "ridge", "inset", "outset", "none", "hidden":
			if sym, ok := ConvertBorderStyle(part); ok {
				result.Style.Properties[SymBorderStyle] = sym
			}
			continue
		}

		// Check for color (named color or hex)
		if r, g, b, ok := ParseColor(CSSValue{Raw: part, Keyword: part}); ok {
			result.Style.Properties[SymBorderColor] = MakeColorValue(r, g, b)
			continue
		}

		// Try to parse as dimension (border width)
		parsed := c.parseShorthandValue(part)
		if parsed.Value != 0 || parsed.Unit != "" {
			dim, err := MakeDimensionValue(parsed)
			if err == nil {
				result.Style.Properties[SymBorderWeight] = dim
			}
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
// KPV uses specific units for different properties (see kpv_units.go):
//   - Vertical spacing (margin-top/bottom, padding-top/bottom): lh
//   - Horizontal spacing (margin-left/right, padding-left/right): %
//   - font-size: rem
//   - text-indent: %
//   - line-height: lh
func (c *Converter) setDimensionProperty(sym KFXSymbol, value CSSValue, result *ConversionResult) {
	// Handle keywords
	if value.IsKeyword() {
		switch strings.ToLower(value.Keyword) {
		case "auto":
			result.Style.Properties[sym] = SymAuto
		case "0", "inherit", "initial":
			// Skip or use default
		}
		return
	}

	// Skip zero values - KPV doesn't include them
	if value.Value == 0 {
		return
	}

	// Convert em to KPV-preferred units based on property type
	convertedValue := value.Value
	var convertedUnit KFXSymbol

	switch {
	case isVerticalSpacingProperty(sym):
		// Vertical spacing: em -> lh using KPVLineHeightRatio
		if value.Unit == "em" {
			convertedValue = value.Value / KPVLineHeightRatio
			convertedUnit = SymUnitLh
		} else {
			var err error
			_, convertedUnit, err = CSSValueToKFX(value)
			if err != nil {
				result.Warnings = append(result.Warnings, "unable to convert vertical spacing: "+err.Error())
				return
			}
		}

	case isHorizontalSpacingProperty(sym):
		// Horizontal spacing: em -> % using KPVEmToPercentHorizontal
		if value.Unit == "em" {
			convertedValue = value.Value * KPVEmToPercentHorizontal
			convertedUnit = SymUnitPercent
		} else {
			var err error
			_, convertedUnit, err = CSSValueToKFX(value)
			if err != nil {
				result.Warnings = append(result.Warnings, "unable to convert horizontal spacing: "+err.Error())
				return
			}
		}

	case sym == SymFontSize:
		// Font-size: % -> rem, em -> rem
		switch value.Unit {
		case "%":
			convertedValue = value.Value / KPVPercentToRem
			fallthrough
		case "em":
			convertedUnit = SymUnitRem
		default:
			var err error
			_, convertedUnit, err = CSSValueToKFX(value)
			if err != nil {
				result.Warnings = append(result.Warnings, "unable to convert font-size: "+err.Error())
				return
			}
		}

	default:
		// Default: preserve CSS units
		var err error
		_, convertedUnit, err = CSSValueToKFX(value)
		if err != nil {
			result.Warnings = append(result.Warnings, "unable to convert dimension: "+err.Error())
			return
		}
	}

	result.Style.Properties[sym] = DimensionValue(convertedValue, convertedUnit)
}

// convertSpecialProperty handles properties that need custom conversion logic.
func (c *Converter) convertSpecialProperty(name string, value CSSValue, result *ConversionResult) {
	switch name {
	case "text-decoration":
		dec := ConvertTextDecoration(value)
		if dec.Underline {
			result.Style.Properties[SymUnderline] = true
		}
		if dec.Strikethrough {
			result.Style.Properties[SymStrikethrough] = true
		}
		// NOTE: text-decoration: none - we intentionally don't set false values.
		// KFX defaults to no decoration, and explicitly setting false can cause
		// issues with some Kindle renderers (e.g., footnotes appearing as strikethrough).
		// If you need to override inherited decoration, set the appropriate true value instead.
		// The original code that set false values:
		// if dec.None {
		// 	result.Style.Properties[SymUnderline] = false
		// 	result.Style.Properties[SymStrikethrough] = false
		// }

	case "vertical-align":
		if vaResult, ok := ConvertVerticalAlign(value); ok {
			if vaResult.UseBaselineStyle {
				result.Style.Properties[SymBaselineStyle] = vaResult.BaselineStyle
			} else if vaResult.UseBaselineShift {
				result.Style.Properties[SymBaselineShift] = vaResult.BaselineShift
			}
		}

	case "display":
		// NOTE: KPV doesn't convert display to render - disabled for now
		// sym, visible, ok := ConvertDisplay(value)
		// if ok {
		// 	if !visible {
		// 		// display:none - we don't have a direct KFX equivalent
		// 		// Log a warning but don't set anything
		// 		c.log.Debug("display:none not directly supported in KFX")
		// 		return
		// 	}
		// 	if sym != 0 {
		// 		result.Style.Properties[SymRender] = sym
		// 	}
		// }
		_ = value // suppress unused warning

	case "page-break-before":
		// In KFX, page-break-before: always is handled by section boundaries, not styles.
		// Only convert "avoid" to yj-break-before: avoid
		if sym, ok := ConvertPageBreak(value); ok && sym == SymAvoid {
			result.Style.Properties[SymKeepFirst] = sym
		}

	case "page-break-after":
		// Only convert "avoid" - "always" is handled by section boundaries
		if sym, ok := ConvertPageBreak(value); ok && sym == SymAvoid {
			result.Style.Properties[SymKeepLast] = sym
		}

	case "page-break-inside":
		if sym, ok := ConvertPageBreak(value); ok && sym == SymAvoid {
			// page-break-inside: avoid means the element should not break internally
			result.Style.Properties[SymBreakInside] = SymbolValue(SymAvoid)
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
			// Merge properties (later rules override)
			maps.Copy(existing.Properties, result.Style.Properties)
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

		c.log.Debug("detected drop cap pattern",
			zap.String("parent", parentName),
			zap.Float64("font-size", fontSize.Value),
			zap.String("unit", fontSize.Unit),
			zap.Int("lines", lines))
	}

	return result
}
