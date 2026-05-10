package kfx

import (
	"reflect"
	"strings"
	"testing"
)

func TestFormatStylePropsAsCSS(t *testing.T) {
	props := StructValue{
		SymStyleName:     "chapter",
		SymFontSize:      DimensionValue(1.5, SymUnitEm),
		SymFontWeight:    SymbolValue(SymBold),
		SymTextAlignment: SymbolValue(SymCenter),
		SymTextColor:     int64(0xFF112233),
	}

	want := "font-size: 1.5em; font-weight: bold; text-align: center; color: #FF112233"
	if got := FormatStylePropsAsCSS(props); got != want {
		t.Fatalf("FormatStylePropsAsCSS() = %q, want %q", got, want)
	}
}

func TestFormatStylePropsAsCSSMultiLine(t *testing.T) {
	props := map[string]any{
		"style_name":   SymbolByName("chapter"),
		"parent_style": ReadSymbolValue("base"),
		"$16":          map[string]any{"$307": "2.08333d-1", "$306": "$308"},
		"$19":          int64(0xFF0000FF),
	}

	want := strings.Join([]string{
		"  CSS:",
		"    .chapter {",
		"      inherits: .base",
		"      font-size: 0.208333em;",
		"      color: #FF0000FF;",
		"    }",
		"",
	}, "\n")
	if got := FormatStylePropsAsCSSMultiLine(props); got != want {
		t.Fatalf("FormatStylePropsAsCSSMultiLine() = %q, want %q", got, want)
	}
}

func TestFormatStylePropsEmptyInputs(t *testing.T) {
	if got := FormatStylePropsAsCSS(nil); got != "" {
		t.Fatalf("FormatStylePropsAsCSS(nil) = %q, want empty", got)
	}
	if got := FormatStylePropsAsCSS(map[string]any{"style_name": "only-skipped"}); got != "" {
		t.Fatalf("FormatStylePropsAsCSS(skipped-only) = %q, want empty", got)
	}
	if got := FormatStylePropsAsCSSMultiLine(nil); got != "" {
		t.Fatalf("FormatStylePropsAsCSSMultiLine(nil) = %q, want empty", got)
	}
}

func TestNormalizeStyleMap(t *testing.T) {
	got := NormalizeStyleMap(map[string]any{
		"$19":        1,
		"custom_key": 2,
		"bad$":       3,
	})
	want := map[string]any{
		"text_color": 1,
		"custom_key": 2,
		"bad$":       3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeStyleMap() = %#v, want %#v", got, want)
	}
}

func TestFormatDimensionAsCSS(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "struct value", value: DimensionValue(2, SymUnitPt), want: "2pt"},
		{name: "symbol map", value: map[KFXSymbol]any{SymValue: int64(75), SymUnit: ReadSymbolValue("percent")}, want: "75%"},
		{name: "string map", value: map[string]any{"$307": "2.08333d-1", "$306": "ratio"}, want: "0.208333"},
		{name: "fallback", value: "raw", want: "\"raw\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatDimensionAsCSS(tt.value); got != tt.want {
				t.Fatalf("FormatDimensionAsCSS() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsDimensionValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "struct", value: DimensionValue(1, SymUnitEm), want: true},
		{name: "string map names", value: map[string]any{"value": 1, "unit": "px"}, want: true},
		{name: "string map symbols", value: map[string]any{"$307": 1, "$306": "px"}, want: true},
		{name: "missing unit", value: map[string]any{"value": 1}, want: false},
		{name: "not map", value: 1, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDimensionValue(tt.value); got != tt.want {
				t.Fatalf("IsDimensionValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExportedCSSFormatHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "float trims zeros", got: FormatCSSFloat(1.25), want: "1.25"},
		{name: "ion decimal", got: FormatIonDecimal("2.08333d-1"), want: "0.208333"},
		{name: "semibold name override", got: FormatCSSSymbolValue(ReadSymbolValue("semibold")), want: "600"},
		{name: "symbol string", got: FormatCSSSymbolValue(ReadSymbolValue("$320")), want: "center"},
		{name: "plain string", got: FormatCSSSymbolValue("custom"), want: "custom"},
		{name: "color", got: FormatCSSColorValue(uint32(0x80112233)), want: "#80112233"},
		{name: "unit symbol string", got: KFXUnitToCSS("$314"), want: "%"},
		{name: "ratio unit", got: KFXUnitToCSS("ratio"), want: ""},
		{name: "bold symbol", got: KFXSymbolToCSS(SymBold), want: "bold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

func TestFormatCSSCollectionHelpers(t *testing.T) {
	if got, want := FormatCSSListValue([]any{SymbolValue(SymBold), "custom"}), "[bold, custom]"; got != want {
		t.Fatalf("FormatCSSListValue() = %q, want %q", got, want)
	}

	mapValue := map[KFXSymbol]any{
		SymFontSize:  DimensionValue(2, SymUnitRem),
		SymTextColor: int64(0xFF010203),
	}
	if got, want := FormatCSSMapValue(mapValue), "{font_size: 2rem; text_color: #FF010203}"; got != want {
		t.Fatalf("FormatCSSMapValue() = %q, want %q", got, want)
	}

	stringMapValue := map[string]any{
		"$19":         int64(0xFF112233),
		"font_weight": SymbolValue(SymBold),
	}
	if got, want := FormatCSSMapStringValue(stringMapValue), "{text_color: #FF112233; font_weight: bold}"; got != want {
		t.Fatalf("FormatCSSMapStringValue() = %q, want %q", got, want)
	}
}

func TestIsColorProperty(t *testing.T) {
	if !IsColorProperty("text_color") {
		t.Fatal("IsColorProperty(text_color) = false, want true")
	}
	if IsColorProperty("font_size") {
		t.Fatal("IsColorProperty(font_size) = true, want false")
	}
}
