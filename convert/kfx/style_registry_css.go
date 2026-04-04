package kfx

import (
	"fmt"
	"reflect"

	"go.uber.org/zap"

	"fbc/css"
)

// RegisterFromCSS adds styles from a parsed CSS stylesheet.
// Later rules override earlier ones for the same style name.
func (sr *StyleRegistry) RegisterFromCSS(styles []StyleDef) {
	for _, def := range styles {
		sr.Register(def)
	}
}

// NewStyleRegistryFromCSS creates a style registry from a pre-parsed CSS stylesheet.
// It starts with default HTML element styles, overlays styles from CSS,
// then applies KFX-specific post-processing for Kindle compatibility.
// Returns the registry and any warnings from CSS conversion.
func NewStyleRegistryFromCSS(sheet *css.Stylesheet, tracer *StyleTracer, log *zap.Logger) (*StyleRegistry, []string) {
	// Start with HTML element defaults only
	sr := DefaultStyleRegistry()
	sr.SetTracer(tracer)

	mapper := NewStyleMapper(log, tracer)
	warnings := make([]string, 0)

	if sheet != nil {
		rules := flattenStylesheetForKFX(sheet)
		if len(rules) > 0 {
			// Extract pseudo-element content BEFORE conversion (content property is not converted to KFX)
			pseudoWarnings := sr.extractPseudoContent(sheet)
			warnings = append(warnings, pseudoWarnings...)

			// Convert to KFX styles (includes drop cap detection)
			styles, cssWarnings := mapper.MapStylesheet(sheet)
			warnings = append(warnings, cssWarnings...)

			// Register CSS styles (overriding defaults where applicable)
			sr.RegisterFromCSS(styles)

			log.Debug("CSS styles loaded",
				zap.Int("rules", len(rules)),
				zap.Int("styles", len(styles)),
				zap.Int("warnings", len(cssWarnings)),
				zap.Int("pseudo_content", len(sr.pseudoContent)))
		}
	}

	// Apply KFX-specific style adjustments before post-processing.
	// This fixes discrepancies between CSS/EPUB behavior and KFX behavior,
	// such as footnote title margins where the -first/-next variants should
	// inherit from the base footnote-title style rather than override it.
	sr.applyKFXStyleAdjustments()

	// Apply KFX-specific post-processing (layout-hints, yj-break, etc.)
	sr.PostProcessForKFX()

	return sr, warnings
}

// parseAndCreateRegistry is a test helper that parses CSS bytes and creates a registry.
func parseAndCreateRegistry(cssData []byte, tracer *StyleTracer, log *zap.Logger) (*StyleRegistry, []string) {
	if len(cssData) == 0 {
		return NewStyleRegistryFromCSS(nil, tracer, log)
	}
	parser := css.NewParser(log)
	sheet := parser.Parse(cssData)
	return NewStyleRegistryFromCSS(sheet, tracer, log)
}

// applyKFXStyleAdjustments modifies CSS styles for KFX-specific requirements.
// This is called after CSS is loaded but before KFX post-processing.
//
// This function modifies existing styles to fix discrepancies between CSS/EPUB
// behavior and KFX behavior.
func (sr *StyleRegistry) applyKFXStyleAdjustments() {
	// Remove vertical margins from footnote-title-first and footnote-title-next.
	//
	// In EPUB, footnote titles use a div wrapper with class "footnote-title" that has
	// margin: 1em 0 0.5em 0, and the p elements inside have class "footnote-title-first"
	// or "footnote-title-next" with margin: 0.2em 0 (for internal spacing).
	//
	// In KFX, there's no div/p hierarchy - styles are applied directly to paragraphs.
	// When both classes are applied ("footnote-title footnote-title-first"), the more
	// specific -first/-next margins (0.2em) override the base margins (1em/0.5em).
	//
	// The reference KP3 output shows footnote title entries with mt=0.833333lh (1em/1.2)
	// and mb=0.416667lh (0.5em/1.2), matching the container margins, not the paragraph margins.
	//
	// By removing vertical margins from -first/-next, they inherit from footnote-title,
	// producing correct output that matches KP3.
	for _, styleName := range []string{"footnote-title-first", "footnote-title-next"} {
		if def, exists := sr.styles[styleName]; exists {
			// Remove vertical margin properties so they inherit from footnote-title
			delete(def.Properties, SymMarginTop)
			delete(def.Properties, SymMarginBottom)
			sr.styles[styleName] = def
		}
	}

	// Inline code styling should not override paragraph alignment.
	// KP3 does not apply code { text-align: left; } to the paragraph when the entire
	// paragraph is a single <code> span and the code style is promoted to block.
	if def, exists := sr.styles["code"]; exists {
		delete(def.Properties, SymTextAlignment)
		sr.styles["code"] = def
	}

	// Footnote titles should not behave like headings in KFX.
	// KP3 reference output keeps footnote title paragraphs justified (inheriting from base paragraph)
	// even though our CSS sets .footnote-title { text-align: left; } for EPUB.
	// Strip alignment so footnote title paragraphs inherit the surrounding paragraph alignment.
	if def, exists := sr.styles["footnote-title"]; exists {
		delete(def.Properties, SymTextAlignment)
		sr.styles["footnote-title"] = def
	}

	// Propagate user CSS changes to heading-context descendant styles.
	//
	// Styles with DescendantReplacement (sub, sup, small) have heading-context
	// descendants (h1--sub, h2--sub, etc.) that completely replace the base style
	// when the element appears inside a heading. These descendants are registered
	// with hardcoded defaults in DefaultStyleRegistry().
	//
	// When user CSS modifies the base style (e.g., sub { vertical-align: baseline; }),
	// the heading-context descendants must be updated to reflect those changes.
	// Otherwise, user CSS is silently ignored in heading contexts.
	sr.propagateToHeadingDescendants()
}

// descendantReplacementDefaults defines the known default properties for each
// DescendantReplacement style. These must match what DefaultStyleRegistry() registers.
// If the base style differs from these defaults after CSS loading, user CSS modified it.
var descendantReplacementDefaults = map[string]map[KFXSymbol]any{
	"sub": {
		SymBaselineStyle: SymbolValue(SymSubscript),
		SymFontSize:      DimensionValue(0.75, SymUnitRem),
	},
	"sup": {
		SymBaselineStyle: SymbolValue(SymSuperscript),
		SymFontSize:      DimensionValue(0.75, SymUnitRem),
	},
	"small": {
		SymFontSize: DimensionValue(0.8333333333333334, SymUnitEm),
	},
}

// propagateToHeadingDescendants updates heading-context descendant styles (h1--sub, etc.)
// when user CSS has modified the base DescendantReplacement style (sub, sup, small).
//
// The heading-context descendants exist to prevent base sub/sup/small absolute font-size
// from overriding inherited heading font-size. When user CSS modifies these styles,
// the heading-context descendants must reflect the user's changes, otherwise the
// user's CSS is silently ignored in heading contexts.
//
// Only properties that exist in the defaults AND were changed by user CSS are propagated:
//   - baseline_style: copied directly if changed
//   - font_size: converted from absolute rem to relative em if changed, so it scales
//     with the heading font-size rather than being an absolute value
//
// Properties NOT in the defaults (line_height, baseline_shift, etc.) are intentionally
// not propagated — they are inherited from the heading context or computed by the resolver.
func (sr *StyleRegistry) propagateToHeadingDescendants() {
	for baseName, defaultProps := range descendantReplacementDefaults {
		baseDef, exists := sr.styles[baseName]
		if !exists {
			continue
		}

		// Check if user CSS modified this style by comparing against defaults.
		if stylePropsMatchDefaults(baseDef.Properties, defaultProps) {
			continue
		}

		// Determine which default properties were changed by user CSS.
		changedProps := make(map[KFXSymbol]any)
		for sym, defaultVal := range defaultProps {
			baseVal, hasIt := baseDef.Properties[sym]
			if !hasIt || !reflect.DeepEqual(baseVal, defaultVal) {
				if hasIt {
					changedProps[sym] = baseVal
				}
				// If !hasIt, the property was removed — we don't propagate absence,
				// the heading-context descendant already has its own default.
			}
		}

		if len(changedProps) == 0 {
			continue
		}

		// Update all heading-context descendants with only the changed properties.
		for i := 1; i <= 6; i++ {
			descName := fmt.Sprintf("h%d--%s", i, baseName)
			descDef, ok := sr.styles[descName]
			if !ok {
				continue
			}

			for sym, val := range changedProps {
				if sym == SymFontSize {
					// Convert font-size from absolute rem to relative em.
					// In heading context, em is relative to the heading's font-size,
					// so 0.25rem (absolute) becomes 0.25em (relative to heading).
					descDef.Properties[sym] = convertFontSizeToEm(val)
				} else {
					descDef.Properties[sym] = val
				}
			}
			sr.styles[descName] = descDef
		}
	}
}

// convertFontSizeToEm converts a font-size dimension value from rem to em.
// If the value is already in em or is not a dimension, it is returned unchanged.
// This is needed for heading-context descendants where font-size should be relative
// to the heading rather than absolute.
func convertFontSizeToEm(val any) any {
	value, unit, ok := measureParts(val)
	if !ok {
		return val
	}
	if unit == SymUnitRem {
		return DimensionValue(value, SymUnitEm)
	}
	return val
}

// stylePropsMatchDefaults returns true if two property maps have the same keys and values.
func stylePropsMatchDefaults(a, b map[KFXSymbol]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(va, vb) {
			return false
		}
	}
	return true
}
