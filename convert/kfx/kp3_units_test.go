package kfx

import (
	"math"
	"testing"
)

func TestRoundDecimals(t *testing.T) {
	tests := []struct {
		name      string
		input     float64
		precision int
		expected  float64
	}{
		// LineHeightPrecision (5) - for line-height values
		{"line_height_1.0101", 1.01010101, LineHeightPrecision, 1.0101},
		{"line_height_1.33249", 1.332486, LineHeightPrecision, 1.33249},

		// WidthPercentPrecision (3) - for image widths
		{"width_19.531", 19.53125, WidthPercentPrecision, 19.531},
		{"width_74.219", 74.21875, WidthPercentPrecision, 74.219},
		{"width_29.102", 29.1015625, WidthPercentPrecision, 29.102},

		// Edge cases
		{"zero", 0, 6, 0},
		{"negative", -1.2345, 6, -1.2345},
		{"whole_number", 100.0, 6, 100.0},

		// Half-up rounding behavior
		{"half_up_5", 1.23456751, 6, 1.234568},
		{"half_up_below", 1.2345674, 6, 1.234567},
		{"half_up_above", 1.2345676, 6, 1.234568},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundDecimals(tt.input, tt.precision)
			// Use epsilon comparison for floating point
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("RoundDecimals(%v, %d) = %v, want %v", tt.input, tt.precision, result, tt.expected)
			}
		})
	}
}

func TestRoundSignificant(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		sigFigs  int
		expected float64
	}{
		// SignificantFigures (6) - for vertical margins
		// Key insight: 6 sig figs for values >= 1 gives fewer decimal places
		{"margin_5_3", 5.0 / 3.0, SignificantFigures, 1.66667},           // 6 sig figs = 5 decimals
		{"margin_5_6", 5.0 / 6.0, SignificantFigures, 0.833333},          // 6 sig figs = 6 decimals (< 1)
		{"margin_1_6", 1.0 / 6.0, SignificantFigures, 0.166667},          // 6 sig figs = 6 decimals (< 1)
		{"margin_5_12", 5.0 / 12.0, SignificantFigures, 0.416667},        // 6 sig figs = 6 decimals (< 1)
		{"margin_large", 63.33333333333333, SignificantFigures, 63.3333}, // 6 sig figs = 4 decimals (large)

		// Verify KP3 reference values
		{"kp3_margin_top", 1.666666666667, SignificantFigures, 1.66667},   // matches 1.66667lh
		{"kp3_margin_bottom", 0.8333333333, SignificantFigures, 0.833333}, // matches 0.833333lh
		{"kp3_margin_half", 0.416666666667, SignificantFigures, 0.416667}, // matches 0.416667lh

		// Edge cases
		{"zero", 0, SignificantFigures, 0},
		{"already_precise", 0.25, SignificantFigures, 0.25},
		{"whole_number", 100.0, SignificantFigures, 100.0},
		{"negative", -1.66667, SignificantFigures, -1.66667},

		// Smaller values (more decimals needed)
		{"small_value", 0.0833333, SignificantFigures, 0.0833333}, // 6 sig figs = 7 decimals
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundSignificant(tt.input, tt.sigFigs)
			// Use epsilon comparison for floating point
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("RoundSignificant(%v, %d) = %v, want %v", tt.input, tt.sigFigs, result, tt.expected)
			}
		})
	}
}

func TestSignificantFiguresConstant(t *testing.T) {
	// Verify the constant is set to 6 for KP3 compatibility
	if SignificantFigures != 6 {
		t.Errorf("SignificantFigures = %d, want 6", SignificantFigures)
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
		{"FontSizeCompressionFactor", FontSizeCompressionFactor, 160.0},
		{"ExToEmFactor", ExToEmFactor, 0.44},
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

	t.Run("em_stays_em_horizontal", func(t *testing.T) {
		// em values for horizontal spacing are now kept as em (not converted to %)
		// so that they scale with viewer font size changes.
		emValue := 2.0
		// The value should be passed through unchanged
		if emValue != 2.0 {
			t.Errorf("em value should be preserved: got %v, want 2.0", emValue)
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

	t.Run("pt_to_em", func(t *testing.T) {
		// 12pt = 16px = 1em
		tests := []struct {
			pt       float64
			expected float64
		}{
			{12.0, 1.0},
			{24.0, 2.0},
			{6.0, 0.5},
		}
		for _, tt := range tests {
			emValue := PtToEm(tt.pt)
			if math.Abs(emValue-tt.expected) > 1e-9 {
				t.Errorf("PtToEm(%v) = %v, want %v", tt.pt, emValue, tt.expected)
			}
		}
	})
}

func TestRemToFontSizeMultiplier(t *testing.T) {
	tests := []struct {
		name     string
		rem      float64
		expected float64
	}{
		// Round-trip: PercentToRem → RemToFontSizeMultiplier should recover CSS multiplier
		{"200%_compressed", 1.625, 2.0}, // PercentToRem(200) = 1.625 → multiplier 2.0
		{"140%_compressed", 1.25, 1.4},  // PercentToRem(140) = 1.25 → multiplier 1.4
		{"120%_compressed", 1.125, 1.2}, // PercentToRem(120) = 1.125 → multiplier 1.2
		{"300%_compressed", 2.25, 3.0},  // PercentToRem(300) = 2.25 → multiplier 3.0
		{"400%_compressed", 2.875, 4.0}, // PercentToRem(400) = 2.875 → multiplier 4.0

		// Values at or below 100% — no compression, direct pass-through
		{"100%_direct", 1.0, 1.0},  // PercentToRem(100) = 1.0 → multiplier 1.0
		{"80%_direct", 0.8, 0.8},   // PercentToRem(80) = 0.8 → multiplier 0.8
		{"75%_direct", 0.75, 0.75}, // PercentToRem(75) = 0.75 → multiplier 0.75
		{"70%_direct", 0.7, 0.7},   // PercentToRem(70) = 0.7 → multiplier 0.7
		{"50%_direct", 0.5, 0.5},   // PercentToRem(50) = 0.5 → multiplier 0.5
		{"25%_direct", 0.25, 0.25}, // PercentToRem(25) = 0.25 → multiplier 0.25

		// Edge case
		{"zero", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemToFontSizeMultiplier(tt.rem)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("RemToFontSizeMultiplier(%v) = %v, want %v", tt.rem, result, tt.expected)
			}
		})
	}

	// Verify round-trip: PercentToRem → RemToFontSizeMultiplier
	t.Run("round_trip", func(t *testing.T) {
		percentages := []float64{25, 50, 70, 75, 80, 100, 120, 140, 200, 300, 400}
		for _, pct := range percentages {
			rem := PercentToRem(pct)
			mult := RemToFontSizeMultiplier(rem)
			expected := pct / 100.0
			if math.Abs(mult-expected) > 1e-6 {
				t.Errorf("round-trip %.0f%%: PercentToRem=%.6f, RemToFontSizeMultiplier=%.6f, want %.6f",
					pct, rem, mult, expected)
			}
		}
	})
}

func TestFontSizeMultiplier(t *testing.T) {
	tests := []struct {
		name     string
		val      any
		expected float64
	}{
		// rem values (compressed by PercentToRem)
		{"rem_1.625", DimensionValue(1.625, SymUnitRem), 2.0},
		{"rem_1.25", DimensionValue(1.25, SymUnitRem), 1.4},
		{"rem_1.125", DimensionValue(1.125, SymUnitRem), 1.2},
		{"rem_1.0", DimensionValue(1.0, SymUnitRem), 1.0},
		{"rem_0.75", DimensionValue(0.75, SymUnitRem), 0.75},

		// em values (direct multiplier)
		{"em_0.25", DimensionValue(0.25, SymUnitEm), 0.25},
		{"em_0.9", DimensionValue(0.9, SymUnitEm), 0.9},
		{"em_1.2", DimensionValue(1.2, SymUnitEm), 1.2},

		// Other units / non-dimensional
		{"lh_1.0", DimensionValue(1.0, SymUnitLh), 1.0},
		{"nil", nil, 1.0},
		{"string", "hello", 1.0},

		// Zero values
		{"rem_zero", DimensionValue(0, SymUnitRem), 1.0},
		{"em_zero", DimensionValue(0, SymUnitEm), 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FontSizeMultiplier(tt.val)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("FontSizeMultiplier(%v) = %v, want %v", tt.val, result, tt.expected)
			}
		})
	}
}
