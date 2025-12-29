package css

import (
	"strings"

	"go.uber.org/zap"

	"fbc/convert/kfx"
)

// Converter converts CSS rules to KFX style definitions.
type Converter struct {
	log *zap.Logger
}

// NewConverter creates a new CSS-to-KFX converter.
func NewConverter(log *zap.Logger) *Converter {
	if log == nil {
		log = zap.NewNop()
	}
	return &Converter{log: log.Named("css-converter")}
}

// ConversionResult holds the result of converting a CSS rule to KFX.
type ConversionResult struct {
	Style    kfx.StyleDef
	Warnings []string
}

// ConvertRule converts a single CSS rule to a KFX StyleDef.
func (c *Converter) ConvertRule(rule CSSRule) ConversionResult {
	result := ConversionResult{
		Style: kfx.StyleDef{
			Name:       rule.Selector.StyleName(),
			Properties: make(map[kfx.KFXSymbol]any),
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
			result.Style.Properties[kfx.SymFontWeight] = sym
		}

	case "font-style":
		if sym, ok := ConvertFontStyle(value); ok {
			result.Style.Properties[kfx.SymFontStyle] = sym
		}

	case "text-align":
		if sym, ok := ConvertTextAlign(value); ok {
			result.Style.Properties[kfx.SymTextAlignment] = sym
		}

	case "float":
		if sym, ok := ConvertFloat(value); ok {
			result.Style.Properties[kfx.SymFloat] = sym
		}

	case "color":
		if r, g, b, ok := ParseColor(value); ok {
			result.Style.Properties[kfx.SymTextColor] = MakeColorValue(r, g, b)
		} else {
			result.Warnings = append(result.Warnings, "unable to parse color: "+value.Raw)
		}

	case "font-family":
		// Font family is stored as string, actual font resolution is separate
		result.Style.Properties[kfx.SymFontFamily] = value.Raw

	default:
		// Dimension properties (font-size, margins, line-height, text-indent, etc.)
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
				result.Style.Properties[kfxSym] = kfx.SymAuto
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
			kfx.SymMarginTop, kfx.SymMarginRight, kfx.SymMarginBottom, kfx.SymMarginLeft)

	case "padding":
		// KFX has limited padding support - we'll use margins as fallback
		// For now, just log that we encountered padding
		c.log.Debug("padding shorthand not fully supported in KFX", zap.String("value", value.Raw))
	}
}

// expandBoxShorthand expands a CSS box model shorthand (margin, padding) to individual properties.
// CSS shorthand formats:
//   - 1 value: all sides
//   - 2 values: top/bottom, left/right
//   - 3 values: top, left/right, bottom
//   - 4 values: top, right, bottom, left
func (c *Converter) expandBoxShorthand(value CSSValue, result *ConversionResult,
	symTop, symRight, symBottom, symLeft kfx.KFXSymbol,
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
func (c *Converter) setDimensionProperty(sym kfx.KFXSymbol, value CSSValue, result *ConversionResult) {
	// Handle keywords
	if value.IsKeyword() {
		switch strings.ToLower(value.Keyword) {
		case "auto":
			result.Style.Properties[sym] = kfx.SymAuto
		case "0", "inherit", "initial":
			// Skip or use default
		}
		return
	}

	// Handle numeric values
	dim, err := MakeDimensionValue(value)
	if err != nil {
		result.Warnings = append(result.Warnings, "unable to convert dimension: "+err.Error())
		return
	}
	result.Style.Properties[sym] = dim
}

// convertSpecialProperty handles properties that need custom conversion logic.
func (c *Converter) convertSpecialProperty(name string, value CSSValue, result *ConversionResult) {
	switch name {
	case "text-decoration":
		dec := ConvertTextDecoration(value)
		if dec.Underline {
			result.Style.Properties[kfx.SymUnderline] = true
		}
		if dec.Strikethrough {
			result.Style.Properties[kfx.SymStrikethrough] = true
		}
		if dec.None {
			result.Style.Properties[kfx.SymUnderline] = false
			result.Style.Properties[kfx.SymStrikethrough] = false
		}

	case "vertical-align":
		if dim, ok := ConvertVerticalAlign(value); ok {
			result.Style.Properties[kfx.SymBaselineShift] = dim
		}

	case "display":
		sym, visible, ok := ConvertDisplay(value)
		if ok {
			if !visible {
				// display:none - we don't have a direct KFX equivalent
				// Log a warning but don't set anything
				c.log.Debug("display:none not directly supported in KFX")
				return
			}
			if sym != 0 {
				result.Style.Properties[kfx.SymRender] = sym
			}
		}

	case "page-break-before":
		if sym, ok := ConvertPageBreak(value); ok {
			result.Style.Properties[kfx.SymKeepFirst] = sym
		}

	case "page-break-after":
		if sym, ok := ConvertPageBreak(value); ok {
			result.Style.Properties[kfx.SymKeepLast] = sym
		}

	case "page-break-inside":
		if sym, ok := ConvertPageBreak(value); ok && sym == kfx.SymAvoid {
			// page-break-inside: avoid means keep together
			result.Style.Properties[kfx.SymKeepFirst] = kfx.SymAvoid
			result.Style.Properties[kfx.SymKeepLast] = kfx.SymAvoid
		}
	}
}

// ConvertStylesheet converts an entire CSS stylesheet to KFX style definitions.
func (c *Converter) ConvertStylesheet(sheet *Stylesheet) ([]kfx.StyleDef, []string) {
	styles := make([]kfx.StyleDef, 0, len(sheet.Rules))
	allWarnings := make([]string, 0)

	// Track seen style names to merge properties for same selector
	styleMap := make(map[string]*kfx.StyleDef)
	var styleOrder []string

	for _, rule := range sheet.Rules {
		result := c.ConvertRule(rule)
		allWarnings = append(allWarnings, result.Warnings...)

		// Skip empty styles
		if len(result.Style.Properties) == 0 {
			continue
		}

		styleName := result.Style.Name
		if existing, ok := styleMap[styleName]; ok {
			// Merge properties (later rules override)
			for k, v := range result.Style.Properties {
				existing.Properties[k] = v
			}
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
