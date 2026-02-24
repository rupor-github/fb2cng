package kfx

import (
	"reflect"
	"testing"
)

func TestMergePropertyCumulativeMargin(t *testing.T) {
	// Test margin-left merge behavior with YJCumulativeInSameContainerRuleMerger.
	// Container identity tracking is now handled in StyleContext.PushBlock and
	// StyleContext.resolveProperties. The merge function itself uses override semantics.
	sr := NewStyleRegistry()
	dst := map[KFXSymbol]any{
		SymMarginLeft: DimensionValue(2, SymUnitEm),
	}

	// Non-zero incoming should override
	sr.mergeProperty(dst, SymMarginLeft, DimensionValue(3, SymUnitEm))

	expected := DimensionValue(3, SymUnitEm)
	if got := dst[SymMarginLeft]; !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected overridden margin-left %v, got %v", expected, got)
	}

	// Zero incoming also overrides (explicit margin: 0 is valid CSS)
	sr.mergeProperty(dst, SymMarginLeft, DimensionValue(0, SymUnitEm))
	expectedZero := DimensionValue(0, SymUnitEm)
	if got := dst[SymMarginLeft]; !reflect.DeepEqual(got, expectedZero) {
		t.Fatalf("expected zero margin-left %v after zero merge, got %v", expectedZero, got)
	}
}

func TestMergePropertyOverrideMaximum(t *testing.T) {
	sr := NewStyleRegistry()
	dst := map[KFXSymbol]any{
		SymMarginTop: DimensionValue(2, SymUnitLh),
	}

	sr.mergeProperty(dst, SymMarginTop, DimensionValue(1, SymUnitLh))
	if got := dst[SymMarginTop]; !reflect.DeepEqual(got, DimensionValue(2, SymUnitLh)) {
		t.Fatalf("expected keep existing larger margin-top, got %v", got)
	}

	sr.mergeProperty(dst, SymMarginTop, DimensionValue(3, SymUnitLh))
	if got := dst[SymMarginTop]; !reflect.DeepEqual(got, DimensionValue(3, SymUnitLh)) {
		t.Fatalf("expected override with larger margin-top, got %v", got)
	}
}

func TestMergeLayoutHintsDedup(t *testing.T) {
	sr := NewStyleRegistry()
	dst := map[KFXSymbol]any{
		SymLayoutHints: []any{SymbolValue(SymTreatAsTitle)},
	}

	incoming := []any{SymbolValue(SymTreatAsTitle), SymbolValue(SymNavContainer)}
	sr.mergeProperty(dst, SymLayoutHints, incoming)

	hints, ok := dst[SymLayoutHints].([]any)
	if !ok {
		t.Fatalf("layout_hints type %T", dst[SymLayoutHints])
	}
	if len(hints) != 2 {
		t.Fatalf("expected 2 layout hints, got %d", len(hints))
	}

	if !containsSymbol(hints, SymTreatAsTitle) || !containsSymbol(hints, SymNavContainer) {
		t.Fatalf("merged hints missing expected symbols: %v", hints)
	}
}

func TestMergeHorizontalPosition(t *testing.T) {
	sr := NewStyleRegistry()
	dst := map[KFXSymbol]any{
		SymFloatClear: SymLeft,
	}

	sr.mergeProperty(dst, SymFloatClear, SymRight)

	if got := dst[SymFloatClear]; got != SymBoth {
		t.Fatalf("expected horizontal position merge to both, got %v", got)
	}
}

func TestMergeRelativePercent(t *testing.T) {
	sr := NewStyleRegistry()
	dst := map[KFXSymbol]any{
		SymFontSize: DimensionValue(1, SymUnitEm),
	}

	sr.mergeProperty(dst, SymFontSize, DimensionValue(140, SymUnitPercent))

	expected := DimensionValue(1.4, SymUnitEm)
	if got := dst[SymFontSize]; !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected relative font-size %v, got %v", expected, got)
	}
}

func containsSymbol(list []any, expected KFXSymbol) bool {
	for _, v := range list {
		if sym, ok := symbolIDFromAny(v); ok && sym == expected {
			return true
		}
	}
	return false
}

// TestSelectMergeRuleFromStyleList verifies that selectMergeRule correctly
// selects rules from stylelist_data.go based on context.
func TestSelectMergeRuleFromStyleList(t *testing.T) {
	tests := []struct {
		name         string
		sym          KFXSymbol
		existing     any
		incoming     any
		ctx          mergeContext
		expectedRule string
	}{
		{
			name:         "margin_top inline uses override-maximum",
			sym:          SymMarginTop,
			existing:     DimensionValue(1, SymUnitLh),
			incoming:     DimensionValue(2, SymUnitLh),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "override-maximum",
		},
		{
			name:         "margin_top container uses cumulative",
			sym:          SymMarginTop,
			existing:     DimensionValue(1, SymUnitLh),
			incoming:     DimensionValue(2, SymUnitLh),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsContainer: true},
			expectedRule: "cumulative",
		},
		{
			name:         "margin_bottom wrapper uses override-maximum",
			sym:          SymMarginBottom,
			existing:     DimensionValue(1, SymUnitLh),
			incoming:     DimensionValue(2, SymUnitLh),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsWrapper: true},
			expectedRule: "override-maximum",
		},
		{
			// Per KP3's YJCumulativeInSameContainerRuleMerger: styles from different containers
			// don't accumulate. We implement this as override-non-zero (0 keeps existing,
			// non-zero overrides). Rule name reflects the KP3 source.
			name:         "margin_left inline uses cumulative-same-container",
			sym:          SymMarginLeft,
			existing:     DimensionValue(1, SymUnitEm),
			incoming:     DimensionValue(2, SymUnitEm),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "cumulative-same-container",
		},
		{
			name:         "font_size em incoming uses relative",
			sym:          SymFontSize,
			existing:     DimensionValue(1, SymUnitEm),
			incoming:     DimensionValue(1.2, SymUnitEm),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "relative",
		},
		{
			name:         "font_size percent incoming uses relative",
			sym:          SymFontSize,
			existing:     DimensionValue(1, SymUnitEm),
			incoming:     DimensionValue(120, SymUnitPercent),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "relative",
		},
		{
			name:         "baseline_style uses baseline-style",
			sym:          SymBaselineStyle,
			existing:     SymNormal,
			incoming:     SymSuperscript,
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "baseline-style",
		},
		{
			name:         "float_clear uses horizontal-position",
			sym:          SymFloatClear,
			existing:     SymLeft,
			incoming:     SymRight,
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "horizontal-position",
		},
		{
			name:         "padding_top inline uses cumulative",
			sym:          SymPaddingTop,
			existing:     DimensionValue(1, SymUnitLh),
			incoming:     DimensionValue(2, SymUnitLh),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "cumulative",
		},
		{
			name:         "padding_bottom wrapper uses cumulative",
			sym:          SymPaddingBottom,
			existing:     DimensionValue(1, SymUnitLh),
			incoming:     DimensionValue(2, SymUnitLh),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsWrapper: true},
			expectedRule: "cumulative",
		},
		{
			name:         "baseline_shift percent uses cumulative",
			sym:          SymBaselineShift,
			existing:     DimensionValue(10, SymUnitPercent),
			incoming:     DimensionValue(20, SymUnitPercent),
			ctx:          mergeContext{allowWritingModeConvert: true, sourceIsInline: true},
			expectedRule: "cumulative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := selectMergeRule(tt.sym, tt.existing, tt.incoming, tt.ctx)
			if rule.name != tt.expectedRule {
				t.Errorf("selectMergeRule(%s) = %q, want %q", tt.sym.Name(), rule.name, tt.expectedRule)
			}
		})
	}
}
