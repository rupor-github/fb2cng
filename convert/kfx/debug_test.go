package kfx

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestCiteSubtitleImageMargin(t *testing.T) {
	log := zap.NewNop()

	cssData, err := os.ReadFile("../../build/test.css")
	if err != nil {
		cssData, err = os.ReadFile("../../convert/default.css")
		if err != nil {
			t.Fatalf("Cannot load CSS: %v", err)
		}
	}

	sr, _ := NewStyleRegistryFromCSS(cssData, nil, log)

	// Simulate: footnote body > cite > subtitle (image-only)
	// footnote body context (no special class)
	ctx := NewStyleContext(sr)

	// Enter cite container
	citeCtx := ctx.PushBlock("blockquote", "cite")
	t.Logf("After PushBlock cite:")
	t.Logf("  inherited margin-left: %v", citeCtx.inherited[SymMarginLeft])

	// Now resolve properties for p.cite-subtitle (the subtitle)
	marginLeft := citeCtx.ResolveProperty("p", "cite-subtitle", SymMarginLeft)
	t.Logf("ResolveProperty(p, cite-subtitle, margin-left): %v", marginLeft)

	textIndent := citeCtx.ResolveProperty("p", "cite-subtitle", SymTextIndent)
	t.Logf("ResolveProperty(p, cite-subtitle, text-indent): %v", textIndent)

	// The marginLeft should be 6.25% inherited from cite container.
	// The p rule in test.css has margin: 0 -8pt 0.3em -8pt, but KFX does not
	// support negative margins, so the -8pt is ignored and the value is
	// inherited from the parent cite container (which has margin: 1em 2em = 6.25%).
	if marginLeft == nil {
		t.Error("Expected margin-left to be resolved from cite context")
	} else {
		val, unit, _ := measureParts(marginLeft)
		t.Logf("margin-left: %.5f%% (unit: %v)", val, unit)
		expected := 6.25 // inherited from cite (2em = 6.25%)
		if val != expected {
			t.Errorf("Expected margin-left %.5f%%, got %.5f%%", expected, val)
		}
	}
}
