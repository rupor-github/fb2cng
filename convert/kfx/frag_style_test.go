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
	// Class selectors like "subtitle" come from CSS, so inferParentStyle returns ""
	// (no parent - line-height is added in BuildFragments for text usage).
	tests := []struct {
		name     string
		expected string
	}{
		{"custom-subtitle", ""}, // "subtitle" not in defaults, no parent
		{"my-title", ""},        // "title" doesn't exist as base, no parent
		{"unknown-style", ""},
		{"section-subtitle", ""}, // "subtitle" not in defaults, no parent
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

	sr, _ := parseAndCreateRegistry(css, nil, log)

	// Now "subtitle" exists from CSS, so inferParentStyle should find it
	// Unknown styles have no parent (line-height added in BuildFragments)
	tests := []struct {
		name     string
		expected string
	}{
		{"custom-subtitle", "subtitle"},
		{"section-subtitle", "subtitle"},
		{"my-title", ""},      // "title" still doesn't exist, no parent
		{"unknown-style", ""}, // no matching suffix, no parent
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
			SymMarginLeft:    DimensionValue(2, SymUnitEm),
		}})
		sr.Register(StyleDef{Name: "stanza", Properties: map[KFXSymbol]any{
			SymLineHeight: DimensionValue(1.4, SymUnitRatio),
		}})
		sr.Register(StyleDef{Name: "verse", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(0, SymUnitPt),
		}})
		sr.Register(StyleDef{Name: "epigraph", Properties: map[KFXSymbol]any{
			SymFontStyle:  SymbolValue(SymItalic),
			SymMarginLeft: DimensionValue(4, SymUnitEm),
		}})
		sr.Register(StyleDef{Name: "cite", Properties: map[KFXSymbol]any{
			SymFontStyle:  SymbolValue(SymItalic),
			SymMarginLeft: DimensionValue(2, SymUnitEm),
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
				SymMarginLeft: DimensionValue(1, SymUnitEm),
			},
		})
		sr.Register(StyleDef{
			Name: "p",
			Properties: map[KFXSymbol]any{
				SymMarginLeft: DimensionValue(2, SymUnitEm),
			},
		})

		def, ok := sr.Get("p")
		if !ok {
			t.Fatalf("style p not found")
		}
		// CSS cascade: second value should override first, not accumulate
		if got := def.Properties[SymMarginLeft]; got == nil {
			t.Fatalf("margin-left missing after merge")
		} else if reflect.DeepEqual(got, DimensionValue(2, SymUnitEm)) == false {
			t.Fatalf("expected CSS cascade override margin-left 2em, got %v", got)
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
		// poem with margin-left: 2em
		sr.Register(StyleDef{Name: "poem", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(2, SymUnitEm),
		}})
		// verse with margin-left: 1em
		sr.Register(StyleDef{Name: "verse", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(1, SymUnitEm),
		}})

		// Push poem, then push verse as nested container
		ctx := NewStyleContext(sr).
			PushBlock("div", "poem").
			PushBlock("div", "verse")

		// Margins from different containers should accumulate: 2em + 1em = 3em
		marginLeft := ctx.inherited[SymMarginLeft]
		if marginLeft == nil {
			t.Fatal("Expected margin-left to be inherited")
		}
		val, unit, ok := measureParts(marginLeft)
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", marginLeft)
		}
		expected := 3.0
		if val != expected {
			t.Errorf("Expected accumulated margin-left %.3fem, got %.3fem", expected, val)
		}
	})

	t.Run("same container margin is not double-counted in resolveProperties", func(t *testing.T) {
		sr := NewStyleRegistry()
		// cite with margin-left: 2em
		sr.Register(StyleDef{Name: "cite", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(2, SymUnitEm),
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
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", marginLeft)
		}
		// Should be 2em, NOT 4em (double-counted)
		expected := 2.0
		if val != expected {
			t.Errorf("Expected margin-left %.2fem (not double-counted), got %.2fem", expected, val)
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
		// poem with margin-left: 3em
		sr.Register(StyleDef{Name: "poem", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(3, SymUnitEm),
		}})
		// p with margin-left: 0 (explicitly)
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(0, SymUnitEm),
		}})
		// verse with margin-left: 2em
		sr.Register(StyleDef{Name: "verse", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(2, SymUnitEm),
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
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", marginLeft)
		}
		// Expected: poem 3em + verse 2em = 5em
		// The p's margin-left: 0 should NOT override the inherited poem margin
		expected := 5.0
		if val != expected {
			t.Errorf("Expected accumulated margin-left %.3fem, got %.3fem", expected, val)
		}
	})

	t.Run("element tag negative margin does not override inherited container margin", func(t *testing.T) {
		// This tests that a negative margin on the tag default (e.g., p { margin-left: -8pt })
		// does not override the accumulated container margin from poem/stanza.
		// The tag default has lower CSS specificity than the class selectors that built
		// the inherited margin, so it should be filtered out by filterTagDefaultsIfInherited.
		sr := NewStyleRegistry()
		// poem with margin-left: 3em
		sr.Register(StyleDef{Name: "poem", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(3, SymUnitEm),
		}})
		// p with negative margin-left (converted from -8pt to em)
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(-0.53333, SymUnitEm),
		}})
		// verse with margin-left: 2em
		sr.Register(StyleDef{Name: "verse", Properties: map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(2, SymUnitEm),
		}})

		// Simulate: poem > p.verse
		poemCtx := NewStyleContext(sr).PushBlock("div", "poem")
		resolved := poemCtx.resolveProperties("p", "verse")

		marginLeft := resolved[SymMarginLeft]
		if marginLeft == nil {
			t.Fatal("Expected margin-left in resolved style")
		}
		val, unit, ok := measureParts(marginLeft)
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", marginLeft)
		}
		// Expected: poem 3em + verse 2em = 5em
		// The p's negative margin-left should NOT override the inherited poem margin
		expected := 5.0
		if val != expected {
			t.Errorf("Expected accumulated margin-left %.3fem, got %.3fem", expected, val)
		}
	})

	t.Run("negative margin preserved at output", func(t *testing.T) {
		// Verify that negative margins pass through to the final output without clamping.
		sr := NewStyleRegistry()

		// Register a resolved style with a negative margin
		props := map[KFXSymbol]any{
			SymMarginLeft: DimensionValue(-1.5, SymUnitEm),
		}
		styleName := sr.RegisterResolved(props, styleUsageText, true)

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		marginLeft := def.Properties[SymMarginLeft]
		if marginLeft == nil {
			t.Fatal("Expected margin-left in resolved style")
		}
		val, unit, ok := measureParts(marginLeft)
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", marginLeft)
		}
		// Negative margin should be preserved as-is
		if val != -1.5 {
			t.Errorf("Expected margin-left -1.500em, got %.3fem", val)
		}
	})

	t.Run("container text-indent is not overridden by p tag default", func(t *testing.T) {
		// This tests the fix for the bug where p { text-indent: 1em } would
		// override the inherited text-indent: 0 from a footnote/poem container.
		sr := NewStyleRegistry()
		// footnote with text-indent: 0
		sr.Register(StyleDef{Name: "footnote", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(0, SymUnitEm),
		}})
		// p with text-indent: 1em (standard paragraph indent)
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(1, SymUnitEm),
		}})

		// Simulate: footnote > p (plain paragraph inside footnote container)
		ctx := NewStyleContext(sr).PushBlock("div", "footnote")
		styleName := ctx.Resolve("p", "")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		textIndent := def.Properties[SymTextIndent]
		if textIndent == nil {
			t.Fatal("Expected text-indent in resolved style")
		}
		val, unit, ok := measureParts(textIndent)
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", textIndent)
		}
		// Should be 0em (from footnote), NOT 1em (from p tag default)
		if val != 0 {
			t.Errorf("Expected text-indent 0em (inherited from footnote), got %.3fem", val)
		}
	})

	t.Run("container text-align is not overridden by p tag default", func(t *testing.T) {
		// Epigraph sets text-align: right, which should not be overridden
		// by p's text-align: justify.
		sr := NewStyleRegistry()
		sr.Register(StyleDef{Name: "epigraph", Properties: map[KFXSymbol]any{
			SymTextAlignment: SymbolValue(SymRight),
			SymFontStyle:     SymbolValue(SymItalic),
		}})
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymTextAlignment: SymbolValue(SymJustify),
			SymTextIndent:    DimensionValue(1, SymUnitEm),
		}})

		ctx := NewStyleContext(sr).PushBlock("div", "epigraph")
		styleName := ctx.Resolve("p", "")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		textAlign := def.Properties[SymTextAlignment]
		if textAlign == nil {
			t.Fatal("Expected text-align in resolved style")
		}
		// Should be right (from epigraph), NOT justify (from p tag default)
		got := symbolToInt(textAlign)
		want := int(SymRight)
		if got != want {
			t.Errorf("Expected text-align right (%d), got %d", want, got)
		}
	})

	t.Run("container font-style is not overridden by tag default", func(t *testing.T) {
		// Container sets font-style: italic, tag default should not override it.
		sr := NewStyleRegistry()
		sr.Register(StyleDef{Name: "cite", Properties: map[KFXSymbol]any{
			SymFontStyle: SymbolValue(SymItalic),
		}})
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymFontStyle: SymbolValue(SymNormal),
		}})

		ctx := NewStyleContext(sr).PushBlock("div", "cite")
		styleName := ctx.Resolve("p", "")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		fontStyle := def.Properties[SymFontStyle]
		if fontStyle == nil {
			t.Fatal("Expected font-style in resolved style")
		}
		got := symbolToInt(fontStyle)
		want := int(SymItalic)
		if got != want {
			t.Errorf("Expected font-style italic (%d), got %d", want, got)
		}
	})

	t.Run("class can still override container inherited property", func(t *testing.T) {
		// Even when container sets text-indent: 0, an explicit class should
		// be able to override it (step 3 of cascade).
		sr := NewStyleRegistry()
		sr.Register(StyleDef{Name: "footnote", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(0, SymUnitEm),
		}})
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(1, SymUnitEm),
		}})
		// An explicit class with its own text-indent
		sr.Register(StyleDef{Name: "indented", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(2, SymUnitEm),
		}})

		ctx := NewStyleContext(sr).PushBlock("div", "footnote")
		styleName := ctx.Resolve("p", "indented")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		textIndent := def.Properties[SymTextIndent]
		if textIndent == nil {
			t.Fatal("Expected text-indent in resolved style")
		}
		val, unit, ok := measureParts(textIndent)
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", textIndent)
		}
		// Should be 2em from the explicit "indented" class, overriding
		// both the container's 0em and the filtered p tag default's 1em
		if val != 2 {
			t.Errorf("Expected text-indent 2em (from explicit class), got %.3fem", val)
		}
	})

	t.Run("root-level p tag defaults apply normally without container", func(t *testing.T) {
		// Without a container, p's tag defaults should apply normally.
		// This ensures the filter doesn't break root-level paragraphs.
		sr := NewStyleRegistry()
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymTextIndent:    DimensionValue(1, SymUnitEm),
			SymTextAlignment: SymbolValue(SymJustify),
		}})

		ctx := NewStyleContext(sr)
		styleName := ctx.Resolve("p", "")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		// text-indent should be 1em (p's default, no container to filter)
		textIndent := def.Properties[SymTextIndent]
		if textIndent == nil {
			t.Fatal("Expected text-indent in resolved style")
		}
		val, _, ok := measureParts(textIndent)
		if !ok || val != 1 {
			t.Errorf("Expected text-indent 1em, got %v", textIndent)
		}

		// text-align should be justify (p's default, no container to filter)
		textAlign := def.Properties[SymTextAlignment]
		if textAlign == nil {
			t.Fatal("Expected text-align in resolved style")
		}
		got := symbolToInt(textAlign)
		if got != int(SymJustify) {
			t.Errorf("Expected text-align justify, got %d", got)
		}
	})

	t.Run("descendant selector overrides inherited property", func(t *testing.T) {
		// descendant selector (footnote--p) should be able to set properties
		// even when container also set them (step 4 of cascade).
		sr := NewStyleRegistry()
		sr.Register(StyleDef{Name: "footnote", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(0, SymUnitEm),
		}})
		sr.Register(StyleDef{Name: "p", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(1, SymUnitEm),
		}})
		// descendant selector: footnote--p sets text-indent to 0.5em
		sr.Register(StyleDef{Name: "footnote--p", Properties: map[KFXSymbol]any{
			SymTextIndent: DimensionValue(0.5, SymUnitEm),
		}})

		ctx := NewStyleContext(sr).PushBlock("div", "footnote")
		styleName := ctx.Resolve("p", "")

		def, ok := sr.Get(styleName)
		if !ok {
			t.Fatalf("Style %q not found", styleName)
		}

		textIndent := def.Properties[SymTextIndent]
		if textIndent == nil {
			t.Fatal("Expected text-indent in resolved style")
		}
		val, unit, ok := measureParts(textIndent)
		if !ok || unit != SymUnitEm {
			t.Fatalf("Expected em unit, got %v", textIndent)
		}
		// Should be 0.5em from descendant selector, NOT 0em from container or 1em from p
		if val != 0.5 {
			t.Errorf("Expected text-indent 0.5em (from descendant selector), got %.3fem", val)
		}
	})
}
