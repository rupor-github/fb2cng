package kfx

import (
	"testing"

	"fbc/css"
)

func TestConvertWritingMode(t *testing.T) {
	tests := []struct {
		name     string
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"horizontal-tb", css.Value{Keyword: "horizontal-tb"}, SymHorizontalTb, true},
		{"horizontal_tb underscore", css.Value{Keyword: "horizontal_tb"}, SymHorizontalTb, true},
		{"vertical-rl", css.Value{Keyword: "vertical-rl"}, SymVerticalRl, true},
		{"vertical-lr", css.Value{Keyword: "vertical-lr"}, SymVerticalLr, true},
		{"from raw", css.Value{Raw: "horizontal-tb"}, SymHorizontalTb, true},
		{"unknown", css.Value{Keyword: "diagonal"}, 0, false},
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
		input css.Value
		ok    bool
	}{
		{"empty", css.Value{}, false},
		{"from raw", css.Value{Raw: "all"}, true},
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
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"mixed", css.Value{Keyword: "mixed"}, SymAuto, true},
		{"upright", css.Value{Keyword: "upright"}, SymUpright, true},
		{"sideways", css.Value{Keyword: "sideways"}, SymSideways, true},
		{"sideways-rl", css.Value{Keyword: "sideways-rl"}, SymSideways, true},
		{"unknown", css.Value{Keyword: "unknown"}, 0, false},
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
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"dot", css.Value{Raw: "dot"}, SymFilledDot, true},
		{"filled dot", css.Value{Raw: "filled dot"}, SymFilledDot, true},
		{"open dot", css.Value{Raw: "open dot"}, SymOpenDot, true},
		{"circle", css.Value{Raw: "circle"}, SymFilledCircle, true},
		{"open circle", css.Value{Raw: "open circle"}, SymOpenCircle, true},
		{"double-circle", css.Value{Raw: "double-circle"}, SymFilledDoubleCircle, true},
		{"open double-circle", css.Value{Raw: "open double-circle"}, SymOpenDoubleCircle, true},
		{"triangle", css.Value{Raw: "triangle"}, SymFilledTriangle, true},
		{"open triangle", css.Value{Raw: "open triangle"}, SymOpenTriangle, true},
		{"sesame", css.Value{Raw: "sesame"}, SymFilledSesame, true},
		{"open sesame", css.Value{Raw: "open sesame"}, SymOpenSesame, true},
		{"empty", css.Value{}, 0, false},
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
		input     css.Value
		wantHoriz KFXSymbol
		wantVert  KFXSymbol
		ok        bool
	}{
		{"over", css.Value{Raw: "over"}, 0, SymTop, true},
		{"under", css.Value{Raw: "under"}, 0, SymBottom, true},
		{"over right", css.Value{Raw: "over right"}, SymRight, SymTop, true},
		{"under left", css.Value{Raw: "under left"}, SymLeft, SymBottom, true},
		{"top", css.Value{Raw: "top"}, 0, SymTop, true},
		{"bottom", css.Value{Raw: "bottom"}, 0, SymBottom, true},
		{"empty", css.Value{}, 0, 0, false},
		{"unknown", css.Value{Raw: "center"}, 0, 0, false},
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
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"left", css.Value{Keyword: "left"}, SymLeft, true},
		{"right", css.Value{Keyword: "right"}, SymRight, true},
		{"none", css.Value{Keyword: "none"}, SymNone, true},
		// "center" maps to SymCenter through symbolIDFromString fallback
		{"center", css.Value{Keyword: "center"}, SymCenter, true},
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
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"left", css.Value{Keyword: "left"}, SymLeft, true},
		{"right", css.Value{Keyword: "right"}, SymRight, true},
		{"both", css.Value{Keyword: "both"}, SymBoth, true},
		{"none", css.Value{Keyword: "none"}, SymNone, true},
		{"unknown", css.Value{Keyword: "all"}, 0, false},
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
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"always", css.Value{Keyword: "always"}, SymAlways, true},
		{"avoid", css.Value{Keyword: "avoid"}, SymAvoid, true},
		{"auto", css.Value{Keyword: "auto"}, SymAuto, true},
		{"unknown", css.Value{Keyword: "never"}, 0, false},
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
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"always keyword", css.Value{Keyword: "always"}, SymAlways, true},
		{"always raw", css.Value{Raw: "always"}, SymAlways, true},
		{"avoid", css.Value{Keyword: "avoid"}, SymAvoid, true},
		{"auto", css.Value{Keyword: "auto"}, SymAuto, true},
		{"unknown", css.Value{Keyword: "never"}, 0, false},
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
		input    css.Value
		expected KFXSymbol
		ok       bool
	}{
		{"center", css.Value{Keyword: "center"}, SymCenter, true},
		{"top", css.Value{Keyword: "top"}, SymTop, true},
		{"bottom", css.Value{Keyword: "bottom"}, SymBottom, true},
		{"superscript", css.Value{Keyword: "superscript"}, SymSuperscript, true},
		{"super alias", css.Value{Keyword: "super"}, SymSuperscript, true},
		{"subscript", css.Value{Keyword: "subscript"}, SymSubscript, true},
		{"sub alias", css.Value{Keyword: "sub"}, SymSubscript, true},
		{"from raw", css.Value{Raw: "top"}, SymTop, true},
		{"unknown", css.Value{Keyword: "middle"}, 0, false},
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
		input    css.Value
		r, g, b  int
		shouldOk bool
	}{
		// Already tested: hex colors, rgb function, basic keywords
		// Add more keyword coverage:
		{"gray", css.Value{Raw: "gray"}, 128, 128, 128, true},
		{"grey", css.Value{Raw: "grey"}, 128, 128, 128, true},
		{"silver", css.Value{Raw: "silver"}, 192, 192, 192, true},
		{"maroon", css.Value{Raw: "maroon"}, 128, 0, 0, true},
		{"navy", css.Value{Raw: "navy"}, 0, 0, 128, true},
		{"teal", css.Value{Raw: "teal"}, 0, 128, 128, true},
		{"olive", css.Value{Raw: "olive"}, 128, 128, 0, true},
		{"purple", css.Value{Raw: "purple"}, 128, 0, 128, true},
		{"fuchsia", css.Value{Raw: "fuchsia"}, 255, 0, 255, true},
		{"magenta", css.Value{Raw: "magenta"}, 255, 0, 255, true},
		{"aqua", css.Value{Raw: "aqua"}, 0, 255, 255, true},
		{"cyan", css.Value{Raw: "cyan"}, 0, 255, 255, true},
		{"lime", css.Value{Raw: "lime"}, 0, 255, 0, true},
		{"yellow", css.Value{Raw: "yellow"}, 255, 255, 0, true},
		{"orange", css.Value{Raw: "orange"}, 255, 165, 0, true},
		{"brown", css.Value{Raw: "brown"}, 165, 42, 42, true},
		{"pink", css.Value{Raw: "pink"}, 255, 192, 203, true},
		{"green", css.Value{Raw: "green"}, 0, 128, 0, true},
		// rgba function (alpha ignored)
		{"rgba", css.Value{Raw: "rgba(100, 150, 200, 0.5)"}, 100, 150, 200, true},
		// Invalid hex length
		{"invalid hex 4", css.Value{Raw: "#1234"}, 0, 0, 0, false},
		{"invalid hex 5", css.Value{Raw: "#12345"}, 0, 0, 0, false},
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
		input css.Value
		ok    bool
	}{
		{"em value", css.Value{Value: 0.5, Unit: "em"}, true},
		{"percentage", css.Value{Value: 50, Unit: "%"}, true},
		{"px value", css.Value{Value: 5, Unit: "px"}, true},
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
