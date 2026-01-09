package kfx

// Special return values for KFXPropertySymbol.
const (
	SymbolSpecialHandling KFXSymbol = -1 // Property requires special handling
	SymbolUnknown         KFXSymbol = -2 // Unknown/unsupported property
)

// CSSToKFXMap maps CSS property names to KFX symbol IDs.
type CSSToKFXMap map[string]KFXSymbol

// cssToKFXProperty maps CSS property names to their KFX symbol equivalents.
// A value of SymbolSpecialHandling (-1) means the property requires special handling.
var cssToKFXProperty = CSSToKFXMap{
	// Typography
	"font-size":        SymFontSize,      // $16
	"font-weight":      SymFontWeight,    // $13
	"font-style":       SymFontStyle,     // $12
	"font-family":      SymFontFamily,    // $11
	"line-height":      SymLineHeight,    // $42
	"letter-spacing":   SymLetterspacing, // $32
	"color":            SymTextColor,     // $19
	"background-color": SymFillColor,     // $70

	// Text Layout
	"text-indent": SymTextIndent,    // $36
	"text-align":  SymTextAlignment, // $34

	// Box Model - Margins
	"margin-top":    SymMarginTop,    // $47
	"margin-bottom": SymMarginBottom, // $49
	"margin-left":   SymMarginLeft,   // $48
	"margin-right":  SymMarginRight,  // $50

	// Box Model - Padding
	"padding-top":    SymPaddingTop,    // $52
	"padding-left":   SymPaddingLeft,   // $53
	"padding-bottom": SymPaddingBottom, // $54
	"padding-right":  SymPaddingRight,  // $55

	// Spacing (alternative to margins in KFX)
	"space-before": SymSpaceBefore, // $39
	"space-after":  SymSpaceAfter,  // $40

	// Dimensions
	"width":  SymWidth,  // $56
	"height": SymHeight, // $57

	// Borders
	"border-style": SymBorderStyle,  // $88
	"border-width": SymBorderWeight, // $93
	"border-color": SymBorderColor,  // $83

	// Text Decoration - special handling
	"text-decoration": -1, // underline->$23, line-through->$27
	"vertical-align":  -1, // super/sub -> baseline_shift

	// Display/Render
	"display": -1, // block->$602, none handled specially

	// Float
	"float": SymFloat, // $140

	// Page breaks
	"page-break-before": -1, // always/avoid
	"page-break-after":  -1,
	"page-break-inside": -1,
	"break-before":      -1,
	"break-after":       -1,
	"break-inside":      -1,

	// Dropcap support
	"dropcap-lines": SymDropcapLines, // $125
	"dropcap-chars": SymDropcapChars, // $126

	// Shorthands that need expansion
	"margin":  -1, // expands to margin-top/right/bottom/left
	"padding": -1, // expands to padding-top/right/bottom/left
	"border":  -1, // expands to border-width/style/color
}

// KFXPropertySymbol returns the KFX symbol for a CSS property.
// Returns SymbolSpecialHandling (-1) if special handling is needed,
// or SymbolUnknown (-2) if the property is not supported.
func KFXPropertySymbol(cssProperty string) KFXSymbol {
	if sym, ok := cssToKFXProperty[cssProperty]; ok {
		return sym
	}
	return SymbolUnknown
}

// IsShorthandProperty returns true if the CSS property is a shorthand that needs expansion.
func IsShorthandProperty(cssProperty string) bool {
	switch cssProperty {
	case "margin", "padding", "border":
		return true
	}
	return false
}

// IsSpecialProperty returns true if the property requires special value handling.
func IsSpecialProperty(cssProperty string) bool {
	switch cssProperty {
	case "text-decoration", "vertical-align", "display",
		"page-break-before", "page-break-after", "page-break-inside",
		"break-before", "break-after", "break-inside":
		return true
	}
	return false
}

// SupportedProperties returns the list of CSS properties we can convert.
func SupportedProperties() []string {
	props := make([]string, 0, len(cssToKFXProperty))
	for name := range cssToKFXProperty {
		props = append(props, name)
	}
	return props
}
