package css

import "fbc/convert/kfx"

// Special return values for KFXPropertySymbol.
const (
	SymbolSpecialHandling kfx.KFXSymbol = -1 // Property requires special handling
	SymbolUnknown         kfx.KFXSymbol = -2 // Unknown/unsupported property
)

// CSSToKFXMap maps CSS property names to KFX symbol IDs.
type CSSToKFXMap map[string]kfx.KFXSymbol

// cssToKFXProperty maps CSS property names to their KFX symbol equivalents.
// A value of SymbolSpecialHandling (-1) means the property requires special handling.
var cssToKFXProperty = CSSToKFXMap{
	// Typography
	"font-size":      kfx.SymFontSize,      // $16
	"font-weight":    kfx.SymFontWeight,    // $13
	"font-style":     kfx.SymFontStyle,     // $12
	"font-family":    kfx.SymFontFamily,    // $11
	"line-height":    kfx.SymLineHeight,    // $42
	"letter-spacing": kfx.SymLetterspacing, // $32
	"color":          kfx.SymTextColor,     // $19

	// Text Layout
	"text-indent": kfx.SymTextIndent,    // $36
	"text-align":  kfx.SymTextAlignment, // $34

	// Box Model - Margins
	"margin-top":    kfx.SymMarginTop,    // $47
	"margin-bottom": kfx.SymMarginBottom, // $49
	"margin-left":   kfx.SymMarginLeft,   // $48
	"margin-right":  kfx.SymMarginRight,  // $50

	// Box Model - Padding
	"padding-top":    kfx.SymPadding, // $51 (KFX uses single padding symbol)
	"padding-bottom": kfx.SymPadding,
	"padding-left":   kfx.SymPadding,
	"padding-right":  kfx.SymPadding,

	// Spacing (alternative to margins in KFX)
	"space-before": kfx.SymSpaceBefore, // $39
	"space-after":  kfx.SymSpaceAfter,  // $40

	// Dimensions
	"width":  kfx.SymWidth,  // $56
	"height": kfx.SymHeight, // $57

	// Text Decoration - special handling
	"text-decoration": -1, // underline->$23, line-through->$27
	"vertical-align":  -1, // super/sub -> baseline_shift

	// Display/Render
	"display": -1, // block->$602, none handled specially

	// Float
	"float": kfx.SymFloat, // $140

	// Page breaks
	"page-break-before": -1, // always/avoid
	"page-break-after":  -1,
	"page-break-inside": -1,

	// Dropcap support
	"dropcap-lines": kfx.SymDropcapLines, // $125
	"dropcap-chars": kfx.SymDropcapChars, // $126

	// Shorthands that need expansion
	"margin":  -1, // expands to margin-top/right/bottom/left
	"padding": -1, // expands to padding-top/right/bottom/left
}

// KFXPropertySymbol returns the KFX symbol for a CSS property.
// Returns SymbolSpecialHandling (-1) if special handling is needed,
// or SymbolUnknown (-2) if the property is not supported.
func KFXPropertySymbol(cssProperty string) kfx.KFXSymbol {
	if sym, ok := cssToKFXProperty[cssProperty]; ok {
		return sym
	}
	return SymbolUnknown
}

// IsShorthandProperty returns true if the CSS property is a shorthand that needs expansion.
func IsShorthandProperty(cssProperty string) bool {
	switch cssProperty {
	case "margin", "padding":
		return true
	}
	return false
}

// IsSpecialProperty returns true if the property requires special value handling.
func IsSpecialProperty(cssProperty string) bool {
	switch cssProperty {
	case "text-decoration", "vertical-align", "display",
		"page-break-before", "page-break-after", "page-break-inside":
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
