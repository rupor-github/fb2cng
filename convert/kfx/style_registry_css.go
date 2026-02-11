package kfx

import (
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

	// Register programmatic descendant selectors.
	// These implement CSS descendant selector semantics (e.g., ".footnote p")
	// that override element defaults for elements inside specific containers.
	// This is needed because CSS class rules like ".footnote { text-indent: 0; }"
	// should not directly apply to child elements - only descendant selectors do.
	sr.registerDescendantSelectors()

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

// registerDescendantSelectors adds programmatic descendant selectors.
// These implement CSS descendant selector semantics (e.g., ".footnote p")
// that are not expressible in the CSS file but needed for correct KFX output.
//
// In CSS, a rule like ".footnote { text-indent: 0; }" applies to the element
// with class="footnote", not to its children. To affect child paragraphs,
// you need a descendant selector like ".footnote p { text-indent: 0; }".
//
// The style_context.go resolveProperties() function looks up selectors using:
// - "ancestor--descendant" for descendant selectors (CSS: ".ancestor descendant")
// - "parent>child" for direct child selectors (CSS: ".parent > child")
func (sr *StyleRegistry) registerDescendantSelectors() {
	// .footnote > p { text-indent: 0; }
	// Direct child paragraphs of footnote should have no text-indent,
	// overriding p { text-indent: 1em; }. Using direct child selector (>)
	// ensures nested elements like cite inside footnote keep their default indent.
	sr.Register(NewStyle("footnote>p").
		TextIndent(0, SymUnitPercent).
		Build())
}

// applyKFXStyleAdjustments modifies CSS styles for KFX-specific requirements.
// This is called after CSS is loaded but before KFX post-processing.
//
// Unlike registerDescendantSelectors which adds new selector rules, this function
// modifies existing styles to fix discrepancies between CSS/EPUB behavior and KFX behavior.
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
}
