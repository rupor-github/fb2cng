package kfx

import (
	"math"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func TestPropagateToHeadingDescendants_SubModifiedByCSS(t *testing.T) {
	// When user CSS modifies "sub" (e.g., vertical-align: baseline; font-size: 25%),
	// heading-context descendants (h1--sub, etc.) must be updated to reflect only
	// the changed default properties:
	//   - baseline_style: subscript -> normal (copied directly)
	//   - font_size: 0.75rem -> 0.25rem -> 0.25em (converted rem->em for heading context)
	// Properties NOT in the defaults (line_height, baseline_shift) must NOT be propagated.
	cssData := []byte(`sub { font-size: 25%; vertical-align: baseline; line-height: 0; }`)
	sr, _ := parseAndCreateRegistry(cssData, nil, zap.NewNop())

	// Verify the base "sub" style was modified by user CSS.
	subDef, ok := sr.Get("sub")
	if !ok {
		t.Fatal("sub style not found")
	}

	// baseline_style should be normal (user set vertical-align: baseline,
	// and Bug 1 fix allows normal to override subscript in CSS cascade).
	if sym, ok := symbolIDFromAny(subDef.Properties[SymBaselineStyle]); !ok || sym != SymNormal {
		t.Errorf("sub baseline_style: got %v, want normal", subDef.Properties[SymBaselineStyle])
	}

	// All heading-context descendants should have ONLY the changed default properties,
	// with font-size converted from rem to em.
	expectedFontSize := DimensionValue(0.25, SymUnitEm)

	for i := 1; i <= 6; i++ {
		descName := "h" + string(rune('0'+i)) + "--sub"
		descDef, ok := sr.Get(descName)
		if !ok {
			t.Errorf("%s style not found", descName)
			continue
		}

		// baseline_style should be updated to normal.
		if sym, ok := symbolIDFromAny(descDef.Properties[SymBaselineStyle]); !ok || sym != SymNormal {
			t.Errorf("%s baseline_style: got %v, want normal",
				descName, descDef.Properties[SymBaselineStyle])
		}

		// font_size should be 0.25em (converted from 0.25rem).
		if !reflect.DeepEqual(descDef.Properties[SymFontSize], expectedFontSize) {
			t.Errorf("%s font_size: got %v, want %v",
				descName, descDef.Properties[SymFontSize], expectedFontSize)
		}

		// line_height and baseline_shift should NOT be present.
		// These are not in the defaults, so they must not be propagated.
		if _, has := descDef.Properties[SymLineHeight]; has {
			t.Errorf("%s should NOT have line_height, but it does: %v",
				descName, descDef.Properties[SymLineHeight])
		}
		if _, has := descDef.Properties[SymBaselineShift]; has {
			t.Errorf("%s should NOT have baseline_shift, but it does: %v",
				descName, descDef.Properties[SymBaselineShift])
		}

		// Should have exactly 2 properties (baseline_style + font_size).
		if len(descDef.Properties) != 2 {
			t.Errorf("%s should have 2 properties, got %d: %v",
				descName, len(descDef.Properties), descDef.Properties)
		}
	}
}

func TestPropagateToHeadingDescendants_SupModifiedByCSS(t *testing.T) {
	// When user CSS changes only vertical-align (not font-size), only baseline_style
	// should be propagated. font_size should keep its original heading-context value.
	cssData := []byte(`sup { vertical-align: baseline; }`)
	sr, _ := parseAndCreateRegistry(cssData, nil, zap.NewNop())

	supDef, ok := sr.Get("sup")
	if !ok {
		t.Fatal("sup style not found")
	}

	// baseline_style should be normal (user's vertical-align: baseline overrides superscript).
	if sym, ok := symbolIDFromAny(supDef.Properties[SymBaselineStyle]); !ok || sym != SymNormal {
		t.Errorf("sup baseline_style: got %v, want normal", supDef.Properties[SymBaselineStyle])
	}

	// font_size in sup was NOT changed by CSS (still 0.75rem = default),
	// so heading-context descendants should keep their original font_size (0.9em).
	expectedFontSize := DimensionValue(0.9, SymUnitEm)

	for i := 1; i <= 6; i++ {
		descName := "h" + string(rune('0'+i)) + "--sup"
		descDef, ok := sr.Get(descName)
		if !ok {
			t.Errorf("%s style not found", descName)
			continue
		}

		// baseline_style should be updated to normal.
		if sym, ok := symbolIDFromAny(descDef.Properties[SymBaselineStyle]); !ok || sym != SymNormal {
			t.Errorf("%s baseline_style: got %v, want normal",
				descName, descDef.Properties[SymBaselineStyle])
		}

		// font_size should remain 0.9em (original heading-context value, unchanged).
		if !reflect.DeepEqual(descDef.Properties[SymFontSize], expectedFontSize) {
			t.Errorf("%s font_size: got %v, want %v",
				descName, descDef.Properties[SymFontSize], expectedFontSize)
		}

		// Should still have exactly 2 properties (baseline_style + font_size).
		if len(descDef.Properties) != 2 {
			t.Errorf("%s should have 2 properties, got %d: %v",
				descName, len(descDef.Properties), descDef.Properties)
		}
	}
}

func TestPropagateToHeadingDescendants_NoChangeWithoutUserCSS(t *testing.T) {
	// Without user CSS, heading-context descendants should keep their original
	// hardcoded values (e.g., h1--sub should have font_size: 0.9em, not 0.75rem).
	sr, _ := parseAndCreateRegistry(nil, nil, zap.NewNop())

	h1Sub, ok := sr.Get("h1--sub")
	if !ok {
		t.Fatal("h1--sub style not found")
	}

	// h1--sub should have the original defaults: baseline_style: subscript, font_size: 0.9em.
	if sym, ok := symbolIDFromAny(h1Sub.Properties[SymBaselineStyle]); !ok || sym != SymSubscript {
		t.Errorf("h1--sub baseline_style: got %v, want subscript", h1Sub.Properties[SymBaselineStyle])
	}

	expectedFontSize := DimensionValue(0.9, SymUnitEm)
	if !reflect.DeepEqual(h1Sub.Properties[SymFontSize], expectedFontSize) {
		t.Errorf("h1--sub font_size: got %v, want %v", h1Sub.Properties[SymFontSize], expectedFontSize)
	}
}

func TestPropagateToHeadingDescendants_SmallModifiedByCSS(t *testing.T) {
	// When user CSS modifies "small" font-size, heading-context descendants should
	// get the updated font-size converted to em.
	cssData := []byte(`small { font-size: 50%; }`)
	sr, _ := parseAndCreateRegistry(cssData, nil, zap.NewNop())

	if _, ok := sr.Get("small"); !ok {
		t.Fatal("small style not found")
	}

	// small has only font_size in its defaults. 50% -> PercentToRem(50) = 0.5rem.
	// The heading-context descendants should get 0.5em (converted from rem).
	expectedFontSize := DimensionValue(0.5, SymUnitEm)

	for i := 1; i <= 6; i++ {
		descName := "h" + string(rune('0'+i)) + "--small"
		descDef, ok := sr.Get(descName)
		if !ok {
			t.Errorf("%s style not found", descName)
			continue
		}

		if !reflect.DeepEqual(descDef.Properties[SymFontSize], expectedFontSize) {
			t.Errorf("%s font_size: got %v, want %v",
				descName, descDef.Properties[SymFontSize], expectedFontSize)
		}

		// h*--small originally had 0 properties (empty Build()).
		// After propagation, it should have exactly 1 property (font_size only).
		if len(descDef.Properties) != 1 {
			t.Errorf("%s should have 1 property, got %d: %v",
				descName, len(descDef.Properties), descDef.Properties)
		}
	}
}

func TestPropagateToHeadingDescendants_FontSizeEmNotConverted(t *testing.T) {
	// When user CSS sets font-size in em (not percent/rem), the CSS merger applies
	// relative merging: 0.75rem (default) * 0.5em = 0.375em (product).
	// Since the merged value is already in em, it should be propagated as-is.
	cssData := []byte(`sub { font-size: 0.5em; }`)
	sr, _ := parseAndCreateRegistry(cssData, nil, zap.NewNop())

	// The merged sub font-size is 0.375em (0.75 * 0.5, relative merge).
	// The heading-context descendants should get 0.375em (already em, no conversion).
	expectedFontSize := DimensionValue(0.375, SymUnitEm)

	h1Sub, ok := sr.Get("h1--sub")
	if !ok {
		t.Fatal("h1--sub style not found")
	}

	if !reflect.DeepEqual(h1Sub.Properties[SymFontSize], expectedFontSize) {
		t.Errorf("h1--sub font_size: got %v, want %v",
			h1Sub.Properties[SymFontSize], expectedFontSize)
	}

	// baseline_style was not changed (still subscript = default), so it should
	// remain at the original heading-context value.
	if sym, ok := symbolIDFromAny(h1Sub.Properties[SymBaselineStyle]); !ok || sym != SymSubscript {
		t.Errorf("h1--sub baseline_style: got %v, want subscript (unchanged)",
			h1Sub.Properties[SymBaselineStyle])
	}
}

func TestConvertFontSizeToEm(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "rem to em",
			input:    DimensionValue(0.25, SymUnitRem),
			expected: DimensionValue(0.25, SymUnitEm),
		},
		{
			name:     "em unchanged",
			input:    DimensionValue(0.5, SymUnitEm),
			expected: DimensionValue(0.5, SymUnitEm),
		},
		{
			name:     "non-dimension unchanged",
			input:    SymbolValue(SymNormal),
			expected: SymbolValue(SymNormal),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertFontSizeToEm(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("convertFontSizeToEm(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAccumulatedFontSize_SubInHeading(t *testing.T) {
	// Test the accumulated font-size tracking for sub inside heading context.
	// This is the core bug: sub { font-size: 25% } inside h1 should produce
	// a correct rem value, not the wrong 0.40625rem from mergeRelative with
	// compressed rem values.
	//
	// Setup:
	//   DefaultStyleRegistry h1 font-size: 2.0rem (browser default, RemToFontSizeMultiplier → 2.6)
	//   CSS .chapter-title-header-next font-size: 200% → PercentToRem(200) = 1.625rem (mult 2.0)
	//   CSS sub font-size: 25% → PercentToRem(25) = 0.25rem, h1--sub gets 0.25em (from propagation)
	//
	// Push("h1") → fontSizeAccumEm = 1.0 × 2.6 = 2.6 (h1's 2.0rem → multiplier 2.6)
	// resolveProperties("", "chapter-title-header-next sub"):
	//   chapter-title-header-next: 1.625rem → mult 2.0 → localAccumEm = 1.0 × 2.0 = 2.0
	//     Wait — the base is sc.fontSizeAccumEm = 2.6. So localAccumEm = 2.6 × 2.0 = 5.2
	//     No! rem resets to base × mult: localAccumEm = sc.fontSizeAccumEm × 2.0 = 2.6 × 2.0 = 5.2
	//   h1--sub: 0.25em → localAccumEm = 5.2 × 0.25 = 1.3
	//   PercentToRem(1.3 × 100) = PercentToRem(130) = 1.1875rem
	cssData := []byte(`
		sub { font-size: 25%; vertical-align: baseline; }
		.chapter-title-header-next { font-size: 200%; }
	`)
	sr, _ := parseAndCreateRegistry(cssData, nil, zap.NewNop())

	// Create context and push into h1 scope
	ctx := NewStyleContext(sr)
	ctx = ctx.Push("h1", "")

	// Verify fontSizeAccumEm after Push("h1")
	// h1 has font-size: 2.0rem (DefaultStyleRegistry base, no CSS override in test)
	// RemToFontSizeMultiplier(2.0) = ((2.0-1)*160/100)+1 = 2.6
	// fontSizeAccumEm = 1.0 × 2.6 = 2.6
	expectedAccum := 2.6
	if math.Abs(ctx.fontSizeAccumEm-expectedAccum) > 1e-9 {
		t.Errorf("after Push(h1): fontSizeAccumEm = %v, want %v", ctx.fontSizeAccumEm, expectedAccum)
	}

	// Resolve inline properties for "chapter-title-header-next sub"
	resolved := ctx.resolveProperties("", "chapter-title-header-next sub")
	fs, ok := resolved[SymFontSize]
	if !ok {
		t.Fatal("resolved properties missing font-size")
	}

	val, unit, ok := measureParts(fs)
	if !ok {
		t.Fatalf("font-size is not a dimension: %v", fs)
	}
	if unit != SymUnitRem {
		t.Errorf("font-size unit = %v, want rem", unit)
	}

	// The corrected value:
	// localAccumEm = 2.6 (from h1) × 2.0 (chapter-title-header-next) = 5.2
	// then × 0.25 (h1--sub em) = 1.3
	// PercentToRem(130) = 1 + (130-100)/160 = 1.1875rem
	expectedRem := PercentToRem(2.6 * 2.0 * 0.25 * 100) // PercentToRem(130) = 1.1875
	if math.Abs(val-expectedRem) > 1e-6 {
		t.Errorf("sub-in-heading font-size = %.6f rem, want %.6f rem",
			val, expectedRem)
	}

	// Critical: verify the old buggy value (0.40625) is NOT produced
	if math.Abs(val-0.40625) < 1e-6 {
		t.Error("font-size is still the old buggy value 0.40625rem; accumulated em tracking is not working")
	}
}

func TestAccumulatedFontSize_PushAccumulation(t *testing.T) {
	// Test that fontSizeAccumEm is correctly accumulated through Push calls.
	// DefaultStyleRegistry h1 = 2.0rem, h2 = 1.125rem, p = 1.0rem (no explicit font-size)
	sr, _ := parseAndCreateRegistry(nil, nil, zap.NewNop())

	ctx := NewStyleContext(sr)
	if ctx.fontSizeAccumEm != 1.0 {
		t.Errorf("root fontSizeAccumEm = %v, want 1.0", ctx.fontSizeAccumEm)
	}

	// Push h1: 2.0rem → RemToFontSizeMultiplier(2.0) = 2.6
	h1Ctx := ctx.Push("h1", "")
	expectedH1 := RemToFontSizeMultiplier(2.0) // 2.6
	if math.Abs(h1Ctx.fontSizeAccumEm-expectedH1) > 1e-9 {
		t.Errorf("after Push(h1): fontSizeAccumEm = %v, want %v", h1Ctx.fontSizeAccumEm, expectedH1)
	}

	// Push h2: 1.5rem → RemToFontSizeMultiplier(1.5) = 1.8
	h2Ctx := ctx.Push("h2", "")
	expectedH2 := RemToFontSizeMultiplier(1.5) // 1.8
	if math.Abs(h2Ctx.fontSizeAccumEm-expectedH2) > 1e-9 {
		t.Errorf("after Push(h2): fontSizeAccumEm = %v, want %v", h2Ctx.fontSizeAccumEm, expectedH2)
	}
}

func TestAccumulatedFontSize_NoEmNoCorrection(t *testing.T) {
	// When no em font-size is involved (all rem), the post-correction should NOT
	// modify the font-size. This ensures the fix only triggers for the specific case.
	cssData := []byte(`.large { font-size: 200%; }`)
	sr, _ := parseAndCreateRegistry(cssData, nil, zap.NewNop())

	ctx := NewStyleContext(sr)

	// Resolve with only rem-based classes — no em, no correction needed
	resolved := ctx.resolveProperties("p", "large")
	fs, ok := resolved[SymFontSize]
	if !ok {
		t.Fatal("resolved properties missing font-size")
	}

	val, unit, ok := measureParts(fs)
	if !ok {
		t.Fatalf("font-size is not a dimension: %v", fs)
	}
	if unit != SymUnitRem {
		t.Errorf("font-size unit = %v, want rem", unit)
	}

	// Should be PercentToRem(200) = 1.625rem (unchanged, no em correction)
	expected := PercentToRem(200)
	if math.Abs(val-expected) > 1e-6 {
		t.Errorf("font-size = %v rem, want %v rem", val, expected)
	}
}
