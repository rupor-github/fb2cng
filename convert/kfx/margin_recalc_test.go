package kfx

import (
	"math"
	"testing"

	"go.uber.org/zap"
)

// TestCSSStylesValueParsing verifies that CSS values from stylemap's CSSStyles
// are properly parsed with value and unit fields populated.
//
// The issue was that CSSStyles entries like "0.67em" were being stored as
// CSSValue{Raw: "0.67em"} without parsing Value and Unit, causing the CSS
// converter to skip them (it checks cssVal.IsNumeric() which requires Value != 0).
func TestCSSStylesValueParsing(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	// Simulate h1 with font-size: 140%
	sel := Selector{Element: "h1", Raw: "h1"}
	props := map[string]CSSValue{
		"font-size": {Value: 140, Unit: "%", Raw: "140%"},
	}

	// Apply stylemap CSS defaults (this adds margin-top from CSSStyles)
	propsAfterStylemap := mapper.applyStyleMapCSS(sel, props)

	// Check margin-top was properly parsed
	mt, ok := propsAfterStylemap["margin-top"]
	if !ok {
		t.Fatal("margin-top not added by stylemap")
	}

	// Verify the CSS value was properly parsed
	if mt.Value != 0.67 {
		t.Errorf("Expected margin-top Value = 0.67, got %v", mt.Value)
	}
	if mt.Unit != "em" {
		t.Errorf("Expected margin-top Unit = 'em', got %q", mt.Unit)
	}
}

// TestMarginConversionWithStylemap verifies the full flow of margin conversion
// when stylemap provides default margins for heading elements.
//
// The formula is: margin_lh = margin_em / LineHeightRatio (1.2)
// For h1 with margin 0.67em: 0.67 / 1.2 = 0.55833lh
func TestMarginConversionWithStylemap(t *testing.T) {
	log := zap.NewNop()
	mapper := NewStyleMapper(log, nil)

	// CSS rule: h1 { font-size: 140%; }
	// Stylemap adds: margin-top: 0.67em, margin-bottom: 0.67em
	sel := Selector{Element: "h1", Raw: "h1"}
	props := map[string]CSSValue{
		"font-size": {Value: 140, Unit: "%", Raw: "140%"},
	}

	kfxProps, _ := mapper.MapRule(sel, props)

	// Check margin-top conversion
	marginTop := kfxProps[SymMarginTop]
	marginVal, marginUnit, ok := measureParts(marginTop)
	if !ok {
		t.Fatal("Failed to extract margin-top")
	}

	// Expected: 0.67em / 1.2 = 0.55833lh
	expected := RoundDecimal(0.67 / LineHeightRatio)
	if marginVal != expected {
		t.Errorf("Expected margin-top = %f, got %f", expected, marginVal)
	}
	if marginUnit != SymUnitLh {
		t.Errorf("Expected margin-top unit = lh, got %v", marginUnit)
	}
}

// TestMarginConversionFullFlow tests the complete stylesheet processing flow.
func TestMarginConversionFullFlow(t *testing.T) {
	// Parse CSS
	cssData := []byte(`h1 { font-size: 140%; }`)
	parser := NewParser(nil)
	sheet := parser.Parse(cssData)

	// Map stylesheet
	mapper := NewStyleMapper(nil, nil)
	styles, _ := mapper.MapStylesheet(sheet)

	// Find h1 style
	var h1Style *StyleDef
	for i := range styles {
		if styles[i].Name == "h1" {
			h1Style = &styles[i]
			break
		}
	}
	if h1Style == nil {
		t.Fatal("h1 style not found in mapped styles")
	}

	// Check margin-top
	marginTop := h1Style.Properties[SymMarginTop]
	marginVal, _, ok := measureParts(marginTop)
	if !ok {
		t.Fatal("Failed to extract margin-top from h1 style")
	}

	// Expected: 0.67em / 1.2 = 0.55833lh
	expected := RoundDecimal(0.67 / LineHeightRatio)
	if marginVal != expected {
		t.Errorf("Expected margin-top = %f, got %f", expected, marginVal)
	}
}

// TestShorthandPropertyPreventsStylemapOverride verifies that when CSS has a
// shorthand property (like "margin"), stylemap defaults for the expanded
// properties (like "margin-left") are not added.
func TestShorthandPropertyPreventsStylemapOverride(t *testing.T) {
	mapper := NewStyleMapper(nil, nil)

	// CSS: blockquote.cite { margin: 1em 2em; }
	// Stylemap for blockquote has CSSStyles: {margin-left: 40px, margin-right: 40px}
	// The stylemap defaults should NOT override because CSS has "margin" shorthand
	sel := Selector{Element: "blockquote", Class: "cite", Raw: "blockquote.cite"}
	props := map[string]CSSValue{
		"margin": {Value: 0, Raw: "1em 2em"},
	}

	propsAfter := mapper.applyStyleMapCSS(sel, props)

	// margin-left should NOT be added (covered by "margin" shorthand)
	if ml, ok := propsAfter["margin-left"]; ok {
		t.Errorf("margin-left should not be added when margin shorthand exists, got %+v", ml)
	}

	// margin shorthand should still be present
	if _, ok := propsAfter["margin"]; !ok {
		t.Error("margin shorthand should still be present")
	}
}

// TestAdjustLineHeightForFontSize verifies that styles with non-default font-size
// get adjusted line-height (1.0101lh) and correspondingly adjusted vertical margins.
func TestAdjustLineHeightForFontSize(t *testing.T) {
	tests := []struct {
		name           string
		props          map[KFXSymbol]any
		wantLineHeight float64
		wantMarginTop  float64
		wantAdjustment bool // whether adjustment should happen
	}{
		{
			name: "no font-size",
			props: map[KFXSymbol]any{
				SymMarginTop: DimensionValue(0.55833, SymUnitLh),
			},
			wantLineHeight: 0, // not set
			wantMarginTop:  0.55833,
			wantAdjustment: false,
		},
		{
			name: "default font-size 1rem",
			props: map[KFXSymbol]any{
				SymFontSize:  DimensionValue(1.0, SymUnitRem),
				SymMarginTop: DimensionValue(0.55833, SymUnitLh),
			},
			wantLineHeight: 0, // not changed
			wantMarginTop:  0.55833,
			wantAdjustment: false,
		},
		{
			name: "non-default font-size 1.25rem",
			props: map[KFXSymbol]any{
				SymFontSize:  DimensionValue(1.25, SymUnitRem),
				SymMarginTop: DimensionValue(0.55833, SymUnitLh),
			},
			wantLineHeight: RoundDecimal(AdjustedLineHeightLh), // 1.0101
			wantMarginTop:  RoundDecimal(0.55833 / AdjustedLineHeightLh),
			wantAdjustment: true,
		},
		{
			name: "small font-size 0.8rem",
			props: map[KFXSymbol]any{
				SymFontSize:  DimensionValue(0.8, SymUnitRem),
				SymMarginTop: DimensionValue(0.41667, SymUnitLh),
			},
			wantLineHeight: RoundDecimal(AdjustedLineHeightLh),
			wantMarginTop:  RoundDecimal(0.41667 / AdjustedLineHeightLh),
			wantAdjustment: true,
		},
		{
			name: "font-size in percent (not rem) - no adjustment",
			props: map[KFXSymbol]any{
				SymFontSize:  DimensionValue(140, SymUnitPercent),
				SymMarginTop: DimensionValue(0.55833, SymUnitLh),
			},
			wantLineHeight: 0,
			wantMarginTop:  0.55833,
			wantAdjustment: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adjustLineHeightForFontSize(tt.props)

			// Check line-height
			if lh, ok := result[SymLineHeight]; ok {
				lhVal, lhUnit, _ := measureParts(lh)
				if !tt.wantAdjustment {
					t.Errorf("line-height should not be set, got %v%v", lhVal, lhUnit)
				} else if lhVal != tt.wantLineHeight {
					t.Errorf("line-height = %v, want %v", lhVal, tt.wantLineHeight)
				}
			} else if tt.wantAdjustment {
				t.Error("line-height should be set but was not")
			}

			// Check margin-top
			if mt, ok := result[SymMarginTop]; ok {
				mtVal, _, _ := measureParts(mt)
				if math.Abs(mtVal-tt.wantMarginTop) > 1e-9 {
					t.Errorf("margin-top = %v, want %v", mtVal, tt.wantMarginTop)
				}
			}
		})
	}
}

// TestAdjustedMarginMatchesKP3 verifies that the full margin conversion with
// line-height adjustment produces values that match KP3 reference output.
//
// KP3 example: CSS margin-top: 0.67em with font-size: 140% → 0.55275lh
func TestAdjustedMarginMatchesKP3(t *testing.T) {
	// The full conversion path:
	// 1. CSS: margin-top: 0.67em → KFX: 0.67 / 1.2 = 0.55833lh
	// 2. For font-size != 1rem: 0.55833 / 1.0101 = 0.55275lh

	marginEm := 0.67
	marginLhInitial := marginEm / LineHeightRatio              // 0.55833...
	marginLhAdjusted := marginLhInitial / AdjustedLineHeightLh // 0.55275...

	expected := 0.55275
	got := RoundDecimal(marginLhAdjusted)

	if got != expected {
		t.Errorf("Adjusted margin = %v, want %v (KP3 reference)", got, expected)
	}
}
