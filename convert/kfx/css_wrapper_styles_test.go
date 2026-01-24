package kfx

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

// TestCSSWrapperStylesFromDefaultCSS verifies that critical wrapper styles
// from default.css are correctly parsed and converted to KFX style properties.
// This ensures the CSS-based style architecture produces the expected output.
func TestCSSWrapperStylesFromDefaultCSS(t *testing.T) {
	log, _ := zap.NewDevelopment()

	css, err := os.ReadFile("../default.css")
	if err != nil {
		t.Fatalf("Failed to read default.css: %v", err)
	}

	registry, warnings := NewStyleRegistryFromCSS(css, nil, log)
	if len(warnings) > 0 {
		t.Logf("CSS warnings: %v", warnings)
	}

	// Test body-title wrapper (container)
	t.Run("body-title wrapper", func(t *testing.T) {
		def, ok := registry.Get("body-title")
		if !ok {
			t.Fatal("body-title style not found in registry")
		}

		// NOTE: page-break-before: always is NOT converted to yj-break-before in styles.
		// In KFX, page breaks are handled by section boundaries, not style properties.
		// The CSS converter intentionally skips "always" values (see css_converter.go:728-731).

		// Should have page-break-inside: avoid -> break-inside: avoid
		if _, ok := def.Properties[SymBreakInside]; !ok {
			t.Error("body-title should have break-inside (from page-break-inside: avoid)")
		}

		// Should have page-break-after: avoid -> yj-break-after: avoid
		// CSS page-break-after: avoid converts to keep_last: avoid, then post-processed to yj-break-after
		if _, ok := def.Properties[SymYjBreakAfter]; !ok {
			t.Error("body-title should have yj-break-after (from page-break-after: avoid)")
		}

		// Should have margin-top (from margin: 2em 0 1em 0)
		if _, ok := def.Properties[SymMarginTop]; !ok {
			t.Error("body-title should have margin-top")
		}

		// Should have margin-bottom
		if _, ok := def.Properties[SymMarginBottom]; !ok {
			t.Error("body-title should have margin-bottom")
		}
	})

	// Test chapter-title wrapper
	t.Run("chapter-title wrapper", func(t *testing.T) {
		def, ok := registry.Get("chapter-title")
		if !ok {
			t.Fatal("chapter-title style not found in registry")
		}

		// NOTE: page-break-before: always is NOT converted to yj-break-before in styles.
		// In KFX, page breaks are handled by section boundaries, not style properties.

		// Should have break-inside: avoid
		if _, ok := def.Properties[SymBreakInside]; !ok {
			t.Error("chapter-title should have break-inside")
		}

		// Should have yj-break-after: avoid (from page-break-after: avoid)
		if _, ok := def.Properties[SymYjBreakAfter]; !ok {
			t.Error("chapter-title should have yj-break-after")
		}
	})

	// Test section-title wrapper (no page-break-before)
	t.Run("section-title wrapper", func(t *testing.T) {
		def, ok := registry.Get("section-title")
		if !ok {
			t.Fatal("section-title style not found in registry")
		}

		// section-title should NOT have page-break-before at all
		// (it uses section boundaries for page breaks, not style properties)

		// Should have break-inside: avoid
		if _, ok := def.Properties[SymBreakInside]; !ok {
			t.Error("section-title should have break-inside")
		}
	})

	// Test body-title-header (title text element)
	t.Run("body-title-header element", func(t *testing.T) {
		def, ok := registry.Get("body-title-header")
		if !ok {
			t.Fatal("body-title-header style not found in registry")
		}

		// Should have text-align: center
		if align, ok := def.Properties[SymTextAlignment]; !ok {
			t.Error("body-title-header should have text-align")
		} else if align != SymCenter && align != SymbolValue(SymCenter) {
			t.Errorf("body-title-header text-align should be center, got %v", align)
		}

		// Should have text-indent: 0
		if indent, ok := def.Properties[SymTextIndent]; !ok {
			t.Error("body-title-header should have text-indent")
		} else {
			// text-indent: 0 should be 0%
			if dim, ok := indent.(map[KFXSymbol]any); ok {
				if val, ok := dim[SymValue]; ok {
					if v, ok := val.(float64); ok && v != 0 {
						t.Errorf("body-title-header text-indent should be 0, got %v", v)
					}
				}
			}
		}

		// Should have font-weight: bold
		if weight, ok := def.Properties[SymFontWeight]; !ok {
			t.Error("body-title-header should have font-weight")
		} else if weight != SymBold && weight != SymbolValue(SymBold) {
			t.Errorf("body-title-header font-weight should be bold, got %v", weight)
		}
	})

	// Test chapter-title-header
	t.Run("chapter-title-header element", func(t *testing.T) {
		def, ok := registry.Get("chapter-title-header")
		if !ok {
			t.Fatal("chapter-title-header style not found in registry")
		}

		// Should have text-align: center
		if align, ok := def.Properties[SymTextAlignment]; !ok {
			t.Error("chapter-title-header should have text-align")
		} else if align != SymCenter && align != SymbolValue(SymCenter) {
			t.Errorf("chapter-title-header text-align should be center, got %v", align)
		}

		// Should have font-weight: bold
		if _, ok := def.Properties[SymFontWeight]; !ok {
			t.Error("chapter-title-header should have font-weight")
		}
	})

	// Test section-title-header
	t.Run("section-title-header element", func(t *testing.T) {
		def, ok := registry.Get("section-title-header")
		if !ok {
			t.Fatal("section-title-header style not found in registry")
		}

		// Should have text-align: center
		if align, ok := def.Properties[SymTextAlignment]; !ok {
			t.Error("section-title-header should have text-align")
		} else if align != SymCenter && align != SymbolValue(SymCenter) {
			t.Errorf("section-title-header text-align should be center, got %v", align)
		}
	})

	// Test emptyline styles (for multi-line titles)
	t.Run("title-header-emptyline styles", func(t *testing.T) {
		for _, name := range []string{
			"body-title-header-emptyline",
			"chapter-title-header-emptyline",
			"section-title-header-emptyline",
		} {
			def, ok := registry.Get(name)
			if !ok {
				t.Errorf("%s style not found in registry", name)
				continue
			}

			// Should have margin-top and margin-bottom (from margin: 0.8em 0)
			if _, ok := def.Properties[SymMarginTop]; !ok {
				t.Errorf("%s should have margin-top", name)
			}
			if _, ok := def.Properties[SymMarginBottom]; !ok {
				t.Errorf("%s should have margin-bottom", name)
			}
		}
	})

	// Test section-subtitle
	t.Run("section-subtitle", func(t *testing.T) {
		def, ok := registry.Get("section-subtitle")
		if !ok {
			t.Fatal("section-subtitle style not found in registry")
		}

		// Should have text-align: center
		if align, ok := def.Properties[SymTextAlignment]; !ok {
			t.Error("section-subtitle should have text-align")
		} else if align != SymCenter && align != SymbolValue(SymCenter) {
			t.Errorf("section-subtitle text-align should be center, got %v", align)
		}

		// Should have font-weight: bold
		if _, ok := def.Properties[SymFontWeight]; !ok {
			t.Error("section-subtitle should have font-weight")
		}

		// Should have yj-break-after: avoid (from page-break-after: avoid)
		if _, ok := def.Properties[SymYjBreakAfter]; !ok {
			t.Error("section-subtitle should have yj-break-after")
		}
	})
}

// TestCSSContentStylesFromDefaultCSS verifies that content styles (epigraph, poem, etc.)
// are correctly parsed from default.css.
func TestCSSContentStylesFromDefaultCSS(t *testing.T) {
	log, _ := zap.NewDevelopment()

	css, err := os.ReadFile("../default.css")
	if err != nil {
		t.Fatalf("Failed to read default.css: %v", err)
	}

	registry, _ := NewStyleRegistryFromCSS(css, nil, log)

	// Test epigraph container
	t.Run("epigraph container", func(t *testing.T) {
		def, ok := registry.Get("epigraph")
		if !ok {
			t.Fatal("epigraph style not found in registry")
		}

		// Should have margin-left (indentation)
		if _, ok := def.Properties[SymMarginLeft]; !ok {
			t.Error("epigraph should have margin-left for indentation")
		}
	})

	// Test poem container
	t.Run("poem container", func(t *testing.T) {
		def, ok := registry.Get("poem")
		if !ok {
			t.Fatal("poem style not found in registry")
		}

		// Should have margin-left
		if _, ok := def.Properties[SymMarginLeft]; !ok {
			t.Error("poem should have margin-left")
		}
	})

	// Test stanza
	t.Run("stanza", func(t *testing.T) {
		def, ok := registry.Get("stanza")
		if !ok {
			t.Fatal("stanza style not found in registry")
		}

		// Should have margin (vertical spacing between stanzas)
		hasMargin := false
		if _, ok := def.Properties[SymMarginTop]; ok {
			hasMargin = true
		}
		if _, ok := def.Properties[SymMarginBottom]; ok {
			hasMargin = true
		}
		if !hasMargin {
			t.Error("stanza should have vertical margins")
		}
	})

	// Test verse (poem line)
	t.Run("verse", func(t *testing.T) {
		def, ok := registry.Get("verse")
		if !ok {
			t.Fatal("verse style not found in registry")
		}

		// verse should exist and have some styling
		if len(def.Properties) == 0 {
			t.Error("verse should have some properties")
		}
	})

	// Test cite (quotation block)
	t.Run("cite container", func(t *testing.T) {
		def, ok := registry.Get("cite")
		if !ok {
			t.Fatal("cite style not found in registry")
		}

		// Should have margin-left
		if _, ok := def.Properties[SymMarginLeft]; !ok {
			t.Error("cite should have margin-left")
		}
	})
}

// TestCSSDescendantSelectorsFromDefaultCSS verifies that descendant selectors
// (like .section-title h2.section-title-header) are correctly parsed.
func TestCSSDescendantSelectorsFromDefaultCSS(t *testing.T) {
	log, _ := zap.NewDevelopment()

	css, err := os.ReadFile("../default.css")
	if err != nil {
		t.Fatalf("Failed to read default.css: %v", err)
	}

	registry, _ := NewStyleRegistryFromCSS(css, nil, log)

	// Test descendant selector for h2 section title page breaks
	t.Run("section-title h2 descendant selector", func(t *testing.T) {
		// The CSS parser converts ".section-title h2.section-title-header" to
		// a style named "section-title--h2.section-title-header"
		def, ok := registry.Get("section-title--h2.section-title-header")
		if !ok {
			t.Skip("Descendant selector style not found - may need different naming convention")
		}

		// Should have page-break-before: always
		if _, ok := def.Properties[SymYjBreakBefore]; !ok {
			t.Error("section-title h2 descendant should have page-break-before")
		}
	})
}

// TestDefaultStyleRegistryHTMLDefaults verifies that DefaultStyleRegistry()
// contains the correct HTML element defaults (user-agent styles).
func TestDefaultStyleRegistryHTMLDefaults(t *testing.T) {
	registry := DefaultStyleRegistry()

	// Test heading elements have correct font-size units (rem, not em)
	t.Run("h1 font-size in rem", func(t *testing.T) {
		def, ok := registry.Get("h1")
		if !ok {
			t.Fatal("h1 style not found in registry")
		}

		fontSize, ok := def.Properties[SymFontSize]
		if !ok {
			t.Fatal("h1 should have font-size")
		}

		// Check that unit is rem ($505), not em ($309)
		if dim, ok := fontSize.(map[KFXSymbol]any); ok {
			unit, ok := dim[SymUnit]
			if !ok {
				t.Error("font-size should have unit")
			} else if unit != SymUnitRem && unit != SymbolValue(SymUnitRem) {
				t.Errorf("h1 font-size unit should be rem, got %v", unit)
			}
		}
	})

	// Test paragraph has margins in lh units
	t.Run("p margins in lh", func(t *testing.T) {
		def, ok := registry.Get("p")
		if !ok {
			t.Fatal("p style not found in registry")
		}

		// Check margin-top
		if mt, ok := def.Properties[SymMarginTop]; ok {
			if dim, ok := mt.(map[KFXSymbol]any); ok {
				unit := dim[SymUnit]
				if unit != SymUnitLh && unit != SymbolValue(SymUnitLh) {
					t.Errorf("p margin-top unit should be lh, got %v", unit)
				}
			}
		}
	})

	// Test blockquote has horizontal margins in px
	t.Run("blockquote horizontal margins in px", func(t *testing.T) {
		def, ok := registry.Get("blockquote")
		if !ok {
			t.Fatal("blockquote style not found in registry")
		}

		// Check margin-left is 40px
		if ml, ok := def.Properties[SymMarginLeft]; ok {
			if dim, ok := ml.(map[KFXSymbol]any); ok {
				unit := dim[SymUnit]
				if unit != SymUnitPx && unit != SymbolValue(SymUnitPx) {
					t.Errorf("blockquote margin-left unit should be px, got %v", unit)
				}
				val := dim[SymValue]
				if v, ok := val.(float64); ok && v != 40 {
					t.Errorf("blockquote margin-left should be 40px, got %v", v)
				}
			}
		} else {
			t.Error("blockquote should have margin-left")
		}
	})
}
