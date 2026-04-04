package kfx

import (
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
