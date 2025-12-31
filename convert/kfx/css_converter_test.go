package kfx

import (
	"testing"

	"go.uber.org/zap"
)

func TestConvertFontWeight(t *testing.T) {
	tests := []struct {
		name     string
		input    CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"bold keyword", CSSValue{Keyword: "bold"}, SymBold, true},
		{"bolder keyword", CSSValue{Keyword: "bolder"}, SymBold, true},
		{"lighter keyword", CSSValue{Keyword: "lighter"}, SymLight, true},
		{"normal keyword", CSSValue{Keyword: "normal"}, SymNormal, true},
		{"numeric 700", CSSValue{Value: 700}, SymBold, true},
		{"numeric 600", CSSValue{Value: 600}, SymSemibold, true},
		{"numeric 500", CSSValue{Value: 500}, SymMedium, true},
		{"numeric 400", CSSValue{Value: 400}, SymNormal, true},
		{"numeric 300", CSSValue{Value: 300}, SymLight, true},
		{"numeric 200", CSSValue{Value: 200}, SymLight, true},
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
		input    CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"italic", CSSValue{Keyword: "italic"}, SymItalic, true},
		{"oblique", CSSValue{Keyword: "oblique"}, SymItalic, true},
		{"normal", CSSValue{Keyword: "normal"}, SymNormal, true},
		{"unknown", CSSValue{Keyword: "unknown"}, 0, false},
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
		input    CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"left", CSSValue{Keyword: "left"}, SymStart, true},
		{"start", CSSValue{Keyword: "start"}, SymStart, true},
		{"right", CSSValue{Keyword: "right"}, SymEnd, true},
		{"end", CSSValue{Keyword: "end"}, SymEnd, true},
		{"center", CSSValue{Keyword: "center"}, SymCenter, true},
		{"justify", CSSValue{Keyword: "justify"}, SymJustify, true},
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
		input         CSSValue
		underline     bool
		strikethrough bool
		none          bool
	}{
		{"underline", CSSValue{Raw: "underline"}, true, false, false},
		{"line-through", CSSValue{Raw: "line-through"}, false, true, false},
		{"both", CSSValue{Raw: "underline line-through"}, true, true, false},
		{"none", CSSValue{Raw: "none"}, false, false, true},
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
		input    CSSValue
		hasValue bool
	}{
		{"super", CSSValue{Keyword: "super"}, true},
		{"sub", CSSValue{Keyword: "sub"}, true},
		{"baseline", CSSValue{Keyword: "baseline"}, true},
		{"numeric", CSSValue{Value: 0.5, Unit: "em"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertVerticalAlign(tt.input)
			if ok != tt.hasValue {
				t.Errorf("expected ok=%v, got ok=%v", tt.hasValue, ok)
			}
			if ok && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestParseColor(t *testing.T) {
	tests := []struct {
		name     string
		input    CSSValue
		r, g, b  int
		shouldOk bool
	}{
		{"hex 6 digit", CSSValue{Raw: "#ff0000"}, 255, 0, 0, true},
		{"hex 3 digit", CSSValue{Raw: "#f00"}, 255, 0, 0, true},
		{"rgb function", CSSValue{Raw: "rgb(255, 128, 0)"}, 255, 128, 0, true},
		{"keyword black", CSSValue{Raw: "black"}, 0, 0, 0, true},
		{"keyword white", CSSValue{Raw: "white"}, 255, 255, 255, true},
		{"keyword red", CSSValue{Raw: "red"}, 255, 0, 0, true},
		{"invalid", CSSValue{Raw: "invalid-color"}, 0, 0, 0, false},
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
		input        CSSValue
		expectedUnit KFXSymbol
		shouldError  bool
	}{
		{"em unit", CSSValue{Value: 1.5, Unit: "em"}, SymUnitEm, false},
		{"px unit", CSSValue{Value: 16, Unit: "px"}, SymUnitPx, false},
		{"pt unit", CSSValue{Value: 12, Unit: "pt"}, SymUnitPt, false},
		{"percent", CSSValue{Value: 150, Unit: "%"}, SymUnitRatio, false},
		{"unitless", CSSValue{Value: 1.2, Unit: ""}, SymUnitRatio, false},
		{"cm unit", CSSValue{Value: 2.5, Unit: "cm"}, SymUnitCm, false},
		{"unknown unit", CSSValue{Value: 1, Unit: "vw"}, 0, true},
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

func TestPercentToRatio(t *testing.T) {
	// 150% should become 1.5 ratio
	css := CSSValue{Value: 150, Unit: "%"}
	value, unit, err := CSSValueToKFX(css)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unit != SymUnitRatio {
		t.Errorf("expected ratio unit, got %d", unit)
	}
	if value != 1.5 {
		t.Errorf("expected value 1.5, got %f", value)
	}
}

func TestConverterConvertRule(t *testing.T) {
	log := zap.NewNop()
	conv := NewConverter(log)

	tests := []struct {
		name          string
		rule          CSSRule
		expectedProps map[KFXSymbol]any
		hasWarnings   bool
	}{
		{
			name: "font-weight bold",
			rule: CSSRule{
				Selector:   Selector{Raw: ".strong", Class: "strong"},
				Properties: map[string]CSSValue{"font-weight": {Keyword: "bold"}},
			},
			expectedProps: map[KFXSymbol]any{SymFontWeight: SymBold},
			hasWarnings:   false,
		},
		{
			name: "font-style italic",
			rule: CSSRule{
				Selector:   Selector{Raw: ".emphasis", Class: "emphasis"},
				Properties: map[string]CSSValue{"font-style": {Keyword: "italic"}},
			},
			expectedProps: map[KFXSymbol]any{SymFontStyle: SymItalic},
			hasWarnings:   false,
		},
		{
			name: "text-align center",
			rule: CSSRule{
				Selector:   Selector{Raw: ".centered", Class: "centered"},
				Properties: map[string]CSSValue{"text-align": {Keyword: "center"}},
			},
			expectedProps: map[KFXSymbol]any{SymTextAlignment: SymCenter},
			hasWarnings:   false,
		},
		{
			name: "margin shorthand 4 values",
			rule: CSSRule{
				Selector:   Selector{Raw: ".box", Class: "box"},
				Properties: map[string]CSSValue{"margin": {Raw: "1em 2em 3em 4em"}},
			},
			expectedProps: map[KFXSymbol]any{
				SymMarginTop:    StructValue{},
				SymMarginRight:  StructValue{},
				SymMarginBottom: StructValue{},
				SymMarginLeft:   StructValue{},
			},
			hasWarnings: false,
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

func TestConverterConvertStylesheet(t *testing.T) {
	log := zap.NewNop()
	parser := NewParser(log)
	conv := NewConverter(log)

	css := []byte(`
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

	sheet := parser.Parse(css)
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
			rule := CSSRule{
				Selector:   Selector{Raw: ".test", Class: "test"},
				Properties: map[string]CSSValue{"margin": {Raw: tt.marginValue}},
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
	parser := NewParser(log)
	conv := NewConverter(log)

	// Two rules with same selector should be merged
	css := []byte(`
		.test {
			font-weight: bold;
		}
		.test {
			font-style: italic;
		}
	`)

	sheet := parser.Parse(css)
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

	css := []byte(`
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

	registry, warnings := NewStyleRegistryFromCSS(css, log)

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

	// Check default styles are still present
	if _, ok := registry.Get("epigraph"); !ok {
		t.Error("expected default 'epigraph' style to be preserved")
	}
	if _, ok := registry.Get("strong"); !ok {
		t.Error("expected default 'strong' style to be preserved")
	}
}

func TestNewStyleRegistryFromCSS_Empty(t *testing.T) {
	log := zap.NewNop()

	registry, warnings := NewStyleRegistryFromCSS(nil, log)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty CSS, got %v", warnings)
	}

	// Should have all default styles
	if _, ok := registry.Get("paragraph"); !ok {
		t.Error("expected default 'paragraph' style")
	}
	if _, ok := registry.Get("h1"); !ok {
		t.Error("expected default 'h1' style")
	}
}

func TestStyleRegistryBuildFragments(t *testing.T) {
	log := zap.NewNop()

	css := []byte(`
		.paragraph { line-height: 1.2; }
		.custom { font-weight: bold; }
	`)

	registry, _ := NewStyleRegistryFromCSS(css, log)

	// Mark some styles as used
	registry.EnsureStyle("paragraph")
	registry.EnsureStyle("custom")
	registry.EnsureStyle("emphasis") // default style

	fragments := registry.BuildFragments()

	// Should only output used styles
	if len(fragments) != 3 {
		t.Errorf("expected 3 fragments, got %d", len(fragments))
	}

	// Check fragment types
	for _, frag := range fragments {
		if frag.FType != SymStyle {
			t.Errorf("expected fragment type $157 (style), got %d", frag.FType)
		}
		t.Logf("Fragment: %s", frag.FIDName)
	}
}
