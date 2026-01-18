package kfx

import (
	"math"
	"testing"
)

func TestRoundKPVDecimal(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		// Basic rounding cases from docstring examples
		{"repeating_decimal", 63.33333333333333, 63.333},
		{"already_precise", 0.25, 0.25},
		{"round_up", 84.765625, 84.766},

		// Edge cases
		{"zero", 0, 0},
		{"negative", -1.2345, -1.235},
		{"whole_number", 100.0, 100.0},
		{"exactly_3_places", 1.234, 1.234},

		// Half-up rounding behavior (matches Java RoundingMode.HALF_UP)
		{"half_up_5", 1.2345, 1.235},     // .5 rounds up
		{"half_up_below", 1.2344, 1.234}, // below .5 rounds down
		{"half_up_above", 1.2346, 1.235}, // above .5 rounds up

		// Real-world values from image sizing
		{"image_width_percent", 19.047619047619, 19.048},
		{"image_height_em", 1.666666666667, 1.667},
		{"margin_lh", 0.416666666667, 0.417},

		// Very small values
		{"tiny_value", 0.0001, 0.0},
		{"small_value", 0.001, 0.001},
		{"small_round_up", 0.0005, 0.001},
		{"small_round_down", 0.0004, 0.0},

		// Large values
		{"large_value", 1000.123456, 1000.123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundKPVDecimal(tt.input)
			// Use epsilon comparison for floating point
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("RoundKPVDecimal(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestKPVDecimalPrecisionConstant(t *testing.T) {
	// Verify the constant matches Amazon's setScale(3)
	if KPVDecimalPrecision != 3 {
		t.Errorf("KPVDecimalPrecision = %d, want 3 (to match Amazon's setScale(3))", KPVDecimalPrecision)
	}
}

func TestIsVerticalSpacingProperty(t *testing.T) {
	verticalProps := []KFXSymbol{SymMarginTop, SymMarginBottom, SymPaddingTop, SymPaddingBottom}
	horizontalProps := []KFXSymbol{SymMarginLeft, SymMarginRight, SymPaddingLeft, SymPaddingRight}

	for _, sym := range verticalProps {
		if !isVerticalSpacingProperty(sym) {
			t.Errorf("isVerticalSpacingProperty(%v) = false, want true", sym)
		}
	}

	for _, sym := range horizontalProps {
		if isVerticalSpacingProperty(sym) {
			t.Errorf("isVerticalSpacingProperty(%v) = true, want false", sym)
		}
	}
}

func TestIsHorizontalSpacingProperty(t *testing.T) {
	horizontalProps := []KFXSymbol{SymMarginLeft, SymMarginRight, SymPaddingLeft, SymPaddingRight}
	verticalProps := []KFXSymbol{SymMarginTop, SymMarginBottom, SymPaddingTop, SymPaddingBottom}

	for _, sym := range horizontalProps {
		if !isHorizontalSpacingProperty(sym) {
			t.Errorf("isHorizontalSpacingProperty(%v) = false, want true", sym)
		}
	}

	for _, sym := range verticalProps {
		if isHorizontalSpacingProperty(sym) {
			t.Errorf("isHorizontalSpacingProperty(%v) = true, want false", sym)
		}
	}
}

func TestKPVUnitConversionConstants(t *testing.T) {
	// Verify constants have expected values
	tests := []struct {
		name     string
		value    float64
		expected float64
	}{
		{"KPVDefaultLineHeightLh", KPVDefaultLineHeightLh, 1.0},
		{"KPVLineHeightRatio", KPVLineHeightRatio, 1.2},
		{"KPVEmToPercentHorizontal", KPVEmToPercentHorizontal, 3.125},
		{"KPVEmToPercentTextIndent", KPVEmToPercentTextIndent, 3.125},
		{"KPVPercentToRem", KPVPercentToRem, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.value, tt.expected)
			}
		})
	}
}

// TestKPVUnitConversions tests that common CSS-to-KFX unit conversions work correctly
func TestKPVUnitConversions(t *testing.T) {
	t.Run("em_to_lh_vertical", func(t *testing.T) {
		// 0.3em CSS → 0.25lh KFX (0.3 / 1.2 = 0.25)
		emValue := 0.3
		lhValue := emValue / KPVLineHeightRatio
		expected := 0.25
		if math.Abs(lhValue-expected) > 1e-9 {
			t.Errorf("em to lh: %v / %v = %v, want %v", emValue, KPVLineHeightRatio, lhValue, expected)
		}
	})

	t.Run("em_to_percent_horizontal", func(t *testing.T) {
		// 1em CSS → 3.125% KFX
		emValue := 1.0
		percentValue := emValue * KPVEmToPercentHorizontal
		expected := 3.125
		if math.Abs(percentValue-expected) > 1e-9 {
			t.Errorf("em to %%: %v * %v = %v, want %v", emValue, KPVEmToPercentHorizontal, percentValue, expected)
		}
	})

	t.Run("percent_to_rem_font_size", func(t *testing.T) {
		// 140% CSS → 1.4rem KFX
		percentValue := 140.0
		remValue := percentValue / KPVPercentToRem
		expected := 1.4
		if math.Abs(remValue-expected) > 1e-9 {
			t.Errorf("percent to rem: %v / %v = %v, want %v", percentValue, KPVPercentToRem, remValue, expected)
		}
	})
}
