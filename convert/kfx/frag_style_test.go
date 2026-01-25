package kfx

import (
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// isResolvedStyleName checks if a style name looks like a resolved style (base36 format like "s1J").
func isResolvedStyleName(name string) bool {
	return strings.HasPrefix(name, "s") && len(name) >= 2
}

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

	// Use StyleContext.Resolve to get a resolved style name (base36 format)
	// This triggers inheritance resolution and deduplication
	resolvedName := NewStyleContext(sr).Resolve("", "poem-subtitle")

	// Mark the resolved style as used for text (like production code does)
	sr.ResolveStyle(resolvedName, styleUsageText)

	// Build fragments - only resolved styles (base36 names) are emitted
	fragments := sr.BuildFragments()

	if len(fragments) != 1 {
		t.Fatalf("Expected 1 fragment, got %d", len(fragments))
	}

	// Verify the resolved name format
	if !isResolvedStyleName(resolvedName) {
		t.Errorf("Expected resolved style name (base36 format), got %q", resolvedName)
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
		ctx := NewStyleContext(sr)
		result := ctx.Resolve("p", "verse")
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
		ctx := NewStyleContext(sr).Push("div", "poem")

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
		ctx := NewStyleContext(sr).
			Push("div", "poem").
			Push("div", "stanza")

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
		ctx := NewStyleContext(sr).Push("div", "poem")
		styleName := ctx.Resolve("p", "verse")

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

	t.Run("scope chain margins stay on wrapper", func(t *testing.T) {
		sr := makeRegistry()
		// poem has margin-left - it should remain on the wrapper, not the child style
		ctx := NewStyleContext(sr).Push("div", "poem")
		styleName := ctx.Resolve("p", "")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Resolved style %q not found in registry", styleName)
		}

		if _, ok := def.Properties[SymMarginLeft]; ok {
			t.Error("margin-left from wrapper should not be applied to child style")
		}
	})

	t.Run("same resolution returns same style name", func(t *testing.T) {
		sr := makeRegistry()
		ctx := NewStyleContext(sr).Push("div", "poem")

		name1 := ctx.Resolve("p", "verse")
		name2 := ctx.Resolve("p", "verse")

		if name1 != name2 {
			t.Errorf("Same resolution should return same style name: %q vs %q", name1, name2)
		}
	})

	t.Run("immutability - push returns new context", func(t *testing.T) {
		sr := makeRegistry()
		ctx1 := NewStyleContext(sr).Push("div", "poem")
		ctx2 := ctx1.Push("div", "stanza")

		// ctx1 should NOT have stanza's line-height
		if _, ok := ctx1.inherited[SymLineHeight]; ok {
			t.Error("ctx1 should not have line-height after ctx2 push")
		}
		// ctx2 should have it
		if _, ok := ctx2.inherited[SymLineHeight]; !ok {
			t.Error("ctx2 should have line-height from stanza")
		}
	})

	t.Run("register uses CSS cascade override", func(t *testing.T) {
		// CSS cascade: later rules override earlier ones for the same selector.
		// When the same style is registered twice, properties should be merged
		// with later values overriding earlier ones (not accumulated).
		sr := NewStyleRegistry()
		sr.Register(StyleDef{
			Name: "p",
			Properties: map[KFXSymbol]any{
				SymMarginLeft: DimensionValue(1, SymUnitPercent),
			},
		})
		sr.Register(StyleDef{
			Name: "p",
			Properties: map[KFXSymbol]any{
				SymMarginLeft: DimensionValue(2, SymUnitPercent),
			},
		})

		def, ok := sr.Get("p")
		if !ok {
			t.Fatalf("style p not found")
		}
		// CSS cascade: second value should override first, not accumulate
		if got := def.Properties[SymMarginLeft]; got == nil {
			t.Fatalf("margin-left missing after merge")
		} else if reflect.DeepEqual(got, DimensionValue(2, SymUnitPercent)) == false {
			t.Fatalf("expected CSS cascade override margin-left 2%%, got %v", got)
		}
	})

	t.Run("PushBlock inherits margins to children", func(t *testing.T) {
		sr := makeRegistry()
		// PushBlock with poem (has margin-left) should pass it to children
		ctx := NewStyleContext(sr).PushBlock("div", "poem")

		// Margin-left SHOULD be inherited in block context
		if _, ok := ctx.inherited[SymMarginLeft]; !ok {
			t.Error("PushBlock should inherit margin-left from poem")
		}
		// Text-align should also be inherited (standard CSS inheritance)
		if _, ok := ctx.inherited[SymTextAlignment]; !ok {
			t.Error("Expected text-align to be inherited from poem")
		}
	})

	t.Run("PushBlock child style includes container margin", func(t *testing.T) {
		sr := makeRegistry()
		// PushBlock with poem (has margin-left) then resolve child paragraph
		ctx := NewStyleContext(sr).PushBlock("div", "poem")
		styleName := ctx.Resolve("p", "")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Resolved style %q not found in registry", styleName)
		}

		// margin-left from poem SHOULD be in the child style when using PushBlock
		if _, ok := def.Properties[SymMarginLeft]; !ok {
			t.Error("PushBlock child should have margin-left from poem container")
		}
	})

	t.Run("chained PushBlock accumulates margins", func(t *testing.T) {
		sr := makeRegistry()
		ctx := NewStyleContext(sr).
			PushBlock("div", "poem").
			PushBlock("div", "stanza")

		// Both poem's margin and stanza's properties should be accumulated
		if _, ok := ctx.inherited[SymMarginLeft]; !ok {
			t.Error("Expected margin-left from poem to be block-inherited")
		}
		if _, ok := ctx.inherited[SymLineHeight]; !ok {
			t.Error("Expected line-height from stanza to be inherited")
		}
	})

	t.Run("PushBlock accumulates margins from different containers", func(t *testing.T) {
		sr := NewStyleRegistry()
		// poem with margin-left: 6.25%
		sr.Register(StyleDef{Name: "poem", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(6.25, SymUnitPercent),
		}})
		// verse with margin-left: 3.125%
		sr.Register(StyleDef{Name: "verse", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(3.125, SymUnitPercent),
		}})

		// Push poem, then push verse as nested container
		ctx := NewStyleContext(sr).
			PushBlock("div", "poem").
			PushBlock("div", "verse")

		// Margins from different containers should accumulate: 6.25% + 3.125% = 9.375%
		marginLeft := ctx.inherited[SymMarginLeft]
		if marginLeft == nil {
			t.Fatal("Expected margin-left to be inherited")
		}
		val, unit, ok := measureParts(marginLeft)
		if !ok || unit != SymUnitPercent {
			t.Fatalf("Expected percent unit, got %v", marginLeft)
		}
		expected := 9.375
		if val != expected {
			t.Errorf("Expected accumulated margin-left %.3f%%, got %.3f%%", expected, val)
		}
	})

	t.Run("same container margin is not double-counted in resolveProperties", func(t *testing.T) {
		sr := NewStyleRegistry()
		// cite with margin-left: 6.25%
		sr.Register(StyleDef{Name: "cite", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(6.25, SymUnitPercent),
		}})
		// p is a simple paragraph
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(1, SymUnitRatio),
		}})

		// PushBlock with cite class, then resolve a paragraph with cite class
		// The cite margin should NOT be applied twice (container + class)
		ctx := NewStyleContext(sr).PushBlock("div", "cite")
		styleName := ctx.Resolve("p", "cite")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		marginLeft := def.Properties[SymMarginLeft]
		if marginLeft == nil {
			t.Fatal("Expected margin-left in resolved style")
		}
		val, unit, ok := measureParts(marginLeft)
		if !ok || unit != SymUnitPercent {
			t.Fatalf("Expected percent unit, got %v", marginLeft)
		}
		// Should be 6.25%, NOT 12.5% (double-counted)
		expected := 6.25
		if val != expected {
			t.Errorf("Expected margin-left %.2f%% (not double-counted), got %.2f%%", expected, val)
		}
	})

	t.Run("Push vs PushBlock margin inheritance difference", func(t *testing.T) {
		sr := makeRegistry()

		// Push does NOT inherit margins
		pushCtx := NewStyleContext(sr).Push("div", "poem")
		if _, ok := pushCtx.inherited[SymMarginLeft]; ok {
			t.Error("Push should NOT inherit margin-left")
		}

		// PushBlock DOES inherit margins
		pushBlockCtx := NewStyleContext(sr).PushBlock("div", "poem")
		if _, ok := pushBlockCtx.inherited[SymMarginLeft]; !ok {
			t.Error("PushBlock SHOULD inherit margin-left")
		}
	})

	t.Run("element tag zero margin does not override inherited container margin", func(t *testing.T) {
		// This tests the fix for the bug where p { margin-left: 0 } would
		// override the inherited container margin from poem/stanza.
		sr := NewStyleRegistry()
		// poem with margin-left: 9.375%
		sr.Register(StyleDef{Name: "poem", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(9.375, SymUnitPercent),
		}})
		// p with margin-left: 0 (explicitly)
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(0, SymUnitPercent),
		}})
		// verse with margin-left: 6.25%
		sr.Register(StyleDef{Name: "verse", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(6.25, SymUnitPercent),
		}})

		// Simulate: poem > p.verse
		poemCtx := NewStyleContext(sr).PushBlock("div", "poem")
		styleName := poemCtx.Resolve("p", "verse")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		marginLeft := def.Properties[SymMarginLeft]
		if marginLeft == nil {
			t.Fatal("Expected margin-left in resolved style")
		}
		val, unit, ok := measureParts(marginLeft)
		if !ok || unit != SymUnitPercent {
			t.Fatalf("Expected percent unit, got %v", marginLeft)
		}
		// Expected: poem 9.375% + verse 6.25% = 15.625%
		// The p's margin-left: 0 should NOT override the inherited poem margin
		expected := 15.625
		if val != expected {
			t.Errorf("Expected accumulated margin-left %.3f%%, got %.3f%%", expected, val)
		}
	})
}
