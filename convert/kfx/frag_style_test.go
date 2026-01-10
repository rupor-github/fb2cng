package kfx

import (
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
	t.Run("empty context", func(t *testing.T) {
		ctx := NewStyleContext()
		result := ctx.Resolve("p", "verse")
		if result != "p verse" {
			t.Errorf("Expected 'p verse', got %q", result)
		}
	})

	t.Run("single ancestor", func(t *testing.T) {
		ctx := NewStyleContext().Push("poem")
		result := ctx.Resolve("p", "verse")
		if result != "p poem verse" {
			t.Errorf("Expected 'p poem verse', got %q", result)
		}
	})

	t.Run("multiple ancestors", func(t *testing.T) {
		ctx := NewStyleContext().Push("poem").Push("stanza")
		result := ctx.Resolve("p", "verse")
		if result != "p poem stanza verse" {
			t.Errorf("Expected 'p poem stanza verse', got %q", result)
		}
	})

	t.Run("deeply nested", func(t *testing.T) {
		ctx := NewStyleContext().Push("cite").Push("poem").Push("stanza")
		result := ctx.Resolve("p", "verse")
		if result != "p cite poem stanza verse" {
			t.Errorf("Expected 'p cite poem stanza verse', got %q", result)
		}
	})

	t.Run("empty element style", func(t *testing.T) {
		ctx := NewStyleContext().Push("epigraph")
		result := ctx.Resolve("p", "")
		if result != "p epigraph" {
			t.Errorf("Expected 'p epigraph', got %q", result)
		}
	})

	t.Run("empty base style", func(t *testing.T) {
		ctx := NewStyleContext().Push("poem")
		result := ctx.Resolve("", "verse")
		if result != "poem verse" {
			t.Errorf("Expected 'poem verse', got %q", result)
		}
	})

	t.Run("push empty context is no-op", func(t *testing.T) {
		ctx := NewStyleContext().Push("poem").Push("").Push("stanza")
		result := ctx.Resolve("p", "verse")
		if result != "p poem stanza verse" {
			t.Errorf("Expected 'p poem stanza verse', got %q", result)
		}
	})

	t.Run("immutability - push returns new context", func(t *testing.T) {
		ctx1 := NewStyleContext().Push("poem")
		ctx2 := ctx1.Push("stanza")

		result1 := ctx1.Resolve("p", "verse")
		result2 := ctx2.Resolve("p", "verse")

		if result1 != "p poem verse" {
			t.Errorf("ctx1 should be 'p poem verse', got %q", result1)
		}
		if result2 != "p poem stanza verse" {
			t.Errorf("ctx2 should be 'p poem stanza verse', got %q", result2)
		}
	})
}
