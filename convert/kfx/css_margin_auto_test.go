package kfx

import (
	"reflect"
	"testing"
)

func TestResolveMarginAutoBlockAxis(t *testing.T) {
	tests := []struct {
		name    string
		input   map[KFXSymbol]any
		wantTop any
		wantBot any
	}{
		{
			name: "margin-top auto → 0em",
			input: map[KFXSymbol]any{
				SymMarginTop: SymAuto,
			},
			wantTop: DimensionValue(0, SymUnitEm),
		},
		{
			name: "margin-bottom auto → 0em",
			input: map[KFXSymbol]any{
				SymMarginBottom: SymAuto,
			},
			wantBot: DimensionValue(0, SymUnitEm),
		},
		{
			name: "margin-top auto as SymbolValue → 0em",
			input: map[KFXSymbol]any{
				SymMarginTop: SymbolValue(SymAuto),
			},
			wantTop: DimensionValue(0, SymUnitEm),
		},
		{
			name: "both block-axis auto → 0em",
			input: map[KFXSymbol]any{
				SymMarginTop:    SymAuto,
				SymMarginBottom: SymbolValue(SymAuto),
			},
			wantTop: DimensionValue(0, SymUnitEm),
			wantBot: DimensionValue(0, SymUnitEm),
		},
		{
			name: "non-auto margin-top preserved",
			input: map[KFXSymbol]any{
				SymMarginTop: DimensionValue(1, SymUnitEm),
			},
			wantTop: DimensionValue(1, SymUnitEm),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveMarginAuto(tt.input)
			if tt.wantTop != nil {
				got := tt.input[SymMarginTop]
				if !reflect.DeepEqual(got, tt.wantTop) {
					t.Errorf("margin-top = %v, want %v", got, tt.wantTop)
				}
			}
			if tt.wantBot != nil {
				got := tt.input[SymMarginBottom]
				if !reflect.DeepEqual(got, tt.wantBot) {
					t.Errorf("margin-bottom = %v, want %v", got, tt.wantBot)
				}
			}
		})
	}
}

func TestResolveMarginAutoPairedInlineAxis(t *testing.T) {
	tests := []struct {
		name         string
		input        map[KFXSymbol]any
		wantBoxAlign any
		wantLeft     bool // if true, SymMarginLeft should still exist
		wantRight    bool // if true, SymMarginRight should still exist
	}{
		{
			name: "both auto → box_align center",
			input: map[KFXSymbol]any{
				SymMarginLeft:  SymAuto,
				SymMarginRight: SymAuto,
			},
			wantBoxAlign: SymbolValue(SymCenter),
			wantLeft:     false,
			wantRight:    false,
		},
		{
			name: "both auto as SymbolValue → box_align center",
			input: map[KFXSymbol]any{
				SymMarginLeft:  SymbolValue(SymAuto),
				SymMarginRight: SymbolValue(SymAuto),
			},
			wantBoxAlign: SymbolValue(SymCenter),
			wantLeft:     false,
			wantRight:    false,
		},
		{
			name: "mixed types both auto → box_align center",
			input: map[KFXSymbol]any{
				SymMarginLeft:  SymAuto,
				SymMarginRight: SymbolValue(SymAuto),
			},
			wantBoxAlign: SymbolValue(SymCenter),
			wantLeft:     false,
			wantRight:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveMarginAuto(tt.input)

			got := tt.input[SymBoxAlign]
			if !reflect.DeepEqual(got, tt.wantBoxAlign) {
				t.Errorf("box_align = %v, want %v", got, tt.wantBoxAlign)
			}
			if _, ok := tt.input[SymMarginLeft]; ok != tt.wantLeft {
				t.Errorf("margin-left present = %v, want %v", ok, tt.wantLeft)
			}
			if _, ok := tt.input[SymMarginRight]; ok != tt.wantRight {
				t.Errorf("margin-right present = %v, want %v", ok, tt.wantRight)
			}
		})
	}
}

func TestResolveMarginAutoSingleSideInlineAxis(t *testing.T) {
	tests := []struct {
		name         string
		input        map[KFXSymbol]any
		wantBoxAlign any
		wantLeft     bool
		wantRight    bool
	}{
		{
			name: "only left auto → box_align right",
			input: map[KFXSymbol]any{
				SymMarginLeft:  SymAuto,
				SymMarginRight: DimensionValue(0, SymUnitEm),
			},
			wantBoxAlign: SymbolValue(SymRight),
			wantLeft:     false,
			wantRight:    true,
		},
		{
			name: "only right auto → box_align left",
			input: map[KFXSymbol]any{
				SymMarginLeft:  DimensionValue(0, SymUnitEm),
				SymMarginRight: SymAuto,
			},
			wantBoxAlign: SymbolValue(SymLeft),
			wantLeft:     true,
			wantRight:    false,
		},
		{
			name: "only left auto, no right → box_align right",
			input: map[KFXSymbol]any{
				SymMarginLeft: SymAuto,
			},
			wantBoxAlign: SymbolValue(SymRight),
			wantLeft:     false,
			wantRight:    false,
		},
		{
			name: "only right auto, no left → box_align left",
			input: map[KFXSymbol]any{
				SymMarginRight: SymbolValue(SymAuto),
			},
			wantBoxAlign: SymbolValue(SymLeft),
			wantLeft:     false,
			wantRight:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveMarginAuto(tt.input)

			got := tt.input[SymBoxAlign]
			if !reflect.DeepEqual(got, tt.wantBoxAlign) {
				t.Errorf("box_align = %v, want %v", got, tt.wantBoxAlign)
			}
			if _, ok := tt.input[SymMarginLeft]; ok != tt.wantLeft {
				t.Errorf("margin-left present = %v, want %v", ok, tt.wantLeft)
			}
			if _, ok := tt.input[SymMarginRight]; ok != tt.wantRight {
				t.Errorf("margin-right present = %v, want %v", ok, tt.wantRight)
			}
		})
	}
}

func TestResolveMarginAutoPreservesExplicitBoxAlign(t *testing.T) {
	tests := []struct {
		name         string
		input        map[KFXSymbol]any
		wantBoxAlign any
		wantLeft     bool
		wantRight    bool
	}{
		{
			name: "explicit box_align not overridden by paired auto",
			input: map[KFXSymbol]any{
				SymMarginLeft:  SymAuto,
				SymMarginRight: SymAuto,
				SymBoxAlign:    SymbolValue(SymLeft),
			},
			wantBoxAlign: SymbolValue(SymLeft),
			wantLeft:     false, // auto values consumed
			wantRight:    false,
		},
		{
			name: "explicit box_align not overridden by single-side auto",
			input: map[KFXSymbol]any{
				SymMarginLeft: SymAuto,
				SymBoxAlign:   SymbolValue(SymCenter),
			},
			wantBoxAlign: SymbolValue(SymCenter),
			wantLeft:     false, // auto value consumed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolveMarginAuto(tt.input)

			got := tt.input[SymBoxAlign]
			if !reflect.DeepEqual(got, tt.wantBoxAlign) {
				t.Errorf("box_align = %v, want %v", got, tt.wantBoxAlign)
			}
			if _, ok := tt.input[SymMarginLeft]; ok != tt.wantLeft {
				t.Errorf("margin-left present = %v, want %v", ok, tt.wantLeft)
			}
			if _, ok := tt.input[SymMarginRight]; ok != tt.wantRight {
				t.Errorf("margin-right present = %v, want %v", ok, tt.wantRight)
			}
		})
	}
}

func TestResolveMarginAutoNoAutoValues(t *testing.T) {
	// When no margin-auto values are present, nothing should change.
	props := map[KFXSymbol]any{
		SymMarginTop:    DimensionValue(1, SymUnitEm),
		SymMarginBottom: DimensionValue(2, SymUnitEm),
		SymMarginLeft:   DimensionValue(3, SymUnitEm),
		SymMarginRight:  DimensionValue(4, SymUnitEm),
	}
	// Snapshot before.
	want := map[KFXSymbol]any{
		SymMarginTop:    DimensionValue(1, SymUnitEm),
		SymMarginBottom: DimensionValue(2, SymUnitEm),
		SymMarginLeft:   DimensionValue(3, SymUnitEm),
		SymMarginRight:  DimensionValue(4, SymUnitEm),
	}

	resolveMarginAuto(props)

	if !reflect.DeepEqual(props, want) {
		t.Errorf("props changed unexpectedly:\ngot  %v\nwant %v", props, want)
	}
}

func TestResolveMarginAutoEmptyMap(t *testing.T) {
	props := map[KFXSymbol]any{}
	resolveMarginAuto(props) // should not panic
	if len(props) != 0 {
		t.Errorf("expected empty map, got %v", props)
	}
}

func TestResolveMarginAutoCombinedBlockAndInline(t *testing.T) {
	// margin: auto (all four sides auto) — the table { margin: 1em auto } case
	// after shorthand expansion would have margin-top/bottom as numeric and
	// margin-left/right as auto.
	props := map[KFXSymbol]any{
		SymMarginTop:    DimensionValue(0.833333, SymUnitLh),
		SymMarginBottom: DimensionValue(0.833333, SymUnitLh),
		SymMarginLeft:   SymAuto,
		SymMarginRight:  SymAuto,
	}

	resolveMarginAuto(props)

	// Block-axis should be unchanged (not auto).
	if got := props[SymMarginTop]; !reflect.DeepEqual(got, DimensionValue(0.833333, SymUnitLh)) {
		t.Errorf("margin-top = %v, want preserved", got)
	}
	if got := props[SymMarginBottom]; !reflect.DeepEqual(got, DimensionValue(0.833333, SymUnitLh)) {
		t.Errorf("margin-bottom = %v, want preserved", got)
	}

	// Inline-axis should become box_align: center.
	if got := props[SymBoxAlign]; !reflect.DeepEqual(got, SymbolValue(SymCenter)) {
		t.Errorf("box_align = %v, want center", got)
	}
	if _, ok := props[SymMarginLeft]; ok {
		t.Error("margin-left should be deleted")
	}
	if _, ok := props[SymMarginRight]; ok {
		t.Error("margin-right should be deleted")
	}
}

func TestResolveMarginAutoAllFourAuto(t *testing.T) {
	// All four margins auto.
	props := map[KFXSymbol]any{
		SymMarginTop:    SymAuto,
		SymMarginBottom: SymbolValue(SymAuto),
		SymMarginLeft:   SymAuto,
		SymMarginRight:  SymbolValue(SymAuto),
	}

	resolveMarginAuto(props)

	// Block-axis → 0em.
	if got := props[SymMarginTop]; !reflect.DeepEqual(got, DimensionValue(0, SymUnitEm)) {
		t.Errorf("margin-top = %v, want 0em", got)
	}
	if got := props[SymMarginBottom]; !reflect.DeepEqual(got, DimensionValue(0, SymUnitEm)) {
		t.Errorf("margin-bottom = %v, want 0em", got)
	}

	// Inline-axis → box_align: center.
	if got := props[SymBoxAlign]; !reflect.DeepEqual(got, SymbolValue(SymCenter)) {
		t.Errorf("box_align = %v, want center", got)
	}
	if _, ok := props[SymMarginLeft]; ok {
		t.Error("margin-left should be deleted")
	}
	if _, ok := props[SymMarginRight]; ok {
		t.Error("margin-right should be deleted")
	}
}
