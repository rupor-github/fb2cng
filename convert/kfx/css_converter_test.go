package kfx

import (
	"fbc/css"
	"math"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/amazon-ion/ion-go/ion"
	"go.uber.org/zap"
)

// TestTitleStylesFromCSS verifies that title styles from default.css have proper formatting.
func TestTitleStylesFromCSS(t *testing.T) {
	cssData, err := os.ReadFile("../../convert/default.css")
	if err != nil {
		// Try alternate path for when running from different directory
		cssData, err = os.ReadFile("../default.css")
		if err != nil {
			t.Skip("Could not read default.css, skipping test")
		}
	}

	log := zap.NewNop()
	registry, _ := parseAndCreateRegistry(cssData, nil, log)

	// Title header styles should have text-align: center from CSS
	titleStyles := []string{
		"body-title-header",
		"chapter-title-header",
		"section-title-header",
	}

	for _, styleName := range titleStyles {
		t.Run(styleName, func(t *testing.T) {
			def, ok := registry.Get(styleName)
			if !ok {
				t.Fatalf("style %q not found in registry", styleName)
			}

			// Check text-align is center ($320)
			// CSS converter stores KFXSymbol directly, not wrapped
			if align, ok := def.Properties[SymTextAlignment]; ok {
				isCenter := align == SymCenter || align == SymbolValue(SymCenter)
				if !isCenter {
					t.Errorf("expected text-align: center, got %v (type %T)", align, align)
				}
			} else {
				t.Error("text-align property not set")
			}

			// Check font-weight is bold ($361)
			if weight, ok := def.Properties[SymFontWeight]; ok {
				isBold := weight == SymBold || weight == SymbolValue(SymBold)
				if !isBold {
					t.Errorf("expected font-weight: bold, got %v (type %T)", weight, weight)
				}
			} else {
				t.Error("font-weight property not set")
			}
		})
	}
}

func TestConvertFontWeight(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"bold keyword", css.CSSValue{Keyword: "bold"}, SymBold, true},
		{"bolder keyword", css.CSSValue{Keyword: "bolder"}, SymBold, true},
		{"lighter keyword", css.CSSValue{Keyword: "lighter"}, SymLight, true},
		{"normal keyword", css.CSSValue{Keyword: "normal"}, SymNormal, true},
		{"numeric 700", css.CSSValue{Value: 700}, SymBold, true},
		{"numeric 600", css.CSSValue{Value: 600}, SymSemibold, true},
		{"numeric 500", css.CSSValue{Value: 500}, SymMedium, true},
		{"numeric 400", css.CSSValue{Value: 400}, SymNormal, true},
		{"numeric 300", css.CSSValue{Value: 300}, SymLight, true},
		{"numeric 200", css.CSSValue{Value: 200}, SymLight, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertFontWeight(tt.input)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got ok=%v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestConvertFontStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"italic", css.CSSValue{Keyword: "italic"}, SymItalic, true},
		{"oblique", css.CSSValue{Keyword: "oblique"}, SymItalic, true},
		{"normal", css.CSSValue{Keyword: "normal"}, SymNormal, true},
		{"unknown", css.CSSValue{Keyword: "unknown"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertFontStyle(tt.input)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got ok=%v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestConvertTextAlign(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"left", css.CSSValue{Keyword: "left"}, SymLeft, true},
		{"start", css.CSSValue{Keyword: "start"}, SymStart, true},
		{"right", css.CSSValue{Keyword: "right"}, SymRight, true},
		{"end", css.CSSValue{Keyword: "end"}, SymEnd, true},
		{"center", css.CSSValue{Keyword: "center"}, SymCenter, true},
		{"justify", css.CSSValue{Keyword: "justify"}, SymJustify, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertTextAlign(tt.input)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got ok=%v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestConvertTextDecoration(t *testing.T) {
	tests := []struct {
		name          string
		input         css.CSSValue
		underline     bool
		strikethrough bool
		none          bool
	}{
		{"underline", css.CSSValue{Raw: "underline"}, true, false, false},
		{"line-through", css.CSSValue{Raw: "line-through"}, false, true, false},
		{"both", css.CSSValue{Raw: "underline line-through"}, true, true, false},
		{"none", css.CSSValue{Raw: "none"}, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertTextDecoration(tt.input)
			if result.Underline != tt.underline {
				t.Errorf("underline: expected %v, got %v", tt.underline, result.Underline)
			}
			if result.Strikethrough != tt.strikethrough {
				t.Errorf("strikethrough: expected %v, got %v", tt.strikethrough, result.Strikethrough)
			}
			if result.None != tt.none {
				t.Errorf("none: expected %v, got %v", tt.none, result.None)
			}
		})
	}
}

func TestConvertVerticalAlign(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		hasValue bool
	}{
		{"super", css.CSSValue{Keyword: "super"}, true},
		{"sub", css.CSSValue{Keyword: "sub"}, true},
		{"baseline", css.CSSValue{Keyword: "baseline"}, true},
		{"numeric", css.CSSValue{Value: 0.5, Unit: "em"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertVerticalAlign(tt.input)
			if ok != tt.hasValue {
				t.Errorf("expected ok=%v, got ok=%v", tt.hasValue, ok)
			}
			if ok && !result.UseBaselineStyle && !result.UseBaselineShift {
				t.Error("expected valid result with baseline_style or baseline_shift")
			}
		})
	}
}

func TestParseColor(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		r, g, b  int
		shouldOk bool
	}{
		{"hex 6 digit", css.CSSValue{Raw: "#ff0000"}, 255, 0, 0, true},
		{"hex 3 digit", css.CSSValue{Raw: "#f00"}, 255, 0, 0, true},
		{"rgb function", css.CSSValue{Raw: "rgb(255, 128, 0)"}, 255, 128, 0, true},
		{"keyword black", css.CSSValue{Raw: "black"}, 0, 0, 0, true},
		{"keyword white", css.CSSValue{Raw: "white"}, 255, 255, 255, true},
		{"keyword red", css.CSSValue{Raw: "red"}, 255, 0, 0, true},
		{"invalid", css.CSSValue{Raw: "invalid-color"}, 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, ok := ParseColor(tt.input)
			if ok != tt.shouldOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.shouldOk, ok)
			}
			if ok {
				if r != tt.r || g != tt.g || b != tt.b {
					t.Errorf("expected rgb(%d,%d,%d), got rgb(%d,%d,%d)", tt.r, tt.g, tt.b, r, g, b)
				}
			}
		})
	}
}

func TestCSSValueToKFX(t *testing.T) {
	tests := []struct {
		name         string
		input        css.CSSValue
		expectedUnit KFXSymbol
		shouldError  bool
	}{
		{"em unit", css.CSSValue{Value: 1.5, Unit: "em"}, SymUnitEm, false},
		{"px unit", css.CSSValue{Value: 16, Unit: "px"}, SymUnitPx, false},
		{"pt unit", css.CSSValue{Value: 12, Unit: "pt"}, SymUnitPt, false},
		{"percent", css.CSSValue{Value: 150, Unit: "%"}, SymUnitPercent, false},
		{"unitless", css.CSSValue{Value: 1.2, Unit: ""}, SymUnitLh, false},
		{"cm unit", css.CSSValue{Value: 2.5, Unit: "cm"}, SymUnitCm, false},
		{"rem unit", css.CSSValue{Value: 0.75, Unit: "rem"}, SymUnitRem, false},
		{"lh unit", css.CSSValue{Value: 1.0, Unit: "lh"}, SymUnitLh, false},
		{"unknown unit", css.CSSValue{Value: 1, Unit: "vw"}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, unit, err := CSSValueToKFX(tt.input)
			if tt.shouldError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if unit != tt.expectedUnit {
					t.Errorf("expected unit %d, got %d", tt.expectedUnit, unit)
				}
			}
		})
	}
}

func TestPercentUnit(t *testing.T) {
	// 150% should stay as 150 percent (not converted to ratio)
	val := css.CSSValue{Value: 150, Unit: "%"}
	value, unit, err := CSSValueToKFX(val)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unit != SymUnitPercent {
		t.Errorf("expected percent unit ($314), got %d", unit)
	}
	if value != 150 {
		t.Errorf("expected value 150, got %f", value)
	}
}

func TestConverterConvertRule(t *testing.T) {
	log := zap.NewNop()
	conv := NewConverter(log)

	tests := []struct {
		name          string
		rule          css.CSSRule
		expectedProps map[KFXSymbol]any
		hasWarnings   bool
	}{
		{
			name: "font-weight bold",
			rule: css.CSSRule{
				Selector:   css.Selector{Raw: ".strong", Class: "strong"},
				Properties: map[string]css.CSSValue{"font-weight": {Keyword: "bold"}},
			},
			expectedProps: map[KFXSymbol]any{SymFontWeight: SymBold},
			hasWarnings:   false,
		},
		{
			name: "font-style italic",
			rule: css.CSSRule{
				Selector:   css.Selector{Raw: ".emphasis", Class: "emphasis"},
				Properties: map[string]css.CSSValue{"font-style": {Keyword: "italic"}},
			},
			expectedProps: map[KFXSymbol]any{SymFontStyle: SymItalic},
			hasWarnings:   false,
		},
		{
			name: "text-align center",
			rule: css.CSSRule{
				Selector:   css.Selector{Raw: ".centered", Class: "centered"},
				Properties: map[string]css.CSSValue{"text-align": {Keyword: "center"}},
			},
			expectedProps: map[KFXSymbol]any{SymTextAlignment: SymCenter},
			hasWarnings:   false,
		},
		{
			name: "margin shorthand 4 values",
			rule: css.CSSRule{
				Selector:   css.Selector{Raw: ".box", Class: "box"},
				Properties: map[string]css.CSSValue{"margin": {Raw: "1em 2em 3em 4em"}},
			},
			expectedProps: map[KFXSymbol]any{
				SymMarginTop:    StructValue{},
				SymMarginRight:  StructValue{},
				SymMarginBottom: StructValue{},
				SymMarginLeft:   StructValue{},
			},
			hasWarnings: false,
		},
		{
			name: "clear both",
			rule: css.CSSRule{
				Selector:   css.Selector{Raw: ".clear", Class: "clear"},
				Properties: map[string]css.CSSValue{"clear": {Keyword: "both"}},
			},
			expectedProps: map[KFXSymbol]any{SymFloatClear: SymBoth},
			hasWarnings:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conv.ConvertRule(tt.rule)

			// Check that expected properties exist
			for expectedKey := range tt.expectedProps {
				if _, ok := result.Style.Properties[expectedKey]; !ok {
					t.Errorf("missing expected property %d", expectedKey)
				}
			}

			// Check warnings
			if tt.hasWarnings && len(result.Warnings) == 0 {
				t.Error("expected warnings, got none")
			}
			if !tt.hasWarnings && len(result.Warnings) > 0 {
				t.Errorf("unexpected warnings: %v", result.Warnings)
			}
		})
	}
}

func TestBreakAliasConversion(t *testing.T) {
	log := zap.NewNop()
	conv := NewConverter(log)

	rule := css.CSSRule{
		Selector: css.Selector{Raw: ".break", Class: "break"},
		Properties: map[string]css.CSSValue{
			"break-before": {Keyword: "avoid-page"},
			"break-after":  {Keyword: "avoid"},
			"break-inside": {Keyword: "avoid"},
		},
	}

	result := conv.ConvertRule(rule)

	checkSym := func(prop KFXSymbol, expected KFXSymbol) {
		val, ok := result.Style.Properties[prop]
		if !ok {
			t.Fatalf("missing property %d", prop)
		}
		switch v := val.(type) {
		case KFXSymbol:
			if v != expected {
				t.Fatalf("property %d expected %d got %d", prop, expected, v)
			}
		case SymbolValue:
			if KFXSymbol(v) != expected {
				t.Fatalf("property %d expected %d got %d (SymbolValue)", prop, expected, v)
			}
		default:
			t.Fatalf("property %d unexpected type %T", prop, val)
		}
	}

	checkSym(SymKeepFirst, SymAvoid)
	checkSym(SymKeepLast, SymAvoid)
	checkSym(SymBreakInside, SymAvoid)
}

func TestConverterConvertStylesheet(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)
	conv := NewConverter(log)

	cssData := []byte(`
		.paragraph {
			line-height: 1.2;
			text-indent: 1.5em;
			text-align: justify;
		}
		h1 {
			font-size: 2em;
			font-weight: bold;
			text-align: center;
		}
		.emphasis {
			font-style: italic;
		}
	`)

	sheet := parser.Parse(cssData)
	styles, warnings := conv.ConvertStylesheet(sheet)

	t.Logf("Converted %d styles with %d warnings", len(styles), len(warnings))
	for _, w := range warnings {
		t.Logf("Warning: %s", w)
	}

	// Should have 3 styles
	if len(styles) != 3 {
		t.Errorf("expected 3 styles, got %d", len(styles))
	}

	// Check style names
	styleNames := make(map[string]bool)
	for _, s := range styles {
		styleNames[s.Name] = true
		t.Logf("Style '%s': %d properties", s.Name, len(s.Properties))
	}

	if !styleNames["paragraph"] {
		t.Error("missing 'paragraph' style")
	}
	if !styleNames["h1"] {
		t.Error("missing 'h1' style")
	}
	if !styleNames["emphasis"] {
		t.Error("missing 'emphasis' style")
	}
}

func TestShorthandExpansion(t *testing.T) {
	log := zap.NewNop()
	conv := NewConverter(log)

	tests := []struct {
		name        string
		marginValue string
		expectAll   bool
	}{
		{"single value", "1em", true},
		{"two values", "1em 2em", true},
		{"three values", "1em 2em 3em", true},
		{"four values", "1em 2em 3em 4em", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := css.CSSRule{
				Selector:   css.Selector{Raw: ".test", Class: "test"},
				Properties: map[string]css.CSSValue{"margin": {Raw: tt.marginValue}},
			}
			result := conv.ConvertRule(rule)

			// Check that all margin properties are set
			if tt.expectAll {
				for _, sym := range []KFXSymbol{SymMarginTop, SymMarginRight, SymMarginBottom, SymMarginLeft} {
					if _, ok := result.Style.Properties[sym]; !ok {
						t.Errorf("missing margin property %d", sym)
					}
				}
			}
		})
	}
}

func TestMergeRulesWithSameSelector(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)
	conv := NewConverter(log)

	// Two rules with same selector should be merged
	cssData := []byte(`
		.test {
			font-weight: bold;
		}
		.test {
			font-style: italic;
		}
	`)

	sheet := parser.Parse(cssData)
	styles, _ := conv.ConvertStylesheet(sheet)

	// Should have 1 merged style
	if len(styles) != 1 {
		t.Errorf("expected 1 style, got %d", len(styles))
	}

	if len(styles) > 0 {
		style := styles[0]
		if style.Name != "test" {
			t.Errorf("expected style name 'test', got '%s'", style.Name)
		}

		// Should have both properties
		if _, ok := style.Properties[SymFontWeight]; !ok {
			t.Error("missing font-weight property")
		}
		if _, ok := style.Properties[SymFontStyle]; !ok {
			t.Error("missing font-style property")
		}
	}
}

func TestNewStyleRegistryFromCSS(t *testing.T) {
	log := zap.NewNop()

	cssData := []byte(`
		.paragraph {
			line-height: 1.5;
			text-indent: 2em;
		}
		h1 {
			font-size: 2.5em;
			font-weight: bold;
		}
		.custom-style {
			font-style: italic;
			margin-top: 1em;
		}
	`)

	registry, warnings := parseAndCreateRegistry(cssData, nil, log)

	t.Logf("Warnings: %v", warnings)

	// Should have default styles plus CSS styles
	names := registry.Names()
	t.Logf("Registered styles: %v", names)

	// Check that CSS styles are registered
	if _, ok := registry.Get("paragraph"); !ok {
		t.Error("expected 'paragraph' style to be registered")
	}
	if _, ok := registry.Get("h1"); !ok {
		t.Error("expected 'h1' style to be registered")
	}
	if _, ok := registry.Get("custom-style"); !ok {
		t.Error("expected 'custom-style' style to be registered")
	}

	// Check that CSS values override defaults
	para, _ := registry.Get("paragraph")
	if lineHeight, ok := para.Properties[SymLineHeight]; ok {
		if sv, ok := lineHeight.(StructValue); ok {
			if val, ok := sv[SymValue].(float64); ok {
				if val != 1.5 {
					t.Errorf("expected paragraph line-height 1.5, got %f", val)
				}
			}
		}
	} else {
		t.Error("paragraph style should have line-height property")
	}

	// Check default HTML element styles are present
	if _, ok := registry.Get("strong"); !ok {
		t.Error("expected default 'strong' style to be preserved")
	}
	if _, ok := registry.Get("p"); !ok {
		t.Error("expected default 'p' style to be preserved")
	}
	// Note: wrapper styles like 'epigraph' come from default.css only (not seeded by Go code)
}

func TestNewStyleRegistryFromCSS_Empty(t *testing.T) {
	log := zap.NewNop()

	registry, warnings := parseAndCreateRegistry(nil, nil, log)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty CSS, got %v", warnings)
	}

	// Should have all default HTML element styles
	if _, ok := registry.Get("p"); !ok {
		t.Error("expected default 'p' style")
	}
	if _, ok := registry.Get("h1"); !ok {
		t.Error("expected default 'h1' style")
	}
	// Note: wrapper styles like 'epigraph' come from default.css only (not seeded by Go code)
}

func TestFontSizeKeywords(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)
	conv := NewConverter(log)

	tests := []struct {
		name     string
		cssText  string
		expected float64 // Expected em value
	}{
		{
			name:     "smaller keyword",
			cssText:  `.test { font-size: smaller; }`,
			expected: 0.833, // Amazon's 5/6 value (rounded to 3 decimals)
		},
		{
			name:     "larger keyword",
			cssText:  `.test { font-size: larger; }`,
			expected: 1.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheet := parser.Parse([]byte(tt.cssText))
			styles, _ := conv.ConvertStylesheet(sheet)

			if len(styles) != 1 {
				t.Fatalf("expected 1 style, got %d", len(styles))
			}

			style := styles[0]
			fontSize, ok := style.Properties[SymFontSize]
			if !ok {
				t.Fatal("expected font-size property")
			}

			sv, ok := fontSize.(StructValue)
			if !ok {
				t.Fatalf("expected StructValue, got %T", fontSize)
			}

			// Value is stored as *ion.Decimal, convert to float64 for comparison
			val := getStructValueAsFloat64(sv, SymValue)
			if val < 0 {
				t.Fatalf("failed to get font-size value from %v", sv)
			}

			// Compare with tolerance due to decimal precision
			if diff := val - tt.expected; diff < -0.001 || diff > 0.001 {
				t.Errorf("expected font-size value ~%f, got %f", tt.expected, val)
			}

			// Unit is stored as SymbolValue (SetSymbol wraps it)
			unit, ok := sv[SymUnit].(SymbolValue)
			if !ok {
				t.Fatalf("expected unit to be SymbolValue, got %T", sv[SymUnit])
			}

			// Should be em unit ($308)
			if KFXSymbol(unit) != SymUnitEm {
				t.Errorf("expected em unit ($308), got %v", unit)
			}
		})
	}
}

// getStructValueAsFloat64 extracts a float64 from a StructValue's SymValue field.
// Returns -1 if extraction fails.
func getStructValueAsFloat64(sv StructValue, sym KFXSymbol) float64 {
	rawVal, ok := sv[sym]
	if !ok {
		return -1
	}
	switch v := rawVal.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case *ion.Decimal:
		return decimalToFloat64Test(v)
	default:
		return -1
	}
}

// decimalToFloat64Test converts ion.Decimal to float64 for testing.
func decimalToFloat64Test(d *ion.Decimal) float64 {
	if d == nil {
		return 0
	}
	coeff, exp := d.CoEx()
	bf := new(big.Float).SetInt(coeff)
	if exp != 0 {
		pow := new(big.Float).SetFloat64(math.Pow10(int(exp)))
		bf.Mul(bf, pow)
	}
	f, _ := bf.Float64()
	return f
}

func TestWhiteSpaceProperty(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)
	conv := NewConverter(log)

	tests := []struct {
		name        string
		cssText     string
		expectProp  bool   // Whether white_space property should be set
		expectValue string // Expected value if set
	}{
		{
			name:        "nowrap sets white_space",
			cssText:     `.test { white-space: nowrap; }`,
			expectProp:  true,
			expectValue: "nowrap",
		},
		{
			name:       "normal does not set white_space",
			cssText:    `.test { white-space: normal; font-weight: bold; }`,
			expectProp: false,
		},
		{
			name:       "pre does not set white_space (handled at content level)",
			cssText:    `.test { white-space: pre; font-weight: bold; }`,
			expectProp: false,
		},
		{
			name:       "pre-wrap does not set white_space",
			cssText:    `.test { white-space: pre-wrap; font-weight: bold; }`,
			expectProp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheet := parser.Parse([]byte(tt.cssText))
			styles, _ := conv.ConvertStylesheet(sheet)

			if len(styles) != 1 {
				t.Fatalf("expected 1 style, got %d", len(styles))
			}

			style := styles[0]
			whiteSpace, hasProp := style.Properties[SymWhiteSpace]

			if tt.expectProp {
				if !hasProp {
					t.Fatal("expected white_space property to be set")
				}
				// Check it's nowrap symbol
				if sv, ok := whiteSpace.(SymbolValue); ok {
					if KFXSymbol(sv) != SymNowrap {
						t.Errorf("expected nowrap symbol, got %v", sv)
					}
				} else {
					t.Errorf("expected SymbolValue, got %T", whiteSpace)
				}
			} else {
				if hasProp {
					t.Errorf("expected white_space property to NOT be set, but got %v", whiteSpace)
				}
			}
		})
	}
}

func TestStyleRegistryBuildFragments(t *testing.T) {
	log := zap.NewNop()

	cssData := []byte(`
		.paragraph { line-height: 1.2; }
		.custom { font-weight: bold; }
	`)

	registry, _ := parseAndCreateRegistry(cssData, nil, log)

	// Use StyleContext.Resolve to get resolved style names (base36 format)
	// This is how styles are typically used in actual code
	name1 := NewStyleContext(registry).Resolve("", "paragraph")
	name2 := NewStyleContext(registry).Resolve("", "custom")
	name3 := NewStyleContext(registry).Resolve("", "strong") // default HTML element style

	// ResolveStyle tracks usage type but doesn't mark as "used" for output.
	// We need to simulate content that references these styles.
	registry.ResolveStyle(name1, styleUsageText)
	registry.ResolveStyle(name2, styleUsageText)
	registry.ResolveStyle(name3, styleUsageText)

	// Create mock content fragments that reference these styles
	contentFragments := NewFragmentList()
	contentList := []any{
		NewStruct().
			SetInt(SymUniqueID, 1000).
			Set(SymStyle, SymbolByName(name1)),
		NewStruct().
			SetInt(SymUniqueID, 1001).
			Set(SymStyle, SymbolByName(name2)),
		NewStruct().
			SetInt(SymUniqueID, 1002).
			Set(SymStyle, SymbolByName(name3)),
	}
	storyline := &Fragment{
		FType:   SymStoryline,
		FIDName: "test",
		Value: StructValue{
			SymContentList: contentList, // $146 = content_list
		},
	}
	contentFragments.Add(storyline)

	// Recompute which styles are actually used
	registry.RecomputeUsedStyles(contentFragments)

	fragments := registry.BuildFragments()

	// Note: "custom" and "strong" both have font-weight: bold and may deduplicate
	// to the same resolved style. So we expect 2 fragments, not 3.
	// This is correct behavior - style deduplication.
	if len(fragments) < 2 {
		t.Errorf("expected at least 2 fragments, got %d", len(fragments))
	}

	// Check fragment types and names
	names := make(map[string]bool)
	for _, frag := range fragments {
		if frag.FType != SymStyle {
			t.Errorf("expected fragment type $157 (style), got %d", frag.FType)
		}
		names[frag.FIDName] = true
		t.Logf("Fragment: %s", frag.FIDName)
	}

	// Verify the resolved names are in the fragments
	if !names[name1] {
		t.Errorf("expected fragment %s not found", name1)
	}
	// name2 and name3 may be the same due to deduplication
	if !names[name2] && !names[name3] {
		t.Errorf("expected at least one of %s or %s", name2, name3)
	}
}

func TestConvertHyphens(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"none", css.CSSValue{Keyword: "none"}, SymNone, true},
		{"auto", css.CSSValue{Keyword: "auto"}, SymAuto, true},
		{"manual", css.CSSValue{Keyword: "manual"}, SymManual, true},
		{"None (uppercase)", css.CSSValue{Keyword: "None"}, SymNone, true},
		{"AUTO (uppercase)", css.CSSValue{Keyword: "AUTO"}, SymAuto, true},
		{"unknown", css.CSSValue{Keyword: "unknown"}, 0, false},
		{"enabled", css.CSSValue{Keyword: "enabled"}, 0, false},
		{"empty", css.CSSValue{Keyword: ""}, 0, false},
		{"invalid", css.CSSValue{Keyword: "invalid"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertHyphens(tt.input)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got ok=%v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestHyphensProperty(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)
	conv := NewConverter(log)

	tests := []struct {
		name        string
		cssText     string
		expectProp  bool
		expectValue KFXSymbol
	}{
		{
			name:        "hyphens auto",
			cssText:     `.test { hyphens: auto; }`,
			expectProp:  true,
			expectValue: SymAuto,
		},
		{
			name:        "hyphens none",
			cssText:     `.test { hyphens: none; }`,
			expectProp:  true,
			expectValue: SymNone,
		},
		{
			name:        "hyphens manual",
			cssText:     `.test { hyphens: manual; }`,
			expectProp:  true,
			expectValue: SymManual,
		},
		{
			name:        "-webkit-hyphens auto",
			cssText:     `.test { -webkit-hyphens: auto; }`,
			expectProp:  true,
			expectValue: SymAuto,
		},
		{
			name:       "hyphens enabled (KFX-specific, ignored)",
			cssText:    `.test { hyphens: enabled; font-weight: bold; }`,
			expectProp: false,
		},
		{
			name:       "hyphens unknown (KFX-specific, ignored)",
			cssText:    `.test { hyphens: unknown; font-weight: bold; }`,
			expectProp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheet := parser.Parse([]byte(tt.cssText))
			styles, _ := conv.ConvertStylesheet(sheet)

			if len(styles) != 1 {
				t.Fatalf("expected 1 style, got %d", len(styles))
			}

			style := styles[0]
			hyphens, hasProp := style.Properties[SymHyphens]

			if tt.expectProp {
				if !hasProp {
					t.Fatal("expected hyphens property to be set")
				}
				if sym, ok := hyphens.(KFXSymbol); ok {
					if sym != tt.expectValue {
						t.Errorf("expected %v, got %v", tt.expectValue, sym)
					}
				} else {
					t.Errorf("expected KFXSymbol, got %T: %v", hyphens, hyphens)
				}
			} else {
				if hasProp {
					t.Errorf("expected hyphens property to NOT be set, but got %v", hyphens)
				}
			}
		})
	}
}

func TestExToEmConversion(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)
	conv := NewConverter(log)

	// CSS with ex units should be converted to em using ExToEmFactor (0.44)
	cssData := []byte(`
		.test {
			text-indent: 2ex;
			margin-top: 1ex;
			font-size: 3ex;
		}
	`)

	sheet := parser.Parse(cssData)
	styles, warnings := conv.ConvertStylesheet(sheet)

	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}

	style := styles[0]

	// text-indent: 2ex → 0.88em → % (0.88 * EmToPercentTextIndent = 2.75%)
	if ti, ok := style.Properties[SymTextIndent]; ok {
		sv, ok := ti.(StructValue)
		if !ok {
			t.Fatalf("expected StructValue for text-indent, got %T", ti)
		}
		val := getStructValueAsFloat64(sv, SymValue)
		expected := 2 * ExToEmFactor * EmToPercentTextIndent // 2 * 0.44 * 3.125 = 2.75
		if math.Abs(val-expected) > 0.001 {
			t.Errorf("text-indent: expected value ~%f%%, got %f", expected, val)
		}
	} else {
		t.Error("text-indent property not set")
	}

	// margin-top: 1ex → 0.44em → lh (0.44 / 1.2 ≈ 0.366667)
	if mt, ok := style.Properties[SymMarginTop]; ok {
		sv, ok := mt.(StructValue)
		if !ok {
			t.Fatalf("expected StructValue for margin-top, got %T", mt)
		}
		val := getStructValueAsFloat64(sv, SymValue)
		expected := ExToEmFactor / LineHeightRatio // 0.44 / 1.2 ≈ 0.3667
		if math.Abs(val-expected) > 0.001 {
			t.Errorf("margin-top: expected value ~%f lh, got %f", expected, val)
		}
	} else {
		t.Error("margin-top property not set")
	}
}

func TestExToEmNormalization(t *testing.T) {
	// Test the normalizeCSSProperties function directly
	props := map[string]css.CSSValue{
		"text-indent": {Value: 2, Unit: "ex", Raw: "2ex"},
		"font-weight": {Keyword: "bold"},
		"margin-left": {Value: 0.5, Unit: "ex", Raw: "0.5ex"},
	}

	normalized := normalizeCSSProperties(props, "", nil, "")

	// text-indent should now be in em
	if ti, ok := normalized["text-indent"]; ok {
		if ti.Unit != "em" {
			t.Errorf("expected unit 'em', got '%s'", ti.Unit)
		}
		expected := 2.0 * ExToEmFactor // 0.88
		if math.Abs(ti.Value-expected) > 1e-9 {
			t.Errorf("expected value %f, got %f", expected, ti.Value)
		}
	} else {
		t.Error("text-indent not found in normalized props")
	}

	// font-weight should be unchanged (keyword, not ex unit)
	if fw, ok := normalized["font-weight"]; ok {
		if fw.Keyword != "bold" {
			t.Errorf("expected keyword 'bold', got '%s'", fw.Keyword)
		}
	} else {
		t.Error("font-weight not found in normalized props")
	}

	// margin-left should now be in em
	if ml, ok := normalized["margin-left"]; ok {
		if ml.Unit != "em" {
			t.Errorf("expected unit 'em', got '%s'", ml.Unit)
		}
		expected := 0.5 * ExToEmFactor // 0.22
		if math.Abs(ml.Value-expected) > 1e-9 {
			t.Errorf("expected value %f, got %f", expected, ml.Value)
		}
	} else {
		t.Error("margin-left not found in normalized props")
	}
}

func TestNegativeMarginWarning(t *testing.T) {
	log := zap.NewNop()
	parser := css.NewParser(log)
	conv := NewConverter(log)

	// CSS with negative margins (not supported in KFX)
	cssData := []byte(`
		.test {
			margin-left: -8pt;
			margin-right: -8pt;
			margin-top: -1em;
			margin-bottom: 1em;
		}
	`)

	sheet := parser.Parse(cssData)
	styles, warnings := conv.ConvertStylesheet(sheet)

	// Should have 3 warnings for the 3 negative margins
	if len(warnings) != 3 {
		t.Errorf("expected 3 warnings for negative margins, got %d: %v", len(warnings), warnings)
	}

	// Check warnings mention "negative margin"
	for _, w := range warnings {
		if !strings.Contains(w, "negative margin") {
			t.Errorf("expected warning about negative margin, got: %s", w)
		}
	}

	// Should have 1 style
	if len(styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(styles))
	}

	style := styles[0]

	// The negative margins should NOT be in the properties
	if _, ok := style.Properties[SymMarginLeft]; ok {
		t.Error("negative margin-left should NOT be set")
	}
	if _, ok := style.Properties[SymMarginRight]; ok {
		t.Error("negative margin-right should NOT be set")
	}
	if _, ok := style.Properties[SymMarginTop]; ok {
		t.Error("negative margin-top should NOT be set")
	}

	// The positive margin-bottom SHOULD be set
	if _, ok := style.Properties[SymMarginBottom]; !ok {
		t.Error("positive margin-bottom SHOULD be set")
	}
}

// TestMakeBorderRadiusValue tests the MakeBorderRadiusValue function directly.
// KP3 reference: BorderRadiusTransformer.java
func TestMakeBorderRadiusValue(t *testing.T) {
	tests := []struct {
		name     string
		cssVal   css.CSSValue
		rawVal   string
		unit     string
		wantOK   bool
		wantList bool // true if result should be a ListValue (two different radii)
	}{
		{
			name:   "single value from CSSValue",
			cssVal: css.CSSValue{Value: 10, Unit: "px", Raw: "10px"},
			wantOK: true,
		},
		{
			name:   "single value from rawVal",
			cssVal: css.CSSValue{},
			rawVal: "10px",
			unit:   "px",
			wantOK: true,
		},
		{
			name:   "two identical values - single dimension",
			cssVal: css.CSSValue{Raw: "10px 10px"},
			unit:   "px",
			wantOK: true,
		},
		{
			name:     "two different values - list of two dimensions",
			cssVal:   css.CSSValue{Raw: "10px 20px"},
			unit:     "px",
			wantOK:   true,
			wantList: true,
		},
		{
			name:     "two different units - list of two dimensions",
			cssVal:   css.CSSValue{Raw: "10px 50%"},
			unit:     "px",
			wantOK:   true,
			wantList: true,
		},
		{
			name:     "two values from rawVal",
			cssVal:   css.CSSValue{},
			rawVal:   "5em 10em",
			unit:     "em",
			wantOK:   true,
			wantList: true,
		},
		{
			name:   "two identical em values - single dimension",
			cssVal: css.CSSValue{Raw: "5em 5em"},
			unit:   "em",
			wantOK: true,
		},
		{
			name:   "empty value",
			cssVal: css.CSSValue{},
			wantOK: false,
		},
		{
			name:   "three values - rejected by KP3",
			cssVal: css.CSSValue{Raw: "10px 20px 30px"},
			unit:   "px",
			wantOK: false,
		},
		{
			name:   "single zero value",
			cssVal: css.CSSValue{Value: 0, Unit: "px", Raw: "0px"},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := MakeBorderRadiusValue(tt.cssVal, tt.rawVal, tt.unit)
			if ok != tt.wantOK {
				t.Fatalf("MakeBorderRadiusValue() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}

			if tt.wantList {
				// Should be a ListValue with two DimensionValue items
				lv, isList := val.(ListValue)
				if !isList {
					t.Fatalf("expected ListValue, got %T", val)
				}
				if len(lv) != 2 {
					t.Fatalf("expected list of 2 items, got %d", len(lv))
				}
				// Each item should be a StructValue (dimension)
				for i, item := range lv {
					sv, isSV := item.(StructValue)
					if !isSV {
						t.Errorf("list item[%d]: expected StructValue, got %T", i, item)
						continue
					}
					if _, ok := sv[SymValue]; !ok {
						t.Errorf("list item[%d]: missing $307 (value)", i)
					}
					if _, ok := sv[SymUnit]; !ok {
						t.Errorf("list item[%d]: missing $306 (unit)", i)
					}
				}
			} else {
				// Should be a single StructValue (dimension)
				sv, isSV := val.(StructValue)
				if !isSV {
					t.Fatalf("expected StructValue, got %T", val)
				}
				if _, ok := sv[SymValue]; !ok {
					t.Error("missing $307 (value)")
				}
				if _, ok := sv[SymUnit]; !ok {
					t.Error("missing $306 (unit)")
				}
			}
		})
	}
}

// TestBorderRadiusTwoValueDimensions verifies the actual numeric values in two-value output.
func TestBorderRadiusTwoValueDimensions(t *testing.T) {
	// "10px 20px" → list of [{value:10, unit:px}, {value:20, unit:px}]
	val, ok := MakeBorderRadiusValue(css.CSSValue{Raw: "10px 20px"}, "", "px")
	if !ok {
		t.Fatal("expected ok=true")
	}

	lv, isList := val.(ListValue)
	if !isList {
		t.Fatalf("expected ListValue, got %T", val)
	}
	if len(lv) != 2 {
		t.Fatalf("expected 2 items, got %d", len(lv))
	}

	// First dimension: 10px
	dim1 := lv[0].(StructValue)
	v1 := getStructValueAsFloat64(dim1, SymValue)
	if math.Abs(v1-10.0) > 0.001 {
		t.Errorf("first radius: expected 10, got %f", v1)
	}
	if u1 := dim1[SymUnit]; u1 != SymbolValue(SymUnitPx) {
		t.Errorf("first radius unit: expected SymUnitPx, got %v", u1)
	}

	// Second dimension: 20px
	dim2 := lv[1].(StructValue)
	v2 := getStructValueAsFloat64(dim2, SymValue)
	if math.Abs(v2-20.0) > 0.001 {
		t.Errorf("second radius: expected 20, got %f", v2)
	}
	if u2 := dim2[SymUnit]; u2 != SymbolValue(SymUnitPx) {
		t.Errorf("second radius unit: expected SymUnitPx, got %v", u2)
	}
}

// TestBorderRadiusMixedUnits verifies two-value output with different units.
func TestBorderRadiusMixedUnits(t *testing.T) {
	// "10px 50%" → list of [{value:10, unit:px}, {value:50, unit:percent}]
	val, ok := MakeBorderRadiusValue(css.CSSValue{Raw: "10px 50%"}, "", "px")
	if !ok {
		t.Fatal("expected ok=true")
	}

	lv, isList := val.(ListValue)
	if !isList {
		t.Fatalf("expected ListValue, got %T", val)
	}
	if len(lv) != 2 {
		t.Fatalf("expected 2 items, got %d", len(lv))
	}

	// First: 10px
	dim1 := lv[0].(StructValue)
	v1 := getStructValueAsFloat64(dim1, SymValue)
	if math.Abs(v1-10.0) > 0.001 {
		t.Errorf("first radius: expected 10, got %f", v1)
	}
	if u1 := dim1[SymUnit]; u1 != SymbolValue(SymUnitPx) {
		t.Errorf("first radius unit: expected SymUnitPx, got %v", u1)
	}

	// Second: 50%
	dim2 := lv[1].(StructValue)
	v2 := getStructValueAsFloat64(dim2, SymValue)
	if math.Abs(v2-50.0) > 0.001 {
		t.Errorf("second radius: expected 50, got %f", v2)
	}
	if u2 := dim2[SymUnit]; u2 != SymbolValue(SymUnitPercent) {
		t.Errorf("second radius unit: expected SymUnitPercent, got %v", u2)
	}
}

// TestBorderRadiusViaConvertStyleMapProp tests the integration with convertStyleMapProp.
func TestBorderRadiusViaConvertStyleMapProp(t *testing.T) {
	log := zap.NewNop()

	tests := []struct {
		name     string
		prop     string
		cssVal   css.CSSValue
		rawVal   string
		unit     string
		wantOK   bool
		wantList bool
	}{
		{
			name:   "top-left single value",
			prop:   "border_radius_top_left",
			cssVal: css.CSSValue{Value: 10, Unit: "px", Raw: "10px"},
			unit:   "px",
			wantOK: true,
		},
		{
			name:     "top-right two different values",
			prop:     "border_radius_top_right",
			cssVal:   css.CSSValue{Raw: "10px 20px"},
			unit:     "px",
			wantOK:   true,
			wantList: true,
		},
		{
			name:   "bottom-left two identical values",
			prop:   "border_radius_bottom_left",
			cssVal: css.CSSValue{Raw: "5em 5em"},
			unit:   "em",
			wantOK: true,
		},
		{
			name:     "bottom-right mixed units",
			prop:     "border_radius_bottom_right",
			cssVal:   css.CSSValue{Raw: "10px 50%"},
			unit:     "px",
			wantOK:   true,
			wantList: true,
		},
		{
			name:   "generic border_radius single value",
			prop:   "border_radius",
			cssVal: css.CSSValue{Value: 8, Unit: "pt", Raw: "8pt"},
			unit:   "pt",
			wantOK: true,
		},
		{
			name:   "empty value",
			prop:   "border_radius_top_left",
			cssVal: css.CSSValue{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := convertStyleMapProp(tt.prop, tt.cssVal, tt.rawVal, tt.unit, "measure", "", log)
			if ok != tt.wantOK {
				t.Fatalf("convertStyleMapProp() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}

			sym := symbolIDOr(tt.prop)
			val, exists := result[sym]
			if !exists {
				t.Fatalf("expected property %s (sym=%d) in result", tt.prop, sym)
			}

			if tt.wantList {
				if _, isList := val.(ListValue); !isList {
					t.Errorf("expected ListValue for two-value radius, got %T", val)
				}
			} else {
				if _, isSV := val.(StructValue); !isSV {
					t.Errorf("expected StructValue for single-value radius, got %T", val)
				}
			}
		})
	}
}
