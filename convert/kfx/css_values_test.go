package kfx

import (
	"fbc/css"
	"testing"
)

func TestConvertWritingMode(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"horizontal-tb", css.CSSValue{Keyword: "horizontal-tb"}, SymHorizontalTb, true},
		{"horizontal_tb underscore", css.CSSValue{Keyword: "horizontal_tb"}, SymHorizontalTb, true},
		{"vertical-rl", css.CSSValue{Keyword: "vertical-rl"}, SymVerticalRl, true},
		{"vertical-lr", css.CSSValue{Keyword: "vertical-lr"}, SymVerticalLr, true},
		{"from raw", css.CSSValue{Raw: "horizontal-tb"}, SymHorizontalTb, true},
		{"unknown", css.CSSValue{Keyword: "diagonal"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertWritingMode(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertTextCombine(t *testing.T) {
	tests := []struct {
		name  string
		input css.CSSValue
		ok    bool
	}{
		{"empty", css.CSSValue{}, false},
		{"from raw", css.CSSValue{Raw: "all"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := ConvertTextCombine(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
		})
	}
}

func TestConvertTextOrientation(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"mixed", css.CSSValue{Keyword: "mixed"}, SymAuto, true},
		{"upright", css.CSSValue{Keyword: "upright"}, SymUpright, true},
		{"sideways", css.CSSValue{Keyword: "sideways"}, SymSideways, true},
		{"sideways-rl", css.CSSValue{Keyword: "sideways-rl"}, SymSideways, true},
		{"unknown", css.CSSValue{Keyword: "unknown"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertTextOrientation(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertTextEmphasisStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"dot", css.CSSValue{Raw: "dot"}, SymFilledDot, true},
		{"filled dot", css.CSSValue{Raw: "filled dot"}, SymFilledDot, true},
		{"open dot", css.CSSValue{Raw: "open dot"}, SymOpenDot, true},
		{"circle", css.CSSValue{Raw: "circle"}, SymFilledCircle, true},
		{"open circle", css.CSSValue{Raw: "open circle"}, SymOpenCircle, true},
		{"double-circle", css.CSSValue{Raw: "double-circle"}, SymFilledDoubleCircle, true},
		{"open double-circle", css.CSSValue{Raw: "open double-circle"}, SymOpenDoubleCircle, true},
		{"triangle", css.CSSValue{Raw: "triangle"}, SymFilledTriangle, true},
		{"open triangle", css.CSSValue{Raw: "open triangle"}, SymOpenTriangle, true},
		{"sesame", css.CSSValue{Raw: "sesame"}, SymFilledSesame, true},
		{"open sesame", css.CSSValue{Raw: "open sesame"}, SymOpenSesame, true},
		{"empty", css.CSSValue{}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertTextEmphasisStyle(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertTextEmphasisPosition(t *testing.T) {
	tests := []struct {
		name      string
		input     css.CSSValue
		wantHoriz KFXSymbol
		wantVert  KFXSymbol
		ok        bool
	}{
		{"over", css.CSSValue{Raw: "over"}, 0, SymTop, true},
		{"under", css.CSSValue{Raw: "under"}, 0, SymBottom, true},
		{"over right", css.CSSValue{Raw: "over right"}, SymRight, SymTop, true},
		{"under left", css.CSSValue{Raw: "under left"}, SymLeft, SymBottom, true},
		{"top", css.CSSValue{Raw: "top"}, 0, SymTop, true},
		{"bottom", css.CSSValue{Raw: "bottom"}, 0, SymBottom, true},
		{"empty", css.CSSValue{}, 0, 0, false},
		{"unknown", css.CSSValue{Raw: "center"}, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			horiz, vert, ok := ConvertTextEmphasisPosition(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok {
				if horiz != tt.wantHoriz {
					t.Errorf("horiz = %d, want %d", horiz, tt.wantHoriz)
				}
				if vert != tt.wantVert {
					t.Errorf("vert = %d, want %d", vert, tt.wantVert)
				}
			}
		})
	}
}

func TestConvertListStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected KFXSymbol
		ok       bool
	}{
		{"disc", "disc", SymListStyleDisc, true},
		{"square", "square", SymListStyleSquare, true},
		{"circle", "circle", SymListStyleCircle, true},
		{"none", "none", SymNone, true},
		{"decimal", "decimal", SymListStyleNumber, true},
		{"numeric", "numeric", SymListStyleNumber, true},
		{"with whitespace", "  disc  ", SymListStyleDisc, true},
		{"unknown", "unknown-style", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertListStyle(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertBorderStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected KFXSymbol
		ok       bool
	}{
		{"solid", "solid", SymSolid, true},
		{"dashed", "dashed", SymDashed, true},
		{"dotted", "dotted", SymDotted, true},
		{"none", "none", SymNone, true},
		{"hidden", "hidden", SymNone, true},
		{"double fallback", "double", SymSolid, true},
		{"groove fallback", "groove", SymSolid, true},
		{"ridge fallback", "ridge", SymSolid, true},
		{"inset fallback", "inset", SymSolid, true},
		{"outset fallback", "outset", SymSolid, true},
		{"unknown", "wavy", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertBorderStyle(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertFloat(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"left", css.CSSValue{Keyword: "left"}, SymLeft, true},
		{"right", css.CSSValue{Keyword: "right"}, SymRight, true},
		{"none", css.CSSValue{Keyword: "none"}, SymNone, true},
		// "center" maps to SymCenter through symbolIDFromString fallback
		{"center", css.CSSValue{Keyword: "center"}, SymCenter, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertFloat(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertClear(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"left", css.CSSValue{Keyword: "left"}, SymLeft, true},
		{"right", css.CSSValue{Keyword: "right"}, SymRight, true},
		{"both", css.CSSValue{Keyword: "both"}, SymBoth, true},
		{"none", css.CSSValue{Keyword: "none"}, SymNone, true},
		{"unknown", css.CSSValue{Keyword: "all"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertClear(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertPageBreak(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"always", css.CSSValue{Keyword: "always"}, SymAlways, true},
		{"avoid", css.CSSValue{Keyword: "avoid"}, SymAvoid, true},
		{"auto", css.CSSValue{Keyword: "auto"}, SymAuto, true},
		{"unknown", css.CSSValue{Keyword: "never"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertPageBreak(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertYjBreak(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"always keyword", css.CSSValue{Keyword: "always"}, SymAlways, true},
		{"always raw", css.CSSValue{Raw: "always"}, SymAlways, true},
		{"avoid", css.CSSValue{Keyword: "avoid"}, SymAvoid, true},
		{"auto", css.CSSValue{Keyword: "auto"}, SymAuto, true},
		{"unknown", css.CSSValue{Keyword: "never"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := convertYjBreak(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestConvertBaselineStyle(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		expected KFXSymbol
		ok       bool
	}{
		{"center", css.CSSValue{Keyword: "center"}, SymCenter, true},
		{"top", css.CSSValue{Keyword: "top"}, SymTop, true},
		{"bottom", css.CSSValue{Keyword: "bottom"}, SymBottom, true},
		{"superscript", css.CSSValue{Keyword: "superscript"}, SymSuperscript, true},
		{"super alias", css.CSSValue{Keyword: "super"}, SymSuperscript, true},
		{"subscript", css.CSSValue{Keyword: "subscript"}, SymSubscript, true},
		{"sub alias", css.CSSValue{Keyword: "sub"}, SymSubscript, true},
		{"from raw", css.CSSValue{Raw: "top"}, SymTop, true},
		{"unknown", css.CSSValue{Keyword: "middle"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertBaselineStyle(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && result != tt.expected {
				t.Errorf("result = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestParseColorExtended(t *testing.T) {
	tests := []struct {
		name     string
		input    css.CSSValue
		r, g, b  int
		shouldOk bool
	}{
		// Already tested: hex colors, rgb function, basic keywords
		// Add more keyword coverage:
		{"gray", css.CSSValue{Raw: "gray"}, 128, 128, 128, true},
		{"grey", css.CSSValue{Raw: "grey"}, 128, 128, 128, true},
		{"silver", css.CSSValue{Raw: "silver"}, 192, 192, 192, true},
		{"maroon", css.CSSValue{Raw: "maroon"}, 128, 0, 0, true},
		{"navy", css.CSSValue{Raw: "navy"}, 0, 0, 128, true},
		{"teal", css.CSSValue{Raw: "teal"}, 0, 128, 128, true},
		{"olive", css.CSSValue{Raw: "olive"}, 128, 128, 0, true},
		{"purple", css.CSSValue{Raw: "purple"}, 128, 0, 128, true},
		{"fuchsia", css.CSSValue{Raw: "fuchsia"}, 255, 0, 255, true},
		{"magenta", css.CSSValue{Raw: "magenta"}, 255, 0, 255, true},
		{"aqua", css.CSSValue{Raw: "aqua"}, 0, 255, 255, true},
		{"cyan", css.CSSValue{Raw: "cyan"}, 0, 255, 255, true},
		{"lime", css.CSSValue{Raw: "lime"}, 0, 255, 0, true},
		{"yellow", css.CSSValue{Raw: "yellow"}, 255, 255, 0, true},
		{"orange", css.CSSValue{Raw: "orange"}, 255, 165, 0, true},
		{"brown", css.CSSValue{Raw: "brown"}, 165, 42, 42, true},
		{"pink", css.CSSValue{Raw: "pink"}, 255, 192, 203, true},
		{"green", css.CSSValue{Raw: "green"}, 0, 128, 0, true},
		// rgba function (alpha ignored)
		{"rgba", css.CSSValue{Raw: "rgba(100, 150, 200, 0.5)"}, 100, 150, 200, true},
		// Invalid hex length
		{"invalid hex 4", css.CSSValue{Raw: "#1234"}, 0, 0, 0, false},
		{"invalid hex 5", css.CSSValue{Raw: "#12345"}, 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, ok := ParseColor(tt.input)
			if ok != tt.shouldOk {
				t.Errorf("ok = %v, want %v", ok, tt.shouldOk)
			}
			if ok {
				if r != tt.r || g != tt.g || b != tt.b {
					t.Errorf("rgb = (%d,%d,%d), want (%d,%d,%d)", r, g, b, tt.r, tt.g, tt.b)
				}
			}
		})
	}
}

func TestMakeColorValue(t *testing.T) {
	tests := []struct {
		name     string
		r, g, b  int
		expected int64
	}{
		{"black", 0, 0, 0, 0xFF000000},
		{"white", 255, 255, 255, 0xFFFFFFFF},
		{"red", 255, 0, 0, 0xFFFF0000},
		{"green", 0, 255, 0, 0xFF00FF00},
		{"blue", 0, 0, 255, 0xFF0000FF},
		{"custom", 100, 150, 200, 0xFF6496C8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MakeColorValue(tt.r, tt.g, tt.b)
			if result != tt.expected {
				t.Errorf("MakeColorValue(%d,%d,%d) = 0x%X, want 0x%X", tt.r, tt.g, tt.b, result, tt.expected)
			}
		})
	}
}

func TestConvertVerticalAlignNumeric(t *testing.T) {
	// Test numeric vertical-align values
	tests := []struct {
		name  string
		input css.CSSValue
		ok    bool
	}{
		{"em value", css.CSSValue{Value: 0.5, Unit: "em"}, true},
		{"percentage", css.CSSValue{Value: 50, Unit: "%"}, true},
		{"px value", css.CSSValue{Value: 5, Unit: "px"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ConvertVerticalAlign(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && !result.UseBaselineShift {
				t.Error("expected UseBaselineShift = true for numeric value")
			}
		})
	}
}
