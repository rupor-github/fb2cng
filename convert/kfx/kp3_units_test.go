package kfx

import (
	"math"
	"testing"
)

func TestRoundDecimal(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		// Basic rounding cases (5 decimal places)
		{"repeating_decimal", 63.33333333333333, 63.33333},
		{"already_precise", 0.25, 0.25},
		{"round_up", 84.765625, 84.76563},

		// Edge cases
		{"zero", 0, 0},
		{"negative", -1.2345, -1.2345},
		{"whole_number", 100.0, 100.0},
		{"exactly_5_places", 1.23456, 1.23456},

		// Half-up rounding behavior
		{"half_up_5", 1.2345651, 1.23457},    // above .5 rounds up
		{"half_up_below", 1.234564, 1.23456}, // below .5 rounds down
		{"half_up_above", 1.234566, 1.23457}, // above .5 rounds up

		// Real-world values from image sizing
		{"image_width_percent", 19.047619047619, 19.04762},
		{"image_height_em", 1.666666666667, 1.66667},
		{"margin_lh", 0.416666666667, 0.41667},

		// Very small values
		{"tiny_value", 0.000001, 0.0},
		{"small_value", 0.00001, 0.00001},
		{"small_round_up", 0.000005, 0.00001},
		{"small_round_down", 0.000004, 0.0},

		// Large values
		{"large_value", 1000.123456, 1000.12346},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundDecimal(tt.input)
			// Use epsilon comparison for floating point
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("RoundDecimal(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDecimalPrecisionConstant(t *testing.T) {
	// Verify the constant is set to 5 for higher precision output
	if DecimalPrecision != 5 {
		t.Errorf("DecimalPrecision = %d, want 5", DecimalPrecision)
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

func TestUnitConversionConstants(t *testing.T) {
	// Verify constants have expected values
	tests := []struct {
		name     string
		value    float64
		expected float64
	}{
		{"DefaultLineHeightLh", DefaultLineHeightLh, 1.0},
		{"AdjustedLineHeightLh", AdjustedLineHeightLh, 100.0 / 99.0},
		{"LineHeightRatio", LineHeightRatio, 1.2},
		{"EmToPercentHorizontal", EmToPercentHorizontal, 3.125},
		{"EmToPercentTextIndent", EmToPercentTextIndent, 3.125},
		{"FontSizeCompressionFactor", FontSizeCompressionFactor, 160.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.expected {
				t.Errorf("%s = %v, want %v", tt.name, tt.value, tt.expected)
			}
		})
	}
}

// TestUnitConversions tests that common CSS-to-KFX unit conversions work correctly
func TestUnitConversions(t *testing.T) {
	t.Run("em_to_lh_vertical", func(t *testing.T) {
		// 0.3em CSS → 0.25lh KFX (0.3 / 1.2 = 0.25)
		emValue := 0.3
		lhValue := emValue / LineHeightRatio
		expected := 0.25
		if math.Abs(lhValue-expected) > 1e-9 {
			t.Errorf("em to lh: %v / %v = %v, want %v", emValue, LineHeightRatio, lhValue, expected)
		}
	})

	t.Run("em_to_percent_horizontal", func(t *testing.T) {
		// 1em CSS → 3.125% KFX
		emValue := 1.0
		percentValue := emValue * EmToPercentHorizontal
		expected := 3.125
		if math.Abs(percentValue-expected) > 1e-9 {
			t.Errorf("em to %%: %v * %v = %v, want %v", emValue, EmToPercentHorizontal, percentValue, expected)
		}
	})

	t.Run("percent_to_rem_font_size", func(t *testing.T) {
		// Test KP3's font-size compression formula
		// Values > 100% are compressed: rem = 1 + (percent - 100) / 160
		// Values <= 100% use direct conversion: rem = percent / 100
		tests := []struct {
			percent  float64
			expected float64
		}{
			{140.0, 1.25},  // compressed: 1 + (140-100)/160 = 1.25
			{120.0, 1.125}, // compressed: 1 + (120-100)/160 = 1.125
			{100.0, 1.0},   // direct: 100/100 = 1.0
			{80.0, 0.8},    // direct: 80/100 = 0.8
			{70.0, 0.7},    // direct: 70/100 = 0.7
		}
		for _, tt := range tests {
			remValue := PercentToRem(tt.percent)
			if math.Abs(remValue-tt.expected) > 1e-9 {
				t.Errorf("PercentToRem(%v) = %v, want %v", tt.percent, remValue, tt.expected)
			}
		}
	})
}
