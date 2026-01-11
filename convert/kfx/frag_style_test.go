package kfx

import (
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestResolveInheritance(t *testing.T) {
	sr := NewStyleRegistry()

	// Register base style
	sr.Register(NewStyle("p").
		LineHeight(1.2, SymUnitRatio).
		TextIndent(1.5, SymUnitEm).
		TextAlign(SymJustify).
		Build())

	// Register child style that inherits from p
	sr.Register(NewStyle("subtitle").
		Inherit("p").
		FontWeight(SymBold).
		TextAlign(SymCenter). // Override parent's TextAlign
		Build())

	// Register grandchild style
	sr.Register(NewStyle("poem-subtitle").
		Inherit("subtitle").
		MarginLeft(2.0, SymUnitEm).
		Build())

	// Mark styles as used
	sr.EnsureStyle("poem-subtitle")

	// Build fragments (this triggers inheritance resolution)
	fragments := sr.BuildFragments()

	if len(fragments) != 1 {
		t.Fatalf("Expected 1 fragment, got %d", len(fragments))
	}

	// Get the resolved style
	frag := fragments[0]
	style, ok := frag.Value.(StructValue)
	if !ok {
		t.Fatal("Fragment value is not StructValue")
	}

	// Check that inherited properties are present
	if _, ok := style[SymLineHeight]; !ok {
		t.Error("LineHeight should be inherited from p")
	}

	if _, ok := style[SymTextIndent]; !ok {
		t.Error("TextIndent should be inherited from p")
	}

	// Check that overridden property from subtitle is present
	if align, ok := style[SymTextAlignment]; !ok {
		t.Error("TextAlign should be present")
	} else if align != SymbolValue(SymCenter) {
		t.Errorf("TextAlign should be Center (from subtitle), got %v", align)
	}

	// Check that FontWeight from subtitle is present
	if _, ok := style[SymFontWeight]; !ok {
		t.Error("FontWeight should be inherited from subtitle")
	}

	// Check that MarginLeft from poem-subtitle is present
	if _, ok := style[SymMarginLeft]; !ok {
		t.Error("MarginLeft should be present from poem-subtitle")
	}
}

func TestInferParentStyle(t *testing.T) {
	sr := DefaultStyleRegistry()

	// With the new architecture, DefaultStyleRegistry only has HTML element selectors.
	// Class selectors like "subtitle" come from CSS, so inferParentStyle falls back to "kfx-unknown".
	tests := []struct {
		name     string
		expected string
	}{
		{"custom-subtitle", "kfx-unknown"}, // "subtitle" not in defaults, falls back to "kfx-unknown"
		{"my-title", "kfx-unknown"},        // "title" doesn't exist as base, falls back to "kfx-unknown"
		{"unknown-style", "kfx-unknown"},
		{"section-subtitle", "kfx-unknown"}, // "subtitle" not in defaults, falls back to "kfx-unknown"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sr.inferParentStyle(tt.name)
			if got != tt.expected {
				t.Errorf("inferParentStyle(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

// TestInferParentStyleWithCSS tests inferParentStyle when CSS defines the parent style.
func TestInferParentStyleWithCSS(t *testing.T) {
	log := zap.NewNop()
	css := []byte(`
		.subtitle {
			font-weight: bold;
			text-align: center;
		}
	`)

	sr, _ := NewStyleRegistryFromCSS(css, nil, log)

	// Now "subtitle" exists from CSS, so inferParentStyle should find it
	tests := []struct {
		name     string
		expected string
	}{
		{"custom-subtitle", "subtitle"},
		{"section-subtitle", "subtitle"},
		{"my-title", "kfx-unknown"},      // "title" still doesn't exist
		{"unknown-style", "kfx-unknown"}, // no matching suffix
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sr.inferParentStyle(tt.name)
			if got != tt.expected {
				t.Errorf("inferParentStyle(%q) = %q, want %q", tt.name, got, tt.expected)
			}
		})
	}
}

func TestStyleContext(t *testing.T) {
	// Helper to create a registry with test styles
	makeRegistry := func() *StyleRegistry {
		sr := NewStyleRegistry()
		// Register styles with properties for testing inheritance
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymFontSize:     DimensionValue(1, SymUnitRatio),
			SymMarginBottom: DimensionValue(0.25, SymUnitRatio),
		}})
		sr.Register(StyleDef{Name: "poem", Properties: map[KFXSymbol]any{
			SymTextAlignment: SymbolValue(SymLeft),
			SymMarginLeft:    DimensionValue(6.25, SymUnitPercent),
		}})
		sr.Register(StyleDef{Name: "stanza", Properties: map[KFXSymbol]any{
			SymLineHeight: DimensionValue(1.4, SymUnitRatio),
		}})
		sr.Register(StyleDef{Name: "verse", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(0, SymUnitPt),
		}})
		sr.Register(StyleDef{Name: "epigraph", Properties: map[KFXSymbol]any{
			SymFontStyle:  SymbolValue(SymItalic),
			SymMarginLeft: DimensionValue(12.5, SymUnitPercent),
		}})
		sr.Register(StyleDef{Name: "cite", Properties: map[KFXSymbol]any{
			SymFontStyle:  SymbolValue(SymItalic),
			SymMarginLeft: DimensionValue(6.25, SymUnitPercent),
		}})
		return sr
	}

	t.Run("empty context with element style", func(t *testing.T) {
		sr := makeRegistry()
		ctx := NewStyleContext()
		result := ctx.Resolve("p", "verse", sr)
		// Should return a registered style name
		if result == "" {
			t.Error("Expected non-empty style name")
		}
		// Verify it starts with 's' (resolved style prefix)
		if !strings.HasPrefix(result, "s") {
			t.Errorf("Expected resolved style name starting with 's', got %q", result)
		}
	})

	t.Run("inherited properties accumulate through Push", func(t *testing.T) {
		sr := makeRegistry()
		// Push poem (has text-align:left which inherits) then resolve
		ctx := NewStyleContext().Push("div", "poem", sr)

		// Check that context accumulated inherited properties
		if _, ok := ctx.inherited[SymTextAlignment]; !ok {
			t.Error("Expected text-align to be inherited from poem")
		}
		// Margin should NOT be inherited
		if _, ok := ctx.inherited[SymMarginLeft]; ok {
			t.Error("margin-left should NOT be inherited")
		}
	})

	t.Run("chained Push accumulates inherited properties", func(t *testing.T) {
		sr := makeRegistry()
		ctx := NewStyleContext().
			Push("div", "poem", sr).
			Push("div", "stanza", sr)

		// Both poem's text-align and stanza's line-height should be accumulated
		if _, ok := ctx.inherited[SymTextAlignment]; !ok {
			t.Error("Expected text-align from poem to be inherited")
		}
		if _, ok := ctx.inherited[SymLineHeight]; !ok {
			t.Error("Expected line-height from stanza to be inherited")
		}
	})

	t.Run("Resolve merges inherited context with element properties", func(t *testing.T) {
		sr := makeRegistry()
		ctx := NewStyleContext().Push("div", "poem", sr)
		styleName := ctx.Resolve("p", "verse", sr)

		// The resolved style should exist in registry
		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Resolved style %q not found in registry", styleName)
		}

		// Check that p's margin-bottom is present (non-inherited from p)
		if _, ok := def.Properties[SymMarginBottom]; !ok {
			t.Error("Expected margin-bottom from p element style")
		}

		// Check that poem's text-align is present (inherited through context)
		if _, ok := def.Properties[SymTextAlignment]; !ok {
			t.Error("Expected text-align inherited from poem context")
		}

		// Check that verse's text-indent is present
		if _, ok := def.Properties[SymTextIndent]; !ok {
			t.Error("Expected text-indent from verse class")
		}
	})

	t.Run("scope chain properties applied to resolved style", func(t *testing.T) {
		sr := makeRegistry()
		// poem has margin-left - in KFX this should propagate to content
		// because KFX flattens nested structures
		ctx := NewStyleContext().Push("div", "poem", sr)
		styleName := ctx.Resolve("p", "", sr)

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Resolved style %q not found in registry", styleName)
		}

		// poem's margin-left SHOULD be in the resolved style for p
		// (KFX needs to flatten nested structure margins onto content)
		if _, ok := def.Properties[SymMarginLeft]; !ok {
			t.Error("margin-left from scope chain should be applied to content element in KFX")
		}
	})

	t.Run("same resolution returns same style name", func(t *testing.T) {
		sr := makeRegistry()
		ctx := NewStyleContext().Push("div", "poem", sr)

		name1 := ctx.Resolve("p", "verse", sr)
		name2 := ctx.Resolve("p", "verse", sr)

		if name1 != name2 {
			t.Errorf("Same resolution should return same style name: %q vs %q", name1, name2)
		}
	})

	t.Run("immutability - push returns new context", func(t *testing.T) {
		sr := makeRegistry()
		ctx1 := NewStyleContext().Push("div", "poem", sr)
		ctx2 := ctx1.Push("div", "stanza", sr)

		// ctx1 should NOT have stanza's line-height
		if _, ok := ctx1.inherited[SymLineHeight]; ok {
			t.Error("ctx1 should not have line-height after ctx2 push")
		}
		// ctx2 should have it
		if _, ok := ctx2.inherited[SymLineHeight]; !ok {
			t.Error("ctx2 should have line-height from stanza")
		}
	})
}
